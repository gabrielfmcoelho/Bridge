package api

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type toolHandlers struct {
	db *database.DB
}

func (h *toolHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	tools, err := models.ListExternalTools(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list tools")
		return
	}
	if tools == nil {
		tools = []models.ExternalTool{}
	}
	jsonOK(w, tools)
}

func (h *toolHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	tool, err := models.GetExternalTool(h.db.SQL, id)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	jsonOK(w, tool)
}

func (h *toolHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req models.ExternalTool
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := models.CreateExternalTool(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create tool")
		return
	}
	jsonCreated(w, req)
}

func (h *toolHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	existing, err := models.GetExternalTool(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}

	var req models.ExternalTool
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ID = id
	// Preserve sync fields if not provided
	if req.Source == "" {
		req.Source = existing.Source
	}
	if existing.Source == "service" {
		req.ServiceID = existing.ServiceID
		req.DNSID = existing.DNSID
		req.Source = existing.Source
	}

	if err := models.UpdateExternalTool(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update tool")
		return
	}
	jsonOK(w, req)
}

func (h *toolHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := models.DeleteExternalTool(h.db.SQL, id); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete tool")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleSyncFromService creates or updates a tool entry linked to a service+DNS pair.
func (h *toolHandlers) handleSyncFromService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID    int64  `json:"service_id"`
		DNSID        int64  `json:"dns_id"`
		EmbedEnabled bool   `json:"embed_enabled"`
		Icon         string `json:"icon"`
		SortOrder    int    `json:"sort_order"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ServiceID == 0 || req.DNSID == 0 {
		jsonError(w, http.StatusBadRequest, "service_id and dns_id are required")
		return
	}

	// Validate service exists.
	svc, err := models.GetService(h.db.SQL, req.ServiceID)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	// Validate DNS exists.
	dns, err := models.GetDNSRecord(h.db.SQL, req.DNSID)
	if err != nil || dns == nil {
		jsonError(w, http.StatusNotFound, "dns record not found")
		return
	}

	// Verify DNS is linked to the service.
	dnsIDs, err := models.GetServiceDNSIDs(h.db.SQL, req.ServiceID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to check service-dns links")
		return
	}
	if !slices.Contains(dnsIDs, req.DNSID) {
		jsonError(w, http.StatusBadRequest, "dns record is not linked to this service")
		return
	}

	// Build URL from DNS.
	scheme := "http"
	if dns.HasHTTPS {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s", scheme, dns.Domain)

	// Check for existing synced tool.
	existing, err := models.GetToolByServiceAndDNS(h.db.SQL, req.ServiceID, req.DNSID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to check existing tool")
		return
	}

	if existing != nil {
		// Update existing.
		existing.Name = svc.Nickname
		existing.Description = svc.Description
		existing.URL = url
		existing.EmbedEnabled = req.EmbedEnabled
		if req.Icon != "" {
			existing.Icon = req.Icon
		}
		existing.SortOrder = req.SortOrder
		if err := models.UpdateExternalTool(h.db.SQL, existing); err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to update synced tool")
			return
		}
		jsonOK(w, existing)
		return
	}

	// Create new synced tool.
	tool := &models.ExternalTool{
		Name:         svc.Nickname,
		Description:  svc.Description,
		URL:          url,
		Icon:         req.Icon,
		EmbedEnabled: req.EmbedEnabled,
		SortOrder:    req.SortOrder,
		ServiceID:    &req.ServiceID,
		DNSID:        &req.DNSID,
		Source:       "service",
	}
	if err := models.CreateExternalTool(h.db.SQL, tool); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create synced tool")
		return
	}
	jsonCreated(w, tool)
}

// handleUnsyncService deletes a synced tool by its ID.
func (h *toolHandlers) handleUnsyncService(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	tool, err := models.GetExternalTool(h.db.SQL, id)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	if tool.Source != "service" {
		jsonError(w, http.StatusBadRequest, "tool is not a synced service")
		return
	}

	if err := models.DeleteExternalTool(h.db.SQL, id); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete synced tool")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleListToolCredentials returns credential summaries for a tool's linked service.
func (h *toolHandlers) handleListToolCredentials(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}

	tool, err := models.GetExternalTool(h.db.SQL, id)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	if tool.ServiceID == nil {
		jsonOK(w, []struct{}{})
		return
	}

	creds, err := models.ListServiceCredentials(h.db.SQL, *tool.ServiceID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list credentials")
		return
	}

	type credSummary struct {
		ID       int64  `json:"id"`
		RoleName string `json:"role_name"`
	}
	result := make([]credSummary, len(creds))
	for i, c := range creds {
		result[i] = credSummary{ID: c.ID, RoleName: c.RoleName}
	}
	jsonOK(w, result)
}

// handleGetToolCredential decrypts and returns a specific credential for a tool's linked service.
func (h *toolHandlers) handleGetToolCredential(w http.ResponseWriter, r *http.Request) {
	toolID, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid tool id")
		return
	}

	tool, err := models.GetExternalTool(h.db.SQL, toolID)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	if tool.ServiceID == nil {
		jsonError(w, http.StatusBadRequest, "tool has no linked service")
		return
	}

	credID, err := pathInt64(r, "credId")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid credential id")
		return
	}

	cred, err := models.GetServiceCredential(h.db.SQL, credID)
	if err != nil || cred == nil {
		jsonError(w, http.StatusNotFound, "credential not found")
		return
	}

	// Ensure the credential belongs to the tool's service.
	if cred.ServiceID != *tool.ServiceID {
		jsonError(w, http.StatusForbidden, "credential does not belong to this tool's service")
		return
	}

	plaintext, err := h.db.Encryptor.Decrypt(cred.CredentialsCiphertext, cred.CredentialsNonce)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to decrypt credentials")
		return
	}

	jsonOK(w, map[string]any{
		"id":          cred.ID,
		"role_name":   cred.RoleName,
		"credentials": plaintext,
	})
}
