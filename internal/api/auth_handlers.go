package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type authHandlers struct {
	db       *database.DB
	registry *auth.ProviderRegistry
}

// handleStatus godoc
//
//	@Summary		Auth status
//	@Description	Public, no auth. Answers {"setup_required": bool, "authenticated": bool, "providers": [{name, type ("oauth" or "direct"), label, icon, color}]} for the login page.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/api/auth/status [get]
func (h *authHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	setupRequired, err := auth.SetupRequired(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to check setup status", err)
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

// authSetupRequest is the first-run master-user body.
type authSetupRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// handleSetup godoc
//
//	@Summary		Create the first admin
//	@Description	Public, no auth. Only works before any user exists (409 afterwards). Sets the session cookie and answers {"user": models.User, "token": string, "expires_at": time}.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		authSetupRequest	true	"Master user credentials"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/auth/setup [post]
func (h *authHandlers) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req authSetupRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := auth.SetupMasterUser(h.db.SQL, req.Username, req.Password, req.DisplayName)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "could not complete setup (already initialized?)", err)
		return
	}

	token, expiresAt, err := auth.CreateSession(h.db.SQL, user.ID)
	if errors.Is(err, auth.ErrServiceAccount) {
		jsonError(w, http.StatusForbidden, "service accounts cannot sign in")
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to create session", err)
		return
	}

	auth.SetSessionCookie(w, token, expiresAt)
	jsonCreated(w, map[string]any{
		"user":       user,
		"token":      token,
		"expires_at": expiresAt,
	})
}

// authLoginRequest is the login body; provider defaults to "local".
type authLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Provider string `json:"provider"`
}

// handleLogin godoc
//
//	@Summary		Log in
//	@Description	Public, no auth. provider defaults to "local"; other providers must be enabled and support direct login (e.g. LDAP, optionally falling back to local). Unknown external identities are auto-provisioned when enabled. Sets the session cookie and answers {"user": models.User, "token": string, "expires_at": time}.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		authLoginRequest	true	"Credentials and optional provider"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Router			/api/auth/login [post]
func (h *authHandlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
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
			if store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), fallbackKey) == "true" {
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
				jsonServerError(w, r, "failed to resolve user", err)
				return
			}
			user = u
		}
	}

	token, expiresAt, err := auth.CreateSession(h.db.SQL, user.ID)
	if errors.Is(err, auth.ErrServiceAccount) {
		jsonError(w, http.StatusForbidden, "service accounts cannot sign in")
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to create session", err)
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
	existing, err := store.NewUserIdentityRepo(h.db.SQL).GetByProviderAndExternalID(context.Background(), identity.ProviderName, identity.ExternalID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		user, err := store.NewUserRepo(h.db.SQL).GetByID(context.Background(), existing.UserID)
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
	autoProvision := store.NewAppSettingsRepo(h.db.SQL).Value(context.Background(), "auth_auto_provision")
	if autoProvision != "true" {
		return nil, fmt.Errorf("account not linked and auto-provisioning is disabled")
	}

	// Determine the default role.
	defaultRole := store.NewAppSettingsRepo(h.db.SQL).Value(context.Background(), "auth_default_role")
	if defaultRole == "" {
		defaultRole = "viewer"
	}

	// Check if external groups map to a specific role.
	if len(identity.Groups) > 0 {
		mappedRole := store.NewPermissionRepo(h.db.SQL).ResolveRoleFromExternalGroups(context.Background(), identity.ProviderName, identity.Groups)
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
	if err := store.NewUserRepo(h.db.SQL).Create(context.Background(), user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Link the external identity.
	link := &models.UserExternalIdentity{
		UserID:       user.ID,
		ProviderName: identity.ProviderName,
		ExternalID:   identity.ExternalID,
	}
	if err := store.NewUserIdentityRepo(h.db.SQL).Create(context.Background(), link); err != nil {
		return nil, fmt.Errorf("link identity: %w", err)
	}

	return user, nil
}

// syncExternalRole updates the user's role from external groups if role sync is enabled.
func (h *authHandlers) syncExternalRole(user *models.User, identity *auth.ExternalIdentity) {
	syncEnabled := store.NewAppSettingsRepo(h.db.SQL).Value(context.Background(), "auth_role_sync_enabled")
	if syncEnabled != "true" || len(identity.Groups) == 0 {
		return
	}

	mappedRole := store.NewPermissionRepo(h.db.SQL).ResolveRoleFromExternalGroups(context.Background(), identity.ProviderName, identity.Groups)
	if mappedRole != "" && mappedRole != user.Role {
		user.Role = mappedRole
		store.NewUserRepo(h.db.SQL).Update(context.Background(), user)
	}
}

// ensureUniqueUsername appends a numeric suffix if the username already exists.
func (h *authHandlers) ensureUniqueUsername(username string) string {
	candidate := username
	suffix := 2
	for {
		existing, _ := store.NewUserRepo(h.db.SQL).GetByUsername(context.Background(), candidate)
		if existing == nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s.%d", username, suffix)
		suffix++
	}
}

// handleLogout godoc
//
//	@Summary		Log out
//	@Description	Any role. Deletes the session and clears the cookie.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	StatusResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/auth/logout [post]
func (h *authHandlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if token != "" {
		auth.DeleteSession(h.db.SQL, token)
	}
	auth.ClearSessionCookie(w)
	jsonOK(w, map[string]string{"status": "logged out"})
}

// handleMe godoc
//
//	@Summary		Current user
//	@Description	Any role. The caller's profile plus "permissions" (codes; every code for admins), "external_identities" [{provider, external_id}] and "entidades" (memberships).
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/auth/me [get]
func (h *authHandlers) handleMe(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Fetch permissions for this user's role.
	permissions, _ := store.NewPermissionRepo(h.db.SQL).ListForRole(r.Context(), user.Role)
	if user.Role == "admin" {
		// Admin gets all permissions.
		allPerms, _ := store.NewPermissionRepo(h.db.SQL).ListPermissions(r.Context())
		permissions = make([]string, len(allPerms))
		for i, p := range allPerms {
			permissions[i] = p.Code
		}
	}
	if permissions == nil {
		permissions = []string{}
	}

	// Fetch external identities.
	identities, _ := store.NewUserIdentityRepo(h.db.SQL).ListByUser(r.Context(), user.ID)

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

	entidades, _ := store.NewUserEntidadeRepo(h.db.SQL).ListForUser(r.Context(), user.ID)
	if entidades == nil {
		entidades = []models.UserEntidade{}
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
		"entidades":           entidades,
		"created_at":          user.CreatedAt,
		"updated_at":          user.UpdatedAt,
	})
}

// userWithEntidades is the /api/users row: the user plus their memberships.
type userWithEntidades struct {
	models.User
	Entidades []models.UserEntidade `json:"entidades"`
}

// applyUserEntidades replaces the user's memberships when the request carried
// entidade_ids (nil = leave untouched). Returns false after writing a 400 on a
// bad primary.
func (h *authHandlers) applyUserEntidades(w http.ResponseWriter, r *http.Request, userID int64, ids *[]int64, primary *int64) bool {
	if ids == nil {
		return true
	}
	var p int64
	if primary != nil {
		p = *primary
	}
	if err := store.NewUserEntidadeRepo(h.db.SQL).Replace(r.Context(), userID, *ids, p); err != nil {
		jsonBadRequest(w, r, "invalid entidade_ids / primary_entidade_id", err)
		return false
	}
	return true
}

// User management (admin only)

// handleListUsers godoc
//
//	@Summary		List users
//	@Description	Admin. Each user with their entidade memberships.
//	@Tags			users
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[userWithEntidades]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/users [get]
func (h *authHandlers) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := store.NewUserRepo(h.db.SQL).List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list users", err)
		return
	}
	memberships, _ := store.NewUserEntidadeRepo(h.db.SQL).ListBulk(r.Context())
	out := make([]userWithEntidades, len(users))
	for i, u := range users {
		ents := memberships[u.ID]
		if ents == nil {
			ents = []models.UserEntidade{}
		}
		out[i] = userWithEntidades{User: u, Entidades: ents}
	}
	jsonPaged(w, r, out)
}

// userCreateRequest is the admin create-user body; role defaults to viewer.
// kind "service" creates a service account: no password, never signs in, owns
// API tokens an admin issues for an integration.
type userCreateRequest struct {
	Kind              string   `json:"kind" example:"person"`
	Username          string   `json:"username"`
	Password          string   `json:"password"`
	DisplayName       string   `json:"display_name"`
	Role              string   `json:"role"`
	EntidadeIDs       *[]int64 `json:"entidade_ids"`
	PrimaryEntidadeID *int64   `json:"primary_entidade_id"`
}

// handleCreateUser godoc
//
//	@Summary		Create a user
//	@Description	Admin. Local user; role defaults to viewer. kind "service" makes a service account (no password; it can't sign in and only authenticates with API tokens admins issue for it). 409 when the username exists.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			body	body		userCreateRequest	true	"User, password and optional entidade memberships"
//	@Success		201		{object}	models.User
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/users [post]
func (h *authHandlers) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req userCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Kind == "" {
		req.Kind = models.UserKindPerson
	}
	if req.Kind != models.UserKindPerson && req.Kind != models.UserKindService {
		jsonError(w, http.StatusBadRequest, "kind must be person or service")
		return
	}
	service := req.Kind == models.UserKindService
	if req.Username == "" || (req.Password == "" && !service) {
		jsonError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}

	// A service account keeps an empty hash: no password ever matches it, and
	// CreateSession refuses it anyway.
	hash := ""
	if !service {
		var err error
		if hash, err = auth.HashPassword(req.Password); err != nil {
			jsonServerError(w, r, "failed to hash password", err)
			return
		}
	}

	u := &models.User{
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		Role:         req.Role,
		Kind:         req.Kind,
	}
	if err := store.NewUserRepo(h.db.SQL).Create(r.Context(), u); err != nil {
		jsonError(w, http.StatusConflict, "username already exists")
		return
	}
	if !h.applyUserEntidades(w, r, u.ID, req.EntidadeIDs, req.PrimaryEntidadeID) {
		return
	}

	jsonCreated(w, u)
}

// userUpdateRequest patches a user: blank fields and a nil entidade_ids are left unchanged.
type userUpdateRequest struct {
	Username          string   `json:"username"`
	DisplayName       string   `json:"display_name"`
	Role              string   `json:"role"`
	Password          string   `json:"password"`
	EntidadeIDs       *[]int64 `json:"entidade_ids"`
	PrimaryEntidadeID *int64   `json:"primary_entidade_id"`
}

// handleUpdateUser godoc
//
//	@Summary		Update a user
//	@Description	Admin. Blank fields stay unchanged; a password resets it; entidade_ids replaces memberships when sent.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"User ID"
//	@Param			body	body		userUpdateRequest	true	"Fields to change"
//	@Success		200		{object}	models.User
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/users/{id} [put]
func (h *authHandlers) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid user id", err)
		return
	}

	var req userUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	user, err := store.NewUserRepo(h.db.SQL).GetByID(r.Context(), id)
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
	if err := store.NewUserRepo(h.db.SQL).Update(r.Context(), user); err != nil {
		jsonServerError(w, r, "failed to update user", err)
		return
	}

	if req.Password != "" && user.Kind != models.UserKindService {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			jsonServerError(w, r, "failed to hash password", err)
			return
		}
		if err := store.NewUserRepo(h.db.SQL).UpdatePassword(r.Context(), id, hash); err != nil {
			jsonServerError(w, r, "failed to update password", err)
			return
		}
	}
	if !h.applyUserEntidades(w, r, id, req.EntidadeIDs, req.PrimaryEntidadeID) {
		return
	}

	jsonOK(w, user)
}

// handleDeleteUser godoc
//
//	@Summary		Delete a user
//	@Description	Admin. Deleting yourself answers 400.
//	@Tags			users
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/users/{id} [delete]
func (h *authHandlers) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid user id", err)
		return
	}

	// Prevent deleting yourself.
	me := auth.UserFromContext(r.Context())
	if me != nil && me.ID == id {
		jsonError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := store.NewUserRepo(h.db.SQL).Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete user", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *authHandlers) registerRoutes(rr routeRegistrar) {
	rr.public("GET /api/auth/status", h.handleStatus)
	rr.public("POST /api/auth/setup", h.handleSetup)
	rr.public("POST /api/auth/login", h.handleLogin)
	rr.auth("POST /api/auth/logout", h.handleLogout)
	rr.auth("GET /api/auth/me", h.handleMe)
	rr.role("admin", "GET /api/users", h.handleListUsers)
	rr.role("admin", "POST /api/users", h.handleCreateUser)
	rr.role("admin", "PUT /api/users/{id}", h.handleUpdateUser)
	rr.role("admin", "DELETE /api/users/{id}", h.handleDeleteUser)
}
