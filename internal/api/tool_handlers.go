package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type toolHandlers struct {
	db *database.DB
}

func (h *toolHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	tools, err := models.ListExternalTools(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list tools")
		return
	}
	if tools == nil {
		tools = []models.ExternalTool{}
	}
	jsonOK(w, tools)
}

func (h *toolHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	tool, err := models.GetExternalTool(h.db.SQL, id)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	jsonOK(w, tool)
}

func (h *toolHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.ExternalTool
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := models.CreateExternalTool(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create tool")
		return
	}
	jsonCreated(w, req)
}

func (h *toolHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	existing, err := models.GetExternalTool(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}

	var req models.ExternalTool
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ID = id
	if err := models.UpdateExternalTool(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update tool")
		return
	}
	jsonOK(w, req)
}

func (h *toolHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := models.DeleteExternalTool(h.db.SQL, id); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete tool")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
