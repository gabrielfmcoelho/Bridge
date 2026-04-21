package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type contactHandlers struct {
	db *database.DB
}

func (h *contactHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	contacts, err := models.ListContacts(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list contacts", err)
		return
	}
	jsonOK(w, contacts)
}

func (h *contactHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.Contact
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := models.CreateContact(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to create contact", err)
		return
	}
	jsonCreated(w, req)
}

func (h *contactHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	var req models.Contact
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	req.ID = id
	if err := models.UpdateContact(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to update contact", err)
		return
	}
	jsonOK(w, req)
}

func (h *contactHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	if err := models.DeleteContact(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete contact", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
