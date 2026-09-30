package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type releaseHandlers struct {
	db *database.DB
}

// projectVisible rejects a release without a project (releases live inside
// one) and 404s when the project is outside the caller's entidade scope.
func (h *releaseHandlers) projectVisible(w http.ResponseWriter, r *http.Request, projectID *int64) bool {
	if projectID == nil || *projectID == 0 {
		jsonError(w, http.StatusBadRequest, "project_id is required")
		return false
	}
	if p, err := store.NewProjectRepo(h.db.SQL).Get(r.Context(), *projectID); err != nil || p == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return false
	}
	return true
}

// releaseWithIssues is a release list row with its linked issue IDs.
type releaseWithIssues struct {
	models.Release
	IssueIDs []int64 `json:"issue_ids"`
}

// handleList godoc
//
//	@Summary		List releases
//	@Description	Releases with their linked issue IDs, optionally for one project. Any role.
//	@Tags			releases
//	@Produce		json
//	@Param			project_id	query		int	false	"Restrict to one project"
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[releaseWithIssues]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/releases [get]
func (h *releaseHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			jsonBadRequest(w, r, "invalid project_id", err)
			return
		}
		projectID = id
	}
	repo := store.NewReleaseRepo(h.db.SQL)
	releases, err := repo.List(r.Context(), projectID)
	if err != nil {
		jsonServerError(w, r, "failed to list releases", err)
		return
	}
	ids := make([]int64, len(releases))
	for i, rel := range releases {
		ids[i] = rel.ID
	}
	issues, err := repo.IssueIDsByRelease(r.Context(), ids)
	if err != nil {
		jsonServerError(w, r, "failed to list releases", err)
		return
	}

	result := make([]releaseWithIssues, len(releases))
	for i, rel := range releases {
		result[i] = releaseWithIssues{Release: rel, IssueIDs: issues[rel.ID]}
	}
	jsonPaged(w, r, result)
}

// handleGet godoc
//
//	@Summary		Get a release
//	@Description	Any role. Body is {"release": models.Release, "issue_ids": [int]}.
//	@Tags			releases
//	@Produce		json
//	@Param			id	path		int	true	"Release ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/releases/{id} [get]
func (h *releaseHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	rel, err := store.NewReleaseRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil || rel == nil {
		jsonError(w, http.StatusNotFound, "release not found")
		return
	}

	issueIDs, _ := store.NewReleaseRepo(h.db.SQL).IssueIDs(r.Context(), id)
	jsonOK(w, map[string]any{
		"release":   rel,
		"issue_ids": issueIDs,
	})
}

// releaseUpsertRequest is the create/update body: the release plus the issue
// IDs it ships (omit issue_ids on update to keep them).
type releaseUpsertRequest struct {
	models.Release
	IssueIDs []int64 `json:"issue_ids"`
}

// handleCreate godoc
//
//	@Summary		Create a release
//	@Description	Editor+. project_id is required and must be visible to the caller; status defaults to "pending".
//	@Tags			releases
//	@Accept			json
//	@Produce		json
//	@Param			body	body		releaseUpsertRequest	true	"Release and the issue IDs it ships"
//	@Success		201		{object}	models.Release
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/releases [post]
func (h *releaseHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req releaseUpsertRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Status == "" {
		req.Status = "pending"
	}
	if !h.projectVisible(w, r, req.ProjectID) {
		return
	}

	if err := store.NewReleaseRepo(h.db.SQL).Create(r.Context(), &req.Release); err != nil {
		jsonServerError(w, r, "failed to create release", err)
		return
	}

	if len(req.IssueIDs) > 0 {
		store.NewReleaseRepo(h.db.SQL).SetIssues(r.Context(), req.Release.ID, req.IssueIDs)
	}

	jsonCreated(w, req.Release)
}

// handleUpdate godoc
//
//	@Summary		Update a release
//	@Description	Editor+. Omitted fields keep their stored value; omit issue_ids to keep the links. Moving to status "live" stamps live_date.
//	@Tags			releases
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"Release ID"
//	@Param			body	body		releaseUpsertRequest	true	"Release fields to change"
//	@Success		200		{object}	models.Release
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/releases/{id} [put]
func (h *releaseHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := store.NewReleaseRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "release not found")
		return
	}

	// Decode onto the stored row, so fields the client omits are kept.
	var req releaseUpsertRequest
	req.Release = *existing
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.Release.ID = id
	if !h.projectVisible(w, r, req.ProjectID) {
		return
	}

	// Auto-set live_date when status transitions to "live"
	if req.Status == "live" && existing.Status != "live" {
		req.LiveDate = time.Now().Format("2006-01-02")
	}

	if err := store.NewReleaseRepo(h.db.SQL).Update(r.Context(), &req.Release); err != nil {
		jsonServerError(w, r, "failed to update release", err)
		return
	}

	if req.IssueIDs != nil {
		store.NewReleaseRepo(h.db.SQL).SetIssues(r.Context(), id, req.IssueIDs)
	}

	jsonOK(w, req.Release)
}

// handleDelete godoc
//
//	@Summary		Delete a release
//	@Description	Admin.
//	@Tags			releases
//	@Produce		json
//	@Param			id	path		int	true	"Release ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/releases/{id} [delete]
func (h *releaseHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	if err := store.NewReleaseRepo(h.db.SQL).Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete release", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *releaseHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/releases", h.handleList)
	rr.role("editor", "POST /api/releases", h.handleCreate)
	rr.auth("GET /api/releases/{id}", h.handleGet)
	rr.role("editor", "PUT /api/releases/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/releases/{id}", h.handleDelete)
}
