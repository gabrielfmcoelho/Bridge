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

// handleList godoc
//
//	@Summary		List entity relations
//	@Description	Any role. Every link between visible inventory entities (host-service, dns-project, ...), used by list pages to group rows.
//	@Tags			relations
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[store.Relation]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/relations [get]
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
