package api

import (
	"errors"
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
	usage, err := h.contacts.UsageCounts(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to count contact usage", err)
		return
	}
	for i := range contacts {
		contacts[i].Usage = usage[contacts[i].ID]
	}
	jsonPaged(w, r, contacts)
}

// handleUsage lists the assets a contact is responsável for (visible ones).
func (h *contactHandlers) handleUsage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if c, err := h.contacts.Get(r.Context(), id); err != nil || c == nil {
		jsonError(w, http.StatusNotFound, "contact not found")
		return
	}
	uses, err := h.contacts.Usage(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load contact usage", err)
		return
	}
	jsonOK(w, uses)
}

func (h *contactHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Contact
		models.AssetGrantsInput
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return
	}
	g, ok := resolveGrants(w, r, req.AssetGrantsInput, nil)
	if !ok {
		return
	}
	if err := h.contacts.Create(r.Context(), &req.Contact); err != nil {
		// Never touch the existing contact (its fields or its entidades).
		var dup store.ErrContactExists
		if errors.As(err, &dup) {
			code := "contact_exists"
			if dup.Trashed {
				code = "contact_in_trash"
			}
			jsonError(w, http.StatusConflict, code)
			return
		}
		jsonServerError(w, r, "failed to create contact", err)
		return
	}
	if err := store.NewAssetEntidadeRepo(h.contacts.DB()).Replace(r.Context(), h.contacts.DB(), store.AssetContact, req.Contact.ID, g); err != nil {
		jsonServerError(w, r, "failed to set entidades", err)
		return
	}
	jsonCreated(w, req.Contact)
}

func (h *contactHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if c, err := h.contacts.Get(r.Context(), id); err != nil || c == nil {
		jsonError(w, http.StatusNotFound, "contact not found")
		return
	}
	var req struct {
		models.Contact
		models.AssetGrantsInput
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return
	}
	req.Contact.ID = id
	if err := h.contacts.Update(r.Context(), &req.Contact); err != nil {
		jsonServerError(w, r, "failed to update contact", err)
		return
	}
	if req.AssetGrantsInput.Present() {
		grants := store.NewAssetEntidadeRepo(h.contacts.DB())
		existing, _ := grants.Get(r.Context(), store.AssetContact, id)
		g, ok := resolveGrants(w, r, req.AssetGrantsInput, &existing)
		if !ok {
			return
		}
		if err := grants.Replace(r.Context(), h.contacts.DB(), store.AssetContact, id, g); err != nil {
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
	}
	jsonOK(w, req.Contact)
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

// registerRoutes wires this group's routes (self-registration, R2).
func (h *contactHandlers) handleListTrash(w http.ResponseWriter, r *http.Request) {
	items, err := h.contacts.ListTrash(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list contact trash", err)
		return
	}
	jsonPaged(w, r, items) // list envelope, like every list endpoint
}

func (h *contactHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	restored, err := h.contacts.Restore(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to restore contact", err)
		return
	}
	if !restored {
		jsonError(w, http.StatusNotFound, "contact not in trash")
		return
	}
	jsonOK(w, map[string]string{"status": "restored"})
}

func (h *contactHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/contacts", h.handleList)
	rr.auth("GET /api/contacts/trash", h.handleListTrash)
	rr.auth("GET /api/contacts/{id}/usage", h.handleUsage)
	rr.role("admin", "POST /api/contacts/{id}/restore", h.handleRestore)
	rr.role("editor", "POST /api/contacts", h.handleCreate)
	rr.role("editor", "PUT /api/contacts/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/contacts/{id}", h.handleDelete)
}
