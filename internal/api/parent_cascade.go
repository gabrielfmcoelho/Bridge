package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// actorUserID extracts the authenticated user's id from the request context,
// returning 0 when no user is attached (e.g. before auth middleware fires,
// or in tests). Useful for dual-write paths that need an audit-row actor
// without re-doing the full auth boilerplate.
func actorUserID(r *http.Request) int64 {
	if u := auth.UserFromContext(r.Context()); u != nil {
		return u.ID
	}
	return 0
}

// cascadeParentDelete soft-deletes every child secret attached to the given
// (scope, parentID) and then executes parentDeleteSQL (one `?` placeholder
// for parent id) within the SAME transaction. Used by the existing service/
// host/tool DELETE handlers so the parent's hard-delete and the child
// secrets' soft-delete commit atomically.
//
// Why this exists: secrets.parent_id is polymorphic (service|host|tool id)
// with no FK enforcement, so a naive parent hard-delete would silently
// orphan its child secrets — leaving them live and queryable through
// /api/secrets even though the parent no longer exists. Routing every
// parent delete through here closes that gap (Task 1.7, spec §4.1 D3).
//
// The actor is taken from the request context — handler middleware has
// already authorized the caller (admin role on the three DELETE routes),
// so we don't re-check here.
func cascadeParentDelete(
	r *http.Request,
	db *database.DB,
	scope models.SecretScope,
	parentID int64,
	parentDeleteSQL string,
) error {
	var actor vault.ActorContext
	if u := auth.UserFromContext(r.Context()); u != nil {
		actor = vault.ActorContext{UserID: u.ID, Role: u.Role}
	}
	tx, err := db.SQL.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := vault.CascadeSoftDelete(r.Context(), tx, scope, parentID, actor); err != nil {
		return err
	}
	if _, err := tx.ExecContext(r.Context(), parentDeleteSQL, parentID); err != nil {
		return err
	}
	return tx.Commit()
}

// cascadeParentSoftDelete is the soft-delete counterpart: it cascade-soft-
// deletes child secrets and then runs parentSoftDeleteSQL (one `?` for
// parent id) — typically `UPDATE <table> SET deleted_at = CURRENT_TIMESTAMP
// WHERE id = ?`. Both rows-affected counts commit in the same transaction.
//
// Used by parent tables that support soft-delete (currently only
// external_tools after migration v62). Services + hosts remain
// hard-delete-only and route through cascadeParentDelete.
func cascadeParentSoftDelete(
	r *http.Request,
	db *database.DB,
	scope models.SecretScope,
	parentID int64,
	parentSoftDeleteSQL string,
) error {
	return cascadeParentDelete(r, db, scope, parentID, parentSoftDeleteSQL)
}

// cascadeParentRestore is the inverse: cascade-restores child secrets, then
// runs parentRestoreSQL — typically `UPDATE <table> SET deleted_at = NULL
// WHERE id = ?`. Both commits happen atomically.
//
// Activates vault.CascadeRestore which until v62 had no live caller.
func cascadeParentRestore(
	r *http.Request,
	db *database.DB,
	scope models.SecretScope,
	parentID int64,
	parentRestoreSQL string,
) error {
	var actor vault.ActorContext
	if u := auth.UserFromContext(r.Context()); u != nil {
		actor = vault.ActorContext{UserID: u.ID, Role: u.Role}
	}
	tx, err := db.SQL.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := vault.CascadeRestore(r.Context(), tx, scope, parentID, actor); err != nil {
		return err
	}
	if _, err := tx.ExecContext(r.Context(), parentRestoreSQL, parentID); err != nil {
		return err
	}
	return tx.Commit()
}
