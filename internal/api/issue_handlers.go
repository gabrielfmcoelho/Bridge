package api

import (
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
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

	f := models.IssueFilter{ProjectID: projectID}
	if s := r.URL.Query().Get("service_id"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			f.ServiceID = v
		}
	}
	f.VisibleSQL, f.VisibleArgs = store.VisibleExprDyn(r.Context(), "entity_type", "entity_id")

	issues, err := models.ListIssues(h.db.SQL, f)
	if err != nil {
		jsonServerError(w, r, "failed to list issues", err)
		return
	}

	jsonPaged(w, r, issues)
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
	// Same defaulting CreateIssue applies, done here so the parent check
	// sees the resolved (entity_type, entity_id).
	if req.EntityType == "" {
		req.EntityType, req.EntityID = "project", projectID
	}
	if ok, err := store.CanSee(r.Context(), h.db.SQL, store.AssetType(req.EntityType), req.EntityID); err != nil || !ok {
		jsonError(w, http.StatusNotFound, "parent not found")
		return
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

	existing := loadVisibleIssue(r, h.db.SQL, issueID)
	if existing == nil {
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
	if loadVisibleIssue(r, h.db.SQL, issueID) == nil {
		jsonError(w, http.StatusNotFound, "issue not found")
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
	if loadVisibleIssue(r, h.db.SQL, issueID) == nil {
		jsonError(w, http.StatusNotFound, "issue not found")
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

	f := models.IssueFilter{ServiceID: serviceID}
	f.VisibleSQL, f.VisibleArgs = store.VisibleExprDyn(r.Context(), "entity_type", "entity_id")
	issues, err := models.ListIssues(h.db.SQL, f)
	if err != nil {
		jsonServerError(w, r, "failed to list issues", err)
		return
	}

	jsonPaged(w, r, issues)
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *issueHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/projects/{id}/issues", h.handleList)
	rr.role("editor", "POST /api/projects/{id}/issues", h.handleCreate)
	rr.role("editor", "PUT /api/projects/{id}/issues/{issueId}", h.handleUpdate)
	rr.role("editor", "PATCH /api/projects/{id}/issues/{issueId}/move", h.handleMove)
	rr.role("admin", "DELETE /api/projects/{id}/issues/{issueId}", h.handleDelete)
	rr.auth("GET /api/services/{id}/issues", h.handleListByService)
}
