package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/proxmox"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// maskedSecret is what the settings UI shows for a stored secret; sent back,
// it means "keep the stored one".
const maskedSecret = "••••••••"

type proxmoxHandlers struct {
	db    *database.DB
	hosts *service.HostService
	// ponytail: one sync per process; a pg_try_advisory_lock if the API ever
	// runs as more than one replica.
	syncing sync.Mutex
}

func (h *proxmoxHandlers) servers() *store.ProxmoxServerRepo {
	return store.NewProxmoxServerRepo(h.db.SQL)
}

// client builds an API client for a stored server, or says why it can't.
func (h *proxmoxHandlers) client(s models.ProxmoxServer) (*proxmox.Client, error) {
	if s.BaseURL == "" || s.TokenID == "" || !s.HasToken {
		return nil, errors.New("base URL / API token not configured")
	}
	secret, err := h.db.Encryptor.Decrypt(s.TokenCipher, s.TokenNonce)
	if err != nil {
		return nil, errors.New("failed to decrypt the token secret")
	}
	return proxmox.NewClient(s.BaseURL, s.TokenID, secret, s.SkipVerify), nil
}

// proxmoxServerSyncResult is one server's outcome in a sync; Error set means
// that server failed (the counts are what it did before failing).
type proxmoxServerSyncResult struct {
	ServerID int64  `json:"server_id"`
	Name     string `json:"name"`
	Error    string `json:"error,omitempty"`
	service.ProxmoxSyncSummary
}

type proxmoxSyncResponse struct {
	Servers []proxmoxServerSyncResult `json:"servers"`
}

// handleSync reads every enabled server in turn and upserts its nodes and
// guests as hosts. Synchronous: the response is the per-server summary.
//
//	@Summary		Sync hosts from Proxmox
//	@Description	Admin. Reads every enabled Proxmox server in turn and upserts its nodes and guests as hosts; synchronous (up to 30 minutes). One server failing doesn't stop the others: its entry carries error. 409 while another sync runs, 400 when the integration is disabled or no server is enabled.
//	@Tags			proxmox
//	@Produce		json
//	@Success		200	{object}	proxmoxSyncResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		409	{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/sync [post]
func (h *proxmoxHandlers) handleSync(w http.ResponseWriter, r *http.Request) {
	// Two overlapping syncs would both create the same new machines.
	if !h.syncing.TryLock() {
		jsonError(w, http.StatusConflict, "a Proxmox sync is already running")
		return
	}
	defer h.syncing.Unlock()
	if store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "proxmox_enabled") != "true" {
		jsonError(w, http.StatusBadRequest, "Proxmox integration is disabled")
		return
	}
	servers, err := h.servers().List(r.Context(), true)
	if err != nil {
		jsonServerError(w, r, "failed to list Proxmox servers", err)
		return
	}
	if len(servers) == 0 {
		jsonError(w, http.StatusBadRequest, "no Proxmox server enabled")
		return
	}
	// Detached like the cert scan: a client giving up mid-sync doesn't abort
	// the writes. The deadline matches the frontend's request timeout.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Minute)
	defer cancel()
	out := proxmoxSyncResponse{Servers: make([]proxmoxServerSyncResult, 0, len(servers))}
	for _, s := range servers {
		res := proxmoxServerSyncResult{ServerID: s.ID, Name: s.Name}
		if err := h.syncServer(ctx, s, &res.ProxmoxSyncSummary); err != nil {
			log.Printf("[proxmox] sync %s (#%d): %v", s.Name, s.ID, err)
			res.Error = err.Error()
		}
		out.Servers = append(out.Servers, res)
	}
	jsonOK(w, out)
}

func (h *proxmoxHandlers) syncServer(ctx context.Context, s models.ProxmoxServer, sum *service.ProxmoxSyncSummary) error {
	c, err := h.client(s)
	if err != nil {
		return err
	}
	ms, err := c.Machines(ctx)
	if err != nil {
		return err
	}
	*sum, err = h.hosts.SyncFromProxmox(ctx, s.ID, ms)
	return err
}

// proxmoxTestRequest names a stored server and/or carries unsaved form
// values; blank fields fall back to the stored server's.
type proxmoxTestRequest struct {
	ServerID    int64  `json:"server_id"`
	BaseURL     string `json:"base_url"`
	TokenID     string `json:"token_id"`
	TokenSecret string `json:"token_secret"`
	SkipVerify  *bool  `json:"skip_verify"`
}

// handleTest checks reachability + token with GET /version. Body values win
// over the stored server's (server_id); nothing is persisted.
//
//	@Summary		Test a Proxmox server connection
//	@Description	Admin. Calls GET /version with the body values over the stored server's (server_id, optional); nothing is persisted. The stored secret is only ever sent to the stored URL. Always 200: {"success": bool, "version"?: ..., "error"?: string}.
//	@Tags			proxmox
//	@Accept			json
//	@Produce		json
//	@Param			body	body		proxmoxTestRequest	true	"Server to test and/or unsaved values"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/test [post]
func (h *proxmoxHandlers) handleTest(w http.ResponseWriter, r *http.Request) {
	var req proxmoxTestRequest
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	var baseURL, tokenID, secret string
	var skip bool
	if req.ServerID != 0 {
		s, err := h.servers().Get(r.Context(), req.ServerID)
		if err != nil || s == nil {
			jsonOK(w, map[string]any{"success": false, "error": "Proxmox server not found"})
			return
		}
		baseURL, tokenID, skip = s.BaseURL, s.TokenID, s.SkipVerify
		if s.HasToken {
			if secret, err = h.db.Encryptor.Decrypt(s.TokenCipher, s.TokenNonce); err != nil {
				jsonOK(w, map[string]any{"success": false, "error": "failed to decrypt the token secret"})
				return
			}
		}
	}
	storedURL := baseURL
	if v := normalizeBaseURL(req.BaseURL); v != "" {
		baseURL = v
	}
	if v := strings.TrimSpace(req.TokenID); v != "" {
		tokenID = v
	}
	if v := strings.TrimSpace(req.TokenSecret); v != "" && v != maskedSecret {
		secret = v
	} else if baseURL != storedURL {
		// The stored secret only ever goes to the stored URL.
		secret = ""
	}
	if req.SkipVerify != nil {
		skip = *req.SkipVerify
	}
	if baseURL == "" || tokenID == "" || secret == "" {
		jsonOK(w, map[string]any{"success": false, "error": "Base URL, token ID and secret are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	v, err := proxmox.NewClient(baseURL, tokenID, secret, skip).Version(ctx)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]any{"success": true, "version": v})
}

func normalizeBaseURL(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

// applyProxmoxInput merges the partial input over s: an omitted field keeps
// its value, an empty or masked token_secret keeps the stored one.
func (h *proxmoxHandlers) applyProxmoxInput(s *models.ProxmoxServer, in models.ProxmoxServerInput) error {
	if in.Name != nil {
		s.Name = strings.TrimSpace(*in.Name)
	}
	if in.BaseURL != nil {
		s.BaseURL = normalizeBaseURL(*in.BaseURL)
	}
	if in.TokenID != nil {
		s.TokenID = strings.TrimSpace(*in.TokenID)
	}
	if in.SkipVerify != nil {
		s.SkipVerify = *in.SkipVerify
	}
	if in.Enabled != nil {
		s.Enabled = *in.Enabled
	}
	if in.TokenSecret != nil {
		if v := strings.TrimSpace(*in.TokenSecret); v != "" && v != maskedSecret {
			c, n, err := h.db.Encryptor.Encrypt(v)
			if err != nil {
				return err
			}
			s.TokenCipher, s.TokenNonce, s.HasToken = c, n, true
		}
	}
	return nil
}

// handleListServers godoc
//
//	@Summary		List Proxmox servers
//	@Description	Admin. Token secrets are never returned, only has_token.
//	@Tags			proxmox
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.ProxmoxServer]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/servers [get]
func (h *proxmoxHandlers) handleListServers(w http.ResponseWriter, r *http.Request) {
	list, err := h.servers().List(r.Context(), false)
	if err != nil {
		jsonServerError(w, r, "failed to list Proxmox servers", err)
		return
	}
	jsonPaged(w, r, list)
}

// handleCreateServer godoc
//
//	@Summary		Add a Proxmox server
//	@Description	Admin. name, base_url, token_id and token_secret are required; the secret is stored encrypted. enabled defaults to true.
//	@Tags			proxmox
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.ProxmoxServerInput	true	"Server"
//	@Success		201		{object}	models.ProxmoxServer
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/servers [post]
func (h *proxmoxHandlers) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var in models.ProxmoxServerInput
	if err := decodeJSON(r, &in); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	s := models.ProxmoxServer{Enabled: true}
	if err := h.applyProxmoxInput(&s, in); err != nil {
		jsonServerError(w, r, "encrypt failed", err)
		return
	}
	if s.Name == "" || s.BaseURL == "" || s.TokenID == "" || !s.HasToken {
		jsonError(w, http.StatusBadRequest, "name, base_url, token_id and token_secret are required")
		return
	}
	if err := h.servers().Create(r.Context(), &s); err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "could not add the Proxmox server (name already in use?)", err)
		return
	}
	jsonCreated(w, s)
}

// handleUpdateServer godoc
//
//	@Summary		Update a Proxmox server
//	@Description	Admin. Partial: an omitted field keeps its value; an empty or masked token_secret keeps the stored one.
//	@Tags			proxmox
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Server ID"
//	@Param			body	body		models.ProxmoxServerInput	true	"Fields to change"
//	@Success		200		{object}	models.ProxmoxServer
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/servers/{id} [put]
func (h *proxmoxHandlers) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	s, err := h.servers().Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "lookup failed", err)
		return
	}
	if s == nil {
		jsonError(w, http.StatusNotFound, "Proxmox server not found")
		return
	}
	var in models.ProxmoxServerInput
	if err := decodeJSON(r, &in); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if err := h.applyProxmoxInput(s, in); err != nil {
		jsonServerError(w, r, "encrypt failed", err)
		return
	}
	if s.Name == "" || s.BaseURL == "" || s.TokenID == "" {
		jsonError(w, http.StatusBadRequest, "name, base_url and token_id can't be empty")
		return
	}
	if err := h.servers().Update(r.Context(), s); err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "could not update the Proxmox server (name already in use?)", err)
		return
	}
	jsonOK(w, s)
}

// handleDeleteServer godoc
//
//	@Summary		Delete a Proxmox server
//	@Description	Admin. Its hosts stay, unlinked; a later sync may re-link them by IP or name.
//	@Tags			proxmox
//	@Produce		json
//	@Param			id	path		int	true	"Server ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/proxmox/servers/{id} [delete]
func (h *proxmoxHandlers) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	if err := h.servers().Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "delete failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *proxmoxHandlers) registerRoutes(rr routeRegistrar) {
	rr.role("admin", "POST /api/proxmox/sync", h.handleSync)
	rr.role("admin", "POST /api/proxmox/test", h.handleTest)
	rr.role("admin", "GET /api/proxmox/servers", h.handleListServers)
	rr.role("admin", "POST /api/proxmox/servers", h.handleCreateServer)
	rr.role("admin", "PUT /api/proxmox/servers/{id}", h.handleUpdateServer)
	rr.role("admin", "DELETE /api/proxmox/servers/{id}", h.handleDeleteServer)
}
