package api

import (
	"errors"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// resolveGrants applies store.ResolveGrants to a request's AssetGrantsInput
// and writes the right error when the caller's scope rejects it: 403 when the
// chosen creator entidade is outside the caller's scope, 400 when a non-admin
// would produce an asset invisible to themselves. ok=false means the response
// was already written. existing is nil on create.
func resolveGrants(w http.ResponseWriter, r *http.Request, in models.AssetGrantsInput, existing *models.AssetGrants) (models.AssetGrants, bool) {
	g, err := store.ResolveGrants(r.Context(), in, existing)
	if errors.Is(err, store.ErrEntidadeForbidden) {
		jsonError(w, http.StatusForbidden, err.Error())
		return g, false
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return g, false
	}
	return g, true
}
