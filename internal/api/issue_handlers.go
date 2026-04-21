package api

import (
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type issueHandlers struct {
	db *database.DB
}

func (h *issueHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	var serviceID *int64
	if s := r.URL.Query().Get("service_id"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			serviceID = &v
		}
	}

	issues, err := models.ListIssuesByProject(h.db.SQL, projectID, serviceID)
	if err != nil {
		jsonServerError(w, r, "failed to list issues", err)
		return
	}

	jsonOK(w, issues)
}

func (h *issueHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	user := auth.UserFromContext(r.Context())

	var req models.Issue
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}

	req.ProjectID = &projectID
	req.CreatedBy = user.ID
	if req.Status == "" {
		req.Status = "backlog"
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}

	if err := models.CreateIssue(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to create issue", err)
		return
	}

	jsonCreated(w, req)
}

func (h *issueHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "issueId")
	if err != nil {
		jsonBadRequest(w, r, "invalid issue id", err)
		return
	}

	existing, err := models.GetIssue(h.db.SQL, issueID)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "issue not found")
		return
	}

	var req models.Issue
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.ID = issueID
	req.ProjectID = existing.ProjectID
	req.CreatedBy = existing.CreatedBy
	if req.Status == "" {
		req.Status = existing.Status
	}
	if req.Priority == "" {
		req.Priority = existing.Priority
	}

	if err := models.UpdateIssue(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to update issue", err)
		return
	}

	jsonOK(w, req)
}

func (h *issueHandlers) handleMove(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "issueId")
	if err != nil {
		jsonBadRequest(w, r, "invalid issue id", err)
		return
	}

	var req struct {
		Status   string  `json:"status"`
		Position float64 `json:"position"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Status == "" {
		jsonError(w, http.StatusBadRequest, "status is required")
		return
	}

	if err := models.MoveIssue(h.db.SQL, issueID, req.Status, req.Position); err != nil {
		jsonServerError(w, r, "failed to move issue", err)
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *issueHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "issueId")
	if err != nil {
		jsonBadRequest(w, r, "invalid issue id", err)
		return
	}

	if err := models.DeleteIssue(h.db.SQL, issueID); err != nil {
		jsonServerError(w, r, "failed to delete issue", err)
		return
	}

	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *issueHandlers) handleListByService(w http.ResponseWriter, r *http.Request) {
	serviceID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid service id", err)
		return
	}

	issues, err := models.ListIssuesByService(h.db.SQL, serviceID)
	if err != nil {
		jsonServerError(w, r, "failed to list issues", err)
		return
	}

	jsonOK(w, issues)
}
