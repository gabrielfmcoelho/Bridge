package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// actorFrom is the single source of truth for turning an authenticated request
// into an ACL-aware vault.ActorContext. ok=false means no user is attached to
// the context — auth middleware should already have rejected such a request,
// so callers treat !ok as 401.
//
// It replaces three earlier copies that did the same thing with different
// signatures: secretHandlers.actor, shareBundleHandlers.actor, and
// apiCatalogHandlers.ownerID.
func actorFrom(r *http.Request) (vault.ActorContext, bool) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return vault.ActorContext{}, false
	}
	return vault.ActorContext{UserID: u.ID, Role: u.Role}, true
}

// actorID returns just the authenticated user's id, or 0 when no user is
// present. For dual-write / audit paths where auth was already enforced
// upstream and a missing user degrades to the system (0) actor rather than a
// hard 401. Replaces the free actorUserID helper.
func actorID(r *http.Request) int64 {
	a, _ := actorFrom(r)
	return a.UserID
}
