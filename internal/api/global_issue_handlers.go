package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type globalIssueHandlers struct {
	db *database.DB
}

func (h *globalIssueHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := models.IssueFilter{
		EntityType:      q.Get("entity_type"),
		Status:          q.Get("status"),
		Priority:        q.Get("priority"),
		Search:          q.Get("search"),
		ExcludeArchived: q.Get("exclude_archived") == "true",
	}
	if v := q.Get("entity_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.EntityID = n
		}
	}
	if v := q.Get("assignee_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.AssigneeID = n
		}
	}

	issues, err := models.ListIssues(h.db.SQL, f)
	if err != nil {
		jsonServerError(w, r, "failed to list issues", err)
		return
	}

	// Enrich with assignee IDs and alert IDs
	assigneeMap, _ := models.GetIssueAssigneesBulk(h.db.SQL)
	alertMap, _ := models.GetIssueAlertsBulk(h.db.SQL)

	type issueWithLinks struct {
		models.Issue
		AssigneeIDs []int64 `json:"assignee_ids"`
		AlertIDs    []int64 `json:"alert_ids"`
	}
	result := make([]issueWithLinks, len(issues))
	for i, issue := range issues {
		result[i] = issueWithLinks{Issue: issue, AssigneeIDs: assigneeMap[issue.ID], AlertIDs: alertMap[issue.ID]}
	}

	jsonOK(w, result)
}

func (h *globalIssueHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	var req struct {
		models.Issue
		AssigneeIDs []int64 `json:"assignee_ids"`
		AlertIDs    []int64 `json:"alert_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.EntityType == "" {
		jsonError(w, http.StatusBadRequest, "entity_type is required")
		return
	}

	req.CreatedBy = user.ID
	if req.Status == "" {
		req.Status = "backlog"
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	// Backward compat: set project_id if entity is a project
	if req.EntityType == "project" {
		id := req.EntityID
		req.ProjectID = &id
	}

	log.Printf("[issues] Creating issue: entity_type=%s entity_id=%d project_id=%v title=%q source=%s alert_ids=%v",
		req.EntityType, req.EntityID, req.ProjectID, req.Title, req.Source, req.AlertIDs)

	if err := models.CreateIssue(h.db.SQL, &req.Issue); err != nil {
		log.Printf("[issues] CreateIssue error: %v", err)
		jsonServerError(w, r, "failed to create issue", err)
		return
	}

	log.Printf("[issues] Created issue #%d, setting links: assignees=%v alerts=%v", req.Issue.ID, req.AssigneeIDs, req.AlertIDs)

	if len(req.AssigneeIDs) > 0 {
		if err := models.SetIssueAssignees(h.db.SQL, req.Issue.ID, req.AssigneeIDs); err != nil {
			log.Printf("[issues] SetIssueAssignees error: %v", err)
		}
	}
	if len(req.AlertIDs) > 0 {
		if err := models.SetIssueAlertLinks(h.db.SQL, req.Issue.ID, req.AlertIDs); err != nil {
			log.Printf("[issues] SetIssueAlertLinks error: %v", err)
		}
	}

	jsonCreated(w, req.Issue)
}

func (h *globalIssueHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid issue id", err)
		return
	}

	existing, err := models.GetIssue(h.db.SQL, issueID)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "issue not found")
		return
	}

	var req struct {
		models.Issue
		AssigneeIDs *[]int64 `json:"assignee_ids"`
		AlertIDs    *[]int64 `json:"alert_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.ID = issueID
	req.CreatedBy = existing.CreatedBy
	if req.EntityType == "" {
		req.EntityType = existing.EntityType
	}
	if req.EntityID == 0 {
		req.EntityID = existing.EntityID
	}
	if req.Status == "" {
		req.Status = existing.Status
	}
	if req.Priority == "" {
		req.Priority = existing.Priority
	}
	if req.Source == "" {
		req.Source = existing.Source
	}
	// Preserve auto-managed date fields
	if req.StartDate == "" {
		req.StartDate = existing.StartDate
	}
	if req.EndDate == "" {
		req.EndDate = existing.EndDate
	}
	if req.AlertID == nil {
		req.AlertID = existing.AlertID
	}

	if err := models.UpdateIssue(h.db.SQL, &req.Issue); err != nil {
		jsonServerError(w, r, "failed to update issue", err)
		return
	}

	if req.AssigneeIDs != nil {
		models.SetIssueAssignees(h.db.SQL, issueID, *req.AssigneeIDs)
	}
	if req.AlertIDs != nil {
		models.SetIssueAlertLinks(h.db.SQL, issueID, *req.AlertIDs)
	}

	// Auto-resolve linked alerts when issue is done
	if req.Status == "done" {
		if err := models.ResolveAlertsByIssueID(h.db.SQL, issueID); err != nil {
			log.Printf("[issues] ResolveAlertsByIssueID error: %v", err)
		}
	}

	jsonOK(w, req.Issue)
}

func (h *globalIssueHandlers) handleMove(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "id")
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

	// Auto-resolve linked alerts when issue is moved to done
	if req.Status == "done" {
		if err := models.ResolveAlertsByIssueID(h.db.SQL, issueID); err != nil {
			log.Printf("[issues] ResolveAlertsByIssueID error: %v", err)
		}
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *globalIssueHandlers) handleArchive(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid issue id", err)
		return
	}

	existing, err := models.GetIssue(h.db.SQL, issueID)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "issue not found")
		return
	}

	existing.Archived = !existing.Archived
	if err := models.UpdateIssue(h.db.SQL, existing); err != nil {
		jsonServerError(w, r, "failed to archive issue", err)
		return
	}

	jsonOK(w, existing)
}

func (h *globalIssueHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	issueID, err := pathInt64(r, "id")
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
