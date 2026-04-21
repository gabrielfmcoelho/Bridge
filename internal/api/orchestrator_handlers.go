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
		jsonServerError(w, r, "failed to list orchestrators", err)
		return
	}
	jsonOK(w, orchs)
}

func (h *orchestratorHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.Orchestrator
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
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
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := models.GetOrchestrator(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "orchestrator not found")
		return
	}

	var req models.Orchestrator
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.ID = id
	if err := models.UpdateOrchestrator(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to update orchestrator", err)
		return
	}
	jsonOK(w, req)
}

func (h *orchestratorHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	if err := models.DeleteOrchestrator(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete orchestrator", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
