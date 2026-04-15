package api

import (
	"fmt"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type authHandlers struct {
	db       *database.DB
	registry *auth.ProviderRegistry
}

func (h *authHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	setupRequired, err := auth.SetupRequired(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}

	authenticated := false
	token := auth.GetSessionToken(r)
	if token != "" {
		if _, err := auth.ValidateSession(h.db.SQL, token); err == nil {
			authenticated = true
		}
	}

	// Build providers list for the login page.
	type providerInfo struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Label string `json:"label"`
		Icon  string `json:"icon"`
		Color string `json:"color"`
	}
	var providers []providerInfo
	if h.registry != nil {
		for _, p := range h.registry.EnabledProviders() {
			pType := "oauth"
			if p.SupportsDirectLogin() {
				pType = "direct"
			}
			info := p.DisplayInfo()
			providers = append(providers, providerInfo{
				Name:  p.Name(),
				Type:  pType,
				Label: info.Label,
				Icon:  info.Icon,
				Color: info.Color,
			})
		}
	}

	jsonOK(w, map[string]any{
		"setup_required": setupRequired,
		"authenticated":  authenticated,
		"providers":      providers,
	})
}

func (h *authHandlers) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := auth.SetupMasterUser(h.db.SQL, req.Username, req.Password, req.DisplayName)
	if err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}

	token, expiresAt, err := auth.CreateSession(h.db.SQL, user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	auth.SetSessionCookie(w, token, expiresAt)
	jsonCreated(w, map[string]any{
		"user":       user,
		"token":      token,
		"expires_at": expiresAt,
	})
}

func (h *authHandlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Provider string `json:"provider"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Provider == "" {
		req.Provider = "local"
	}

	var user *models.User

	if req.Provider == "local" {
		// Direct local auth (original behavior).
		u, err := auth.Login(h.db.SQL, req.Username, req.Password)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		user = u
	} else {
		// Delegate to the named provider.
		provider, ok := h.registry.Get(req.Provider)
		if !ok || !provider.Enabled() || !provider.SupportsDirectLogin() {
			jsonError(w, http.StatusBadRequest, "unsupported auth provider")
			return
		}
		identity, err := provider.Authenticate(r.Context(), req.Username, req.Password)
		if err != nil {
			// Fallback to local auth if configured (e.g., LDAP unreachable).
			fallbackKey := "auth_" + req.Provider + "_fallback_to_local"
			if models.GetAppSettingValue(h.db.SQL, fallbackKey) == "true" {
				u, localErr := auth.Login(h.db.SQL, req.Username, req.Password)
				if localErr == nil {
					user = u
				} else {
					// Both provider and local auth failed — return the original error.
					jsonError(w, http.StatusUnauthorized, "invalid credentials")
					return
				}
			} else {
				jsonError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
		} else {
			u, err := h.resolveOrProvisionUser(identity)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, "failed to resolve user")
				return
			}
			user = u
		}
	}

	token, expiresAt, err := auth.CreateSession(h.db.SQL, user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	auth.SetSessionCookie(w, token, expiresAt)
	jsonOK(w, map[string]any{
		"user":       user,
		"token":      token,
		"expires_at": expiresAt,
	})
}

// resolveOrProvisionUser looks up a local user by external identity, or auto-provisions one.
func (h *authHandlers) resolveOrProvisionUser(identity *auth.ExternalIdentity) (*models.User, error) {
	// Check if this external identity is already linked to a local user.
	existing, err := models.GetIdentityByProviderAndExternalID(h.db.SQL, identity.ProviderName, identity.ExternalID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		user, err := models.GetUserByID(h.db.SQL, existing.UserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, fmt.Errorf("linked user not found")
		}

		// Optionally sync role from external groups.
		h.syncExternalRole(user, identity)

		return user, nil
	}

	// Check if auto-provisioning is enabled.
	autoProvision := models.GetAppSettingValue(h.db.SQL, "auth_auto_provision")
	if autoProvision != "true" {
		return nil, fmt.Errorf("account not linked and auto-provisioning is disabled")
	}

	// Determine the default role.
	defaultRole := models.GetAppSettingValue(h.db.SQL, "auth_default_role")
	if defaultRole == "" {
		defaultRole = "viewer"
	}

	// Check if external groups map to a specific role.
	if len(identity.Groups) > 0 {
		mappedRole := models.ResolveRoleFromExternalGroups(h.db.SQL, identity.ProviderName, identity.Groups)
		if mappedRole != "" {
			defaultRole = mappedRole
		}
	}

	// Ensure unique username.
	username := identity.Username
	if username == "" {
		username = identity.ExternalID
	}
	username = h.ensureUniqueUsername(username)

	user := &models.User{
		Username:     username,
		PasswordHash: "!external", // unusable bcrypt hash
		DisplayName:  identity.DisplayName,
		Role:         defaultRole,
		AuthProvider: identity.ProviderName,
		Email:        identity.Email,
	}
	if err := models.CreateUser(h.db.SQL, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Link the external identity.
	link := &models.UserExternalIdentity{
		UserID:       user.ID,
		ProviderName: identity.ProviderName,
		ExternalID:   identity.ExternalID,
	}
	if err := models.CreateUserExternalIdentity(h.db.SQL, link); err != nil {
		return nil, fmt.Errorf("link identity: %w", err)
	}

	return user, nil
}

// syncExternalRole updates the user's role from external groups if role sync is enabled.
func (h *authHandlers) syncExternalRole(user *models.User, identity *auth.ExternalIdentity) {
	syncEnabled := models.GetAppSettingValue(h.db.SQL, "auth_role_sync_enabled")
	if syncEnabled != "true" || len(identity.Groups) == 0 {
		return
	}

	mappedRole := models.ResolveRoleFromExternalGroups(h.db.SQL, identity.ProviderName, identity.Groups)
	if mappedRole != "" && mappedRole != user.Role {
		user.Role = mappedRole
		models.UpdateUser(h.db.SQL, user)
	}
}

// ensureUniqueUsername appends a numeric suffix if the username already exists.
func (h *authHandlers) ensureUniqueUsername(username string) string {
	candidate := username
	suffix := 2
	for {
		existing, _ := models.GetUserByUsername(h.db.SQL, candidate)
		if existing == nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s.%d", username, suffix)
		suffix++
	}
}

func (h *authHandlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if token != "" {
		auth.DeleteSession(h.db.SQL, token)
	}
	auth.ClearSessionCookie(w)
	jsonOK(w, map[string]string{"status": "logged out"})
}

func (h *authHandlers) handleMe(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Fetch permissions for this user's role.
	permissions, _ := models.ListPermissionsForRole(h.db.SQL, user.Role)
	if user.Role == "admin" {
		// Admin gets all permissions.
		allPerms, _ := models.ListPermissions(h.db.SQL)
		permissions = make([]string, len(allPerms))
		for i, p := range allPerms {
			permissions[i] = p.Code
		}
	}
	if permissions == nil {
		permissions = []string{}
	}

	// Fetch external identities.
	identities, _ := models.ListIdentitiesByUser(h.db.SQL, user.ID)

	type identitySummary struct {
		Provider   string `json:"provider"`
		ExternalID string `json:"external_id"`
	}
	var extIDs []identitySummary
	for _, id := range identities {
		extIDs = append(extIDs, identitySummary{
			Provider:   id.ProviderName,
			ExternalID: id.ExternalID,
		})
	}
	if extIDs == nil {
		extIDs = []identitySummary{}
	}

	jsonOK(w, map[string]any{
		"id":                  user.ID,
		"username":            user.Username,
		"display_name":        user.DisplayName,
		"role":                user.Role,
		"auth_provider":       user.AuthProvider,
		"email":               user.Email,
		"permissions":         permissions,
		"external_identities": extIDs,
		"created_at":          user.CreatedAt,
		"updated_at":          user.UpdatedAt,
	})
}

// User management (admin only)

func (h *authHandlers) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := models.ListUsers(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	jsonOK(w, users)
}

func (h *authHandlers) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	u := &models.User{
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		Role:         req.Role,
	}
	if err := models.CreateUser(h.db.SQL, u); err != nil {
		jsonError(w, http.StatusConflict, "username already exists")
		return
	}

	jsonCreated(w, u)
}

func (h *authHandlers) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := models.GetUserByID(h.db.SQL, id)
	if err != nil || user == nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if err := models.UpdateUser(h.db.SQL, user); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		if err := models.UpdateUserPassword(h.db.SQL, id, hash); err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to update password")
			return
		}
	}

	jsonOK(w, user)
}

func (h *authHandlers) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Prevent deleting yourself.
	me := auth.UserFromContext(r.Context())
	if me != nil && me.ID == id {
		jsonError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := models.DeleteUser(h.db.SQL, id); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
