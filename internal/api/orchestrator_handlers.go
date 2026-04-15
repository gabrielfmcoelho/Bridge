package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type orchestratorHandlers struct {
	db *database.DB
}

func (h *orchestratorHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	orchs, err := models.ListOrchestrators(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list orchestrators")
		return
	}
	jsonOK(w, orchs)
}

func (h *orchestratorHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.Orchestrator
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.HostID == 0 {
		jsonError(w, http.StatusBadRequest, "host_id is required")
		return
	}

	if err := models.CreateOrchestrator(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusConflict, "orchestrator already exists for this host")
		return
	}
	jsonCreated(w, req)
}

func (h *orchestratorHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	existing, err := models.GetOrchestrator(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "orchestrator not found")
		return
	}

	var req models.Orchestrator
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ID = id
	if err := models.UpdateOrchestrator(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update orchestrator")
		return
	}
	jsonOK(w, req)
}

func (h *orchestratorHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := models.DeleteOrchestrator(h.db.SQL, id); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete orchestrator")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
