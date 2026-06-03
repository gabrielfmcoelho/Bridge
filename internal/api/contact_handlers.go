package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// contactHandlers is the reference for the post-Phase-1 handler shape: it holds
// a repository (not a raw *database.DB) and stays thin — parse, call repo,
// render. The repo is injected at construction time (router.go), which the
// Phase 2 DI container will centralize.
type contactHandlers struct {
	contacts *store.ContactRepo
}

func (h *contactHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	contacts, err := h.contacts.List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list contacts", err)
		return
	}
	jsonOK(w, contacts)
}

func (h *contactHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.Contact
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return
	}
	if err := h.contacts.Create(r.Context(), &req); err != nil {
		jsonServerError(w, r, "failed to create contact", err)
		return
	}
	jsonCreated(w, req)
}

func (h *contactHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req models.Contact
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return
	}
	req.ID = id
	if err := h.contacts.Update(r.Context(), &req); err != nil {
		jsonServerError(w, r, "failed to update contact", err)
		return
	}
	jsonOK(w, req)
}

func (h *contactHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.contacts.Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete contact", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
