package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type hostChamadoHandlers struct {
	db *database.DB
}

func (h *hostChamadoHandlers) resolveHost(w http.ResponseWriter, r *http.Request) *models.Host {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return nil
	}
	return host
}

func (h *hostChamadoHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	chamados, err := models.ListHostChamados(h.db.SQL, host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to list chamados", err)
		return
	}

	jsonOK(w, chamados)
}

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

	id, err := models.CreateHostChamado(h.db.SQL, host.ID, &req)
	if err != nil {
		jsonServerError(w, r, "failed to create chamado", err)
		return
	}

	chamado, err := models.GetHostChamado(h.db.SQL, id)
	if err != nil || chamado == nil {
		jsonCreated(w, map[string]int64{"id": id})
		return
	}

	jsonCreated(w, chamado)
}

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

	existing, err := models.GetHostChamado(h.db.SQL, chamadoID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "chamado not found")
		return
	}

	var req models.HostChamadoInput
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	if err := models.UpdateHostChamado(h.db.SQL, chamadoID, &req); err != nil {
		jsonServerError(w, r, "failed to update chamado", err)
		return
	}

	updated, _ := models.GetHostChamado(h.db.SQL, chamadoID)
	jsonOK(w, updated)
}

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

	existing, err := models.GetHostChamado(h.db.SQL, chamadoID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "chamado not found")
		return
	}

	if err := models.DeleteHostChamado(h.db.SQL, chamadoID); err != nil {
		jsonServerError(w, r, "failed to delete chamado", err)
		return
	}

	jsonOK(w, map[string]string{"status": "deleted"})
}
