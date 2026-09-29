package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// projectEmbedHandlers manage the BIs / observability tools a project shows
// in iframes. Every route goes through the project: invisible → 404.
type projectEmbedHandlers struct {
	db *database.DB
}

// project resolves {id} to a project the caller can see, or writes the error.
func (h *projectEmbedHandlers) project(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return 0, false
	}
	// Get applies the entidade scope and skips deleted projects.
	if p, err := store.NewProjectRepo(h.db.SQL).Get(r.Context(), id); err != nil || p == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return 0, false
	}
	return id, true
}

// decode reads and validates an embed body. Only http(s) URLs: the iframe
// must not be pointed at javascript: or data: sources.
func (h *projectEmbedHandlers) decode(w http.ResponseWriter, r *http.Request, e *models.ProjectEmbed) bool {
	if err := decodeJSON(r, e); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return false
	}
	e.Title = strings.TrimSpace(e.Title)
	e.URL = strings.TrimSpace(e.URL)
	if e.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return false
	}
	if u, err := url.Parse(e.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		jsonError(w, http.StatusBadRequest, "url must be http(s)")
		return false
	}
	if e.Height <= 0 {
		e.Height = 600
	}
	return true
}

func (h *projectEmbedHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.project(w, r)
	if !ok {
		return
	}
	embeds, err := store.NewProjectEmbedRepo(h.db.SQL).List(r.Context(), projectID)
	if err != nil {
		jsonServerError(w, r, "failed to list embeds", err)
		return
	}
	jsonOK(w, embeds)
}

func (h *projectEmbedHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.project(w, r)
	if !ok {
		return
	}
	var e models.ProjectEmbed
	if !h.decode(w, r, &e) {
		return
	}
	e.ProjectID = projectID
	if err := store.NewProjectEmbedRepo(h.db.SQL).Create(r.Context(), &e); err != nil {
		jsonServerError(w, r, "failed to create embed", err)
		return
	}
	jsonCreated(w, e)
}

func (h *projectEmbedHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.project(w, r)
	if !ok {
		return
	}
	id, err := pathInt64(r, "embedId")
	if err != nil {
		jsonBadRequest(w, r, "invalid embed id", err)
		return
	}
	var e models.ProjectEmbed
	if !h.decode(w, r, &e) {
		return
	}
	e.ID, e.ProjectID = id, projectID
	found, err := store.NewProjectEmbedRepo(h.db.SQL).Update(r.Context(), &e)
	if err != nil {
		jsonServerError(w, r, "failed to update embed", err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "embed not found")
		return
	}
	jsonOK(w, e)
}

func (h *projectEmbedHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.project(w, r)
	if !ok {
		return
	}
	id, err := pathInt64(r, "embedId")
	if err != nil {
		jsonBadRequest(w, r, "invalid embed id", err)
		return
	}
	if err := store.NewProjectEmbedRepo(h.db.SQL).Delete(r.Context(), projectID, id); err != nil {
		jsonServerError(w, r, "failed to delete embed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *projectEmbedHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/projects/{id}/embeds", h.handleList)
	rr.role("editor", "POST /api/projects/{id}/embeds", h.handleCreate)
	rr.role("editor", "PUT /api/projects/{id}/embeds/{embedId}", h.handleUpdate)
	rr.role("editor", "DELETE /api/projects/{id}/embeds/{embedId}", h.handleDelete)
}
