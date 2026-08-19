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
	jsonPaged(w, r, contacts)
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
	g, err := store.ResolveGrants(r.Context(), req.AssetGrantsInput, nil)
	if errors.Is(err, store.ErrEntidadeForbidden) {
		jsonError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.contacts.Create(r.Context(), &req.Contact); err != nil {
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
		g, err := store.ResolveGrants(r.Context(), req.AssetGrantsInput, &existing)
		if errors.Is(err, store.ErrEntidadeForbidden) {
			jsonError(w, http.StatusForbidden, err.Error())
			return
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
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
func (h *contactHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/contacts", h.handleList)
	rr.role("editor", "POST /api/contacts", h.handleCreate)
	rr.role("editor", "PUT /api/contacts/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/contacts/{id}", h.handleDelete)
}
