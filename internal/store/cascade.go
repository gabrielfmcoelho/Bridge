package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
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
// strategy (hard vs soft) comes from the registered ParentSpec. Replaces the
// hand-wired cascadeParentDelete / cascadeParentSoftDelete helpers.
func DeleteParent(ctx context.Context, db *sql.DB, actor vault.ActorContext, scope models.SecretScope, id int64) error {
	spec, ok := ParentOf(scope)
	if !ok {
		return fmt.Errorf("scope %q is not a registered secret-bearing parent", scope)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	if _, err := vault.CascadeSoftDelete(ctx, tx, scope, id, actor); err != nil {
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
// whose ParentSpec.SoftDelete is true. Replaces cascadeParentRestore.
func RestoreParent(ctx context.Context, db *sql.DB, actor vault.ActorContext, scope models.SecretScope, id int64) error {
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
	if _, err := vault.CascadeRestore(ctx, tx, scope, id, actor); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, spec.Table),
		id); err != nil {
		return err
	}
	return tx.Commit()
}
