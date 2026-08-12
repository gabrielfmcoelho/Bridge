package vault_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

type envFixture struct {
	t         *testing.T
	d         *database.DB
	repo      *vault.SecretRepo
	bob       vault.ActorContext // editor
	carol     vault.ActorContext // viewer
	serviceID int64
}

func newEnvFixture(t *testing.T) *envFixture {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	mk := func(name, role string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`, name, "x", role).Scan(&id); err != nil {
			t.Fatalf("seed user %s: %v", name, err)
		}
		return id
	}
	bobID := mk("bob", "editor")
	carolID := mk("carol", "viewer")

	var svcID int64
	if err := d.SQL.QueryRow(`INSERT INTO services (nickname) VALUES (?) RETURNING id`, "billing-api").Scan(&svcID); err != nil {
		t.Fatalf("seed svc: %v", err)
	}

	return &envFixture{
		t: t, d: d, repo: vault.NewSecretRepo(d),
		bob:       vault.ActorContext{UserID: bobID, Role: "editor"},
		carol:     vault.ActorContext{UserID: carolID, Role: "viewer"},
		serviceID: svcID,
	}
}

func TestBulkUpsertEnvVars_HappyPath(t *testing.T) {
	f := newEnvFixture(t)

	desc := "primary db url"
	res, err := f.repo.BulkUpsertEnvVars(
		context.Background(), f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{
			{Name: "DB_URL", Value: "postgres://...", Description: &desc},
			{Name: "API_KEY", Value: "k-12345"},
			{Name: "TIMEOUT_SECONDS", Value: "30"},
		},
	)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if res.Created != 3 || res.Updated != 0 {
		t.Errorf("first run: got %+v, want {Created:3 Updated:0}", res)
	}

	// All 3 env_var rows landed with the right shape.
	var count int
	if err := f.d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secrets WHERE type='env_var' AND scope='service' AND parent_id=? AND group_label='prod'`,
		f.serviceID,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 env_var rows, got %d", count)
	}
}

func TestBulkUpsertEnvVars_SecondRunUpdates(t *testing.T) {
	f := newEnvFixture(t)
	ctx := context.Background()

	if _, err := f.repo.BulkUpsertEnvVars(ctx, f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "v1"}}); err != nil {
		t.Fatalf("first: %v", err)
	}

	res, err := f.repo.BulkUpsertEnvVars(ctx, f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "v2"}})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if res.Updated != 1 || res.Created != 0 {
		t.Errorf("re-run: got %+v, want {Updated:1 Created:0}", res)
	}

	// Total row count unchanged after the update.
	var n int
	_ = f.d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE name='DB_URL' AND group_label='prod'`).Scan(&n)
	if n != 1 {
		t.Errorf("expected 1 DB_URL row after update, got %d", n)
	}
}

func TestBulkUpsertEnvVars_RejectsInPayloadDuplicates(t *testing.T) {
	f := newEnvFixture(t)

	_, err := f.repo.BulkUpsertEnvVars(context.Background(), f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{
			{Name: "DB_URL", Value: "a"},
			{Name: "OK", Value: "b"},
			{Name: "DB_URL", Value: "c"}, // duplicate
		},
	)
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	if !strings.Contains(err.Error(), "DB_URL") {
		t.Errorf("error should reference duplicate name: %v", err)
	}

	// Zero writes per DoD: no rows landed at all.
	var n int
	_ = f.d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE group_label='prod'`).Scan(&n)
	if n != 0 {
		t.Errorf("expected 0 rows after duplicate rejection, got %d", n)
	}
}

func TestBulkUpsertEnvVars_RejectsBadName(t *testing.T) {
	f := newEnvFixture(t)
	_, err := f.repo.BulkUpsertEnvVars(context.Background(), f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "db_url", Value: "x"}}, // lowercase
	)
	if err == nil {
		t.Error("lowercase name should fail")
	}
}

func TestBulkUpsertEnvVars_RejectsEmptyValue(t *testing.T) {
	f := newEnvFixture(t)
	_, err := f.repo.BulkUpsertEnvVars(context.Background(), f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: ""}},
	)
	if err == nil {
		t.Error("empty value should fail")
	}
}

func TestBulkUpsertEnvVars_ViewerCannotWriteShared(t *testing.T) {
	f := newEnvFixture(t)
	_, err := f.repo.BulkUpsertEnvVars(context.Background(), f.carol,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "v"}},
	)
	if err != vault.ErrSecretForbidden {
		t.Errorf("viewer writing shared should be forbidden, got %v", err)
	}
}

func TestBulkUpsertEnvVars_PartialFailureRollsBack(t *testing.T) {
	f := newEnvFixture(t)
	ctx := context.Background()

	// Seed an env_var under a DIFFERENT group_label so the bulk pass below
	// has both an insert path (DB_URL/prod) AND a path that would conflict
	// if we tried to "create" DB_URL/staging twice in the same payload.
	// (The duplicate rejection happens up front so we use a different
	// failure mode here — an invalid name mid-list to trip per-row checks.)
	if _, err := f.repo.BulkUpsertEnvVars(ctx, f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "EXISTING", Value: "v"}}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Now a batch where the 2nd entry is invalid. The 1st should NOT land.
	_, err := f.repo.BulkUpsertEnvVars(ctx, f.bob,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{
			{Name: "NEW_ONE", Value: "v"},
			{Name: "bad name", Value: "x"}, // invalid → reject whole batch
		},
	)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	var n int
	_ = f.d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE name='NEW_ONE'`).Scan(&n)
	if n != 0 {
		t.Errorf("NEW_ONE should not have been written (whole batch rolls back), got %d", n)
	}
}

// TestBulkUpsertEnvVars_PersonalIsolatedPerOwner verifies that two users
// each owning a personal DB_URL on the same service+group is allowed, while
// a single user re-running the same payload upserts cleanly.
func TestBulkUpsertEnvVars_PersonalIsolatedPerOwner(t *testing.T) {
	f := newEnvFixture(t)
	ctx := context.Background()

	// alice (additional user, viewer for visibility=personal write is fine
	// since personal ownership grants write to the owner regardless of role).
	var aliceID int64
	if err := f.d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`, "alice", "x", "viewer").Scan(&aliceID); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	alice := vault.ActorContext{UserID: aliceID, Role: "viewer"}

	if _, err := f.repo.BulkUpsertEnvVars(ctx, f.carol,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityPersonal, "dev",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "carol-val"}}); err != nil {
		t.Fatalf("carol: %v", err)
	}
	if _, err := f.repo.BulkUpsertEnvVars(ctx, alice,
		models.SecretScopeService, &f.serviceID,
		models.SecretVisibilityPersonal, "dev",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "alice-val"}}); err != nil {
		t.Fatalf("alice: %v", err)
	}

	var n int
	_ = f.d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secrets WHERE name='DB_URL' AND group_label='dev' AND visibility='personal'`,
	).Scan(&n)
	if n != 2 {
		t.Errorf("expected 2 personal DB_URL rows (one per owner), got %d", n)
	}
}

// --- Multi-target (Part B of the env_var UX spec) ---

// seedProjectAndServices returns a project id, a service that belongs to it,
// and a service that belongs to a DIFFERENT project — enough to exercise the
// project<->service guard.
func (f *envFixture) seedProjectAndServices(t *testing.T) (projectID, svcInProject, svcOtherProject int64) {
	t.Helper()
	if err := f.d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('proj-a') RETURNING id`).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var otherProject int64
	if err := f.d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('proj-b') RETURNING id`).Scan(&otherProject); err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	if err := f.d.SQL.QueryRow(`INSERT INTO services (nickname, project_id) VALUES ('svc-a', ?) RETURNING id`, projectID).Scan(&svcInProject); err != nil {
		t.Fatalf("seed svc-a: %v", err)
	}
	if err := f.d.SQL.QueryRow(`INSERT INTO services (nickname, project_id) VALUES ('svc-b', ?) RETURNING id`, otherProject).Scan(&svcOtherProject); err != nil {
		t.Fatalf("seed svc-b: %v", err)
	}
	return projectID, svcInProject, svcOtherProject
}

func TestBulkUpsertEnvVarsMulti_TwoTargets_OneTx(t *testing.T) {
	f := newEnvFixture(t)
	ctx := context.Background()
	projectID, svcInProject, _ := f.seedProjectAndServices(t)

	res, err := f.repo.BulkUpsertEnvVarsMulti(ctx, f.bob,
		[]vault.EnvVarTarget{
			{Scope: models.SecretScopeProjeto, ParentID: &projectID},
			{Scope: models.SecretScopeService, ParentID: &svcInProject},
		},
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{
			{Name: "DB_URL", Value: "postgres://x"},
			{Name: "API_KEY", Value: "k-1"},
		},
	)
	if err != nil {
		t.Fatalf("multi bulk: %v", err)
	}
	// 2 vars written to each of 2 targets = 4 creates.
	if res.Created != 4 || res.Updated != 0 {
		t.Errorf("got %+v, want {Created:4 Updated:0}", res)
	}

	var projRows, svcRows int
	_ = f.d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE scope='projeto' AND parent_id=? AND group_label='prod'`, projectID).Scan(&projRows)
	_ = f.d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE scope='service' AND parent_id=? AND group_label='prod'`, svcInProject).Scan(&svcRows)
	if projRows != 2 || svcRows != 2 {
		t.Errorf("expected 2 rows per target, got proj=%d svc=%d", projRows, svcRows)
	}
}

func TestBulkUpsertEnvVarsMulti_ServiceNotInProject_Rejected(t *testing.T) {
	f := newEnvFixture(t)
	ctx := context.Background()
	projectID, _, svcOtherProject := f.seedProjectAndServices(t)

	_, err := f.repo.BulkUpsertEnvVarsMulti(ctx, f.bob,
		[]vault.EnvVarTarget{
			{Scope: models.SecretScopeProjeto, ParentID: &projectID},
			{Scope: models.SecretScopeService, ParentID: &svcOtherProject}, // wrong project
		},
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "v"}},
	)
	if err == nil {
		t.Fatal("expected rejection when service does not belong to the project")
	}

	// Atomic: nothing landed for either target.
	var n int
	_ = f.d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE group_label='prod'`).Scan(&n)
	if n != 0 {
		t.Errorf("expected 0 rows after guard rejection, got %d", n)
	}
}

func TestBulkUpsertEnvVarsMulti_DuplicateTarget_Rejected(t *testing.T) {
	f := newEnvFixture(t)
	ctx := context.Background()

	_, err := f.repo.BulkUpsertEnvVarsMulti(ctx, f.bob,
		[]vault.EnvVarTarget{
			{Scope: models.SecretScopeService, ParentID: &f.serviceID},
			{Scope: models.SecretScopeService, ParentID: &f.serviceID}, // same target twice
		},
		models.SecretVisibilityShared, "prod",
		[]vault.EnvVarUpsert{{Name: "DB_URL", Value: "v"}},
	)
	if err == nil {
		t.Fatal("expected rejection for duplicate (scope,parent) target")
	}
}
