package api

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	gossh "golang.org/x/crypto/ssh"
)

type coolifyHandlers struct {
	db *database.DB
}

func (h *coolifyHandlers) getClient() (*coolify.Client, error) {
	get := func(key string) string { return models.GetAppSettingValue(h.db.SQL, key) }

	if get("coolify_enabled") != "true" {
		return nil, fmt.Errorf("coolify integration is not enabled")
	}

	baseURL := get("coolify_base_url")
	cipherHex := get("coolify_api_token_cipher")
	nonceHex := get("coolify_api_token_nonce")
	if cipherHex == "" || nonceHex == "" || baseURL == "" {
		return nil, fmt.Errorf("coolify integration is not configured")
	}
	cipher, _ := hex.DecodeString(cipherHex)
	nonce, _ := hex.DecodeString(nonceHex)
	apiToken, err := h.db.Encryptor.Decrypt(cipher, nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt coolify token: %w", err)
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
	if err := models.CreateOperationLog(h.db.SQL, ol); err != nil {
		log.Printf("[coolify] failed to log operation: %v", err)
	}
}

// handleStatus returns whether the Coolify integration is enabled and configured.
func (h *coolifyHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	enabled := models.GetAppSettingValue(h.db.SQL, "coolify_enabled") == "true"
	configured := models.GetAppSettingValue(h.db.SQL, "coolify_api_token_cipher") != ""
	jsonOK(w, map[string]any{
		"enabled":    enabled,
		"configured": configured,
	})
}

// handleTestConnection tests the Coolify connection using the healthcheck endpoint.
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
func (h *coolifyHandlers) handleGetServerStatus(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
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
func (h *coolifyHandlers) handleCheckHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
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
		models.SetHostCoolifyUUID(h.db.SQL, host.ID, &server.UUID)
		h.logOp(r, host.ID, "coolify-check", "success", fmt.Sprintf("found server %s (%s)", server.UUID, server.Name))
		jsonOK(w, map[string]any{"found": true, "server": server})
		return
	}

	h.logOp(r, host.ID, "coolify-check", "success", "server not found in coolify")
	jsonOK(w, map[string]any{"found": false})
}

// resolvePrivateKeyUUID finds or uploads a private key in Coolify and returns
// its UUID. Matching order: exact name, then fingerprint (both against the
// current key list), then create-and-handle-422 (re-list and fingerprint-match
// when Coolify reports the key already exists).
func (h *coolifyHandlers) resolvePrivateKeyUUID(client *coolify.Client, privKey, keyName, description string) (string, error) {
	existingKeys, listErr := client.ListPrivateKeys()
	if listErr != nil {
		log.Printf("[coolify] failed to list keys: %v", listErr)
	}
	log.Printf("[coolify] found %d existing keys in Coolify", len(existingKeys))

	for _, k := range existingKeys {
		log.Printf("[coolify] key: uuid=%s name=%q fingerprint=%q", k.UUID, k.Name, k.Fingerprint)
		if k.Name == keyName {
			log.Printf("[coolify] matched by name: %s", k.UUID)
			return k.UUID, nil
		}
	}

	ourFingerprint := ""
	if signer, err := gossh.ParsePrivateKey([]byte(privKey)); err == nil {
		ourFingerprint = strings.TrimPrefix(gossh.FingerprintSHA256(signer.PublicKey()), "SHA256:")
		log.Printf("[coolify] our key fingerprint: %s", ourFingerprint)
	} else {
		log.Printf("[coolify] failed to parse private key for fingerprint: %v", err)
	}

	if ourFingerprint != "" {
		for _, k := range existingKeys {
			if k.Fingerprint != "" && k.Fingerprint == ourFingerprint {
				log.Printf("[coolify] matched by fingerprint: %s (name=%q)", k.UUID, k.Name)
				return k.UUID, nil
			}
		}
	}

	uuid, createErr := client.CreatePrivateKey(coolify.CreateKeyRequest{
		Name:        keyName,
		Description: description,
		PrivateKey:  privKey,
	})
	if createErr == nil {
		log.Printf("[coolify] key created: %s", uuid)
		return uuid, nil
	}

	log.Printf("[coolify] key create failed: %v", createErr)
	if !strings.Contains(createErr.Error(), "422") && !strings.Contains(createErr.Error(), "already exists") {
		return "", createErr
	}
	log.Printf("[coolify] 422 duplicate — re-listing for fingerprint match, our fp=%q", ourFingerprint)
	freshKeys, _ := client.ListPrivateKeys()
	for _, k := range freshKeys {
		log.Printf("[coolify] re-list key: uuid=%s name=%q fingerprint=%q", k.UUID, k.Name, k.Fingerprint)
		if ourFingerprint != "" && k.Fingerprint == ourFingerprint {
			log.Printf("[coolify] matched on re-list by fingerprint: %s", k.UUID)
			return k.UUID, nil
		}
	}
	return "", createErr
}

// selectRegistrationKey resolves which sshcm SSH key should be uploaded to
// Coolify for a given host. Priority: explicit sshKeyID from the caller,
// then host_remote_users link for `targetUser`, then the host's own
// connection key (backward compatibility). Returns the decrypted private-key
// text, the Coolify key name to use, and a description.
func (h *coolifyHandlers) selectRegistrationKey(host *models.Host, sshKeyID int64, targetUser string) (privKey, keyName, description string, err error) {
	load := func(id int64) (*models.SSHKey, string, error) {
		k, gerr := models.GetSSHKey(h.db.SQL, id)
		if gerr != nil {
			return nil, "", gerr
		}
		if k == nil {
			return nil, "", fmt.Errorf("ssh key %d not found", id)
		}
		if len(k.PrivKeyCiphertext) == 0 {
			return nil, "", fmt.Errorf("ssh key %d has no private key stored", id)
		}
		plain, derr := h.db.Encryptor.Decrypt(k.PrivKeyCiphertext, k.PrivKeyNonce)
		if derr != nil {
			return nil, "", fmt.Errorf("decrypt ssh key %d: %w", id, derr)
		}
		return k, plain, nil
	}

	if sshKeyID > 0 {
		k, plain, lerr := load(sshKeyID)
		if lerr != nil {
			return "", "", "", lerr
		}
		return plain, coolifyManagedKeyName(k), fmt.Sprintf("Managed by SSHCM key %q", k.Name), nil
	}

	if targetUser != "" {
		if link, lerr := models.GetHostRemoteUserByUsername(h.db.SQL, host.ID, targetUser); lerr == nil && link != nil && link.SSHKeyID != nil {
			k, plain, lderr := load(*link.SSHKeyID)
			if lderr == nil {
				return plain, coolifyManagedKeyName(k), fmt.Sprintf("Managed by SSHCM key %q (remote user %s)", k.Name, targetUser), nil
			}
			log.Printf("[coolify] host_remote_users link present for host=%d user=%s key_id=%d but load failed: %v", host.ID, targetUser, *link.SSHKeyID, lderr)
		}
	}

	if len(host.PrivKeyCiphertext) == 0 {
		return "", "", "", fmt.Errorf("host has no private key stored and no linked ssh key available")
	}
	plain, derr := h.db.Encryptor.Decrypt(host.PrivKeyCiphertext, host.PrivKeyNonce)
	if derr != nil {
		return "", "", "", fmt.Errorf("decrypt host key: %w", derr)
	}
	return plain, fmt.Sprintf("sshcm-%s", host.OficialSlug), fmt.Sprintf("Managed by SSHCM for host %s", host.Nickname), nil
}

// coolifyManagedKeyName produces a Coolify-side name for an sshcm key that is
// shared across hosts. Kept distinct from the per-host `sshcm-<slug>` name so
// multiple hosts can reference the same uploaded key without colliding.
func coolifyManagedKeyName(k *models.SSHKey) string {
	safe := strings.Map(func(r rune) rune {
		if r == ' ' || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, k.Name)
	return "sshcm-key-" + safe
}

// handleRegisterHost uploads the chosen SSH key and creates a server in Coolify.
func (h *coolifyHandlers) handleRegisterHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	// Optional body; legacy callers POST with no payload.
	var req struct {
		SSHKeyID int64 `json:"ssh_key_id"`
	}
	_ = decodeJSON(r, &req)

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Coolify rejects usernames with dots. Use configured default user or "root".
	coolifyUser := models.GetAppSettingValue(h.db.SQL, "coolify_default_user")
	if coolifyUser == "" {
		coolifyUser = "root"
	}

	privKey, keyName, description, err := h.selectRegistrationKey(host, req.SSHKeyID, coolifyUser)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	privateKeyUUID, err := h.resolvePrivateKeyUUID(client, privKey, keyName, description)
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

	models.SetHostCoolifyUUID(h.db.SQL, host.ID, &serverUUID)
	h.logOp(r, host.ID, "coolify-register", "success", fmt.Sprintf("created server %s with key %s", serverUUID, privateKeyUUID))
	jsonOK(w, map[string]any{"uuid": serverUUID})
}

// handleValidateHost triggers Coolify server validation.
func (h *coolifyHandlers) handleValidateHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
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
func (h *coolifyHandlers) handleUpdateServerKey(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		jsonError(w, http.StatusBadRequest, "host is not linked to a coolify server")
		return
	}

	var req struct {
		SSHKeyID int64 `json:"ssh_key_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.SSHKeyID <= 0 {
		jsonError(w, http.StatusBadRequest, "ssh_key_id is required")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Pass empty targetUser so selectRegistrationKey uses only the explicit key
	// (no auto-resolution fallback — the caller asked for a specific key).
	privKey, keyName, description, err := h.selectRegistrationKey(host, req.SSHKeyID, "")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	privateKeyUUID, err := h.resolvePrivateKeyUUID(client, privKey, keyName, description)
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
func (h *coolifyHandlers) handleSyncHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
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

	coolifyUser := models.GetAppSettingValue(h.db.SQL, "coolify_default_user")
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
func (h *coolifyHandlers) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
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

	models.SetHostCoolifyUUID(h.db.SQL, host.ID, nil)
	h.logOp(r, host.ID, "coolify-delete", "success", "deleted server "+uuid)
	jsonOK(w, map[string]any{"success": true})
}

// handleCheckKey checks if a managed SSH key exists in Coolify by fingerprint.
func (h *coolifyHandlers) handleCheckKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	key, err := models.GetSSHKey(h.db.SQL, id)
	if err != nil || key == nil {
		jsonError(w, http.StatusNotFound, "key not found")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Compute fingerprint without SHA256: prefix to match Coolify format
	fp := strings.TrimPrefix(key.Fingerprint, "SHA256:")

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
func (h *coolifyHandlers) handleSyncKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	key, err := models.GetSSHKey(h.db.SQL, id)
	if err != nil || key == nil {
		jsonError(w, http.StatusNotFound, "key not found")
		return
	}
	if len(key.PrivKeyCiphertext) == 0 {
		jsonError(w, http.StatusBadRequest, "key has no private key stored")
		return
	}

	privKeyText, err := h.db.Encryptor.Decrypt(key.PrivKeyCiphertext, key.PrivKeyNonce)
	if err != nil {
		jsonServerError(w, r, "failed to decrypt key", err)
		return
	}

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
			fp := strings.TrimPrefix(key.Fingerprint, "SHA256:")
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

