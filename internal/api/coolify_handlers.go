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

// handleRegisterHost uploads the host's SSH key and creates a server in Coolify.
func (h *coolifyHandlers) handleRegisterHost(w http.ResponseWriter, r *http.Request) {
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

	// Decrypt the host's private key
	if len(host.PrivKeyCiphertext) == 0 {
		jsonError(w, http.StatusBadRequest, "host has no private key stored")
		return
	}
	privKey, err := h.db.Encryptor.Decrypt(host.PrivKeyCiphertext, host.PrivKeyNonce)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to decrypt host key")
		return
	}

	// Resolve private key UUID in Coolify: try name match, then create, then fingerprint match on 422.
	keyName := fmt.Sprintf("sshcm-%s", host.OficialSlug)
	var privateKeyUUID string

	existingKeys, listErr := client.ListPrivateKeys()
	if listErr != nil {
		log.Printf("[coolify] failed to list keys: %v", listErr)
	}
	log.Printf("[coolify] found %d existing keys in Coolify", len(existingKeys))
	for _, k := range existingKeys {
		log.Printf("[coolify] key: uuid=%s name=%q fingerprint=%q", k.UUID, k.Name, k.Fingerprint)
		if k.Name == keyName {
			privateKeyUUID = k.UUID
			log.Printf("[coolify] matched by name: %s", privateKeyUUID)
			break
		}
	}

	// Compute our key's fingerprint for matching (strip SHA256: prefix to match Coolify format)
	ourFingerprint := ""
	if signer, err := gossh.ParsePrivateKey([]byte(privKey)); err == nil {
		ourFingerprint = strings.TrimPrefix(gossh.FingerprintSHA256(signer.PublicKey()), "SHA256:")
		log.Printf("[coolify] our key fingerprint: %s", ourFingerprint)
	} else {
		log.Printf("[coolify] failed to parse our private key for fingerprint: %v", err)
	}

	// If no name match, try fingerprint match before attempting create
	if privateKeyUUID == "" && ourFingerprint != "" {
		for _, k := range existingKeys {
			if k.Fingerprint != "" && k.Fingerprint == ourFingerprint {
				privateKeyUUID = k.UUID
				log.Printf("[coolify] matched by fingerprint: %s (name=%q)", privateKeyUUID, k.Name)
				break
			}
		}
	}

	if privateKeyUUID == "" {
		uuid, createErr := client.CreatePrivateKey(coolify.CreateKeyRequest{
			Name:        keyName,
			Description: fmt.Sprintf("Managed by SSHCM for host %s", host.Nickname),
			PrivateKey:  privKey,
		})
		if createErr != nil {
			log.Printf("[coolify] key create failed: %v", createErr)
			// 422 = duplicate content. Try fingerprint match as last resort.
			if strings.Contains(createErr.Error(), "422") || strings.Contains(createErr.Error(), "already exists") {
				log.Printf("[coolify] 422 duplicate — fingerprint match had %d candidates, our fp=%q", len(existingKeys), ourFingerprint)
				// Re-list in case the first list was stale or empty
				freshKeys, _ := client.ListPrivateKeys()
				for _, k := range freshKeys {
					log.Printf("[coolify] re-list key: uuid=%s name=%q fingerprint=%q", k.UUID, k.Name, k.Fingerprint)
					if ourFingerprint != "" && k.Fingerprint == ourFingerprint {
						privateKeyUUID = k.UUID
						log.Printf("[coolify] matched on re-list by fingerprint: %s", privateKeyUUID)
						break
					}
				}
			}
			if privateKeyUUID == "" {
				h.logOp(r, host.ID, "coolify-register", "failed", "key upload: "+createErr.Error())
				jsonError(w, http.StatusBadGateway, "failed to upload key to coolify: "+createErr.Error())
				return
			}
		} else {
			privateKeyUUID = uuid
			log.Printf("[coolify] key created: %s", uuid)
		}
	}

	// Parse port
	port := 22
	if p, err := strconv.Atoi(host.Port); err == nil && p > 0 {
		port = p
	}

	// Coolify rejects usernames with dots. Use configured default user or "root".
	coolifyUser := models.GetAppSettingValue(h.db.SQL, "coolify_default_user")
	if coolifyUser == "" {
		coolifyUser = "root"
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
		jsonError(w, http.StatusBadRequest, "invalid id")
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
		jsonError(w, http.StatusBadRequest, "invalid id")
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
		jsonError(w, http.StatusInternalServerError, "failed to decrypt key")
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

