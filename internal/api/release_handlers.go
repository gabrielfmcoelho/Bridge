package api

import (
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type releaseHandlers struct {
	db *database.DB
}

func (h *releaseHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	releases, err := models.ListReleases(h.db.SQL)
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
		issueIDs, _ := models.GetReleaseIssueIDs(h.db.SQL, rel.ID)
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

	rel, err := models.GetRelease(h.db.SQL, id)
	if err != nil || rel == nil {
		jsonError(w, http.StatusNotFound, "release not found")
		return
	}

	issueIDs, _ := models.GetReleaseIssueIDs(h.db.SQL, id)
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

	if err := models.CreateRelease(h.db.SQL, &req.Release); err != nil {
		jsonServerError(w, r, "failed to create release", err)
		return
	}

	if len(req.IssueIDs) > 0 {
		models.SetReleaseIssues(h.db.SQL, req.Release.ID, req.IssueIDs)
	}

	jsonCreated(w, req.Release)
}

func (h *releaseHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := models.GetRelease(h.db.SQL, id)
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

	if err := models.UpdateRelease(h.db.SQL, &req.Release); err != nil {
		jsonServerError(w, r, "failed to update release", err)
		return
	}

	if req.IssueIDs != nil {
		models.SetReleaseIssues(h.db.SQL, id, req.IssueIDs)
	}

	jsonOK(w, req.Release)
}

func (h *releaseHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	if err := models.DeleteRelease(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete release", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
