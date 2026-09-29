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

	type releaseWithIssues struct {
		models.Release
		IssueIDs []int64 `json:"issue_ids"`
	}
	result := make([]releaseWithIssues, len(releases))
	for i, rel := range releases {
		result[i] = releaseWithIssues{Release: rel, IssueIDs: issues[rel.ID]}
	}
	jsonPaged(w, r, result)
}

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

func (h *releaseHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Release
		IssueIDs []int64 `json:"issue_ids"`
	}
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
	var req struct {
		models.Release
		IssueIDs []int64 `json:"issue_ids"`
	}
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
