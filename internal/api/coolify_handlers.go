package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

type coolifyHandlers struct {
	db  *database.DB
	dns *service.DNSService
}

func (h *coolifyHandlers) getClient() (*coolify.Client, error) {
	get := func(key string) string { return store.NewAppSettingsRepo(h.db.SQL).Value(context.Background(), key) }

	if get("coolify_enabled") != "true" {
		return nil, fmt.Errorf("coolify integration is not enabled")
	}

	baseURL := get("coolify_base_url")
	apiToken, ok, err := store.NewAppSecretRepo(h.db.SQL).Reveal(context.Background(), h.db.Encryptor, "coolify_api_token")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt coolify token: %w", err)
	}
	if !ok || baseURL == "" {
		return nil, fmt.Errorf("coolify integration is not configured")
	}

	return coolify.NewClient(baseURL, apiToken), nil
}

func (h *coolifyHandlers) logOp(r *http.Request, hostID int64, opType, status, output string) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return
	}
	ol := &models.OperationLog{
		HostID:        hostID,
		UserID:        user.ID,
		OperationType: opType,
		Status:        status,
		Output:        output,
	}
	if err := store.NewOperationLogRepo(h.db.SQL).Create(r.Context(), ol); err != nil {
		log.Printf("[coolify] failed to log operation: %v", err)
	}
}

// handleStatus returns whether the Coolify integration is enabled and configured.
//
//	@Summary		Coolify integration status
//	@Description	Any role. Returns {enabled, configured}.
//	@Tags			coolify
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/status [get]
func (h *coolifyHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	enabled := store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "coolify_enabled") == "true"
	configured := store.NewAppSecretRepo(h.db.SQL).Configured(r.Context(), "coolify_api_token")
	jsonOK(w, map[string]any{
		"enabled":    enabled,
		"configured": configured,
	})
}

// handleTestConnection tests the Coolify connection using the healthcheck endpoint.
//
//	@Summary		Test the Coolify connection
//	@Description	Admin. Calls the Coolify healthcheck. Always 200: {success: bool, error?: string}.
//	@Tags			coolify
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/test [post]
func (h *coolifyHandlers) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	if err := client.Healthcheck(); err != nil {
		jsonOK(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]any{"success": true})
}

// handleGetServerStatus fetches the current status of a host's linked Coolify server.
//
//	@Summary		Linked Coolify server status
//	@Description	Editor+. Returns {server} with the Coolify server linked to the host. 400 when the host is not linked; 503 when the integration is disabled or unconfigured.
//	@Tags			coolify
//	@Produce		json
//	@Param			slug	path		string	true	"Host slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/server-status/{slug} [get]
func (h *coolifyHandlers) handleGetServerStatus(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		jsonError(w, http.StatusBadRequest, "host is not linked to a coolify server")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	server, err := client.GetServer(*host.CoolifyServerUUID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to get server: "+err.Error())
		return
	}

	jsonOK(w, map[string]any{
		"server": server,
	})
}

// handleCheckHost searches Coolify for a server matching this host's IP.
//
//	@Summary		Find the host's server in Coolify
//	@Description	Editor+. Searches Coolify for a server matching the host's IP; when found, stores its UUID on the host. Returns {found, server?}.
//	@Tags			coolify
//	@Produce		json
//	@Param			slug	path		string	true	"Host slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/check/{slug} [post]
func (h *coolifyHandlers) handleCheckHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	server, err := client.FindServerByIP(host.Hostname)
	if err != nil {
		h.logOp(r, host.ID, "coolify-check", "failed", err.Error())
		jsonError(w, http.StatusBadGateway, "coolify api error: "+err.Error())
		return
	}

	if server != nil {
		// Store the UUID on the host for future operations
		store.NewHostRepo(h.db.SQL).SetCoolifyUUID(r.Context(), host.ID, &server.UUID)
		h.logOp(r, host.ID, "coolify-check", "success", fmt.Sprintf("found server %s (%s)", server.UUID, server.Name))
		jsonOK(w, map[string]any{"found": true, "server": server})
		return
	}

	h.logOp(r, host.ID, "coolify-check", "success", "server not found in coolify")
	jsonOK(w, map[string]any{"found": false})
}

// (resolvePrivateKeyUUID moved to coolify.Client.EnsurePrivateKey — R4b.)

// selectRegistrationKey resolves which SSH key should be uploaded to Coolify
// for a host. Priority: the vault key the caller picked, then the key the
// remote user `targetUser` links to (host_remote_users.key_secret_id), then
// the host's own connection key. Returns the private key, the Coolify key
// name and a description.
func (h *coolifyHandlers) selectRegistrationKey(host *models.Host, keySecretID int64, targetUser string) (privKey, keyName, description string, err error) {
	ctx := context.Background()
	if keySecretID > 0 {
		k, lerr := vault.LoadSharedKey(ctx, h.db, keySecretID)
		if lerr != nil {
			return "", "", "", lerr
		}
		return k.PrivateKeyPEM, coolifyManagedKeyName(k.Name), fmt.Sprintf("Managed by SSHCM key %q", k.Name), nil
	}

	if targetUser != "" {
		if link, lerr := store.NewHostRemoteUserRepo(h.db.SQL).GetByUsername(ctx, host.ID, targetUser); lerr == nil && link != nil && link.KeySecretID != nil {
			k, kerr := vault.LoadSharedKey(ctx, h.db, *link.KeySecretID)
			if kerr == nil {
				return k.PrivateKeyPEM, coolifyManagedKeyName(k.Name), fmt.Sprintf("Managed by SSHCM key %q (remote user %s)", k.Name, targetUser), nil
			}
			log.Printf("[coolify] host_remote_users link present for host=%d user=%s key=%d but load failed: %v", host.ID, targetUser, *link.KeySecretID, kerr)
		}
	}

	key, ok, derr := vault.HostGetSSHKey(ctx, h.db, host.ID)
	if derr != nil {
		return "", "", "", fmt.Errorf("load host key: %w", derr)
	}
	if !ok || key.PrivateKeyPEM == "" {
		return "", "", "", fmt.Errorf("host has no private key stored and no linked ssh key available")
	}
	return key.PrivateKeyPEM, fmt.Sprintf("sshcm-%s", host.OficialSlug), fmt.Sprintf("Managed by SSHCM for host %s", host.Nickname), nil
}

// coolifyManagedKeyName produces a Coolify-side name for a key shared across
// hosts. Kept distinct from the per-host `sshcm-<slug>` name so multiple hosts
// can reference the same uploaded key without colliding. Keys migrated from
// ssh_keys kept their names, so the uploaded key is found again.
func coolifyManagedKeyName(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r == ' ' || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return "sshcm-key-" + safe
}

// coolifyKeyRequest picks the vault SSH key to upload to Coolify (register and
// key-swap bodies).
type coolifyKeyRequest struct {
	KeySecretID int64 `json:"key_secret_id"`
	SSHKeyID    int64 `json:"ssh_key_id"` // pre-v91 clients
}

// handleRegisterHost uploads the chosen SSH key and creates a server in Coolify.
//
//	@Summary		Register the host as a Coolify server
//	@Description	Admin. Uploads the SSH key (the chosen vault key, else the default user's linked key, else the host's own key) and creates the server in Coolify. The body is optional. Returns {uuid}.
//	@Tags			coolify
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string				true	"Host slug"
//	@Param			body	body		coolifyKeyRequest	false	"Vault key to upload (optional)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/register/{slug} [post]
func (h *coolifyHandlers) handleRegisterHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	// Optional body; legacy callers POST with no payload.
	var req coolifyKeyRequest
	_ = decodeJSON(r, &req)
	keySecretID, ok := resolveKeySecret(w, r, h.db, req.KeySecretID, req.SSHKeyID)
	if !ok {
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Coolify rejects usernames with dots. Use configured default user or "root".
	coolifyUser := store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "coolify_default_user")
	if coolifyUser == "" {
		coolifyUser = "root"
	}

	privKey, keyName, description, err := h.selectRegistrationKey(host, keySecretID, coolifyUser)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	privateKeyUUID, err := client.EnsurePrivateKey(privKey, keyName, description)
	if err != nil {
		h.logOp(r, host.ID, "coolify-register", "failed", "key upload: "+err.Error())
		jsonError(w, http.StatusBadGateway, "failed to upload key to coolify: "+err.Error())
		return
	}

	// Parse port
	port := 22
	if p, err := strconv.Atoi(host.Port); err == nil && p > 0 {
		port = p
	}

	// Create server
	createReq := coolify.CreateServerRequest{
		Name:            host.Nickname,
		Description:     fmt.Sprintf("Managed by SSHCM (%s)", host.OficialSlug),
		IP:              host.Hostname,
		Port:            port,
		User:            coolifyUser,
		PrivateKeyUUID:  privateKeyUUID,
		InstantValidate: true,
	}
	log.Printf("[coolify] creating server: name=%q ip=%q port=%d user=%q key=%q", createReq.Name, createReq.IP, createReq.Port, createReq.User, createReq.PrivateKeyUUID)
	serverUUID, err := client.CreateServer(createReq)
	if err != nil {
		h.logOp(r, host.ID, "coolify-register", "failed", err.Error())
		jsonError(w, http.StatusBadGateway, "failed to create server in coolify: "+err.Error())
		return
	}

	store.NewHostRepo(h.db.SQL).SetCoolifyUUID(r.Context(), host.ID, &serverUUID)
	h.logOp(r, host.ID, "coolify-register", "success", fmt.Sprintf("created server %s with key %s", serverUUID, privateKeyUUID))
	jsonOK(w, map[string]any{"uuid": serverUUID})
}

// handleValidateHost triggers Coolify server validation.
//
//	@Summary		Trigger Coolify server validation
//	@Description	Admin. 400 when the host is not linked to a Coolify server. Returns {message}.
//	@Tags			coolify
//	@Produce		json
//	@Param			slug	path		string	true	"Host slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/validate/{slug} [post]
func (h *coolifyHandlers) handleValidateHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		jsonError(w, http.StatusBadRequest, "host is not linked to a coolify server")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	if err := client.ValidateServer(*host.CoolifyServerUUID); err != nil {
		h.logOp(r, host.ID, "coolify-validate", "failed", err.Error())
		jsonError(w, http.StatusBadGateway, "validation failed: "+err.Error())
		return
	}

	h.logOp(r, host.ID, "coolify-validate", "success", "validation triggered for "+*host.CoolifyServerUUID)
	jsonOK(w, map[string]any{"message": "validation started"})
}

// handleUpdateServerKey swaps the private key a Coolify server uses to SSH into
// the host. Uploads the selected sshcm key to Coolify (reusing any existing
// match), then PATCHes the server with the new key's UUID.
//
//	@Summary		Swap the Coolify server's SSH key
//	@Description	Admin. Uploads the selected vault key to Coolify (reusing a match) and points the linked server at it. Returns {success, private_key_uuid}.
//	@Tags			coolify
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string				true	"Host slug"
//	@Param			body	body		coolifyKeyRequest	true	"Vault key to use (key_secret_id required)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/server/{slug}/key [post]
func (h *coolifyHandlers) handleUpdateServerKey(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		jsonError(w, http.StatusBadRequest, "host is not linked to a coolify server")
		return
	}

	var req coolifyKeyRequest
	if err := decodeJSON(r, &req); err != nil || (req.KeySecretID <= 0 && req.SSHKeyID <= 0) {
		jsonError(w, http.StatusBadRequest, "key_secret_id is required")
		return
	}
	keySecretID, ok := resolveKeySecret(w, r, h.db, req.KeySecretID, req.SSHKeyID)
	if !ok {
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Pass empty targetUser so selectRegistrationKey uses only the explicit key
	// (no auto-resolution fallback — the caller asked for a specific key).
	privKey, keyName, description, err := h.selectRegistrationKey(host, keySecretID, "")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	privateKeyUUID, err := client.EnsurePrivateKey(privKey, keyName, description)
	if err != nil {
		h.logOp(r, host.ID, "coolify-update-key", "failed", "key upload: "+err.Error())
		jsonError(w, http.StatusBadGateway, "failed to upload key to coolify: "+err.Error())
		return
	}

	if err := client.UpdateServer(*host.CoolifyServerUUID, coolify.UpdateServerRequest{
		PrivateKeyUUID: privateKeyUUID,
	}); err != nil {
		h.logOp(r, host.ID, "coolify-update-key", "failed", err.Error())
		jsonError(w, http.StatusBadGateway, "update failed: "+err.Error())
		return
	}

	h.logOp(r, host.ID, "coolify-update-key", "success", fmt.Sprintf("server %s key=%s", *host.CoolifyServerUUID, privateKeyUUID))
	jsonOK(w, map[string]any{"success": true, "private_key_uuid": privateKeyUUID})
}

// handleSyncHost updates the Coolify server with current host info.
//
//	@Summary		Push host info to its Coolify server
//	@Description	Admin. Updates name, description, IP, port and user on the linked Coolify server. Returns {success}.
//	@Tags			coolify
//	@Produce		json
//	@Param			slug	path		string	true	"Host slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/sync/{slug} [post]
func (h *coolifyHandlers) handleSyncHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		jsonError(w, http.StatusBadRequest, "host is not linked to a coolify server")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	port := 22
	if p, err := strconv.Atoi(host.Port); err == nil && p > 0 {
		port = p
	}

	coolifyUser := store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "coolify_default_user")
	if coolifyUser == "" {
		coolifyUser = "root"
	}

	if err := client.UpdateServer(*host.CoolifyServerUUID, coolify.UpdateServerRequest{
		Name:        host.Nickname,
		Description: fmt.Sprintf("Managed by SSHCM (%s)", host.OficialSlug),
		IP:          host.Hostname,
		Port:        port,
		User:        coolifyUser,
	}); err != nil {
		h.logOp(r, host.ID, "coolify-sync", "failed", err.Error())
		jsonError(w, http.StatusBadGateway, "sync failed: "+err.Error())
		return
	}

	h.logOp(r, host.ID, "coolify-sync", "success", "synced to "+*host.CoolifyServerUUID)
	jsonOK(w, map[string]any{"success": true})
}

// handleDeleteHost removes the server from Coolify and clears the UUID.
//
//	@Summary		Delete the host's Coolify server
//	@Description	Admin. Deletes the server in Coolify and clears the link on the host. Returns {success}.
//	@Tags			coolify
//	@Produce		json
//	@Param			slug	path		string	true	"Host slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/coolify/server/{slug} [delete]
func (h *coolifyHandlers) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		jsonError(w, http.StatusBadRequest, "host is not linked to a coolify server")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	uuid := *host.CoolifyServerUUID
	if err := client.DeleteServer(uuid); err != nil {
		h.logOp(r, host.ID, "coolify-delete", "failed", err.Error())
		jsonError(w, http.StatusBadGateway, "delete failed: "+err.Error())
		return
	}

	store.NewHostRepo(h.db.SQL).SetCoolifyUUID(r.Context(), host.ID, nil)
	h.logOp(r, host.ID, "coolify-delete", "success", "deleted server "+uuid)
	jsonOK(w, map[string]any{"success": true})
}

// pathSharedKey loads the vault key {id} (a shared SSH key the caller may
// see); writes the error and returns false otherwise.
func (h *coolifyHandlers) pathSharedKey(w http.ResponseWriter, r *http.Request) (vault.SharedKey, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return vault.SharedKey{}, false
	}
	if id, ok := resolveKeySecret(w, r, h.db, id, 0); !ok {
		return vault.SharedKey{}, false
	} else if k, err := vault.LoadSharedKey(r.Context(), h.db, id); err != nil {
		jsonError(w, http.StatusNotFound, "key not found")
		return vault.SharedKey{}, false
	} else {
		return k, true
	}
}

// handleCheckKey checks if a managed SSH key exists in Coolify by fingerprint.
//
//	@Summary		Check whether a vault SSH key exists in Coolify
//	@Description	Editor+. Matches by fingerprint. Returns {found, coolify_uuid?, coolify_name?}.
//	@Tags			coolify
//	@Produce		json
//	@Param			id	path		int	true	"Vault SSH key secret ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Failure		503	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/keys/{id}/check [get]
func (h *coolifyHandlers) handleCheckKey(w http.ResponseWriter, r *http.Request) {
	key, ok := h.pathSharedKey(w, r)
	if !ok {
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Coolify lists fingerprints without the SHA256: prefix.
	fp := strings.TrimPrefix(key.Fingerprint(), "SHA256:")

	coolifyKeys, err := client.ListPrivateKeys()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "coolify api error: "+err.Error())
		return
	}

	for _, ck := range coolifyKeys {
		if fp != "" && ck.Fingerprint == fp {
			jsonOK(w, map[string]any{"found": true, "coolify_uuid": ck.UUID, "coolify_name": ck.Name})
			return
		}
	}

	jsonOK(w, map[string]any{"found": false})
}

// handleSyncKey uploads or updates a managed SSH key in Coolify.
//
//	@Summary		Upload a vault SSH key to Coolify
//	@Description	Admin. Creates the key in Coolify, or returns the existing one matched by fingerprint. Returns {uuid, name, already_existed}.
//	@Tags			coolify
//	@Produce		json
//	@Param			id	path		int	true	"Vault SSH key secret ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Failure		503	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/keys/{id}/sync [post]
func (h *coolifyHandlers) handleSyncKey(w http.ResponseWriter, r *http.Request) {
	key, ok := h.pathSharedKey(w, r)
	if !ok {
		return
	}
	privKeyText := key.PrivateKeyPEM

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	keyName := fmt.Sprintf("sshcm-%s", strings.ReplaceAll(key.Name, " ", "_"))
	uuid, createErr := client.CreatePrivateKey(coolify.CreateKeyRequest{
		Name:        keyName,
		Description: fmt.Sprintf("Managed by SSHCM — %s", key.Name),
		PrivateKey:  privKeyText,
	})
	if createErr != nil {
		// Already exists — find it by fingerprint
		if strings.Contains(createErr.Error(), "422") || strings.Contains(createErr.Error(), "already exists") {
			fp := strings.TrimPrefix(key.Fingerprint(), "SHA256:")
			coolifyKeys, _ := client.ListPrivateKeys()
			for _, ck := range coolifyKeys {
				if fp != "" && ck.Fingerprint == fp {
					jsonOK(w, map[string]any{"uuid": ck.UUID, "name": ck.Name, "already_existed": true})
					return
				}
			}
		}
		jsonError(w, http.StatusBadGateway, "failed to upload key: "+createErr.Error())
		return
	}

	jsonOK(w, map[string]any{"uuid": uuid, "name": keyName, "already_existed": false})
}

// handleDNSSync pulls every application/service domain from Coolify into
// dns_records, linked to the Bridge host running it.
//
//	@Summary		Sync DNS records from Coolify
//	@Description	Admin. Pulls every application/service domain from Coolify into dns_records, linked to the Bridge host running it and to the service the inventory sync stamped with that resource. 400 when the integration is disabled or unconfigured.
//	@Tags			coolify
//	@Produce		json
//	@Success		200	{object}	service.CoolifySyncSummary
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/dns-sync [post]
func (h *coolifyHandlers) handleDNSSync(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	apps, err := client.ListApplications()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify applications: "+err.Error())
		return
	}
	svcs, err := client.ListServices()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify services: "+err.Error())
		return
	}
	sum, err := h.dns.SyncFromCoolify(r.Context(), coolify.DomainRefs(apps, svcs, client.BaseHost()))
	if err != nil {
		jsonServerError(w, r, "failed to sync DNS from coolify", err)
		return
	}
	jsonOK(w, sum)
}

// coolifySyncResponse is the inventory sync plus the DNS sync it runs after.
type coolifySyncResponse struct {
	service.CoolifyInventorySummary
	DNS service.CoolifySyncSummary `json:"dns"`
}

// handleInventorySync stamps Coolify's resources onto Bridge services, then
// runs the DNS sync (which now also links each domain to its service).
//
//	@Summary		Sync the inventory from Coolify
//	@Description	Admin. Folds redeploy copies of containers, stamps each Coolify resource's project/environment/stack/repo (credentials stripped) on the services running it, creates offline placeholders for resources no scan has seen, fills hosts' Coolify server uuid, applies the project mapping, links APIs through their URLs' DNS, then syncs DNS (host and service links). 400 when the integration is disabled or unconfigured.
//	@Tags			coolify
//	@Produce		json
//	@Success		200	{object}	coolifySyncResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/sync [post]
func (h *coolifyHandlers) handleInventorySync(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	apps, err := client.ListApplications()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify applications: "+err.Error())
		return
	}
	svcs, err := client.ListServices()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify services: "+err.Error())
		return
	}
	dbs, err := client.ListDatabases()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify databases: "+err.Error())
		return
	}
	projects, err := client.ListProjects()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify projects: "+err.Error())
		return
	}
	invSvc := service.NewCoolifyInventoryService(h.db.SQL)
	inv, err := invSvc.Sync(r.Context(), coolify.Resources(apps, svcs, dbs, projects, client.BaseHost()))
	if err != nil {
		jsonServerError(w, r, "failed to sync inventory from coolify", err)
		return
	}
	dnsSum, err := h.dns.SyncFromCoolify(r.Context(), coolify.DomainRefs(apps, svcs, client.BaseHost()))
	if err != nil {
		jsonServerError(w, r, "failed to sync DNS from coolify", err)
		return
	}
	if inv.APIsLinked, err = invSvc.LinkAPIs(r.Context()); err != nil {
		jsonServerError(w, r, "failed to link APIs to services", err)
		return
	}
	jsonOK(w, coolifySyncResponse{CoolifyInventorySummary: inv, DNS: dnsSum})
}

// coolifyProjectOption is one Coolify project with its environment names.
type coolifyProjectOption struct {
	Name         string   `json:"name"`
	Environments []string `json:"environments"`
}

// handleListProjects lists Coolify projects and environments — the options of
// a Bridge project's Coolify mapping.
//
//	@Summary		List Coolify projects
//	@Description	Editor+. Coolify projects with their environment names, for the project ↔ Coolify mapping. 400 when the integration is disabled or unconfigured.
//	@Tags			coolify
//	@Produce		json
//	@Success		200	{array}		coolifyProjectOption
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/coolify/projects [get]
func (h *coolifyHandlers) handleListProjects(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	projects, err := client.ListProjects()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list coolify projects: "+err.Error())
		return
	}
	out := make([]coolifyProjectOption, 0, len(projects))
	for _, p := range projects {
		o := coolifyProjectOption{Name: p.Name, Environments: []string{}}
		for _, e := range p.Environments {
			o.Environments = append(o.Environments, e.Name)
		}
		out = append(out, o)
	}
	jsonOK(w, out)
}

// registerRoutes binds the Coolify integration: status/test plus the per-host
// and per-key check/register/sync operations.
func (h *coolifyHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/coolify/status", h.handleStatus)
	rr.role("admin", "POST /api/coolify/test", h.handleTestConnection)
	rr.role("admin", "POST /api/coolify/dns-sync", h.handleDNSSync)
	rr.role("admin", "POST /api/coolify/sync", h.handleInventorySync)
	rr.role("editor", "GET /api/coolify/projects", h.handleListProjects)
	rr.role("editor", "GET /api/coolify/server-status/{slug}", h.handleGetServerStatus)
	rr.role("editor", "POST /api/coolify/check/{slug}", h.handleCheckHost)
	rr.role("admin", "POST /api/coolify/register/{slug}", h.handleRegisterHost)
	rr.role("admin", "POST /api/coolify/validate/{slug}", h.handleValidateHost)
	rr.role("admin", "POST /api/coolify/sync/{slug}", h.handleSyncHost)
	rr.role("admin", "POST /api/coolify/server/{slug}/key", h.handleUpdateServerKey)
	rr.role("admin", "DELETE /api/coolify/server/{slug}", h.handleDeleteHost)
	rr.role("editor", "GET /api/coolify/keys/{id}/check", h.handleCheckKey)
	rr.role("admin", "POST /api/coolify/keys/{id}/sync", h.handleSyncKey)
}
