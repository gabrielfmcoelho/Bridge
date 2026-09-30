package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/proxmox"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

type proxmoxHandlers struct {
	db    *database.DB
	hosts *service.HostService
	// ponytail: one sync per process; a pg_try_advisory_lock if the API ever
	// runs as more than one replica.
	syncing sync.Mutex
}

// resolveClient returns a client from the stored settings, or nil and the
// reason the integration isn't usable.
func (h *proxmoxHandlers) resolveClient() (*proxmox.Client, string) {
	s, err := proxmox.LoadSettings(h.db.SQL, h.db.Encryptor)
	switch {
	case err != nil:
		return nil, "failed to load settings"
	case !s.Enabled:
		return nil, "Proxmox integration is disabled"
	case s.BaseURL == "" || s.TokenID == "" || s.TokenSecret == "":
		return nil, "Proxmox base URL / API token not configured"
	}
	return proxmox.NewClient(s.BaseURL, s.TokenID, s.TokenSecret, s.SkipVerify), ""
}

// handleSync reads the whole cluster and upserts its nodes and guests as
// hosts. Synchronous: the response is the sync summary.
//
//	@Summary		Sync hosts from Proxmox
//	@Description	Admin. Reads the whole cluster and upserts its nodes and guests as hosts; synchronous (up to 10 minutes). 409 while another sync runs, 400 when the integration is disabled or unconfigured, 502 when Proxmox fails.
//	@Tags			proxmox
//	@Produce		json
//	@Success		200	{object}	service.ProxmoxSyncSummary
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		409	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/sync [post]
func (h *proxmoxHandlers) handleSync(w http.ResponseWriter, r *http.Request) {
	// Two overlapping syncs would both create the same new machines.
	if !h.syncing.TryLock() {
		jsonError(w, http.StatusConflict, "a Proxmox sync is already running")
		return
	}
	defer h.syncing.Unlock()
	client, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusBadRequest, reason)
		return
	}
	// Detached like the cert scan: a client giving up mid-sync doesn't abort
	// the writes. The deadline matches the frontend's request timeout.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
	defer cancel()
	ms, err := client.Machines(ctx)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Proxmox: "+err.Error())
		return
	}
	sum, err := h.hosts.SyncFromProxmox(ctx, ms)
	if err != nil {
		jsonServerError(w, r, "failed to sync Proxmox hosts", err)
		return
	}
	jsonOK(w, sum)
}

// proxmoxTestRequest carries unsaved form values; blank fields fall back to
// the stored settings.
type proxmoxTestRequest struct {
	BaseURL     string `json:"base_url"`
	TokenID     string `json:"token_id"`
	TokenSecret string `json:"token_secret"`
	SkipVerify  *bool  `json:"skip_verify"`
}

// handleTest checks reachability + token with GET /version. Unsaved form
// values in the body win over stored settings; nothing is persisted.
//
//	@Summary		Test the Proxmox connection
//	@Description	Admin. Calls GET /version with the body values over the stored settings; nothing is persisted. Always 200: {"success": bool, "version"?: ..., "error"?: string}. The body is optional.
//	@Tags			proxmox
//	@Accept			json
//	@Produce		json
//	@Param			body	body		proxmoxTestRequest	false	"Unsaved settings to test"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/test [post]
func (h *proxmoxHandlers) handleTest(w http.ResponseWriter, r *http.Request) {
	var req proxmoxTestRequest
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	s, err := proxmox.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": "failed to read settings"})
		return
	}
	storedURL := s.BaseURL
	if v := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"); v != "" {
		s.BaseURL = v
	}
	if v := strings.TrimSpace(req.TokenID); v != "" {
		s.TokenID = v
	}
	if v := strings.TrimSpace(req.TokenSecret); v != "" && v != "••••••••" {
		s.TokenSecret = v
	} else if s.BaseURL != storedURL {
		// The stored secret only ever goes to the stored URL.
		s.TokenSecret = ""
	}
	if req.SkipVerify != nil {
		s.SkipVerify = *req.SkipVerify
	}
	if s.BaseURL == "" || s.TokenID == "" || s.TokenSecret == "" {
		jsonOK(w, map[string]any{"success": false, "error": "Base URL, token ID and secret are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	v, err := proxmox.NewClient(s.BaseURL, s.TokenID, s.TokenSecret, s.SkipVerify).Version(ctx)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]any{"success": true, "version": v})
}

func (h *proxmoxHandlers) registerRoutes(rr routeRegistrar) {
	rr.role("admin", "POST /api/proxmox/sync", h.handleSync)
	rr.role("admin", "POST /api/proxmox/test", h.handleTest)
}
