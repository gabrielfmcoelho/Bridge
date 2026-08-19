package api

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type toolHandlers struct {
	db *database.DB
}

func (h *toolHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	tools, err := store.NewExternalToolRepo(h.db.SQL).List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list tools", err)
		return
	}
	if tools == nil {
		tools = []models.ExternalTool{}
	}
	jsonPaged(w, r, tools)
}

func (h *toolHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	tool, err := store.NewExternalToolRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	jsonOK(w, tool)
}

func (h *toolHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.ExternalTool
		models.AssetGrantsInput
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	g, ok := resolveGrants(w, r, req.AssetGrantsInput, nil)
	if !ok {
		return
	}

	if err := store.NewExternalToolRepo(h.db.SQL).Create(r.Context(), &req.ExternalTool); err != nil {
		jsonServerError(w, r, "failed to create tool", err)
		return
	}
	if err := store.NewAssetEntidadeRepo(h.db.SQL).Replace(r.Context(), h.db.SQL, store.AssetTool, req.ExternalTool.ID, g); err != nil {
		jsonServerError(w, r, "failed to set entidades", err)
		return
	}
	jsonCreated(w, req.ExternalTool)
}

func (h *toolHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := store.NewExternalToolRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}

	var req struct {
		models.ExternalTool
		models.AssetGrantsInput
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
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

	if err := store.NewExternalToolRepo(h.db.SQL).Update(r.Context(), &req.ExternalTool); err != nil {
		jsonServerError(w, r, "failed to update tool", err)
		return
	}
	if req.AssetGrantsInput.Present() {
		grants := store.NewAssetEntidadeRepo(h.db.SQL)
		existingGrants, _ := grants.Get(r.Context(), store.AssetTool, id)
		g, ok := resolveGrants(w, r, req.AssetGrantsInput, &existingGrants)
		if !ok {
			return
		}
		if err := grants.Replace(r.Context(), h.db.SQL, store.AssetTool, id, g); err != nil {
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
	}
	jsonOK(w, req.ExternalTool)
}

func (h *toolHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	// Soft delete (v62): flip external_tools.deleted_at and cascade-soft-
	// delete child secrets in the same transaction. The tool scope is
	// registered as SoftDelete in the store cascade registry. Reachable via
	// /api/tools/{id}/restore.
	actor, _ := actorFrom(r)
	if err := store.DeleteParent(r.Context(), h.db.SQL, actor, models.SecretScopeTool, id); err != nil {
		jsonServerError(w, r, "failed to delete tool", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleRestore reverses a soft delete on the external_tool and cascade-
// restores any child secrets that were soft-deleted by the parent's
// previous delete (matched by audit metadata or just by being currently
// soft-deleted with the same parent — CascadeRestore is idempotent).
func (h *toolHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	actor, _ := actorFrom(r)
	if err := store.RestoreParent(r.Context(), h.db.SQL, actor, models.SecretScopeTool, id); err != nil {
		jsonServerError(w, r, "failed to restore tool", err)
		return
	}
	jsonOK(w, map[string]string{"status": "restored"})
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
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.ServiceID == 0 || req.DNSID == 0 {
		jsonError(w, http.StatusBadRequest, "service_id and dns_id are required")
		return
	}

	// Validate service exists.
	svc, err := store.NewServiceRepo(h.db.SQL).Get(r.Context(), req.ServiceID)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	// Validate DNS exists.
	dns, err := store.NewDNSRepo(h.db.SQL).Get(r.Context(), req.DNSID)
	if err != nil || dns == nil {
		jsonError(w, http.StatusNotFound, "dns record not found")
		return
	}

	// Verify DNS is linked to the service.
	dnsIDs, err := store.NewServiceRepo(h.db.SQL).DNSIDs(r.Context(), req.ServiceID)
	if err != nil {
		jsonServerError(w, r, "failed to check service-dns links", err)
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
	existing, err := store.NewExternalToolRepo(h.db.SQL).GetByServiceAndDNS(r.Context(), req.ServiceID, req.DNSID)
	if err != nil {
		jsonServerError(w, r, "failed to check existing tool", err)
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
		if err := store.NewExternalToolRepo(h.db.SQL).Update(r.Context(), existing); err != nil {
			jsonServerError(w, r, "failed to update synced tool", err)
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
	if err := store.NewExternalToolRepo(h.db.SQL).Create(r.Context(), tool); err != nil {
		jsonServerError(w, r, "failed to create synced tool", err)
		return
	}
	// A synced tool inherits its service's entidade grants.
	if err := store.NewAssetEntidadeRepo(h.db.SQL).CopyFrom(r.Context(), h.db.SQL, store.AssetService, req.ServiceID, store.AssetTool, tool.ID); err != nil {
		jsonServerError(w, r, "failed to set entidades", err)
		return
	}
	jsonCreated(w, tool)
}

// handleUnsyncService deletes a synced tool by its ID.
func (h *toolHandlers) handleUnsyncService(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	tool, err := store.NewExternalToolRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil || tool == nil {
		jsonError(w, http.StatusNotFound, "tool not found")
		return
	}
	if tool.Source != "service" {
		jsonError(w, http.StatusBadRequest, "tool is not a synced service")
		return
	}

	if err := store.NewExternalToolRepo(h.db.SQL).Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete synced tool", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// Legacy tool credential handlers were removed in the Phase 1 cutover.
// Callers query /api/secrets?scope=service&parent_id=<tool.service_id> for
// list, and /api/secrets/{id}/reveal for plaintext.

// registerRoutes wires this group's routes (self-registration, R2).
func (h *toolHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/tools", h.handleList)
	rr.role("admin", "POST /api/tools", h.handleCreate)
	rr.role("admin", "POST /api/tools/sync-service", h.handleSyncFromService)
	rr.role("admin", "DELETE /api/tools/sync-service/{id}", h.handleUnsyncService)
	rr.auth("GET /api/tools/{id}", h.handleGet)
	rr.role("admin", "PUT /api/tools/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/tools/{id}", h.handleDelete)
	rr.role("admin", "POST /api/tools/{id}/restore", h.handleRestore)
}
