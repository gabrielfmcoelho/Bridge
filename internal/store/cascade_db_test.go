package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// seedSecret inserts a child secret directly (bypassing the vault repo, which
// store tests must not depend on). Returns the new id.
func seedSecret(t *testing.T, d *database.DB, scope models.SecretScope, parentID, owner int64, name string) int64 {
	t.Helper()
	res, err := d.SQL.Exec(
		`INSERT INTO secrets (type, scope, visibility, parent_id, owner_user_id, name, payload_ciphertext, payload_nonce, key_version, created_by)
		 VALUES ('password', ?, 'shared', ?, ?, ?, X'00', X'00', 1, ?)`,
		string(scope), parentID, owner, name, owner)
	if err != nil {
		t.Fatalf("seed secret: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func deletedAt(t *testing.T, d *database.DB, secretID int64) bool {
	t.Helper()
	var del *string
	if err := d.SQL.QueryRow(`SELECT deleted_at FROM secrets WHERE id = ?`, secretID).Scan(&del); err != nil {
		t.Fatalf("read deleted_at: %v", err)
	}
	return del != nil
}

func auditCount(t *testing.T, d *database.DB, secretID int64, action string) int {
	t.Helper()
	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM secret_audit_log WHERE secret_id = ? AND action = ?`, secretID, action).Scan(&n); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return n
}

func TestDeleteParent_HardDeleteCascadesChildSecrets(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	ures, _ := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin')`)
	owner, _ := ures.LastInsertId()
	hres, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h','h')`)
	hostID, _ := hres.LastInsertId()

	s1 := seedSecret(t, d, models.SecretScopeHost, hostID, owner, "pw1")
	s2 := seedSecret(t, d, models.SecretScopeHost, hostID, owner, "pw2")
	actor := models.ActorContext{UserID: owner, Role: "admin"}

	if err := store.DeleteParent(ctx, d.SQL, actor, models.SecretScopeHost, hostID); err != nil {
		t.Fatalf("DeleteParent: %v", err)
	}

	// Parent hard-deleted.
	var n int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM hosts WHERE id = ?`, hostID).Scan(&n)
	if n != 0 {
		t.Fatalf("host still present after hard DeleteParent")
	}
	// Both child secrets soft-deleted, each with a `delete` audit row.
	if !deletedAt(t, d, s1) || !deletedAt(t, d, s2) {
		t.Fatalf("child secrets not soft-deleted")
	}
	if auditCount(t, d, s1, "delete") != 1 || auditCount(t, d, s2, "delete") != 1 {
		t.Fatalf("expected one delete audit row per child")
	}
}

func TestDeleteRestoreParent_SoftDeleteRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	ures, _ := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin')`)
	owner, _ := ures.LastInsertId()
	tres, _ := d.SQL.Exec(`INSERT INTO external_tools (name, url) VALUES ('t','http://t')`)
	toolID, _ := tres.LastInsertId()

	s1 := seedSecret(t, d, models.SecretScopeTool, toolID, owner, "tk")
	actor := models.ActorContext{UserID: owner, Role: "admin"}

	if err := store.DeleteParent(ctx, d.SQL, actor, models.SecretScopeTool, toolID); err != nil {
		t.Fatalf("DeleteParent(tool): %v", err)
	}
	if !deletedAt(t, d, s1) {
		t.Fatalf("tool secret not soft-deleted")
	}
	// Tool itself soft-deleted (deleted_at set), not removed.
	var toolDel *string
	d.SQL.QueryRow(`SELECT deleted_at FROM external_tools WHERE id = ?`, toolID).Scan(&toolDel)
	if toolDel == nil {
		t.Fatalf("tool should be soft-deleted, not hard-deleted")
	}

	if err := store.RestoreParent(ctx, d.SQL, actor, models.SecretScopeTool, toolID); err != nil {
		t.Fatalf("RestoreParent(tool): %v", err)
	}
	if deletedAt(t, d, s1) {
		t.Fatalf("tool secret not restored")
	}
	if auditCount(t, d, s1, "restore") != 1 {
		t.Fatalf("expected one restore audit row")
	}
}
