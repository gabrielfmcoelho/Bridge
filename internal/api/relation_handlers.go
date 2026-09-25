package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// relationHandlers serves the links between inventory entities that list
// pages group by (hosts by service, DNS by project, ...).
type relationHandlers struct {
	relations *store.RelationRepo
}

func (h *relationHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	rels, err := h.relations.All(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list relations", err)
		return
	}
	jsonPaged(w, r, rels)
}

func (h *relationHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/relations", h.handleList)
}
