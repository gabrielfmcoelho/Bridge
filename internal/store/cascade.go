package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ParentSpec describes a secret-bearing parent table — one whose deletion must
// cascade to the child secrets that point at it via secrets.parent_id. Because
// secrets.parent_id is polymorphic with NO foreign key, a naive parent delete
// would silently orphan its child secrets (still live, still queryable through
// /api/secrets) — spec §4.1 D3. Routing every parent delete through this
// registry closes that gap and, crucially, makes it IMPOSSIBLE to add a new
// secret-bearing parent and forget the cascade: registering the scope is the
// one required step, and DeleteParent fails loudly for unregistered scopes.
type ParentSpec struct {
	// Table is the parent's table name (trusted in-code constant).
	Table string
	// SoftDelete selects the delete strategy: true flips deleted_at, false
	// hard-DELETEs the row. Restore is only meaningful when SoftDelete is true.
	SoftDelete bool
}

// parentRegistry maps a secret scope to its parent table. avulso is absent by
// design: avulso secrets have no parent (CHECK constraint) so they never
// cascade. Populated by the init below; extend via RegisterParent.
var parentRegistry = map[models.SecretScope]ParentSpec{}

func init() {
	// The four secret-bearing parents that exist today. Adding a fifth means
	// adding one line here (or a RegisterParent call in the entity's repo).
	RegisterParent(models.SecretScopeService, ParentSpec{Table: "services", SoftDelete: false})
	RegisterParent(models.SecretScopeHost, ParentSpec{Table: "hosts", SoftDelete: false})
	RegisterParent(models.SecretScopeProjeto, ParentSpec{Table: "projects", SoftDelete: false})
	RegisterParent(models.SecretScopeTool, ParentSpec{Table: "external_tools", SoftDelete: true})
}

// RegisterParent records that `scope` secrets hang off the given parent table.
// Idempotent-by-overwrite; intended to be called from init() / startup only.
func RegisterParent(scope models.SecretScope, spec ParentSpec) {
	parentRegistry[scope] = spec
}

// ParentOf returns the registered spec for a scope.
func ParentOf(scope models.SecretScope) (ParentSpec, bool) {
	spec, ok := parentRegistry[scope]
	return spec, ok
}

// DeleteParent deletes the parent row identified by (scope, id) AND cascade-
// soft-deletes its child secrets, atomically in one transaction. The delete
// strategy (hard vs soft) comes from the registered ParentSpec.
func DeleteParent(ctx context.Context, db *sql.DB, actor models.ActorContext, scope models.SecretScope, id int64) error {
	spec, ok := ParentOf(scope)
	if !ok {
		return fmt.Errorf("scope %q is not a registered secret-bearing parent", scope)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	if _, err := cascadeSoftDelete(ctx, tx, scope, id, actor); err != nil {
		return err
	}
	var parentSQL string
	if spec.SoftDelete {
		parentSQL = fmt.Sprintf(`UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, spec.Table)
	} else {
		parentSQL = fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, spec.Table)
	}
	if _, err := tx.ExecContext(ctx, parentSQL, id); err != nil {
		return err
	}
	return tx.Commit()
}

// RestoreParent reverses a soft DeleteParent: clears deleted_at on the parent
// row AND cascade-restores its child secrets, atomically. Only valid for scopes
// whose ParentSpec.SoftDelete is true.
func RestoreParent(ctx context.Context, db *sql.DB, actor models.ActorContext, scope models.SecretScope, id int64) error {
	spec, ok := ParentOf(scope)
	if !ok {
		return fmt.Errorf("scope %q is not a registered secret-bearing parent", scope)
	}
	if !spec.SoftDelete {
		return fmt.Errorf("scope %q uses hard delete and cannot be restored", scope)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	if _, err := cascadeRestore(ctx, tx, scope, id, actor); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, spec.Table),
		id); err != nil {
		return err
	}
	return tx.Commit()
}

// cascadeSoftDelete soft-deletes every live secret matching (scope, parentID)
// inside the given tx and writes one `delete` audit row per affected secret.
// Both visibilities cascade (the parent's deletion is the source of truth for
// its entire surface). avulso never matches (no parent). Already-deleted rows
// are skipped so a double-cascade doesn't duplicate audit rows. Returns the
// number of secrets touched. (Moved here from internal/vault so the store
// layer no longer depends on vault — breaks the store↔vault cycle.)
func cascadeSoftDelete(ctx context.Context, tx *sql.Tx, scope models.SecretScope, parentID int64, actor models.ActorContext) (int64, error) {
	if scope == models.SecretScopeAvulso {
		return 0, nil
	}
	if !scope.Valid() {
		return 0, fmt.Errorf("invalid scope %q", scope)
	}
	ids, err := childSecretIDs(ctx, tx, scope, parentID, false)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE secrets SET deleted_at = CURRENT_TIMESTAMP WHERE scope = ? AND parent_id = ? AND deleted_at IS NULL`,
		string(scope), parentID); err != nil {
		return 0, fmt.Errorf("soft-delete: %w", err)
	}
	meta := cascadeMeta(scope, parentID)
	for _, id := range ids {
		if err := writeSecretAuditTx(ctx, tx, id, models.SecretAuditActionDelete, actor, meta); err != nil {
			return 0, fmt.Errorf("audit %d: %w", id, err)
		}
	}
	return int64(len(ids)), nil
}

// cascadeRestore is the symmetric inverse of cascadeSoftDelete.
func cascadeRestore(ctx context.Context, tx *sql.Tx, scope models.SecretScope, parentID int64, actor models.ActorContext) (int64, error) {
	if scope == models.SecretScopeAvulso {
		return 0, nil
	}
	if !scope.Valid() {
		return 0, fmt.Errorf("invalid scope %q", scope)
	}
	ids, err := childSecretIDs(ctx, tx, scope, parentID, true)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE secrets SET deleted_at = NULL WHERE scope = ? AND parent_id = ? AND deleted_at IS NOT NULL`,
		string(scope), parentID); err != nil {
		return 0, fmt.Errorf("restore: %w", err)
	}
	meta := cascadeMeta(scope, parentID)
	for _, id := range ids {
		if err := writeSecretAuditTx(ctx, tx, id, models.SecretAuditActionRestore, actor, meta); err != nil {
			return 0, fmt.Errorf("audit %d: %w", id, err)
		}
	}
	return int64(len(ids)), nil
}

// childSecretIDs collects the ids of child secrets for (scope, parentID),
// filtered to live (deleted=false) or soft-deleted (deleted=true) rows. A
// two-step select-then-update keeps the cascade portable across dialects.
func childSecretIDs(ctx context.Context, tx *sql.Tx, scope models.SecretScope, parentID int64, deleted bool) ([]int64, error) {
	cond := "deleted_at IS NULL"
	if deleted {
		cond = "deleted_at IS NOT NULL"
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM secrets WHERE scope = ? AND parent_id = ? AND `+cond,
		string(scope), parentID)
	if err != nil {
		return nil, fmt.Errorf("collect children: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// cascadeMeta returns the audit metadata blob attached to every cascade-
// triggered audit row, so operators can distinguish cascades from direct
// deletes/restores when reading the audit log.
func cascadeMeta(scope models.SecretScope, parentID int64) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"cascade_from": fmt.Sprintf("%s:%d", scope, parentID),
	})
	return b
}

// writeSecretAuditTx inserts a secret_audit_log row inside the given tx. This
// mirrors the vault repo's internal audit writer for the cascade case; a
// UserID of 0 records a NULL actor.
func writeSecretAuditTx(ctx context.Context, tx *sql.Tx, secretID int64, action models.SecretAuditAction, actor models.ActorContext, metadata json.RawMessage) error {
	var actorID *int64
	if actor.UserID != 0 {
		a := actor.UserID
		actorID = &a
	}
	var metaParam any
	if len(metadata) > 0 {
		metaParam = string(metadata)
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO secret_audit_log (secret_id, action, actor_user_id, share_link_id, metadata)
		 VALUES (?,?,?,?,?)`,
		secretID, string(action), actorID, nil, metaParam)
	return err
}
