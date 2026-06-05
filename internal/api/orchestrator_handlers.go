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

func (h *orchestratorHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	orchs, err := h.repo().List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list orchestrators", err)
		return
	}
	jsonOK(w, orchs)
}

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
