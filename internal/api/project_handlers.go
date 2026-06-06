package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// projectHandlers is a Phase 2 (R2) handler: it holds a domain service (not a
// raw *database.DB), and each method is parse → call service → render. The
// enrichment/orchestration lives in service.ProjectService.
type projectHandlers struct {
	project *service.ProjectService
}

func (h *projectHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	projects, err := h.project.List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list projects", err)
		return
	}
	jsonOK(w, projects)
}

func (h *projectHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	detail, err := h.project.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "project lookup failed", err)
		return
	}
	if detail == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	jsonOK(w, detail)
}

// projectWriteRequest is the create/update wire shape: a Project plus its
// relations. Pointer slices distinguish "absent" (leave unchanged) from
// "present" (set) on update.
type projectWriteRequest struct {
	models.Project
	Tags         *[]string                  `json:"tags"`
	Responsaveis *[]models.ResponsavelInput `json:"responsaveis"`
}

func (req *projectWriteRequest) toWrite() *service.ProjectWrite {
	return &service.ProjectWrite{Project: req.Project, Tags: req.Tags, Responsaveis: req.Responsaveis}
}

func (h *projectHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req projectWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return
	}
	wr := req.toWrite()
	if err := h.project.Create(r.Context(), wr); err != nil {
		jsonServerError(w, r, "failed to create project", err)
		return
	}
	jsonCreated(w, wr.Project)
}

func (h *projectHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req projectWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	wr := req.toWrite()
	found, err := h.project.Update(r.Context(), id, wr)
	if err != nil {
		jsonServerError(w, r, "failed to update project", err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	jsonOK(w, wr.Project)
}

func (h *projectHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	// Cascade-soft-delete any vault entries scoped to this project, then
	// hard-delete the project row — atomically, via the store cascade registry.
	actor, _ := actorFrom(r)
	if err := h.project.Delete(r.Context(), actor, id); err != nil {
		jsonServerError(w, r, "failed to delete project", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *projectHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/projects", h.handleList)
	rr.auth("GET /api/projects/trash", h.handleListTrash)
	rr.role("admin", "POST /api/projects/{id}/restore", h.handleRestore)
	rr.role("editor", "POST /api/projects", h.handleCreate)
	rr.auth("GET /api/projects/{id}", h.handleGet)
	rr.role("editor", "PUT /api/projects/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/projects/{id}", h.handleDelete)
}

func (h *projectHandlers) handleListTrash(w http.ResponseWriter, r *http.Request) {
	items, err := h.project.ListTrash(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list project trash", err)
		return
	}
	if items == nil {
		items = []models.Project{}
	}
	jsonOK(w, items)
}

func (h *projectHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actor, _ := actorFrom(r)
	if err := h.project.Restore(r.Context(), actor, id); err != nil {
		jsonServerError(w, r, "failed to restore project", err)
		return
	}
	jsonOK(w, map[string]string{"status": "restored"})
}
