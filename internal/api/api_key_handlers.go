package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/seadkeys"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// apiKeyHandlers manage the access keys of catalogued APIs
// (/api/api-catalog/{id}/keys*) and the per-API key-management settings.
//
// Two modes: "manual" keys are created elsewhere and registered here;
// "sead" keys are issued, rotated, revoked and synced through the SEAD
// built-in /admin/keys API. Either way the plaintext of every key Bridge
// knows is kept in the vault (api_key secret scoped to the API), and reading
// it back needs apis.keys.manage like every write here. Seeing the key list
// only needs to see the API. The SEAD master key is admin-only and write-only.
type apiKeyHandlers struct {
	db *database.DB
}

// apiKeyLabelRe is the label shape SEAD accepts (no spaces); manual keys use
// it too so a later switch to sead mode can't trip on an old label.
var apiKeyLabelRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

const apiKeyHeader = "X-API-Key"

// apiKeyCreateRequest issues (sead) or registers (manual) a key.
type apiKeyCreateRequest struct {
	Label              string   `json:"label" example:"painel-rh"`
	Owner              string   `json:"owner" example:"rh@sead.pi.gov.br"`
	OwnerContactID     *int64   `json:"owner_contact_id"`
	Notes              string   `json:"notes"`
	ExpiresDays        *int     `json:"expires_days"` // nil or 0 = never
	Scopes             []string `json:"scopes"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute"`
	Value              string   `json:"value"`  // manual only: the key itself
	Header             string   `json:"header"` // manual only: header it goes in (default X-API-Key)
}

// apiKeyCreateResponse carries the plaintext once, for immediate use; it is
// also stored in the vault.
type apiKeyCreateResponse struct {
	Key       models.APIKey `json:"key"`
	Plaintext string        `json:"plaintext"`
}

// apiKeyUpdateRequest edits Bridge's own metadata. expires_at applies to
// manual keys only (SEAD has no update endpoint: revoke and reissue there).
type apiKeyUpdateRequest struct {
	Owner          string     `json:"owner"`
	OwnerContactID *int64     `json:"owner_contact_id"`
	Notes          string     `json:"notes"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

type apiKeyRotateRequest struct {
	GraceDays int `json:"grace_days" example:"7"`
}

// keyManagementRequest sets an API's key mode and SEAD connection. Blank
// admin_key / api_key keep the stored ones; clear_* drop them.
type keyManagementRequest struct {
	KeyManagement string `json:"key_management" example:"sead"`
	AdminBaseURL  string `json:"admin_base_url" example:"https://api.folha.sead.gov.br/folha"`
	AdminKey      string `json:"admin_key"`
	APIKey        string `json:"api_key"`
	ClearAdminKey bool   `json:"clear_admin_key"`
	ClearAPIKey   bool   `json:"clear_api_key"`
}

// keyManagementTestResponse is the connection check's verdict (always 200).
type keyManagementTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Keys    int    `json:"keys,omitempty"`
}

// apiKeySyncResponse counts what a sync changed.
type apiKeySyncResponse struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *apiKeyHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/api-catalog/{id}/keys", h.handleList)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys", h.handleCreate)
	rr.perm("apis.keys.manage", "POST /api/api-catalog/{id}/keys/sync", h.handleSync)
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

// seadClient builds the SEAD client from the API's stored connection, or
// writes 400 when the API is not in sead mode or is missing its master key.
func (h *apiKeyHandlers) seadClient(w http.ResponseWriter, r *http.Request, a *models.APICatalog) (*seadkeys.Client, bool) {
	if a.KeyManagement != models.APIKeyManagementSEAD {
		jsonError(w, http.StatusBadRequest, "this API's keys are not managed through SEAD")
		return nil, false
	}
	adminKey, apiKey, err := store.NewAPICatalogRepo(h.db.SQL).AdminCredentials(r.Context(), a.ID)
	if err != nil {
		jsonServerError(w, r, "load sead credentials", err)
		return nil, false
	}
	if a.AdminBaseURL == "" || adminKey == nil {
		jsonError(w, http.StatusBadRequest, "SEAD connection not configured: set the admin base URL and master key")
		return nil, false
	}
	admin, err := h.db.Encryptor.Decrypt(adminKey.Cipher, adminKey.Nonce)
	if err != nil {
		jsonServerError(w, r, "decrypt sead master key", err)
		return nil, false
	}
	var xAPIKey string
	if apiKey != nil {
		if xAPIKey, err = h.db.Encryptor.Decrypt(apiKey.Cipher, apiKey.Nonce); err != nil {
			jsonServerError(w, r, "decrypt sead api key", err)
			return nil, false
		}
	}
	return seadkeys.New(a.AdminBaseURL, admin, xAPIKey), true
}

// seadError answers a failed SEAD call: 404 stays 404 (the key is gone
// there), a rejected request keeps the service's message as 400, anything
// else is a 502 bad gateway.
func seadError(w http.ResponseWriter, r *http.Request, err error) {
	var se *seadkeys.Error
	switch {
	case errors.As(err, &se) && se.Status == http.StatusNotFound:
		jsonError(w, http.StatusNotFound, "SEAD: "+se.Message)
	case errors.As(err, &se) && (se.Status == http.StatusBadRequest || se.Status == http.StatusUnprocessableEntity):
		jsonError(w, http.StatusBadRequest, "SEAD: "+se.Message)
	default:
		jsonErrorLogged(w, r, http.StatusBadGateway, "SEAD key service: "+err.Error(), err)
	}
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

func keyPayload(value, header string) string {
	if header == "" {
		header = apiKeyHeader
	}
	b, _ := json.Marshal(map[string]string{"value": value, "header": header})
	return string(b)
}

// storeKeySecret keeps a key's plaintext in the vault as a shared api_key
// secret on the API and returns its id.
func (h *apiKeyHandlers) storeKeySecret(ctx context.Context, r *http.Request, apiID int64, label, value, header string) (int64, error) {
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
	}, keyPayload(value, header))
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
//	@Description	apis.keys.manage. In sead mode the key is created on the service with the chosen scopes (from GET …/keys/scopes; the service rejects unknown ones) and its plaintext comes back once here; in manual mode send the existing key as value. Either way the plaintext is also stored in the vault. Label: letters, digits, . _ - (max 64), unique among the API's live keys.
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
		jsonError(w, http.StatusBadRequest, "rate_limit_per_minute must be 0 (unlimited) or more")
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
		APIID:              a.ID,
		Label:              req.Label,
		Owner:              strings.TrimSpace(req.Owner),
		OwnerContactID:     req.OwnerContactID,
		Notes:              req.Notes,
		Scopes:             req.Scopes,
		RateLimitPerMinute: req.RateLimitPerMinute,
		CreatedBy:          &actor.UserID,
	}
	if req.ExpiresDays != nil && *req.ExpiresDays > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresDays)
		k.ExpiresAt = &exp
	}

	var plaintext, header string
	var remote *seadkeys.CreateResponse
	var client *seadkeys.Client
	switch a.KeyManagement {
	case models.APIKeyManagementManual:
		plaintext, header = strings.TrimSpace(req.Value), strings.TrimSpace(req.Header)
		if plaintext == "" {
			jsonError(w, http.StatusBadRequest, "value is required for a manually registered key")
			return
		}
		k.Source = models.APIKeySourceManual
	case models.APIKeyManagementSEAD:
		if client, ok = h.seadClient(w, r, a); !ok {
			return
		}
		var err error
		remote, err = client.Create(r.Context(), seadkeys.CreateRequest{
			Label:              req.Label,
			Owner:              k.Owner,
			ExpiresDays:        req.ExpiresDays,
			Scopes:             req.Scopes,
			RateLimitPerMinute: req.RateLimitPerMinute,
			Notes:              req.Notes,
		})
		if err != nil {
			seadError(w, r, err)
			return
		}
		plaintext, header = remote.Plaintext, apiKeyHeader
		k.Source = models.APIKeySourceSEAD
		applySummary(&k, remote.KeySummary)
	default:
		jsonError(w, http.StatusBadRequest, "enable key management on this API first (manual or sead)")
		return
	}

	// From here a SEAD key exists remotely: if Bridge can't record it, revoke
	// it again so no key is left live that nobody tracks.
	undo := func() {
		if remote != nil {
			if _, err := client.Revoke(context.WithoutCancel(r.Context()), remote.Label); err != nil {
				logReqErr(r, "revoke untracked sead key "+remote.Label, err)
			}
		}
	}
	secretID, err := h.storeKeySecret(r.Context(), r, a.ID, k.Label, plaintext, header)
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

// applySummary copies SEAD's view of a key onto k.
func applySummary(k *models.APIKey, s seadkeys.KeySummary) {
	label := s.Label
	k.Label = label
	k.ExternalLabel = &label
	k.Owner = s.Owner
	k.Scopes = s.Scopes
	k.RateLimitPerMinute = s.RateLimitPerMinute
	k.ExpiresAt = seadkeys.Time(s.ExpiresAt)
	k.RevokedAt = seadkeys.Time(s.RevokedAt)
	k.GraceUntil = seadkeys.Time(s.GraceUntil)
	k.LastUsedAt = seadkeys.Time(s.LastUsedAt)
	k.LifetimeUses = s.LifetimeUses
	now := time.Now()
	k.SyncedAt = &now
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
//	@Summary		Edit an access key's metadata
//	@Description	apis.keys.manage. Changes Bridge's owner, contact and notes (and expiry, for manual keys). A SEAD key's expiry, scopes and rate limit can't change on the service: revoke and issue a new key.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"API catalog ID"
//	@Param			keyId	path		int					true	"Key ID"
//	@Param			body	body		apiKeyUpdateRequest	true	"Metadata"
//	@Success		200		{object}	models.APIKey
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
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
	if !h.contactVisible(w, r, req.OwnerContactID) {
		return
	}
	k.Owner, k.OwnerContactID, k.Notes = strings.TrimSpace(req.Owner), req.OwnerContactID, req.Notes
	if k.Source == models.APIKeySourceManual {
		k.ExpiresAt = req.ExpiresAt
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
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
//	@Description	apis.keys.manage. A SEAD key is revoked on the service first (a key already gone there still counts). The vault copy is renamed so the label can be reused.
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
	if k.Source == models.APIKeySourceSEAD && k.ExternalLabel != nil && k.Status != models.APIKeyStatusRevoked {
		client, ok := h.seadClient(w, r, a)
		if !ok {
			return
		}
		var se *seadkeys.Error
		if _, err := client.Revoke(r.Context(), *k.ExternalLabel); err != nil && !(errors.As(err, &se) && se.Status == http.StatusNotFound) {
			seadError(w, r, err)
			return
		}
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	if err := keys.MarkRevoked(r.Context(), a.ID, k.ID); err != nil {
		jsonServerError(w, r, "revoke api key", err)
		return
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
//	@Summary		Rotate a SEAD access key
//	@Description	apis.keys.manage, sead mode only. The service issues a new key under the same label and keeps the old one working for grace_days; the vault copy gets the new plaintext (its history keeps the old). The new plaintext also comes back once here.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"API catalog ID"
//	@Param			keyId	path		int					true	"Key ID"
//	@Param			body	body		apiKeyRotateRequest	true	"Grace period for the old key"
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
	if k.Source != models.APIKeySourceSEAD || k.ExternalLabel == nil {
		jsonError(w, http.StatusBadRequest, "only SEAD keys can be rotated; register a new manual key instead")
		return
	}
	if k.Status != models.APIKeyStatusActive {
		jsonError(w, http.StatusBadRequest, "only an active key can be rotated")
		return
	}
	req := apiKeyRotateRequest{GraceDays: 7}
	if r.ContentLength > 0 && !decodeBody(w, r, &req) {
		return
	}
	if req.GraceDays < 0 {
		jsonError(w, http.StatusBadRequest, "grace_days must be 0 or more")
		return
	}
	client, ok := h.seadClient(w, r, a)
	if !ok {
		return
	}
	remote, err := client.Rotate(r.Context(), *k.ExternalLabel, req.GraceDays)
	if err != nil {
		seadError(w, r, err)
		return
	}
	// The label now names the new key: the row follows it. The old key shows
	// up as "<label>__rotated__…" on the next sync.
	keys := store.NewAPIKeyRepo(h.db.SQL)
	applySummary(k, remote.KeySummary)
	if err := keys.ApplyRemote(r.Context(), k.ID, k); err != nil {
		jsonServerError(w, r, "save rotated api key", err)
		return
	}
	if k.SecretID != nil {
		payload := keyPayload(remote.Plaintext, apiKeyHeader)
		err = vault.NewSecretRepo(h.db).Update(r.Context(), keyActor(r), *k.SecretID, vault.SecretPatch{Payload: &payload})
	}
	if k.SecretID == nil || errors.Is(err, vault.ErrSecretNotFound) {
		var id int64
		if id, err = h.storeKeySecret(r.Context(), r, a.ID, k.Label, remote.Plaintext, apiKeyHeader); err == nil {
			err = keys.SetSecret(r.Context(), k.ID, id)
		}
	}
	if err != nil {
		// The service already rotated: say so rather than lose the plaintext.
		logReqErr(r, "store rotated key in vault", err)
	}
	h.syncFrom(r.Context(), client, a.ID) // pick up the rotated-away key
	rotated, gerr := keys.Get(r.Context(), a.ID, k.ID)
	if gerr != nil || rotated == nil {
		jsonServerError(w, r, "reload api key", gerr)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonOK(w, apiKeyCreateResponse{Key: *rotated, Plaintext: remote.Plaintext})
}

// handleSync godoc
//
//	@Summary		Sync keys from the SEAD service
//	@Description	apis.keys.manage, sead mode only. Imports keys created elsewhere (CLI, other admins) as metadata-only rows and refreshes status, expiry and usage of known ones. Bridge's own owner contact, notes and vault copy are kept.
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
	client, ok := h.seadClient(w, r, a)
	if !ok {
		return
	}
	res, err := h.syncFrom(r.Context(), client, a.ID)
	if err != nil {
		var se *seadkeys.Error
		if errors.As(err, &se) || strings.HasPrefix(err.Error(), "sead keys:") {
			seadError(w, r, err)
			return
		}
		jsonServerError(w, r, "sync api keys", err)
		return
	}
	jsonOK(w, res)
}

// syncFrom upserts the service's keys into api_keys by external label.
func (h *apiKeyHandlers) syncFrom(ctx context.Context, client *seadkeys.Client, apiID int64) (apiKeySyncResponse, error) {
	var res apiKeySyncResponse
	remote, err := client.List(ctx)
	if err != nil {
		return res, err
	}
	keys := store.NewAPIKeyRepo(h.db.SQL)
	known, err := keys.ByExternalLabel(ctx, apiID)
	if err != nil {
		return res, err
	}
	for _, s := range remote {
		k := models.APIKey{APIID: apiID, Source: models.APIKeySourceSEAD, Notes: s.Notes}
		applySummary(&k, s)
		if id, ok := known[s.Label]; ok {
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
	res.Total = len(remote)
	return res, nil
}

// handleScopes godoc
//
//	@Summary		Scopes a SEAD key can carry
//	@Description	Any role that can see the API; sead mode only. The service's catalogue: "*" (everything), route scopes with the route patterns they open, and output modifiers such as "demo". A service without the catalogue endpoint answers 404.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{array}		seadkeys.ScopeInfo
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
	client, ok := h.seadClient(w, r, a)
	if !ok {
		return
	}
	scopes, err := client.Scopes(r.Context())
	if err != nil {
		seadError(w, r, err)
		return
	}
	jsonOK(w, scopes)
}

// handleUsage godoc
//
//	@Summary		A SEAD key's request counts
//	@Description	Any role that can see the API. Lifetime and per-day counts ("YYYYMMDD" → requests) for the last days (default 30), straight from the service.
//	@Tags			atlas
//	@Produce		json
//	@Param			id		path		int	true	"API catalog ID"
//	@Param			keyId	path		int	true	"Key ID"
//	@Param			days	query		int	false	"Days back (1-90, default 30)"
//	@Success		200		{object}	seadkeys.Usage
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
	if k.Source != models.APIKeySourceSEAD || k.ExternalLabel == nil {
		jsonError(w, http.StatusBadRequest, "usage is only tracked for SEAD keys")
		return
	}
	days := 30
	if _, err := fmt.Sscan(r.URL.Query().Get("days"), &days); err != nil || days < 1 || days > 90 {
		days = 30
	}
	client, ok := h.seadClient(w, r, a)
	if !ok {
		return
	}
	u, err := client.Usage(r.Context(), *k.ExternalLabel, days)
	if err != nil {
		seadError(w, r, err)
		return
	}
	jsonOK(w, u)
}

// handleSetKeyManagement godoc
//
//	@Summary		Set an API's key management
//	@Description	Admin. key_management none, manual or sead; for sead the admin base URL (the service root, before /admin/keys) and master key (sent as X-Admin-Key) plus an optional X-API-Key. Credentials are write-only: blank keeps the stored one, clear_* removes it.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"API catalog ID"
//	@Param			body	body		keyManagementRequest	true	"Mode and SEAD connection"
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
	if !decodeBody(w, r, &req) {
		return
	}
	if !models.ValidKeyManagement(req.KeyManagement) {
		jsonError(w, http.StatusBadRequest, "key_management must be none, manual or sead")
		return
	}
	u := store.KeyManagementUpdate{
		Mode:          req.KeyManagement,
		AdminBaseURL:  req.AdminBaseURL,
		ClearAdminKey: req.ClearAdminKey,
		ClearAPIKey:   req.ClearAPIKey,
	}
	if u.AdminBaseURL != "" && !strings.HasPrefix(u.AdminBaseURL, "http://") && !strings.HasPrefix(u.AdminBaseURL, "https://") {
		jsonError(w, http.StatusBadRequest, "admin_base_url must be an http(s) URL")
		return
	}
	var err error
	if u.AdminKey, err = h.encrypt(req.AdminKey); err != nil {
		jsonServerError(w, r, "encrypt master key", err)
		return
	}
	if u.APIKey, err = h.encrypt(req.APIKey); err != nil {
		jsonServerError(w, r, "encrypt api key", err)
		return
	}
	repo := store.NewAPICatalogRepo(h.db.SQL)
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

// encrypt returns nil for a blank value (keep the stored one).
func (h *apiKeyHandlers) encrypt(v string) (*store.EncryptedValue, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	c, n, err := h.db.Encryptor.Encrypt(v)
	if err != nil {
		return nil, err
	}
	return &store.EncryptedValue{Cipher: c, Nonce: n}, nil
}

// handleTestKeyManagement godoc
//
//	@Summary		Test an API's SEAD connection
//	@Description	Admin. Lists the service's keys with the given (unsaved) values, falling back to the stored ones for anything left blank. Always 200: {success, error | keys}.
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
	baseURL := strings.TrimSpace(req.AdminBaseURL)
	if baseURL == "" {
		baseURL = a.AdminBaseURL
	}
	adminKey, apiKey := strings.TrimSpace(req.AdminKey), strings.TrimSpace(req.APIKey)
	storedAdmin, storedAPI, err := store.NewAPICatalogRepo(h.db.SQL).AdminCredentials(r.Context(), a.ID)
	if err != nil {
		jsonServerError(w, r, "load sead credentials", err)
		return
	}
	if adminKey == "" && storedAdmin != nil {
		adminKey, _ = h.db.Encryptor.Decrypt(storedAdmin.Cipher, storedAdmin.Nonce)
	}
	if apiKey == "" && storedAPI != nil && !req.ClearAPIKey {
		apiKey, _ = h.db.Encryptor.Decrypt(storedAPI.Cipher, storedAPI.Nonce)
	}
	keys, err := seadkeys.New(baseURL, adminKey, apiKey).List(r.Context())
	if err != nil {
		jsonOK(w, keyManagementTestResponse{Success: false, Error: err.Error()})
		return
	}
	jsonOK(w, keyManagementTestResponse{Success: true, Keys: len(keys)})
}
