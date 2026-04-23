package api

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth/providers"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	grafanaclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/grafana"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	outlineclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/outline"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type integrationSettingsHandlers struct {
	db       *database.DB
	registry *auth.ProviderRegistry
}

// Prefixes for each integration group.
var integrationGroups = map[string][]string{
	"ldap": {
		"auth_ldap_host",
		"auth_ldap_port",
		"auth_ldap_use_tls",
		"auth_ldap_skip_verify",
		"auth_ldap_base_dn",
		"auth_ldap_bind_dn",
		"auth_ldap_bind_password",
		"auth_ldap_user_filter",
		"auth_ldap_username_attr",
		"auth_ldap_display_name_attr",
		"auth_ldap_email_attr",
		"auth_ldap_fallback_to_local",
	},
	"gitlab": {
		"auth_gitlab_enabled",
		"auth_gitlab_base_url",
		"auth_gitlab_client_id",
		"auth_gitlab_client_secret",
		"gitlab_integration_enabled",
		"gitlab_code_service_token",
		"gitlab_code_default_ref",
	},
	"keycloak": {
		"auth_keycloak_base_url",
		"auth_keycloak_realm",
		"auth_keycloak_client_id",
		"auth_keycloak_client_secret",
	},
	"llm": {
		"llm_enabled",
		"llm_base_url",
		"llm_api_key",
		"llm_model_text",
		"llm_model_vision",
		"llm_max_tokens",
	},
	"coolify": {
		"coolify_enabled",
		"coolify_base_url",
		"coolify_api_token",
		"coolify_default_user",
	},
	"grafana": {
		"grafana_enabled",
		"grafana_base_url",
		"grafana_api_token",
		"grafana_webhook_secret",
		"grafana_host_default_dashboard_uid",
		"grafana_service_default_dashboard_uid",
		"grafana_prom_remote_write_url",
		"grafana_prom_remote_write_username",
		"grafana_prom_remote_write_password",
		"grafana_datasource_uid",
	},
	"outline": {
		"outline_enabled",
		"outline_base_url",
		"outline_api_token",
		"outline_common_collection_id",
	},
	"glpi": {
		"glpi_enabled",
		"glpi_base_url",
		"glpi_app_token",
		"glpi_default_entity_id",
	},
	"general": {
		"auth_active_provider",
		"auth_auto_provision",
		"auth_default_role",
		"auth_role_sync_enabled",
	},
}

// secretKeys are settings that need encryption (stored as _cipher/_nonce pairs).
var secretKeys = map[string]bool{
	"auth_ldap_bind_password":              true,
	"auth_keycloak_client_secret":          true,
	"auth_gitlab_client_secret":            true,
	"gitlab_code_service_token":            true,
	"llm_api_key":                          true,
	"coolify_api_token":                    true,
	"grafana_api_token":                    true,
	"grafana_webhook_secret":               true,
	"grafana_prom_remote_write_password":   true,
	"outline_api_token":                    true,
	"glpi_app_token":                       true,
}

// handleGetIntegrations returns all integration settings grouped by provider.
func (h *integrationSettingsHandlers) handleGetIntegrations(w http.ResponseWriter, r *http.Request) {
	result := make(map[string]map[string]string)

	for group, keys := range integrationGroups {
		settings := make(map[string]string)
		for _, key := range keys {
			if secretKeys[key] {
				// For secrets, just indicate whether a value is set (never return the actual value).
				cipher := models.GetAppSettingValue(h.db.SQL, key+"_cipher")
				if cipher != "" {
					settings[key] = "••••••••"
				} else {
					settings[key] = ""
				}
			} else {
				settings[key] = models.GetAppSettingValue(h.db.SQL, key)
			}
		}
		result[group] = settings
	}

	jsonOK(w, result)
}

// handleUpdateIntegrationGroup updates settings for a specific integration group.
func (h *integrationSettingsHandlers) handleUpdateIntegrationGroup(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	keys, ok := integrationGroups[group]
	if !ok {
		jsonError(w, http.StatusBadRequest, "unknown integration group")
		return
	}

	var req map[string]string
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}

	// Only allow updating keys in this group.
	allowedKeys := make(map[string]bool)
	for _, k := range keys {
		allowedKeys[k] = true
	}

	for key, value := range req {
		if !allowedKeys[key] {
			continue
		}

		if secretKeys[key] {
			// Both empty and the masked placeholder mean "no change". Secrets are
			// only cleared via DELETE /api/settings/integrations/{group}/secret/{key}
			// — an admin removing bullets from the input must NOT silently wipe
			// the stored cipher (past incident broke SSO in prod).
			if value == "" || value == "••••••••" {
				continue
			}
			// Encrypt the secret value.
			cipher, nonce, err := h.db.Encryptor.Encrypt(value)
			if err != nil {
				jsonServerError(w, r, "failed to encrypt "+key, err)
				return
			}
			models.SetAppSettingValue(h.db.SQL, key+"_cipher", hex.EncodeToString(cipher))
			models.SetAppSettingValue(h.db.SQL, key+"_nonce", hex.EncodeToString(nonce))
		} else {
			models.SetAppSettingValue(h.db.SQL, key, value)
		}
	}

	jsonOK(w, map[string]string{"status": "updated"})
}

// handleTestLDAP tests the LDAP connection with current saved settings.
func (h *integrationSettingsHandlers) handleTestLDAP(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.registry.Get("ldap")
	if !ok {
		jsonError(w, http.StatusBadRequest, "LDAP provider not registered")
		return
	}

	ldapProvider, ok := provider.(*providers.LDAPProvider)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "invalid LDAP provider type")
		return
	}

	if err := ldapProvider.TestConnection(); err != nil {
		errMsg := err.Error()
		// Strip sensitive details but keep useful info.
		if strings.Contains(errMsg, "connection refused") {
			errMsg = "Connection refused — check host and port"
		} else if strings.Contains(errMsg, "no such host") {
			errMsg = "Host not found — check hostname"
		} else if strings.Contains(errMsg, "bind failed") {
			errMsg = "Bind failed — check bind DN and password"
		}
		jsonOK(w, map[string]any{"success": false, "error": errMsg})
		return
	}

	jsonOK(w, map[string]any{"success": true})
}

// handleClearIntegrationSecret wipes the cipher/nonce pair for one secret key.
// This is the ONLY path that can zero a stored secret — the save handler never clears.
func (h *integrationSettingsHandlers) handleClearIntegrationSecret(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	key := r.PathValue("key")

	keys, ok := integrationGroups[group]
	if !ok {
		jsonError(w, http.StatusBadRequest, "unknown integration group")
		return
	}
	inGroup := false
	for _, k := range keys {
		if k == key {
			inGroup = true
			break
		}
	}
	if !inGroup || !secretKeys[key] {
		jsonError(w, http.StatusBadRequest, "key is not a clearable secret in this group")
		return
	}

	models.SetAppSettingValue(h.db.SQL, key+"_cipher", "")
	models.SetAppSettingValue(h.db.SQL, key+"_nonce", "")
	jsonOK(w, map[string]string{"status": "cleared"})
}

// handleTestOutline verifies the stored (or caller-supplied) Outline base URL + API
// token by calling POST /api/auth.info. Returns the authenticated Outline user and
// workspace name on success so admins can sanity-check which identity they wired.
func (h *integrationSettingsHandlers) handleTestOutline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL string `json:"base_url"`
		Token   string `json:"token"`
	}
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Token = strings.TrimSpace(req.Token)

	baseURL := req.BaseURL
	token := req.Token

	if baseURL == "" || token == "" {
		stored, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
		if err != nil {
			jsonOK(w, map[string]any{"success": false, "error": "failed to read settings"})
			return
		}
		if baseURL == "" {
			baseURL = stored.BaseURL
		}
		if token == "" {
			token = stored.APIToken
		}
	}
	if baseURL == "" {
		jsonOK(w, map[string]any{"success": false, "error": "Base URL not configured"})
		return
	}
	if token == "" {
		jsonOK(w, map[string]any{"success": false, "error": "API token not configured"})
		return
	}

	client := outlineclient.NewClient(baseURL, token)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	info, err := client.AuthInfo(ctx)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": mapOutlineError(err)})
		return
	}

	jsonOK(w, map[string]any{
		"success":        true,
		"user":           info.User.Name,
		"user_email":     info.User.Email,
		"workspace":      info.Team.Name,
		"workspace_url":  info.Team.URL,
	})
}

// mapOutlineError converts a raw Outline client error into a user-friendly message.
func mapOutlineError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"):
		return "Authentication failed — check API token"
	case strings.Contains(msg, "404"):
		return "Endpoint not found — check Base URL"
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "connection refused"):
		return "Host unreachable — check Base URL"
	case strings.Contains(msg, "context deadline exceeded"):
		return "Timed out — Outline took too long to respond"
	}
	return msg
}

// handleTestGrafana probes Grafana with the stored (or caller-supplied) base URL + API
// token, verifying reachability and that the token resolves to a real user/service
// account. Mirrors handleTestGitLabCode's UX: accept unsaved form values in the body,
// fall back to stored settings for anything empty, and never alter persisted secrets.
func (h *integrationSettingsHandlers) handleTestGrafana(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL string `json:"base_url"`
		Token   string `json:"token"`
	}
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Token = strings.TrimSpace(req.Token)

	baseURL := req.BaseURL
	token := req.Token

	if baseURL == "" || token == "" {
		stored, err := grafanaclient.LoadSettings(h.db.SQL, h.db.Encryptor)
		if err != nil {
			jsonOK(w, map[string]any{"success": false, "error": "failed to read settings"})
			return
		}
		if baseURL == "" {
			baseURL = stored.BaseURL
		}
		if token == "" {
			token = stored.APIToken
		}
	}

	if baseURL == "" {
		jsonOK(w, map[string]any{"success": false, "error": "Base URL not configured"})
		return
	}
	if token == "" {
		jsonOK(w, map[string]any{"success": false, "error": "API token not configured"})
		return
	}

	client := grafanaclient.NewClient(baseURL, token)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Health first (cheap, validates URL); then CurrentUser (validates token).
	health, err := client.Health(ctx)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": mapGrafanaError(err), "stage": "health"})
		return
	}
	user, err := client.CurrentUser(ctx)
	if err != nil {
		jsonOK(w, map[string]any{
			"success": false,
			"error":   mapGrafanaError(err),
			"stage":   "auth",
			"version": health.Version,
		})
		return
	}

	jsonOK(w, map[string]any{
		"success":  true,
		"version":  health.Version,
		"database": health.Database,
		"user":     user.Login,
		"name":     user.Name,
		"org_id":   user.OrgID,
	})
}

// mapGrafanaError converts a raw HTTP error from the Grafana client into a
// user-friendly message for the settings UI.
func mapGrafanaError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"):
		return "Authentication failed — check API token"
	case strings.Contains(msg, "404"):
		return "Endpoint not found — check Base URL"
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "connection refused"):
		return "Host unreachable — check Base URL"
	case strings.Contains(msg, "context deadline exceeded"):
		return "Timed out — Grafana took too long to respond"
	}
	return msg
}

// handleTestLLM checks the LLM integration by listing available models against
// the OpenAI-compatible endpoint. Accepts optional overrides in the request body
// so admins can validate unsaved form values; empty fields fall back to stored settings.
func (h *integrationSettingsHandlers) handleTestLLM(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Model = strings.TrimSpace(req.Model)

	baseURL := req.BaseURL
	apiKey := req.APIKey
	model := req.Model

	get := func(key string) string { return models.GetAppSettingValue(h.db.SQL, key) }
	if baseURL == "" {
		baseURL = get("llm_base_url")
	}
	if model == "" {
		model = get("llm_model_text")
	}
	if apiKey == "" {
		cipherHex := get("llm_api_key_cipher")
		nonceHex := get("llm_api_key_nonce")
		if cipherHex != "" && nonceHex != "" {
			cipher, err1 := hex.DecodeString(cipherHex)
			nonce, err2 := hex.DecodeString(nonceHex)
			if err1 == nil && err2 == nil {
				if decrypted, err := h.db.Encryptor.Decrypt(cipher, nonce); err == nil {
					apiKey = decrypted
				}
			}
		}
	}

	if baseURL == "" {
		jsonOK(w, map[string]any{"success": false, "error": "Base URL not configured"})
		return
	}
	if apiKey == "" {
		jsonOK(w, map[string]any{"success": false, "error": "API key not configured"})
		return
	}

	// Generous max_tokens — reasoning models spend tokens "thinking" before
	// producing visible output, so very low caps can yield empty replies on
	// an otherwise-healthy provider.
	client := llm.NewClient(baseURL, apiKey, model, 64)

	// Step 1: /models — validates URL + auth cheaply.
	modelsCtx, cancelModels := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancelModels()
	ids, err := client.ListModels(modelsCtx)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": mapLLMError(err), "stage": "models"})
		return
	}

	modelAvailable := false
	if model != "" {
		for _, id := range ids {
			if id == model {
				modelAvailable = true
				break
			}
		}
	}

	// Step 2: tiny chat completion — proves the chat endpoint + configured model
	// actually generate text. Keep this bounded at ~45s because reasoning models
	// can take a while even for trivial prompts.
	if model == "" {
		jsonOK(w, map[string]any{
			"success":         true,
			"models_count":    len(ids),
			"model":           "",
			"model_available": false,
			"chat_ok":         false,
			"warning":         "No text model configured — skipped chat test.",
		})
		return
	}

	chatCtx, cancelChat := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancelChat()
	reply, err := client.Complete(chatCtx, []llm.ChatMessage{
		{Role: "user", Content: "Reply with only the word OK."},
	})
	if err != nil {
		jsonOK(w, map[string]any{
			"success":         false,
			"error":           mapLLMError(err),
			"stage":           "chat",
			"models_count":    len(ids),
			"model":           model,
			"model_available": modelAvailable,
		})
		return
	}

	// Trim + truncate the reply for display — reasoning models can dump
	// unexpectedly long rationales even for trivial prompts.
	reply = strings.TrimSpace(reply)
	if len(reply) > 120 {
		reply = reply[:117] + "…"
	}

	jsonOK(w, map[string]any{
		"success":         true,
		"models_count":    len(ids),
		"model":           model,
		"model_available": modelAvailable,
		"chat_ok":         true,
		"chat_reply":      reply,
	})
}

// mapLLMError converts a raw LLM client error into a user-friendly message.
func mapLLMError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"):
		return "Authentication failed — check API key"
	case strings.Contains(msg, "404"):
		return "Endpoint not found — check Base URL (include /v1 if required)"
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "connection refused"):
		return "Host unreachable — check Base URL"
	case strings.Contains(msg, "context deadline exceeded"):
		return "Timed out — provider took too long to respond"
	}
	return msg
}

// handleTestGitLabCode verifies a GitLab service PAT by hitting /user. Callers may
// pass {base_url, token} in the request body to test unsaved form values; empty
// fields fall back to the encrypted settings already stored in app_settings.
func (h *integrationSettingsHandlers) handleTestGitLabCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL string `json:"base_url"`
		Token   string `json:"token"`
	}
	// Body is optional — an empty request tests stored settings.
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Token = strings.TrimSpace(req.Token)

	baseURL := req.BaseURL
	token := req.Token

	if baseURL == "" || token == "" {
		stored, err := gitlabclient.LoadSettings(h.db.SQL, h.db.Encryptor)
		if err != nil {
			jsonOK(w, map[string]any{"success": false, "error": "failed to read settings"})
			return
		}
		if baseURL == "" {
			baseURL = stored.BaseURL
		}
		if token == "" {
			token = stored.ServiceToken
		}
	}

	if token == "" {
		jsonOK(w, map[string]any{"success": false, "error": "No service token configured"})
		return
	}

	client := gitlabclient.NewClient(baseURL, token)
	user, err := client.GetCurrentUser()
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "401") {
			errMsg = "Authentication failed — token invalid or revoked"
		} else if strings.Contains(errMsg, "403") {
			errMsg = "Token lacks required scope (needs read_api)"
		}
		jsonOK(w, map[string]any{"success": false, "error": errMsg})
		return
	}

	jsonOK(w, map[string]any{
		"success":  true,
		"username": user.Username,
		"name":     user.Name,
	})
}

// --- Permissions management ---

// handleGetPermissions returns all permissions and the role-permission matrix.
func (h *integrationSettingsHandlers) handleGetPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := models.ListPermissions(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list permissions", err)
		return
	}

	rolePerms, err := models.ListAllRolePermissions(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list role permissions", err)
		return
	}

	// Build a map: role -> set of permissions
	matrix := make(map[string][]string)
	for _, rp := range rolePerms {
		matrix[rp.Role] = append(matrix[rp.Role], rp.Permission)
	}

	jsonOK(w, map[string]any{
		"permissions": perms,
		"matrix":      matrix,
	})
}

// handleUpdatePermissions replaces the permission set for a role.
func (h *integrationSettingsHandlers) handleUpdatePermissions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if req.Role == "" {
		jsonError(w, http.StatusBadRequest, "role is required")
		return
	}
	// Prevent editing admin permissions via this endpoint.
	if req.Role == "admin" {
		jsonError(w, http.StatusForbidden, "admin permissions cannot be modified")
		return
	}

	if err := models.SetRolePermissions(h.db.SQL, req.Role, req.Permissions); err != nil {
		jsonServerError(w, r, "failed to update permissions", err)
		return
	}

	jsonOK(w, map[string]string{"status": "updated"})
}

// --- Role mappings management ---

// handleGetRoleMappings returns all external group-to-role mappings.
func (h *integrationSettingsHandlers) handleGetRoleMappings(w http.ResponseWriter, r *http.Request) {
	mappings, err := models.ListAuthRoleMappings(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list role mappings", err)
		return
	}
	if mappings == nil {
		mappings = []models.AuthRoleMapping{}
	}
	jsonOK(w, mappings)
}

// handleCreateRoleMapping creates a new external group-to-role mapping.
func (h *integrationSettingsHandlers) handleCreateRoleMapping(w http.ResponseWriter, r *http.Request) {
	var req models.AuthRoleMapping
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if req.ProviderName == "" || req.ExternalGroup == "" || req.LocalRole == "" {
		jsonError(w, http.StatusBadRequest, "provider_name, external_group, and local_role are required")
		return
	}

	if err := models.CreateAuthRoleMapping(h.db.SQL, &req); err != nil {
		jsonError(w, http.StatusConflict, "mapping already exists or failed to create")
		return
	}

	jsonCreated(w, req)
}

// handleDeleteRoleMapping deletes a role mapping by ID.
func (h *integrationSettingsHandlers) handleDeleteRoleMapping(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid mapping id", err)
		return
	}

	if err := models.DeleteAuthRoleMapping(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete mapping", err)
		return
	}

	jsonOK(w, map[string]string{"status": "deleted"})
}
