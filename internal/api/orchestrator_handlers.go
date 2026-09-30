package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type orchestratorHandlers struct {
	db *database.DB
}

// repo builds the OrchestratorRepo on demand. Inline construction is the
// transitional form used during the repository migration; the Phase 2 DI
// container will hoist this into a shared instance.
func (h *orchestratorHandlers) repo() *store.OrchestratorRepo {
	return store.NewOrchestratorRepo(h.db.SQL)
}

// handleList godoc
//
//	@Summary		List orchestrators
//	@Description	Any role. Only orchestrators on visible hosts.
//	@Tags			orchestrators
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.Orchestrator]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/orchestrators [get]
func (h *orchestratorHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	orchs, err := h.repo().List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list orchestrators", err)
		return
	}
	jsonPaged(w, r, orchs)
}

// handleCreate godoc
//
//	@Summary		Create an orchestrator
//	@Description	Editor+. host_id is required; a host that already has one answers 409.
//	@Tags			orchestrators
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.Orchestrator	true	"Orchestrator"
//	@Success		201		{object}	models.Orchestrator
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/orchestrators [post]
func (h *orchestratorHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.Orchestrator
	if !decodeBody(w, r, &req) {
		return
	}
	if req.HostID == 0 {
		jsonError(w, http.StatusBadRequest, "host_id is required")
		return
	}
	if err := h.repo().Create(r.Context(), &req); err != nil {
		jsonError(w, http.StatusConflict, "orchestrator already exists for this host")
		return
	}
	jsonCreated(w, req)
}

// handleUpdate godoc
//
//	@Summary		Update an orchestrator
//	@Description	Editor+.
//	@Tags			orchestrators
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Orchestrator ID"
//	@Param			body	body		models.Orchestrator	true	"Orchestrator"
//	@Success		200		{object}	models.Orchestrator
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/orchestrators/{id} [put]
func (h *orchestratorHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	existing, err := h.repo().Get(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "orchestrator not found")
		return
	}
	var req models.Orchestrator
	if !decodeBody(w, r, &req) {
		return
	}
	req.ID = id
	if err := h.repo().Update(r.Context(), &req); err != nil {
		jsonServerError(w, r, "failed to update orchestrator", err)
		return
	}
	jsonOK(w, req)
}

// handleDelete godoc
//
//	@Summary		Delete an orchestrator
//	@Description	Admin.
//	@Tags			orchestrators
//	@Produce		json
//	@Param			id	path		int	true	"Orchestrator ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/orchestrators/{id} [delete]
func (h *orchestratorHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.repo().Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete orchestrator", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *orchestratorHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/orchestrators", h.handleList)
	rr.role("editor", "POST /api/orchestrators", h.handleCreate)
	rr.role("editor", "PUT /api/orchestrators/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/orchestrators/{id}", h.handleDelete)
}
