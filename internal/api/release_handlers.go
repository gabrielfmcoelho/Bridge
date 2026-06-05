package api

import (
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type releaseHandlers struct {
	db *database.DB
}

func (h *releaseHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	releases, err := store.NewReleaseRepo(h.db.SQL).List(r.Context())
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
		issueIDs, _ := store.NewReleaseRepo(h.db.SQL).IssueIDs(r.Context(), rel.ID)
		result[i] = releaseWithIssues{Release: rel, IssueIDs: issueIDs}
	}
	jsonOK(w, result)
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

	var req struct {
		models.Release
		IssueIDs []int64 `json:"issue_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.Release.ID = id

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
	rr.public("GET /api/releases", h.handleList)
	rr.role("editor", "POST /api/releases", h.handleCreate)
	rr.public("GET /api/releases/{id}", h.handleGet)
	rr.role("editor", "PUT /api/releases/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/releases/{id}", h.handleDelete)
}
