package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

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
