package vault_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// secretTestEnv bundles a fresh DB and seeded user IDs for repo tests.
type secretTestEnv struct {
	d           *database.DB
	repo        *vault.SecretRepo
	alice       vault.ActorContext // admin
	bob         vault.ActorContext // editor
	carol       vault.ActorContext // viewer
	systemOwner int64        // for legacy-shared placeholder
}

func newSecretTestEnv(t *testing.T) *secretTestEnv {
	t.Helper()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	seedUser := func(name, role string) int64 {
		res, err := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
			name, "x", role)
		if err != nil {
			t.Fatalf("seed user %s: %v", name, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	aliceID := seedUser("alice", "admin")
	bobID := seedUser("bob", "editor")
	carolID := seedUser("carol", "viewer")
	systemID := seedUser("system", "admin")

	return &secretTestEnv{
		d:           d,
		repo:        vault.NewSecretRepo(d),
		alice:       vault.ActorContext{UserID: aliceID, Role: "admin"},
		bob:         vault.ActorContext{UserID: bobID, Role: "editor"},
		carol:       vault.ActorContext{UserID: carolID, Role: "viewer"},
		systemOwner: systemID,
	}
}

// mkInput is a helper to build a Secret skeleton for Create tests.
// Pass scope='avulso' with parent=0 to omit parent_id.
func mkInput(scope, vis, name string, parent int64, owner vault.ActorContext) *models.Secret {
	var pid *int64
	if scope != "avulso" {
		p := parent
		pid = &p
	}
	s := &models.Secret{
		Type:        models.SecretTypePassword,
		Scope:       models.SecretScope(scope),
		Visibility:  models.SecretVisibility(vis),
		ParentID:    pid,
		OwnerUserID: owner.UserID,
		Name:        name,
		KeyVersion:  1,
		CreatedBy:   owner.UserID,
	}
	return s
}

func TestSecretRepo_Create_EncryptsAndAudits(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("avulso", "personal", "wifi-pwd", 0, env.bob)
	id, err := env.repo.Create(ctx, env.bob, s, "hunter2")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("Create returned id=0")
	}

	// Verify row was persisted, payload is encrypted (not plaintext)
	var ct []byte
	if err := env.d.SQL.QueryRow(`SELECT payload_ciphertext FROM secrets WHERE id = ?`, id).Scan(&ct); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if string(ct) == "hunter2" {
		t.Error("payload was stored as plaintext — encryption skipped")
	}

	// Verify audit row exists with action='create'
	var auditAction string
	if err := env.d.SQL.QueryRow(
		`SELECT action FROM secret_audit_log WHERE secret_id = ? ORDER BY id DESC LIMIT 1`, id,
	).Scan(&auditAction); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if auditAction != "create" {
		t.Errorf("audit action = %q, want create", auditAction)
	}
}

func TestSecretRepo_Create_RejectsDuplicate(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("host", "shared", "team-key", 42, env.bob)
	if _, err := env.repo.Create(ctx, env.bob, s, "v1"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Duplicate (same scope+parent+name on shared) should fail
	s2 := mkInput("host", "shared", "team-key", 42, env.bob)
	if _, err := env.repo.Create(ctx, env.bob, s2, "v2"); err == nil {
		t.Error("duplicate shared secret should be rejected by partial-unique index")
	}
}

func TestSecretRepo_GetMetadata_ExcludesPayload(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("avulso", "shared", "wifi-pwd", 0, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "hunter2")

	view, err := env.repo.GetMetadata(ctx, env.bob, id)
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if view == nil {
		t.Fatal("GetMetadata returned nil")
	}
	if view.Name != "wifi-pwd" {
		t.Errorf("Name = %q", view.Name)
	}
	if len(view.PayloadCiphertext) != 0 {
		t.Error("GetMetadata view should NOT include payload bytes")
	}
}

func TestSecretRepo_Reveal_OwnerCanReveal(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("avulso", "personal", "my-pwd", 0, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "letmein")

	plain, err := env.repo.Reveal(ctx, env.bob, id)
	if err != nil {
		t.Fatalf("owner Reveal: %v", err)
	}
	if plain != "letmein" {
		t.Errorf("Reveal got %q, want letmein", plain)
	}

	// Audit row written for reveal
	var auditAction string
	if err := env.d.SQL.QueryRow(
		`SELECT action FROM secret_audit_log WHERE secret_id = ? AND action = 'reveal' LIMIT 1`, id,
	).Scan(&auditAction); err != nil {
		t.Errorf("reveal should write audit row: %v", err)
	}
}

func TestSecretRepo_Reveal_AdminCannotRevealOthersPersonal(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	// bob (editor) owns a personal secret
	s := mkInput("avulso", "personal", "bobs-pwd", 0, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "secret")

	// alice (admin) cannot reveal bob's personal — spec D1/§5.2
	_, err := env.repo.Reveal(ctx, env.alice, id)
	if err == nil {
		t.Error("admin should NOT be able to reveal another user's personal secret (D1)")
	}
}

func TestSecretRepo_Reveal_SharedSecretAllowsRoleBased(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("host", "shared", "shared-key", 1, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "team-secret")

	// carol (viewer) can reveal shared per §5.1
	plain, err := env.repo.Reveal(ctx, env.carol, id)
	if err != nil {
		t.Fatalf("viewer should reveal shared: %v", err)
	}
	if plain != "team-secret" {
		t.Errorf("got %q", plain)
	}
}

func TestSecretRepo_List_FiltersDeleted(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	// Use personal-scope secrets so the owner (bob) is allowed to soft-delete
	// them. Shared deletion is admin-only per spec §5.1 and is exercised by
	// TestSecretRepo_ListTrash_ScopedToCaller.
	live := mkInput("avulso", "personal", "live", 0, env.bob)
	liveID, _ := env.repo.Create(ctx, env.bob, live, "p1")
	gone := mkInput("avulso", "personal", "gone", 0, env.bob)
	goneID, _ := env.repo.Create(ctx, env.bob, gone, "p2")
	if err := env.repo.SoftDelete(ctx, env.bob, goneID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	// Default list omits deleted
	views, err := env.repo.List(ctx, env.bob, vault.SecretFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 1 || views[0].ID != liveID {
		t.Errorf("List default should return only live; got %d items", len(views))
	}

	// IncludeDeleted=true returns both
	views2, _ := env.repo.List(ctx, env.bob, vault.SecretFilter{IncludeDeleted: true})
	if len(views2) != 2 {
		t.Errorf("List(IncludeDeleted) should return 2; got %d", len(views2))
	}
	_ = goneID
}

func TestSecretRepo_List_HidesOthersPersonalFromNonAdmin(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	// bob creates a personal
	s := mkInput("avulso", "personal", "bobs-thing", 0, env.bob)
	_, _ = env.repo.Create(ctx, env.bob, s, "x")

	// carol (viewer) should NOT see bob's personal
	views, _ := env.repo.List(ctx, env.carol, vault.SecretFilter{})
	for _, v := range views {
		if v.Visibility == models.SecretVisibilityPersonal && v.OwnerUserID != env.carol.UserID {
			t.Errorf("carol should not see bob's personal in list; saw %+v", v)
		}
	}
}

func TestSecretRepo_List_AdminSeesOthersPersonalRedacted(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	desc := "private to bob"
	s := mkInput("avulso", "personal", "bobs-thing", 0, env.bob)
	s.Description = &desc
	_, _ = env.repo.Create(ctx, env.bob, s, "x")

	views, _ := env.repo.List(ctx, env.alice, vault.SecretFilter{})
	var found *vault.SecretView
	for i := range views {
		if views[i].Name == "bobs-thing" {
			found = &views[i]
			break
		}
	}
	if found == nil {
		t.Fatal("admin should see bob's personal as metadata in list")
	}
	// Description redacted per D6
	if found.Description != nil && *found.Description != "" {
		t.Errorf("admin viewing others' personal should have redacted description; got %q", *found.Description)
	}
}

func TestSecretRepo_Update_WritesChangedFieldsAudit(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("avulso", "personal", "wifi", 0, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "old-pw")

	newName := "wifi-renamed"
	newPlain := "new-pw"
	patch := vault.SecretPatch{Name: &newName, Payload: &newPlain}
	if err := env.repo.Update(ctx, env.bob, id, patch); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Reveal returns the new payload
	plain, _ := env.repo.Reveal(ctx, env.bob, id)
	if plain != "new-pw" {
		t.Errorf("reveal after update: got %q, want new-pw", plain)
	}

	// Audit row for the update has changed_fields metadata
	var metaRaw []byte
	if err := env.d.SQL.QueryRow(
		`SELECT metadata FROM secret_audit_log WHERE secret_id = ? AND action = 'update' LIMIT 1`, id,
	).Scan(&metaRaw); err != nil {
		t.Fatalf("read update audit metadata: %v", err)
	}
	var meta map[string][]string
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("parse audit metadata: %v", err)
	}
	changed := meta["changed_fields"]
	gotName, gotPayload := false, false
	for _, f := range changed {
		if f == "name" {
			gotName = true
		}
		if f == "payload" {
			gotPayload = true
		}
	}
	if !gotName || !gotPayload {
		t.Errorf("changed_fields should include name and payload; got %v", changed)
	}
}

func TestSecretRepo_SoftDelete_RestoreRoundTrip(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("avulso", "personal", "tmp", 0, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "x")

	if err := env.repo.SoftDelete(ctx, env.bob, id); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	// Default list excludes it
	views, _ := env.repo.List(ctx, env.bob, vault.SecretFilter{})
	for _, v := range views {
		if v.ID == id {
			t.Error("soft-deleted secret should be hidden from default list")
		}
	}

	// Restore brings it back
	if err := env.repo.Restore(ctx, env.bob, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	views2, _ := env.repo.List(ctx, env.bob, vault.SecretFilter{})
	found := false
	for _, v := range views2 {
		if v.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("restored secret should appear in default list again")
	}

	// Both audit rows present
	var n int
	env.d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secret_audit_log WHERE secret_id = ? AND action IN ('delete','restore')`, id,
	).Scan(&n)
	if n != 2 {
		t.Errorf("expected 2 lifecycle audit rows; got %d", n)
	}
}

func TestSecretRepo_ListTrash_ScopedToCaller(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	// bob deletes his own personal
	b := mkInput("avulso", "personal", "bob-trash", 0, env.bob)
	bID, _ := env.repo.Create(ctx, env.bob, b, "x")
	env.repo.SoftDelete(ctx, env.bob, bID)

	// alice (admin) deletes a shared (admin can per §5.1)
	a := mkInput("avulso", "shared", "team-trash", 0, env.alice)
	aID, _ := env.repo.Create(ctx, env.alice, a, "y")
	env.repo.SoftDelete(ctx, env.alice, aID)

	// bob's trash should NOT include alice's shared (bob is editor, not admin)
	bobTrash, err := env.repo.ListTrash(ctx, env.bob)
	if err != nil {
		t.Fatalf("ListTrash(bob): %v", err)
	}
	for _, v := range bobTrash {
		if v.ID == aID {
			t.Error("bob (editor) should not see admin-deletable shared in trash")
		}
	}
	// But bob's own deleted personal should appear
	foundOwn := false
	for _, v := range bobTrash {
		if v.ID == bID {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Error("bob's own deleted personal should appear in his trash")
	}
}

func TestSecretRepo_History_ReturnsAuditTrail(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	s := mkInput("avulso", "personal", "with-history", 0, env.bob)
	id, _ := env.repo.Create(ctx, env.bob, s, "v1")
	_, _ = env.repo.Reveal(ctx, env.bob, id)
	patch := vault.SecretPatch{Payload: ptrStr("v2")}
	_ = env.repo.Update(ctx, env.bob, id, patch)

	history, err := env.repo.History(ctx, env.bob, id)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) < 3 {
		t.Errorf("History should have >=3 entries (create+reveal+update); got %d", len(history))
	}
	// Sanity: actions cover the lifecycle
	actions := map[string]bool{}
	for _, h := range history {
		actions[string(h.Action)] = true
	}
	for _, want := range []string{"create", "reveal", "update"} {
		if !actions[want] {
			t.Errorf("History missing action %q", want)
		}
	}
}

func ptrStr(s string) *string { return &s }
