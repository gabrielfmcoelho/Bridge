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

// handleList godoc
//
//	@Summary		List projects
//	@Description	Any role. Visible projects, enriched with their relations. Paginated in SQL.
//	@Tags			projects
//	@Produce		json
//	@Param			search		query		string	false	"Free-text search"
//	@Param			situacao	query		string	false	"Filter by situação"
//	@Param			tag			query		string	false	"Filter by tag"
//	@Param			sort_by		query		string	false	"Sort column"
//	@Param			sort_dir	query		string	false	"Sort direction (asc, desc)"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[service.ProjectListItem]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/projects [get]
func (h *projectHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	pp := parsePageParams(r)
	f := models.ProjectFilter{
		Search:   r.URL.Query().Get("search"),
		Situacao: r.URL.Query().Get("situacao"),
		Tag:      r.URL.Query().Get("tag"),
		SortBy:   r.URL.Query().Get("sort_by"),
		SortDir:  r.URL.Query().Get("sort_dir"),
		Page:     pp.Page,
		PerPage:  pp.PerPage,
	}
	items, err := h.project.List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list projects", err)
		return
	}
	// Real server-side pagination (R4 envelope). When per_page is set we report
	// the matching Count; an unbounded request reports the returned length.
	total := len(items)
	if !pp.Unbounded() {
		if n, err := h.project.Count(r.Context(), f); err == nil {
			total = n
		}
	}
	jsonList(w, items, metaFor(pp, total))
}

// handleGet godoc
//
//	@Summary		Get a project
//	@Description	Any role. The project with its relations. Invisible projects answer 404.
//	@Tags			projects
//	@Produce		json
//	@Param			id	path		int	true	"Project ID"
//	@Success		200	{object}	service.ProjectDetail
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/projects/{id} [get]
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
	models.AssetGrantsInput
	Tags         *[]string                  `json:"tags"`
	Responsaveis *[]models.ResponsavelInput `json:"responsaveis"`
	// Project-side links: services join the project (services.project_id);
	// hosts and DNS are direct links. Absent = leave unchanged.
	ServiceIDs *[]int64 `json:"service_ids"`
	HostIDs    *[]int64 `json:"host_ids"`
	DNSIDs     *[]int64 `json:"dns_ids"`
}

func (req *projectWriteRequest) toWrite() *service.ProjectWrite {
	return &service.ProjectWrite{Project: req.Project, Tags: req.Tags, Responsaveis: req.Responsaveis,
		ServiceIDs: req.ServiceIDs, DirectHostIDs: req.HostIDs, DirectDNSIDs: req.DNSIDs}
}

// handleCreate godoc
//
//	@Summary		Create a project
//	@Description	Editor+. name is required.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Param			body	body		projectWriteRequest	true	"Project, links and entidade grants"
//	@Success		201		{object}	models.Project
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/projects [post]
func (h *projectHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req projectWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return
	}
	wr := req.toWrite()
	grants, ok := resolveGrants(w, r, req.AssetGrantsInput, nil)
	if !ok {
		return
	}
	wr.Grants = &grants
	if err := h.project.Create(r.Context(), wr); err != nil {
		jsonServerError(w, r, "failed to create project", err)
		return
	}
	jsonCreated(w, wr.Project)
}

// handleUpdate godoc
//
//	@Summary		Update a project
//	@Description	Editor+. Partial: decoded over the stored project; links change only when sent, grants only when a grant field is sent.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Project ID"
//	@Param			body	body		projectWriteRequest	true	"Project fields, links and optional entidade grants"
//	@Success		200		{object}	models.Project
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/projects/{id} [put]
func (h *projectHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	// Decode onto the stored project, as hosts and services do: a field the
	// payload omits (gitlab_url, outline/GLPI ids…) keeps its value.
	current, err := h.project.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load project", err)
		return
	}
	if current == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	req := projectWriteRequest{Project: *current.Project}
	if !decodeBody(w, r, &req) {
		return
	}
	wr := req.toWrite()
	if req.AssetGrantsInput.Present() {
		existing, _ := h.project.Grants(r.Context(), id)
		grants, ok := resolveGrants(w, r, req.AssetGrantsInput, &existing)
		if !ok {
			return
		}
		wr.Grants = &grants
	}
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

// handleDelete godoc
//
//	@Summary		Delete a project
//	@Description	Admin. Soft-deletes the vault entries scoped to it.
//	@Tags			projects
//	@Produce		json
//	@Param			id	path		int	true	"Project ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/projects/{id} [delete]
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

// handleListTrash godoc
//
//	@Summary		List trashed projects
//	@Description	Any role.
//	@Tags			projects
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.Project]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/projects/trash [get]
func (h *projectHandlers) handleListTrash(w http.ResponseWriter, r *http.Request) {
	items, err := h.project.ListTrash(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list project trash", err)
		return
	}
	jsonPaged(w, r, items)
}

// handleRestore godoc
//
//	@Summary		Restore a project from the trash
//	@Description	Admin.
//	@Tags			projects
//	@Produce		json
//	@Param			id	path		int	true	"Project ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/projects/{id}/restore [post]
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
