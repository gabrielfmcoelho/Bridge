package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type hostChamadoHandlers struct {
	db *database.DB
}

func (h *hostChamadoHandlers) resolveHost(w http.ResponseWriter, r *http.Request) *models.Host {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return nil
	}
	return host
}

// handleList godoc
//
//	@Summary		List a host's chamados
//	@Description	Any role. Invisible hosts answer 404.
//	@Tags			host-chamados
//	@Produce		json
//	@Param			slug		path		string	true	"Host oficial slug"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.HostChamado]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/chamados [get]
func (h *hostChamadoHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	chamados, err := store.NewHostChamadoRepo(h.db.SQL).ListByHost(r.Context(), host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to list chamados", err)
		return
	}

	jsonPaged(w, r, chamados)
}

// handleCreate godoc
//
//	@Summary		Create a host chamado
//	@Description	Editor+. Answers the created chamado (or just {"id": N} if it cannot be re-read).
//	@Tags			host-chamados
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string					true	"Host oficial slug"
//	@Param			body	body		models.HostChamadoInput	true	"Chamado"
//	@Success		201		{object}	models.HostChamado
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/chamados [post]
func (h *hostChamadoHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	var req models.HostChamadoInput
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	id, err := store.NewHostChamadoRepo(h.db.SQL).Create(r.Context(), host.ID, &req)
	if err != nil {
		jsonServerError(w, r, "failed to create chamado", err)
		return
	}

	chamado, err := store.NewHostChamadoRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil || chamado == nil {
		jsonCreated(w, map[string]int64{"id": id})
		return
	}

	jsonCreated(w, chamado)
}

// handleUpdate godoc
//
//	@Summary		Update a host chamado
//	@Description	Editor+.
//	@Tags			host-chamados
//	@Accept			json
//	@Produce		json
//	@Param			slug		path		string					true	"Host oficial slug"
//	@Param			chamadoId	path		int						true	"Chamado ID"
//	@Param			body		body		models.HostChamadoInput	true	"Chamado"
//	@Success		200			{object}	models.HostChamado
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/chamados/{chamadoId} [put]
func (h *hostChamadoHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	chamadoID, err := pathInt64(r, "chamadoId")
	if err != nil {
		jsonBadRequest(w, r, "invalid chamado id", err)
		return
	}

	existing, err := store.NewHostChamadoRepo(h.db.SQL).Get(r.Context(), chamadoID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "chamado not found")
		return
	}

	var req models.HostChamadoInput
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	if err := store.NewHostChamadoRepo(h.db.SQL).Update(r.Context(), chamadoID, &req); err != nil {
		jsonServerError(w, r, "failed to update chamado", err)
		return
	}

	updated, _ := store.NewHostChamadoRepo(h.db.SQL).Get(r.Context(), chamadoID)
	jsonOK(w, updated)
}

// handleDelete godoc
//
//	@Summary		Delete a host chamado
//	@Description	Admin.
//	@Tags			host-chamados
//	@Produce		json
//	@Param			slug		path		string	true	"Host oficial slug"
//	@Param			chamadoId	path		int		true	"Chamado ID"
//	@Success		200			{object}	StatusResponse
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/chamados/{chamadoId} [delete]
func (h *hostChamadoHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	chamadoID, err := pathInt64(r, "chamadoId")
	if err != nil {
		jsonBadRequest(w, r, "invalid chamado id", err)
		return
	}

	existing, err := store.NewHostChamadoRepo(h.db.SQL).Get(r.Context(), chamadoID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "chamado not found")
		return
	}

	if err := store.NewHostChamadoRepo(h.db.SQL).Delete(r.Context(), chamadoID); err != nil {
		jsonServerError(w, r, "failed to delete chamado", err)
		return
	}

	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *hostChamadoHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/hosts/{slug}/chamados", h.handleList)
	rr.role("editor", "POST /api/hosts/{slug}/chamados", h.handleCreate)
	rr.role("editor", "PUT /api/hosts/{slug}/chamados/{chamadoId}", h.handleUpdate)
	rr.role("admin", "DELETE /api/hosts/{slug}/chamados/{chamadoId}", h.handleDelete)
}
