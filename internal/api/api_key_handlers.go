package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/api/docs"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/apicatalog"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// apiKeyHandlers manage the access keys of catalogued APIs
// (/api/api-catalog/{id}/keys*) and the per-API key-management settings.
//
// Two modes: "manual" keys are created elsewhere and registered here;
// "keycloak" keys are Keycloak clients ("<scope_prefix>-<label>") issued,
// rotated, revoked and synced through the keycloak_apis integration, with the
// API's scopes (from its GET /escopos) as optional client scopes. Either way
// the plaintext Bridge knows — the key, or the client secret — is kept in the
// vault (api_key secret scoped to the API), and reading it back needs
// apis.keys.manage like every write here. Seeing the key list only needs to
// see the API.
//
// Bridge itself sits in the catalogue (scope_prefix "bridge", seeded by
// SeedBridgeCatalog): its scopes come from auth.KeycloakCatalogue, and each
// client also gets a service-account user that its tokens act as.
type apiKeyHandlers struct {
	db *database.DB

	mu    sync.Mutex // guards kc/kcCfg
	kc    *kcadmin.Client
	kcCfg kcadmin.Config
}

// apiKeyLabelRe is the label shape (no spaces): it ends up in a Keycloak clientId.
var apiKeyLabelRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// scopePrefixRe: lowercase, no ":" (the scope separator) and no "-" (the
// clientId separator), so no prefix's clients can overlap another's.
var scopePrefixRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.]{0,31}$`)

const apiKeyHeader = "X-API-Key"

// apiKeyCreateRequest issues (keycloak) or registers (manual) a key.
type apiKeyCreateRequest struct {
	Label              string   `json:"label" example:"painel-rh"`
	Owner              string   `json:"owner" example:"rh@sead.pi.gov.br"`
	OwnerContactID     *int64   `json:"owner_contact_id"`
	Notes              string   `json:"notes"`
	ExpiresDays        *int     `json:"expires_days"` // manual only; nil or 0 = never
	Scopes             []string `json:"scopes"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute"` // keycloak: the token claim; nil or 0 = the API's default
	Value              string   `json:"value"`                 // manual only: the key itself
	Header             string   `json:"header"`                // manual only: header it goes in (default X-API-Key)
}

// apiKeyCreateResponse carries the plaintext once, for immediate use; it is
// also stored in the vault. For a Keycloak key it is the client secret.
type apiKeyCreateResponse struct {
	Key       models.APIKey `json:"key"`
	Plaintext string        `json:"plaintext"`
}

// apiKeyUpdateRequest edits a key, partially: an omitted field keeps its value.
// owner, contact and notes are Bridge's ("" / null clears); expires_at applies
// to manual keys (null clears); scopes and rate_limit_per_minute to active
// Keycloak keys (rate 0 = the API's default).
type apiKeyUpdateRequest struct {
	Owner              *string             `json:"owner"`
	OwnerContactID     optional[int64]     `json:"owner_contact_id" swaggertype:"integer"`
	Notes              *string             `json:"notes"`
	ExpiresAt          optional[time.Time] `json:"expires_at" swaggertype:"string" format:"date-time"`
	Scopes             []string            `json:"scopes"`
	RateLimitPerMinute *int                `json:"rate_limit_per_minute"`
}

// optional tells an omitted JSON field (Set false: keep) from an explicit null
// (Set true, Value nil: clear) — a plain pointer can't.
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// keyManagementRequest sets an API's key mode, its base URL (the root where
// GET /escopos and GET /admin/uso live) and its scope prefix.
type keyManagementRequest struct {
	KeyManagement string `json:"key_management" example:"keycloak"`
	AdminBaseURL  string `json:"admin_base_url" example:"http://10.0.122.91:8000"`
	ScopePrefix   string `json:"scope_prefix" example:"servidores"`
}

// keyManagementTestResponse is the connection check's verdict (always 200).
type keyManagementTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Keys    int    `json:"keys,omitempty"`
	Scopes  int    `json:"scopes,omitempty"`
}

// apiKeySyncResponse counts what a sync changed.
type apiKeySyncResponse struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

// apiKeyScopeSyncResponse counts the client scopes a scope sync created.
type apiKeyScopeSyncResponse struct {
	Created int `json:"created"`
	Total   int `json:"total"`
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *apiKeyHandlers) registerRoutes(rr routeRegistrar) {
	rr.public("GET /api/escopos", h.handleEscopos)
	rr.auth("GET /api/api-catalog/{id}/keys", h.handleList)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys", h.handleCreate)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys/sync", h.handleSync)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys/sync-scopes", h.handleSyncScopes)
	rr.auth("GET /api/api-catalog/{id}/keys/scopes", h.handleScopes)
	rr.perm("apis.keys.manage", "PUT /api/api-catalog/{id}/keys/{keyId}", h.handleUpdate)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys/{keyId}/revoke", h.handleRevoke)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys/{keyId}/rotate", h.handleRotate)
	rr.auth("GET /api/api-catalog/{id}/keys/{keyId}/usage", h.handleUsage)
	rr.role("admin", "PUT /api/api-catalog/{id}/key-management", h.handleSetKeyManagement)
	rr.role("admin", "POST /api/api-catalog/{id}/key-management/test", h.handleTestKeyManagement)
}

// loadAPI resolves {id} to a live, visible API, or writes 400/404.
func (h *apiKeyHandlers) loadAPI(w http.ResponseWriter, r *http.Request) (*models.APICatalog, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return nil, false
	}
	a, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get api", err)
		return nil, false
	}
	if a == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return nil, false
	}
	return a, true
}

// loadKey resolves {id}/{keyId} to a key of a visible API, or writes 400/404.
func (h *apiKeyHandlers) loadKey(w http.ResponseWriter, r *http.Request) (*models.APICatalog, *models.APIKey, bool) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return nil, nil, false
	}
	keyID, ok := pathID(w, r, "keyId")
	if !ok {
		return nil, nil, false
	}
	k, err := store.NewAPIKeyRepo(h.db.SQL).Get(r.Context(), a.ID, keyID)
	if err != nil {
		jsonServerError(w, r, "get api key", err)
		return nil, nil, false
	}
	if k == nil {
		jsonError(w, http.StatusNotFound, "api key not found")
		return nil, nil, false
	}
	return a, k, true
}

// isBridge reports whether a is Bridge's own catalogue entry.
func isBridge(a *models.APICatalog) bool { return a.ScopePrefix == auth.BridgeScopePrefix }

// kcClient returns the Keycloak admin client for API a, or writes 400 when
// the API is not in keycloak mode or the integration isn't configured.
func (h *apiKeyHandlers) kcClient(w http.ResponseWriter, r *http.Request, a *models.APICatalog) (*kcadmin.Client, bool) {
	kc, err := h.kcFor(r.Context(), a)
	if err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return nil, false
	}
	return kc, true
}

// kcFor builds the admin client from the keycloak_apis settings (reused
// while they don't change).
func (h *apiKeyHandlers) kcFor(ctx context.Context, a *models.APICatalog) (*kcadmin.Client, error) {
	if a.KeyManagement != models.APIKeyManagementKeycloak {
		return nil, errors.New("this API's keys are not managed through Keycloak")
	}
	if a.ScopePrefix == "" {
		return nil, errors.New("set this API's scope prefix first")
	}
	cfg, err := h.kcConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Configured() {
		return nil, errors.New("Keycloak integration not configured: set it in Settings → Integrations (Keycloak — SEAD APIs)")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.kc == nil || h.kcCfg != cfg {
		h.kc, h.kcCfg = kcadmin.New(cfg), cfg
	}
	return h.kc, nil
}

// kcConfig reads the keycloak_apis settings group (secrets decrypted).
func (h *apiKeyHandlers) kcConfig(ctx context.Context) (kcadmin.Config, error) {
	settings, secrets := store.NewAppSettingsRepo(h.db.SQL), store.NewAppSecretRepo(h.db.SQL)
	var serr error
	cfg := kcadmin.FromSettings(func(key string) string {
		if key == kcadmin.SettingClientSecret || key == kcadmin.SettingUsageSecret {
			v, _, err := secrets.Reveal(ctx, h.db.Encryptor, key)
			if err != nil {
				serr = err
			}
			return v
		}
		return settings.Value(ctx, key)
	})
	if serr != nil {
		return cfg, fmt.Errorf("decrypt the keycloak client secrets: %w", serr)
	}
	return cfg, nil
}

// reservedKey refuses (403) any change to a key row that stands for one of
// the integration's own clients (admin or usage): rotating it would hand out
// the manage-clients secret, revoking it would break the integration.
func (h *apiKeyHandlers) reservedKey(w http.ResponseWriter, r *http.Request, k *models.APIKey) bool {
	if k.Source != models.APIKeySourceKeycloak || k.ExternalLabel == nil {
		return false
	}
	cfg, err := h.kcConfig(r.Context())
	if err != nil {
		jsonServerError(w, r, "load keycloak settings", err)
		return true
	}
	if cfg.Reserved(*k.ExternalLabel) {
		jsonError(w, http.StatusForbidden, "client "+*k.ExternalLabel+" is the Keycloak integration's own client and can't be changed here")
		return true
	}
	return false
}

// apiScopesOnly checks every scope belongs to API a ("<prefix>:…"), or
// writes 400: a key of one API can't carry another API's scopes.
func apiScopesOnly(w http.ResponseWriter, a *models.APICatalog, scopes []string) bool {
	for _, s := range scopes {
		if !strings.HasPrefix(s, a.ScopePrefix+":") {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("scope %q is not a scope of this API (%s:…)", s, a.ScopePrefix))
			return false
		}
	}
	return true
}

// kcError answers a failed Keycloak or API call: 404 stays 404, 409 (client
// exists) stays 409, a rejected request keeps the message as 400, anything
// else is a 502 bad gateway.
func kcError(w http.ResponseWriter, r *http.Request, err error) {
	var ke *kcadmin.Error
	switch {
	case errors.Is(err, kcadmin.ErrUsageNotConfigured):
		jsonError(w, http.StatusBadRequest, "usage client not configured: set kc_apis_usage_client_id and its secret in Settings → Integrations (Keycloak — SEAD APIs)")
	case errors.As(err, &ke) && ke.Status == http.StatusNotFound:
		jsonError(w, http.StatusNotFound, "Keycloak: "+ke.Message)
	case errors.As(err, &ke) && ke.Status == http.StatusConflict:
		jsonError(w, http.StatusConflict, "Keycloak: "+ke.Message)
	case errors.As(err, &ke) && (ke.Status == http.StatusBadRequest || ke.Status == http.StatusUnprocessableEntity):
		jsonError(w, http.StatusBadRequest, "Keycloak: "+ke.Message)
	default:
		jsonErrorLogged(w, r, http.StatusBadGateway, "Keycloak: "+err.Error(), err)
	}
}

// remoteClient finds the Keycloak client of key k (nil when it is gone there).
func remoteClient(ctx context.Context, kc *kcadmin.Client, k *models.APIKey) (*kcadmin.ClientInfo, error) {
	if k.ExternalLabel == nil {
		return nil, nil
	}
	return kc.FindClient(ctx, *k.ExternalLabel)
}

// keyActor is who writes the key's vault secret. The route already required
// apis.keys.manage, which is the permission that governs API key secrets, so
// the write runs with at least editor rights (the vault's floor for shared
// secrets) while the audit trail keeps the real user.
func keyActor(r *http.Request) vault.ActorContext {
	a, _ := actorFrom(r)
	if a.Role != "admin" {
		a.Role = "editor"
	}
	return a
}

// logReqErr logs a failure the response doesn't carry (best-effort cleanup).
func logReqErr(r *http.Request, msg string, err error) {
	log.Printf("[api] %s %s: %s: %v", r.Method, scrubPath(r.URL.Path), msg, err)
}

// keyPayload is the vault payload: {value, header} for a manual key,
// {value: secret, client_id} for a Keycloak client.
func keyPayload(value, header, clientID string) string {
	m := map[string]string{"value": value}
	switch {
	case clientID != "":
		m["client_id"] = clientID
	case header == "":
		m["header"] = apiKeyHeader
	default:
		m["header"] = header
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// storeKeySecret keeps a key's plaintext in the vault as a shared api_key
// secret on the API and returns its id.
func (h *apiKeyHandlers) storeKeySecret(ctx context.Context, r *http.Request, apiID int64, label, payload string) (int64, error) {
	actor := keyActor(r)
	desc := "Access key " + label
	return vault.NewSecretRepo(h.db).Create(ctx, actor, &models.Secret{
		Type:        models.SecretTypeAPIKey,
		Scope:       models.SecretScopeAPICatalog,
		Visibility:  models.SecretVisibilityShared,
		ParentID:    &apiID,
		OwnerUserID: actor.UserID,
		Name:        label,
		Description: &desc,
		KeyVersion:  1,
		CreatedBy:   actor.UserID,
	}, payload)
}

// bridgeRole checks a Bridge client's scopes and returns the least role its
// service account needs for them. The caller must be able to use every scope
// itself, so apis.keys.manage can't mint more than its holder has. Writes
// 400/403 and returns "" on failure.
func (h *apiKeyHandlers) bridgeRole(w http.ResponseWriter, r *http.Request, scopes []string) string {
	perms := store.NewPermissionRepo(h.db.SQL)
	bare := make([]string, 0, len(scopes))
	for _, s := range scopes {
		rest, ok := strings.CutPrefix(s, auth.BridgeScopePrefix+":")
		if !ok {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("scope %q is not a Bridge scope (bridge:…)", s))
			return ""
		}
		bare = append(bare, rest)
	}
	role := auth.LeastRoleFor(bare, func(role, code string) bool { return perms.Has(r.Context(), role, code) })
	if role == "" {
		jsonError(w, http.StatusBadRequest, "unknown Bridge scope (GET /api/escopos)")
		return ""
	}
	caller := auth.UserFromContext(r.Context())
	callerPerm := func(code string) bool { return caller != nil && perms.Has(r.Context(), caller.Role, code) }
	tokenScopes, viaToken := auth.TokenScopesFromContext(r.Context())
	for _, sc := range bare {
		if caller == nil || !auth.ScopeUsable(caller.Role, callerPerm, sc) || (viaToken && !auth.ScopeAllowed(tokenScopes, sc)) {
			jsonError(w, http.StatusForbidden, fmt.Sprintf("scope %q is beyond your own role or token", sc))
			return ""
		}
	}
	return role
}

// bridgeServiceUser returns the service account a Bridge client acts as,
// creating it (username = clientId) and its keycloak-apis identity when
// missing, and sets its role.
func (h *apiKeyHandlers) bridgeServiceUser(ctx context.Context, clientID, role string) (*models.User, error) {
	users, idents := store.NewUserRepo(h.db.SQL), store.NewUserIdentityRepo(h.db.SQL)
	ident, err := idents.GetByProviderAndExternalID(ctx, auth.KeycloakAPIsProvider, clientID)
	if err != nil {
		return nil, err
	}
	if ident != nil {
		u, err := users.GetByID(ctx, ident.UserID)
		if err != nil || u == nil {
			return nil, fmt.Errorf("service user of %s: %v", clientID, err)
		}
		if u.Role != role {
			u.Role = role
			if err := users.Update(ctx, u); err != nil {
				return nil, err
			}
		}
		return u, nil
	}
	u := &models.User{Username: clientID, DisplayName: "Keycloak: " + clientID, Role: role,
		Kind: models.UserKindService, AuthProvider: auth.KeycloakAPIsProvider}
	if err := users.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("create service user %s (username taken?): %w", clientID, err)
	}
	if err := idents.Create(ctx, &models.UserExternalIdentity{UserID: u.ID, ProviderName: auth.KeycloakAPIsProvider, ExternalID: clientID}); err != nil {
		_ = users.Delete(ctx, u.ID)
		return nil, err
	}
	return u, nil
}

// unlinkBridgeClient drops the keycloak-apis identity of a revoked Bridge
// client: RequireAuth refuses an unmapped azp at once, so tokens it already
// issued stop working before they expire. Its service user stays, for the
// audit trail. Saving the key's scopes again (after re-enabling the client)
// recreates the identity through bridgeServiceUser.
func (h *apiKeyHandlers) unlinkBridgeClient(ctx context.Context, clientID string) error {
	return store.NewUserIdentityRepo(h.db.SQL).DeleteByProviderAndExternalID(ctx, auth.KeycloakAPIsProvider, clientID)
}

// handleEscopos godoc
//
//	@Summary		Bridge's scopes as Keycloak client scopes
//	@Description	Public. Every token scope with the "bridge:" prefix (no "*" wildcard) — the same shape the catalogued APIs answer at their GET /escopos, so Bridge's own Keycloak clients are picked and synced like theirs.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{array}	auth.ScopeInfo
//	@Router			/api/escopos [get]
func (h *apiKeyHandlers) handleEscopos(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, auth.KeycloakCatalogue())
}

// handleList godoc
//
//	@Summary		List an API's access keys
//	@Description	Any role that can see the API (404 otherwise). Metadata only; the plaintext is read through the vault and needs apis.keys.manage. Revoked keys and passed grace periods are hidden unless include_revoked=true.
//	@Tags			atlas
//	@Produce		json
//	@Param			id				path		int		true	"API catalog ID"
//	@Param			include_revoked	query		bool	false	"Also list revoked keys"
//	@Param			page			query		int		false	"Page (1-based)"
//	@Param			per_page		query		int		false	"Page size (max 200); omit for every row"
//	@Success		200				{object}	ListEnvelope[models.APIKey]
//	@Failure		400				{object}	httpx.ErrorResponse
//	@Failure		401				{object}	httpx.ErrorResponse
//	@Failure		404				{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys [get]
func (h *apiKeyHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	keys, err := store.NewAPIKeyRepo(h.db.SQL).List(r.Context(), a.ID, r.URL.Query().Get("include_revoked") == "true")
	if err != nil {
		jsonServerError(w, r, "list api keys", err)
		return
	}
	jsonPaged(w, r, keys)
}

// handleCreate godoc
//
//	@Summary		Issue or register an access key
//	@Description	apis.keys.manage. In keycloak mode a Keycloak client "<scope_prefix>-<label>" is created with the chosen scopes (from GET …/keys/scopes) as optional client scopes (every one must carry the API's "<scope_prefix>:") and the rate limit as its rate_limit_per_minute claim; its secret comes back once here. The integration's own client ids are refused. On Bridge's own entry the client also gets a service-account user (the least role its scopes need, never above the caller's role nor, for a token caller, its token's scopes). In manual mode send the existing key as value. Either way the plaintext is also stored in the vault. Label: letters, digits, . _ - (max 64), unique among the API's live keys.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"API catalog ID"
//	@Param			body	body		apiKeyCreateRequest	true	"Key"
//	@Success		201		{object}	apiKeyCreateResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys [post]
func (h *apiKeyHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	var req apiKeyCreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if !apiKeyLabelRe.MatchString(req.Label) {
		jsonError(w, http.StatusBadRequest, "label must be 1-64 letters, digits, dots, dashes or underscores")
		return
	}
	if req.ExpiresDays != nil && *req.ExpiresDays < 0 {
		jsonError(w, http.StatusBadRequest, "expires_days must be 0 (never) or more")
		return
	}
	if req.RateLimitPerMinute != nil && *req.RateLimitPerMinute < 0 {
		jsonError(w, http.StatusBadRequest, "rate_limit_per_minute must be 0 (the API's default) or more")
		return
	}
	if !h.contactVisible(w, r, req.OwnerContactID) {
		return
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	if live, err := keys.List(r.Context(), a.ID, false); err == nil {
		for _, k := range live {
			if k.Label == req.Label {
				jsonError(w, http.StatusConflict, "a live key with this label already exists")
				return
			}
		}
	}

	actor, _ := actorFrom(r)
	k := models.APIKey{
		APIID:          a.ID,
		Label:          req.Label,
		Owner:          strings.TrimSpace(req.Owner),
		OwnerContactID: req.OwnerContactID,
		Notes:          req.Notes,
		Scopes:         req.Scopes,
		CreatedBy:      &actor.UserID,
	}

	var payload, plaintext string
	var undo func()
	switch a.KeyManagement {
	case models.APIKeyManagementManual:
		plaintext = strings.TrimSpace(req.Value)
		if plaintext == "" {
			jsonError(w, http.StatusBadRequest, "value is required for a manually registered key")
			return
		}
		if req.ExpiresDays != nil && *req.ExpiresDays > 0 {
			exp := time.Now().AddDate(0, 0, *req.ExpiresDays)
			k.ExpiresAt = &exp
		}
		k.Source = models.APIKeySourceManual
		k.RateLimitPerMinute = req.RateLimitPerMinute
		payload = keyPayload(plaintext, strings.TrimSpace(req.Header), "")
	case models.APIKeyManagementKeycloak:
		kc, ok := h.kcClient(w, r, a)
		if !ok {
			return
		}
		if len(req.Scopes) == 0 {
			jsonError(w, http.StatusBadRequest, "choose at least one scope (GET …/keys/scopes)")
			return
		}
		if !apiScopesOnly(w, a, req.Scopes) {
			return
		}
		role := ""
		if isBridge(a) {
			if role = h.bridgeRole(w, r, req.Scopes); role == "" {
				return
			}
		}
		clientID := a.ScopePrefix + "-" + req.Label
		if cfg, _ := h.kcConfig(r.Context()); cfg.Reserved(clientID) {
			jsonError(w, http.StatusBadRequest, "client "+clientID+" is the Keycloak integration's own client: pick another label")
			return
		}
		rate := 0
		if req.RateLimitPerMinute != nil && *req.RateLimitPerMinute > 0 {
			rate = *req.RateLimitPerMinute
			k.RateLimitPerMinute = &rate
		}
		desc := strings.TrimSpace(strings.Join([]string{k.Owner, strings.TrimSpace(req.Notes)}, " — "))
		remoteID, secret, err := kc.CreateClient(r.Context(), clientID, strings.Trim(desc, " —"), req.Scopes, rate)
		if err != nil {
			kcError(w, r, err)
			return
		}
		// From here a client exists in Keycloak: if Bridge can't record it,
		// delete it again so nothing is left live that nobody tracks.
		undo = func() {
			if err := kc.DeleteClient(context.WithoutCancel(r.Context()), remoteID); err != nil {
				logReqErr(r, "delete untracked keycloak client "+clientID, err)
			}
		}
		if isBridge(a) {
			if _, err := h.bridgeServiceUser(r.Context(), clientID, role); err != nil {
				undo()
				jsonServerError(w, r, "create service account", err)
				return
			}
		}
		now := time.Now()
		k.Source, k.ExternalLabel, k.SyncedAt = models.APIKeySourceKeycloak, &clientID, &now
		plaintext, payload = secret, keyPayload(secret, "", clientID)
	default:
		jsonError(w, http.StatusBadRequest, "enable key management on this API first (manual or keycloak)")
		return
	}
	if undo == nil {
		undo = func() {}
	}

	secretID, err := h.storeKeySecret(r.Context(), r, a.ID, k.Label, payload)
	if err != nil {
		undo()
		writeErr(w, r, err)
		return
	}
	k.SecretID = &secretID
	if err := keys.Create(r.Context(), &k); err != nil {
		undo()
		jsonServerError(w, r, "save api key", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonCreated(w, apiKeyCreateResponse{Key: k, Plaintext: plaintext})
}

// contactVisible checks an optional owner contact exists and is visible.
func (h *apiKeyHandlers) contactVisible(w http.ResponseWriter, r *http.Request, id *int64) bool {
	if id == nil {
		return true
	}
	if c, err := store.NewContactRepo(h.db.SQL).Get(r.Context(), *id); err != nil || c == nil {
		jsonError(w, http.StatusNotFound, "contact not found")
		return false
	}
	return true
}

// handleUpdate godoc
//
//	@Summary		Edit an access key
//	@Description	apis.keys.manage. Partial: an omitted field keeps its value. Changes Bridge's owner, contact and notes; a manual key's expiry; an active Keycloak key's scopes and rate limit (applied to the client: tokens issued from then on carry them; scopes null keeps them, rate 0 = the API's default).
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"API catalog ID"
//	@Param			keyId	path		int					true	"Key ID"
//	@Param			body	body		apiKeyUpdateRequest	true	"Changes"
//	@Success		200		{object}	models.APIKey
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/{keyId} [put]
func (h *apiKeyHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	a, k, ok := h.loadKey(w, r)
	if !ok {
		return
	}
	var req apiKeyUpdateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !h.contactVisible(w, r, req.OwnerContactID.Value) || h.reservedKey(w, r, k) {
		return
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	remoteChange := k.Source == models.APIKeySourceKeycloak && (req.Scopes != nil || req.RateLimitPerMinute != nil)
	if remoteChange {
		if k.Status != models.APIKeyStatusActive || k.ExternalLabel == nil {
			jsonError(w, http.StatusBadRequest, "only an active Keycloak key's scopes and rate limit can change")
			return
		}
		if req.Scopes != nil && len(req.Scopes) == 0 {
			jsonError(w, http.StatusBadRequest, "choose at least one scope")
			return
		}
		if !apiScopesOnly(w, a, req.Scopes) {
			return
		}
		if req.RateLimitPerMinute != nil && *req.RateLimitPerMinute < 0 {
			jsonError(w, http.StatusBadRequest, "rate_limit_per_minute must be 0 (the API's default) or more")
			return
		}
		kc, ok := h.kcClient(w, r, a)
		if !ok {
			return
		}
		role := ""
		if isBridge(a) && req.Scopes != nil {
			if role = h.bridgeRole(w, r, req.Scopes); role == "" {
				return
			}
		}
		ci, err := remoteClient(r.Context(), kc, k)
		if err != nil {
			kcError(w, r, err)
			return
		}
		if ci == nil {
			jsonError(w, http.StatusNotFound, "Keycloak: client "+*k.ExternalLabel+" not found (sync the keys)")
			return
		}
		if req.Scopes != nil {
			if err := kc.SetScopes(r.Context(), ci.ID, req.Scopes); err != nil {
				kcError(w, r, err)
				return
			}
			if role != "" {
				if _, err := h.bridgeServiceUser(r.Context(), *k.ExternalLabel, role); err != nil {
					jsonServerError(w, r, "update service account", err)
					return
				}
			}
			k.Scopes = req.Scopes
		}
		if req.RateLimitPerMinute != nil {
			if err := kc.SetRateLimit(r.Context(), ci.ID, *req.RateLimitPerMinute); err != nil {
				kcError(w, r, err)
				return
			}
			k.RateLimitPerMinute = nil
			if *req.RateLimitPerMinute > 0 {
				k.RateLimitPerMinute = req.RateLimitPerMinute
			}
		}
		if err := keys.ApplyRemote(r.Context(), k.ID, k); err != nil {
			jsonServerError(w, r, "save api key", err)
			return
		}
	}
	if req.Owner != nil {
		k.Owner = strings.TrimSpace(*req.Owner)
	}
	if req.OwnerContactID.Set {
		k.OwnerContactID = req.OwnerContactID.Value
	}
	if req.Notes != nil {
		k.Notes = *req.Notes
	}
	if k.Source == models.APIKeySourceManual && req.ExpiresAt.Set {
		k.ExpiresAt = req.ExpiresAt.Value
	}
	if _, err := keys.UpdateMeta(r.Context(), k); err != nil {
		jsonServerError(w, r, "update api key", err)
		return
	}
	updated, err := keys.Get(r.Context(), a.ID, k.ID)
	if err != nil || updated == nil {
		jsonServerError(w, r, "reload api key", err)
		return
	}
	jsonOK(w, updated)
}

// handleRevoke godoc
//
//	@Summary		Revoke an access key
//	@Description	apis.keys.manage. A Keycloak key's client is disabled first (no new tokens; one already gone there still counts). For Bridge's own entry its service-account link is dropped too, so tokens it already issued stop at once; re-enabling then needs the key's scopes saved again. The vault copy is renamed so the label can be reused for a manual key; a Keycloak client id stays taken.
//	@Tags			atlas
//	@Produce		json
//	@Param			id		path		int	true	"API catalog ID"
//	@Param			keyId	path		int	true	"Key ID"
//	@Success		200		{object}	models.APIKey
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/{keyId}/revoke [post]
func (h *apiKeyHandlers) handleRevoke(w http.ResponseWriter, r *http.Request) {
	a, k, ok := h.loadKey(w, r)
	if !ok {
		return
	}
	if h.reservedKey(w, r, k) {
		return
	}
	if k.Source == models.APIKeySourceKeycloak && k.ExternalLabel != nil && k.Status != models.APIKeyStatusRevoked {
		kc, ok := h.kcClient(w, r, a)
		if !ok {
			return
		}
		ci, err := remoteClient(r.Context(), kc, k)
		if err == nil && ci != nil {
			err = kc.SetEnabled(r.Context(), ci.ID, false)
		}
		if err != nil {
			kcError(w, r, err)
			return
		}
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	if err := keys.MarkRevoked(r.Context(), a.ID, k.ID); err != nil {
		jsonServerError(w, r, "revoke api key", err)
		return
	}
	if isBridge(a) && k.ExternalLabel != nil {
		if err := h.unlinkBridgeClient(r.Context(), *k.ExternalLabel); err != nil {
			jsonServerError(w, r, "unlink revoked bridge client", err)
			return
		}
	}
	if k.SecretID != nil {
		name := fmt.Sprintf("%s (revogada %s)", k.Label, time.Now().Format("2006-01-02 15:04"))
		if err := vault.NewSecretRepo(h.db).Update(r.Context(), keyActor(r), *k.SecretID, vault.SecretPatch{Name: &name}); err != nil &&
			!errors.Is(err, vault.ErrSecretNotFound) {
			logReqErr(r, "rename revoked key secret", err)
		}
	}
	revoked, err := keys.Get(r.Context(), a.ID, k.ID)
	if err != nil || revoked == nil {
		jsonServerError(w, r, "reload api key", err)
		return
	}
	jsonOK(w, revoked)
}

// handleRotate godoc
//
//	@Summary		Rotate a Keycloak key's secret
//	@Description	apis.keys.manage, keycloak keys only. Keycloak issues a new client secret and the old one stops working at once (tokens already issued live until they expire). The vault copy gets the new secret (its history keeps the old); it also comes back once here.
//	@Tags			atlas
//	@Produce		json
//	@Param			id		path		int	true	"API catalog ID"
//	@Param			keyId	path		int	true	"Key ID"
//	@Success		200		{object}	apiKeyCreateResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/{keyId}/rotate [post]
func (h *apiKeyHandlers) handleRotate(w http.ResponseWriter, r *http.Request) {
	a, k, ok := h.loadKey(w, r)
	if !ok {
		return
	}
	if k.Source != models.APIKeySourceKeycloak || k.ExternalLabel == nil {
		jsonError(w, http.StatusBadRequest, "only Keycloak keys can be rotated; register a new manual key instead")
		return
	}
	if k.Status != models.APIKeyStatusActive {
		jsonError(w, http.StatusBadRequest, "only an active key can be rotated")
		return
	}
	if h.reservedKey(w, r, k) {
		return
	}
	kc, ok := h.kcClient(w, r, a)
	if !ok {
		return
	}
	ci, err := remoteClient(r.Context(), kc, k)
	if err != nil {
		kcError(w, r, err)
		return
	}
	if ci == nil {
		jsonError(w, http.StatusNotFound, "Keycloak: client "+*k.ExternalLabel+" not found (sync the keys)")
		return
	}
	secret, err := kc.RegenerateSecret(r.Context(), ci.ID)
	if err != nil {
		kcError(w, r, err)
		return
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	payload := keyPayload(secret, "", *k.ExternalLabel)
	if k.SecretID != nil {
		err = vault.NewSecretRepo(h.db).Update(r.Context(), keyActor(r), *k.SecretID, vault.SecretPatch{Payload: &payload})
	}
	if k.SecretID == nil || errors.Is(err, vault.ErrSecretNotFound) {
		var id int64
		if id, err = h.storeKeySecret(r.Context(), r, a.ID, k.Label, payload); err == nil {
			err = keys.SetSecret(r.Context(), k.ID, id)
		}
	}
	if err != nil {
		// Keycloak already rotated: answer with the secret rather than lose it.
		logReqErr(r, "store rotated key in vault", err)
	}
	rotated, gerr := keys.Get(r.Context(), a.ID, k.ID)
	if gerr != nil || rotated == nil {
		jsonServerError(w, r, "reload api key", gerr)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonOK(w, apiKeyCreateResponse{Key: *rotated, Plaintext: secret})
}

// handleSync godoc
//
//	@Summary		Sync keys from Keycloak
//	@Description	apis.keys.manage, keycloak mode only. Imports the realm's clients named "<scope_prefix>-…" created elsewhere (except the integration's own admin and usage clients) as metadata-only rows and refreshes scopes, rate limit and revocation (a disabled client) of known ones. Bridge's own owner, contact, notes and vault copy are kept.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{object}	apiKeySyncResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/sync [post]
func (h *apiKeyHandlers) handleSync(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	kc, ok := h.kcClient(w, r, a)
	if !ok {
		return
	}
	remote, err := kc.ListClients(r.Context(), a.ScopePrefix)
	if err != nil {
		kcError(w, r, err)
		return
	}
	res, err := h.syncFrom(r.Context(), a, remote)
	if err != nil {
		jsonServerError(w, r, "sync api keys", err)
		return
	}
	jsonOK(w, res)
}

// syncFrom upserts Keycloak's clients into api_keys by client id.
func (h *apiKeyHandlers) syncFrom(ctx context.Context, a *models.APICatalog, remote []kcadmin.ClientInfo) (apiKeySyncResponse, error) {
	res := apiKeySyncResponse{Total: len(remote)}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	known, err := keys.ByExternalLabel(ctx, a.ID)
	if err != nil {
		return res, err
	}
	now := time.Now()
	for _, ci := range remote {
		clientID := ci.ClientID
		k := models.APIKey{APIID: a.ID, Source: models.APIKeySourceKeycloak, Label: strings.TrimPrefix(clientID, a.ScopePrefix+"-"),
			ExternalLabel: &clientID, Owner: ci.Description, Scopes: ci.Scopes(), RateLimitPerMinute: ci.RateLimit(), SyncedAt: &now}
		if !ci.Enabled {
			k.RevokedAt = &now
			if isBridge(a) {
				if err := h.unlinkBridgeClient(ctx, clientID); err != nil {
					return res, err
				}
			}
		}
		if id, ok := known[clientID]; ok {
			if err := keys.ApplyRemote(ctx, id, &k); err != nil {
				return res, err
			}
			res.Updated++
			continue
		}
		if err := keys.Create(ctx, &k); err != nil {
			return res, err
		}
		res.Created++
	}
	return res, nil
}

// apiScopes is the API's scope catalogue: Bridge's own, or the API's
// GET <admin_base_url>/escopos.
func apiScopes(ctx context.Context, a *models.APICatalog) ([]kcadmin.ScopeInfo, error) {
	if isBridge(a) {
		out := []kcadmin.ScopeInfo{}
		for _, s := range auth.KeycloakCatalogue() {
			out = append(out, kcadmin.ScopeInfo{Name: s.Name, Kind: s.Kind, Description: s.Description, Routes: s.Routes})
		}
		return out, nil
	}
	if a.AdminBaseURL == "" {
		return nil, &kcadmin.Error{Status: http.StatusBadRequest, Message: "set this API's base URL first"}
	}
	return kcadmin.Catalogue(ctx, a.AdminBaseURL)
}

// handleSyncScopes godoc
//
//	@Summary		Create the API's scopes in Keycloak
//	@Description	apis.keys.manage, keycloak mode only. Reads the API's scope catalogue (its GET /escopos; Bridge's own for Bridge) and creates every scope the realm lacks as an OIDC client scope, so Keycloak's scopes follow the API's code. Existing scopes are left as they are.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{object}	apiKeyScopeSyncResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/sync-scopes [post]
func (h *apiKeyHandlers) handleSyncScopes(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	kc, ok := h.kcClient(w, r, a)
	if !ok {
		return
	}
	scopes, err := apiScopes(r.Context(), a)
	if err != nil {
		kcError(w, r, err)
		return
	}
	created, err := kc.EnsureClientScopes(r.Context(), scopes)
	if err != nil {
		kcError(w, r, err)
		return
	}
	jsonOK(w, apiKeyScopeSyncResponse{Created: created, Total: len(scopes)})
}

// handleScopes godoc
//
//	@Summary		Scopes a key of this API can carry
//	@Description	Any role that can see the API; keycloak mode only. The API's catalogue from its GET /escopos (Bridge's own for Bridge): route scopes with the route patterns they open, and output modifiers. Names carry the API's "<scope_prefix>:".
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{array}		kcadmin.ScopeInfo
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/scopes [get]
func (h *apiKeyHandlers) handleScopes(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	if a.KeyManagement != models.APIKeyManagementKeycloak {
		jsonError(w, http.StatusBadRequest, "this API's keys are not managed through Keycloak")
		return
	}
	scopes, err := apiScopes(r.Context(), a)
	if err != nil {
		kcError(w, r, err)
		return
	}
	jsonOK(w, scopes)
}

// handleUsage godoc
//
//	@Summary		A Keycloak key's request counts
//	@Description	Any role that can see the API. Lifetime and per-day counts ("YYYY-MM-DD" → requests) straight from the API's GET /admin/uso, read with a token of the integration's usage client (kc_apis_usage_client_id, no realm-management roles) holding "<scope_prefix>:admin.uso" — the admin client's token never reaches an API. 400 when the usage client isn't configured. Not available for Bridge's own clients.
//	@Tags			atlas
//	@Produce		json
//	@Param			id		path		int	true	"API catalog ID"
//	@Param			keyId	path		int	true	"Key ID"
//	@Success		200		{object}	kcadmin.Usage
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/{keyId}/usage [get]
func (h *apiKeyHandlers) handleUsage(w http.ResponseWriter, r *http.Request) {
	a, k, ok := h.loadKey(w, r)
	if !ok {
		return
	}
	if k.Source != models.APIKeySourceKeycloak || k.ExternalLabel == nil || isBridge(a) {
		jsonError(w, http.StatusBadRequest, "usage is only tracked for Keycloak keys of a catalogued API")
		return
	}
	if a.AdminBaseURL == "" {
		jsonError(w, http.StatusBadRequest, "set this API's base URL first")
		return
	}
	kc, ok := h.kcClient(w, r, a)
	if !ok {
		return
	}
	u, err := kc.Usage(r.Context(), a.AdminBaseURL, a.ScopePrefix, *k.ExternalLabel)
	if err != nil {
		kcError(w, r, err)
		return
	}
	jsonOK(w, u)
}

// validKeyManagement checks a key-management request against API a, or
// writes 400.
func validKeyManagement(w http.ResponseWriter, a *models.APICatalog, req *keyManagementRequest) bool {
	req.AdminBaseURL = strings.TrimSpace(req.AdminBaseURL)
	req.ScopePrefix = strings.TrimSpace(req.ScopePrefix)
	if !models.ValidKeyManagement(req.KeyManagement) {
		jsonError(w, http.StatusBadRequest, "key_management must be none, manual or keycloak")
		return false
	}
	if req.AdminBaseURL != "" && !strings.HasPrefix(req.AdminBaseURL, "http://") && !strings.HasPrefix(req.AdminBaseURL, "https://") {
		jsonError(w, http.StatusBadRequest, "admin_base_url must be an http(s) URL")
		return false
	}
	if req.ScopePrefix != "" && !scopePrefixRe.MatchString(req.ScopePrefix) {
		jsonError(w, http.StatusBadRequest, "scope_prefix must be lowercase letters, digits, . _ (max 32)")
		return false
	}
	if (req.ScopePrefix == auth.BridgeScopePrefix) != isBridge(a) {
		jsonError(w, http.StatusBadRequest, `scope_prefix "bridge" is Bridge's own entry`)
		return false
	}
	if req.KeyManagement == models.APIKeyManagementKeycloak {
		if req.ScopePrefix == "" {
			jsonError(w, http.StatusBadRequest, "scope_prefix is required in keycloak mode")
			return false
		}
		if req.AdminBaseURL == "" && !isBridge(a) {
			jsonError(w, http.StatusBadRequest, "admin_base_url (the API root, where /escopos lives) is required in keycloak mode")
			return false
		}
	}
	return true
}

// handleSetKeyManagement godoc
//
//	@Summary		Set an API's key management
//	@Description	Admin. key_management none, manual or keycloak; for keycloak the API's base URL (its root, where GET /escopos and GET /admin/uso live) and its scope prefix (scopes "<prefix>:…", clients "<prefix>-<label>"). The Keycloak connection itself is the keycloak_apis integration setting. "bridge" is reserved for Bridge's own entry.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"API catalog ID"
//	@Param			body	body		keyManagementRequest	true	"Mode, base URL and scope prefix"
//	@Success		200		{object}	models.APICatalog
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/key-management [put]
func (h *apiKeyHandlers) handleSetKeyManagement(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	var req keyManagementRequest
	if !decodeBody(w, r, &req) || !validKeyManagement(w, a, &req) {
		return
	}
	repo := store.NewAPICatalogRepo(h.db.SQL)
	if req.ScopePrefix != "" {
		if other, _, err := repo.ByScopePrefix(r.Context(), req.ScopePrefix); err != nil {
			jsonServerError(w, r, "check scope prefix", err)
			return
		} else if other != 0 && other != a.ID {
			jsonError(w, http.StatusConflict, fmt.Sprintf("scope_prefix %q is already used by another API", req.ScopePrefix))
			return
		}
	}
	u := store.KeyManagementUpdate{Mode: req.KeyManagement, AdminBaseURL: req.AdminBaseURL, ScopePrefix: req.ScopePrefix}
	if found, err := repo.SetKeyManagement(r.Context(), a.ID, u); err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	} else if !found {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	updated, err := repo.Get(r.Context(), a.ID)
	if err != nil || updated == nil {
		jsonServerError(w, r, "reload api", err)
		return
	}
	jsonOK(w, updated)
}

// handleTestKeyManagement godoc
//
//	@Summary		Test an API's Keycloak connection
//	@Description	Admin. With the given (unsaved) base URL and scope prefix, falling back to the stored ones: reads the API's scope catalogue and lists its Keycloak clients through the keycloak_apis integration. Always 200: {success, error | keys, scopes}.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"API catalog ID"
//	@Param			body	body		keyManagementRequest	false	"Values to try"
//	@Success		200		{object}	keyManagementTestResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/key-management/test [post]
func (h *apiKeyHandlers) handleTestKeyManagement(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	var req keyManagementRequest
	if r.ContentLength > 0 && !decodeBody(w, r, &req) {
		return
	}
	try := *a
	try.KeyManagement = models.APIKeyManagementKeycloak
	if v := strings.TrimSpace(req.AdminBaseURL); v != "" {
		try.AdminBaseURL = v
	}
	if v := strings.TrimSpace(req.ScopePrefix); v != "" {
		try.ScopePrefix = v
	}
	scopes, err := apiScopes(r.Context(), &try)
	if err != nil {
		jsonOK(w, keyManagementTestResponse{Success: false, Error: "escopos: " + err.Error()})
		return
	}
	kc, err := h.kcFor(r.Context(), &try)
	if err != nil {
		jsonOK(w, keyManagementTestResponse{Success: false, Error: err.Error()})
		return
	}
	clients, err := kc.ListClients(r.Context(), try.ScopePrefix)
	if err != nil {
		jsonOK(w, keyManagementTestResponse{Success: false, Error: err.Error()})
		return
	}
	jsonOK(w, keyManagementTestResponse{Success: true, Keys: len(clients), Scopes: len(scopes)})
}

// SeedBridgeCatalog puts Bridge itself in its API catalogue once: keycloak
// mode, scope_prefix "bridge", its own OpenAPI spec, owned by the first admin
// and with no entidade grants (admin-only until an admin shares it). A row
// with that prefix, even trashed, means it was seeded already: a live one gets
// its spec refreshed when this build's differs. Without an admin yet it does
// nothing and the next start tries again.
func SeedBridgeCatalog(ctx context.Context, db *database.DB) error {
	repo := store.NewAPICatalogRepo(db.SQL)
	ps, err := apicatalog.Parse([]byte(docs.SwaggerInfo.ReadDoc()))
	if err != nil {
		return fmt.Errorf("parse bridge spec: %w", err)
	}
	// The repo's writes filter by visibility: run them as admin.
	adminCtx := store.WithScope(ctx, store.Scope{Admin: true})
	id, hash, err := repo.ByScopePrefix(ctx, auth.BridgeScopePrefix)
	if err != nil {
		return err
	}
	if id != 0 {
		if hash == ps.SpecHash {
			return nil
		}
		err := repo.UpdateSpec(adminCtx, id, string(ps.SpecJSON), ps.SpecHash, ps.SpecVersion, ps.Title, ps.VersionLabel, ps.ExternalURL, toModelOps(ps.Operations))
		if errors.Is(err, sql.ErrNoRows) {
			return nil // trashed: leave it be
		}
		return err
	}
	users, err := store.NewUserRepo(db.SQL).List(ctx)
	if err != nil {
		return err
	}
	var owner int64
	for _, u := range users {
		if u.Role == "admin" && u.Kind != models.UserKindService {
			owner = u.ID
			break
		}
	}
	if owner == 0 {
		return nil
	}
	a := &models.APICatalog{
		Name:         "Bridge",
		Description:  "A API do próprio Bridge. Chaves são clientes Keycloak (bridge-<rótulo>) que agem como uma conta de serviço.",
		SourceType:   models.APICatalogSourceUpload,
		ExternalURL:  ps.ExternalURL,
		SpecVersion:  ps.SpecVersion,
		SpecJSON:     string(ps.SpecJSON),
		SpecHash:     ps.SpecHash,
		Title:        ps.Title,
		VersionLabel: ps.VersionLabel,
		OwnerUserID:  owner,
		CreatedBy:    owner,
	}
	if err := repo.Create(ctx, a, toModelOps(ps.Operations)); err != nil {
		return err
	}
	_, err = repo.SetKeyManagement(adminCtx, a.ID, store.KeyManagementUpdate{Mode: models.APIKeyManagementKeycloak, ScopePrefix: auth.BridgeScopePrefix})
	return err
}
