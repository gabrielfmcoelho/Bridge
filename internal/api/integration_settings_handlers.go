package api

import (
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth/providers"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
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
		"auth_gitlab_base_url",
		"auth_gitlab_client_id",
		"auth_gitlab_client_secret",
		"gitlab_integration_enabled",
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
	"general": {
		"auth_active_provider",
		"auth_auto_provision",
		"auth_default_role",
		"auth_role_sync_enabled",
	},
}

// secretKeys are settings that need encryption (stored as _cipher/_nonce pairs).
var secretKeys = map[string]bool{
	"auth_ldap_bind_password":      true,
	"auth_keycloak_client_secret":  true,
	"auth_gitlab_client_secret":    true,
	"llm_api_key":                  true,
	"coolify_api_token":             true,
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
			// Skip if the value is the masked placeholder or empty.
			if value == "••••••••" || value == "" {
				if value == "" {
					// Clear the secret.
					models.SetAppSettingValue(h.db.SQL, key+"_cipher", "")
					models.SetAppSettingValue(h.db.SQL, key+"_nonce", "")
				}
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
