package vault_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// cascadeFixture seeds the minimum surface for testing cascade:
//   - alice (admin) owns secrets
//   - serviceA  → 1 shared cred + 1 personal password (both children of A)
//   - serviceB  → 1 shared cred (control — should never be touched)
//   - hostX     → 1 shared password (control — should never be touched)
//   - avulso    → 1 personal note (control — never cascaded, no parent)
//
// Returns the env plus the seeded IDs needed for assertions.
type cascadeFixture struct {
	d           *database.DB
	repo        *vault.SecretRepo
	alice       vault.ActorContext
	serviceAID  int64
	serviceBID  int64
	hostXID     int64
	aSharedID   int64 // child of serviceA, shared
	aPersonalID int64 // child of serviceA, personal (owned by alice)
	bSharedID   int64 // child of serviceB, shared
	xSharedID   int64 // child of hostX,    shared
	avulsoID    int64 // avulso, personal owned by alice
}

func newCascadeFixture(t *testing.T) *cascadeFixture {
	t.Helper()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	res, err := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
		"alice", "x", "admin")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	aliceID, _ := res.LastInsertId()
	alice := vault.ActorContext{UserID: aliceID, Role: "admin"}

	mkService := func(name string) int64 {
		r, err := d.SQL.Exec(`INSERT INTO services (nickname) VALUES (?)`, name)
		if err != nil {
			t.Fatalf("seed service %s: %v", name, err)
		}
		id, _ := r.LastInsertId()
		return id
	}
	serviceAID := mkService("svc-a")
	serviceBID := mkService("svc-b")

	hostRes, err := d.SQL.Exec(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?)`,
		"host-x", "host-x", "10.0.0.99", "deploy",
	)
	if err != nil {
		t.Fatalf("seed host: %v", err)
	}
	hostXID, _ := hostRes.LastInsertId()

	repo := vault.NewSecretRepo(d)
	ctx := context.Background()

	mk := func(scope models.SecretScope, vis models.SecretVisibility, parent *int64, typ models.SecretType, name, plain string) int64 {
		id, err := repo.Create(ctx, alice, &models.Secret{
			Type:        typ,
			Scope:       scope,
			Visibility:  vis,
			ParentID:    parent,
			OwnerUserID: aliceID,
			Name:        name,
			KeyVersion:  1,
			CreatedBy:   aliceID,
		}, plain)
		if err != nil {
			t.Fatalf("seed secret %s: %v", name, err)
		}
		return id
	}
	aShared := mk(models.SecretScopeService, models.SecretVisibilityShared, &serviceAID, models.SecretTypeCred, "a-shared", "p1")
	aPersonal := mk(models.SecretScopeService, models.SecretVisibilityPersonal, &serviceAID, models.SecretTypePassword, "a-personal", "p2")
	bShared := mk(models.SecretScopeService, models.SecretVisibilityShared, &serviceBID, models.SecretTypeCred, "b-shared", "p3")
	xShared := mk(models.SecretScopeHost, models.SecretVisibilityShared, &hostXID, models.SecretTypePassword, "x-shared", "p4")
	avulso := mk(models.SecretScopeAvulso, models.SecretVisibilityPersonal, nil, models.SecretTypePassword, "avulso-note", "p5")

	return &cascadeFixture{
		d: d, repo: repo, alice: alice,
		serviceAID: serviceAID, serviceBID: serviceBID, hostXID: hostXID,
		aSharedID: aShared, aPersonalID: aPersonal, bSharedID: bShared,
		xSharedID: xShared, avulsoID: avulso,
	}
}

func (f *cascadeFixture) deletedAt(t *testing.T, id int64) *string {
	t.Helper()
	var ts *string
	err := f.d.SQL.QueryRow(`SELECT deleted_at FROM secrets WHERE id = ?`, id).Scan(&ts)
	if err != nil {
		t.Fatalf("read deleted_at(%d): %v", id, err)
	}
	return ts
}

// assertDeleted fails if id's deleted_at is NULL.
func (f *cascadeFixture) assertDeleted(t *testing.T, id int64, msg string) {
	t.Helper()
	if ts := f.deletedAt(t, id); ts == nil {
		t.Errorf("%s: id=%d expected deleted_at to be SET, got NULL", msg, id)
	}
}

// assertLive fails if id's deleted_at is set.
func (f *cascadeFixture) assertLive(t *testing.T, id int64, msg string) {
	t.Helper()
	if ts := f.deletedAt(t, id); ts != nil {
		t.Errorf("%s: id=%d expected deleted_at to be NULL, got %v", msg, id, *ts)
	}
}

func TestCascadeSoftDelete_ServiceA_AffectsBothVisibilities(t *testing.T) {
	f := newCascadeFixture(t)

	tx, err := f.d.SQL.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	n, err := vault.CascadeSoftDelete(context.Background(), tx, models.SecretScopeService, f.serviceAID, f.alice)
	if err != nil {
		t.Fatalf("cascade: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n != 2 {
		t.Errorf("cascade affected: got %d, want 2 (shared+personal child of service A)", n)
	}

	// Both A children: soft-deleted.
	f.assertDeleted(t, f.aSharedID, "shared child of A")
	f.assertDeleted(t, f.aPersonalID, "personal child of A")
	// Controls: untouched.
	f.assertLive(t, f.bSharedID, "shared child of B")
	f.assertLive(t, f.xSharedID, "shared child of host X")
	f.assertLive(t, f.avulsoID, "avulso secret")
}

func TestCascadeSoftDelete_AvulsoNeverCascaded(t *testing.T) {
	f := newCascadeFixture(t)

	// Cascading on scope=avulso must be a no-op even if a caller misuses it.
	tx, err := f.d.SQL.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	n, err := vault.CascadeSoftDelete(context.Background(), tx, models.SecretScopeAvulso, 0, f.alice)
	if err != nil {
		t.Fatalf("cascade avulso: %v", err)
	}
	_ = tx.Commit()
	if n != 0 {
		t.Errorf("avulso cascade affected %d rows; want 0", n)
	}
	f.assertLive(t, f.avulsoID, "avulso must stay live")
}

func TestCascadeSoftDelete_OrphanParentNoop(t *testing.T) {
	f := newCascadeFixture(t)

	// A parent_id with no matching children → zero rows, no error.
	tx, _ := f.d.SQL.BeginTx(context.Background(), nil)
	n, err := vault.CascadeSoftDelete(context.Background(), tx, models.SecretScopeService, 99999, f.alice)
	_ = tx.Commit()
	if err != nil {
		t.Fatalf("orphan cascade: %v", err)
	}
	if n != 0 {
		t.Errorf("orphan cascade affected: got %d, want 0", n)
	}
	// All seeded rows still live.
	f.assertLive(t, f.aSharedID, "")
	f.assertLive(t, f.aPersonalID, "")
}

func TestCascadeRestore_RoundTrip(t *testing.T) {
	f := newCascadeFixture(t)
	ctx := context.Background()

	// Soft-delete service A's children.
	tx, _ := f.d.SQL.BeginTx(ctx, nil)
	if _, err := vault.CascadeSoftDelete(ctx, tx, models.SecretScopeService, f.serviceAID, f.alice); err != nil {
		t.Fatalf("soft: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit soft: %v", err)
	}
	f.assertDeleted(t, f.aSharedID, "pre-restore")
	f.assertDeleted(t, f.aPersonalID, "pre-restore")

	// Restore them.
	tx, _ = f.d.SQL.BeginTx(ctx, nil)
	n, err := vault.CascadeRestore(ctx, tx, models.SecretScopeService, f.serviceAID, f.alice)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit restore: %v", err)
	}
	if n != 2 {
		t.Errorf("restore affected: got %d, want 2", n)
	}
	f.assertLive(t, f.aSharedID, "after restore")
	f.assertLive(t, f.aPersonalID, "after restore")
}

func TestCascadeSoftDelete_WritesAuditRows(t *testing.T) {
	f := newCascadeFixture(t)

	tx, _ := f.d.SQL.BeginTx(context.Background(), nil)
	if _, err := vault.CascadeSoftDelete(context.Background(), tx, models.SecretScopeService, f.serviceAID, f.alice); err != nil {
		t.Fatalf("cascade: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	for _, id := range []int64{f.aSharedID, f.aPersonalID} {
		var count int
		if err := f.d.SQL.QueryRow(
			`SELECT COUNT(*) FROM secret_audit_log WHERE secret_id = ? AND action = 'delete'`, id,
		).Scan(&count); err != nil {
			t.Fatalf("count audit: %v", err)
		}
		if count != 1 {
			t.Errorf("secret %d should have exactly 1 delete-audit row, got %d", id, count)
		}
	}
}

// TestCascadeSoftDelete_RespectsAlreadyDeleted: if the cascade is called twice
// in a row, the second pass must not re-flip already-deleted rows (and must
// not write extra audit rows). Guards against double-cascade bugs in handlers.
func TestCascadeSoftDelete_RespectsAlreadyDeleted(t *testing.T) {
	f := newCascadeFixture(t)
	ctx := context.Background()

	tx, _ := f.d.SQL.BeginTx(ctx, nil)
	if _, err := vault.CascadeSoftDelete(ctx, tx, models.SecretScopeService, f.serviceAID, f.alice); err != nil {
		t.Fatalf("first: %v", err)
	}
	_ = tx.Commit()

	tx, _ = f.d.SQL.BeginTx(ctx, nil)
	n, err := vault.CascadeSoftDelete(ctx, tx, models.SecretScopeService, f.serviceAID, f.alice)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	_ = tx.Commit()
	if n != 0 {
		t.Errorf("second cascade affected: got %d, want 0", n)
	}

	for _, id := range []int64{f.aSharedID, f.aPersonalID} {
		var count int
		_ = f.d.SQL.QueryRow(
			`SELECT COUNT(*) FROM secret_audit_log WHERE secret_id = ? AND action = 'delete'`, id,
		).Scan(&count)
		if count != 1 {
			t.Errorf("secret %d: expected 1 delete-audit row after double cascade, got %d", id, count)
		}
	}
}
