package api

import (
	"log"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type hostAlertHandlers struct {
	db *database.DB
}

func (h *hostAlertHandlers) resolveHost(w http.ResponseWriter, r *http.Request) *models.Host {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return nil
	}
	return host
}

func (h *hostAlertHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	alerts, err := models.ListHostAlerts(h.db.SQL, host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to list alerts", err)
		return
	}

	// Enrich with linked issue IDs
	linkedIssues, _ := models.GetAlertLinkedIssueIDs(h.db.SQL, host.ID)

	type alertWithLink struct {
		models.HostAlert
		LinkedIssueID *int64 `json:"linked_issue_id,omitempty"`
	}

	result := make([]alertWithLink, len(alerts))
	for i, a := range alerts {
		result[i] = alertWithLink{HostAlert: a}
		if issueID, ok := linkedIssues[a.ID]; ok {
			id := issueID
			result[i].LinkedIssueID = &id
		}
	}

	jsonOK(w, result)
}

func (h *hostAlertHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	var req models.HostAlert
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Type == "" || req.Message == "" {
		jsonError(w, http.StatusBadRequest, "type and message are required")
		return
	}

	req.HostID = host.ID
	if req.Source == "" {
		req.Source = "manual"
	}

	log.Printf("[alerts] Creating alert: host_id=%d type=%s level=%s source=%s message=%q",
		req.HostID, req.Type, req.Level, req.Source, req.Message)

	if err := models.CreateHostAlert(h.db.SQL, &req); err != nil {
		log.Printf("[alerts] CreateHostAlert error: %v", err)
		jsonServerError(w, r, "failed to create alert", err)
		return
	}

	log.Printf("[alerts] Created alert #%d", req.ID)
	jsonCreated(w, req)
}

func (h *hostAlertHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	alertID, err := pathInt64(r, "alertId")
	if err != nil {
		jsonBadRequest(w, r, "invalid alert id", err)
		return
	}

	existing, err := models.GetHostAlert(h.db.SQL, alertID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "alert not found")
		return
	}
	if existing.Source != "manual" {
		jsonError(w, http.StatusForbidden, "cannot edit auto-generated alerts")
		return
	}

	var req models.HostAlert
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	existing.Type = req.Type
	existing.Level = req.Level
	existing.Message = req.Message
	existing.Description = req.Description

	if err := models.UpdateHostAlert(h.db.SQL, existing); err != nil {
		jsonServerError(w, r, "failed to update alert", err)
		return
	}

	jsonOK(w, existing)
}

func (h *hostAlertHandlers) handleConclude(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	alertID, err := pathInt64(r, "alertId")
	if err != nil {
		jsonBadRequest(w, r, "invalid alert id", err)
		return
	}

	existing, err := models.GetHostAlert(h.db.SQL, alertID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "alert not found")
		return
	}

	// Check if alert has linked issues — if so, it can only be resolved via issue completion
	linkedIssues, _ := models.GetAlertLinkedIssueIDs(h.db.SQL, host.ID)
	if _, hasIssue := linkedIssues[alertID]; hasIssue {
		jsonError(w, http.StatusConflict, "alert has linked issue — resolve the issue to conclude this alert")
		return
	}

	if err := models.ResolveHostAlert(h.db.SQL, alertID); err != nil {
		log.Printf("[alerts] ResolveHostAlert error: %v", err)
		jsonServerError(w, r, "failed to conclude alert", err)
		return
	}

	jsonOK(w, map[string]string{"status": "resolved"})
}

func (h *hostAlertHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	host := h.resolveHost(w, r)
	if host == nil {
		return
	}

	alertID, err := pathInt64(r, "alertId")
	if err != nil {
		jsonBadRequest(w, r, "invalid alert id", err)
		return
	}

	existing, err := models.GetHostAlert(h.db.SQL, alertID)
	if err != nil || existing == nil || existing.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "alert not found")
		return
	}
	if existing.Source != "manual" {
		jsonError(w, http.StatusForbidden, "cannot delete auto-generated alerts")
		return
	}

	if err := models.DeleteHostAlert(h.db.SQL, alertID); err != nil {
		jsonServerError(w, r, "failed to delete alert", err)
		return
	}

	jsonOK(w, map[string]string{"status": "deleted"})
}
