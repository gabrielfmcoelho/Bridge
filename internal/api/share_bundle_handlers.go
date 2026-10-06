package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Share-bundle handlers — authenticated owner routes for creating/listing/
// revoking heterogeneous share bundles (secrets + API docs), plus the public
// GET /api/share-bundle/{token} redemption (registered unwrapped in router.go,
// like /api/share/{token}). Bundles reuse vault.SecretRepo because they lean
// on the same crypto, ACL helpers, and secret reveal path.
type bundleHandlers struct {
	repo *vault.SecretRepo
}

func (h *bundleHandlers) actor(r *http.Request) (vault.ActorContext, bool) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return vault.ActorContext{}, false
	}
	return vault.ActorContext{UserID: u.ID, Role: u.Role}, true
}

type createBundleRequest struct {
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	TTLSeconds  int                     `json:"ttl_seconds"`
	MaxViews    int                     `json:"max_views"`
	Passphrase  string                  `json:"passphrase"`
	Items       []vault.BundleItemInput `json:"items"`
	// Who the link is for: a contact id, or free text. Both optional.
	RecipientContactID *int64 `json:"recipient_contact_id"`
	RecipientLabel     string `json:"recipient_label"`
}

// handleCreate godoc
//
//	@Summary		Create a share bundle
//	@Description	Any role; the caller must be able to access every item (secrets must be personal). Answers the bundle metadata plus the raw "token" and its guest "url" (/share/{token}) — shown only here. ttl_seconds < 0 means no expiry. 502 when a wiki item is requested but the wiki integration is unavailable.
//	@Tags			share-bundles
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createBundleRequest	true	"Title, lifetime, optional passphrase and items"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles [post]
func (h *bundleHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req createBundleRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	tok, view, err := h.repo.CreateBundle(r.Context(), actor, req.Items, vault.CreateBundleOpts{
		Title:       req.Title,
		Description: req.Description,
		TTL:         time.Duration(req.TTLSeconds) * time.Second,
		MaxViews:    req.MaxViews,
		Passphrase:  req.Passphrase,
		NoExpiry:    req.TTLSeconds < 0,

		RecipientContactID: req.RecipientContactID,
		RecipientLabel:     req.RecipientLabel,
	})
	switch {
	case err == nil:
		jsonCreated(w, map[string]any{
			"id":                   view.ID,
			"title":                view.Title,
			"description":          view.Description,
			"expires_at":           view.ExpiresAt,
			"max_views":            view.MaxViews,
			"has_passphrase":       view.HasPassphrase,
			"items":                view.Items,
			"recipient_contact_id": view.RecipientContactID,
			"recipient_label":      view.RecipientLabel,
			"recoverable":          view.Recoverable,
			"token":                tok,
			"url":                  fmt.Sprintf("/share/%s", tok),
		})
	case errors.Is(err, vault.ErrBundleEmpty):
		jsonError(w, http.StatusBadRequest, "a share bundle must contain at least one item")
	case errors.Is(err, vault.ErrShareTargetNotPersonal):
		jsonError(w, http.StatusBadRequest, "only personal secrets may be shared")
	case errors.Is(err, vault.ErrBundleInvalidItem):
		jsonError(w, http.StatusBadRequest, "invalid bundle item type")
	case errors.Is(err, vault.ErrBundleRecipientInvalid):
		jsonError(w, http.StatusBadRequest, "recipient contact not found")
	case errors.Is(err, vault.ErrSecretNotFound), errors.Is(err, vault.ErrBundleItemNotFound):
		jsonError(w, http.StatusNotFound, "bundle item not found")
	case errors.Is(err, vault.ErrSecretForbidden), errors.Is(err, vault.ErrBundleItemForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, vault.ErrBundleWikiUnavailable):
		jsonError(w, http.StatusBadGateway, "wiki integration is not available")
	default:
		jsonServerError(w, r, "create share bundle", err)
	}
}

// handleList godoc
//
//	@Summary		List share bundles
//	@Description	Any role. The caller's bundles; an admin gets every bundle. item_type with ref_id (or ref_key for wiki items) returns every bundle containing that item; secret_id returns only single-secret bundles for that secret.
//	@Tags			share-bundles
//	@Produce		json
//	@Param			item_type	query		string	false	"Item type to filter by (with ref_id or ref_key)"
//	@Param			ref_id		query		int		false	"Item ID (with item_type)"
//	@Param			ref_key		query		string	false	"Item key for wiki items (with item_type)"
//	@Param			secret_id	query		int		false	"Only single-secret bundles for this secret"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[vault.BundleView]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles [get]
func (h *bundleHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	// Generalized ?item_type=&ref_id= filter: return every bundle that CONTAINS
	// that item — the per-API / per-secret "already emitted links" list. Unlike
	// the legacy ?secret_id= filter below, this matches multi-item bundles (a
	// link carrying an API doc plus attached secrets still counts) and is
	// resolved server-side rather than over the full bundle set.
	itemType := r.URL.Query().Get("item_type")
	if rid := r.URL.Query().Get("ref_id"); rid != "" && itemType != "" {
		refID, perr := strconv.ParseInt(rid, 10, 64)
		if perr != nil {
			jsonBadRequest(w, r, "invalid ref_id", perr)
			return
		}
		bundles, err := h.repo.ListBundlesForItem(r.Context(), actor, itemType, refID)
		if err != nil {
			jsonServerError(w, r, "list share bundles", err)
			return
		}
		jsonPaged(w, r, bundles)
		return
	}
	// String-keyed sibling for wiki items, whose ref lives in ref_key (ref_id is
	// 0). Same "links exposing this item" semantics as the ref_id branch above.
	if rkey := r.URL.Query().Get("ref_key"); rkey != "" && itemType != "" {
		bundles, err := h.repo.ListBundlesForItemKey(r.Context(), actor, itemType, rkey)
		if err != nil {
			jsonServerError(w, r, "list share bundles", err)
			return
		}
		jsonPaged(w, r, bundles)
		return
	}

	bundles, err := h.repo.ListBundles(r.Context(), actor)
	if err != nil {
		jsonServerError(w, r, "list share bundles", err)
		return
	}
	// Optional ?secret_id= filter: return only the single-secret bundles for
	// that secret — the per-secret "active share links" list the secret UI
	// shows (the R3 replacement for GET /api/secrets/{id}/share-links).
	if sid := r.URL.Query().Get("secret_id"); sid != "" {
		if secretID, perr := strconv.ParseInt(sid, 10, 64); perr == nil {
			filtered := make([]vault.BundleView, 0, len(bundles))
			for _, b := range bundles {
				if len(b.Items) == 1 && b.Items[0].Type == "secret" && b.Items[0].RefID == secretID {
					filtered = append(filtered, b)
				}
			}
			bundles = filtered
		}
	}
	jsonPaged(w, r, bundles)
}

type reissueBundleRequest struct {
	Token       string                  `json:"token"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	TTLSeconds  int                     `json:"ttl_seconds"`
	MaxViews    int                     `json:"max_views"`
	Passphrase  string                  `json:"passphrase"`
	Items       []vault.BundleItemInput `json:"items"`
	// Who the link is for: a contact id, or free text. Both optional.
	RecipientContactID *int64 `json:"recipient_contact_id"`
	RecipientLabel     string `json:"recipient_label"`
}

// handleReissue rebuilds a bundle under a caller-supplied raw token, reviving a
// link whose row was already hard-deleted so the exact URL works again. Mirrors
// handleCreate's validation; 409 if a live link already owns the token.
//
//	@Summary		Reissue a share bundle under an existing token
//	@Description	Any role. Rebuilds a hard-deleted link under the caller-supplied raw token so the same URL works again; same item validation as create. 409 when a live link already owns the token.
//	@Tags			share-bundles
//	@Accept			json
//	@Produce		json
//	@Param			body	body		reissueBundleRequest	true	"Raw token, title, lifetime, optional passphrase and items"
//	@Success		201		{object}	vault.BundleView
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/reissue [post]
func (h *bundleHandlers) handleReissue(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req reissueBundleRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	view, err := h.repo.ReissueBundle(r.Context(), actor, req.Token, req.Items, vault.CreateBundleOpts{
		Title:       req.Title,
		Description: req.Description,
		TTL:         time.Duration(req.TTLSeconds) * time.Second,
		MaxViews:    req.MaxViews,
		Passphrase:  req.Passphrase,
		NoExpiry:    req.TTLSeconds < 0,

		RecipientContactID: req.RecipientContactID,
		RecipientLabel:     req.RecipientLabel,
	})
	switch {
	case err == nil:
		jsonCreated(w, view)
	case errors.Is(err, vault.ErrBundleTokenInUse):
		jsonError(w, http.StatusConflict, "a live link already uses this token; renew it instead")
	case errors.Is(err, vault.ErrBundleTokenInvalid):
		jsonError(w, http.StatusBadRequest, "token is required")
	case errors.Is(err, vault.ErrBundleEmpty):
		jsonError(w, http.StatusBadRequest, "a share bundle must contain at least one item")
	case errors.Is(err, vault.ErrShareTargetNotPersonal):
		jsonError(w, http.StatusBadRequest, "only personal secrets may be shared")
	case errors.Is(err, vault.ErrBundleInvalidItem):
		jsonError(w, http.StatusBadRequest, "invalid bundle item type")
	case errors.Is(err, vault.ErrBundleRecipientInvalid):
		jsonError(w, http.StatusBadRequest, "recipient contact not found")
	case errors.Is(err, vault.ErrSecretNotFound), errors.Is(err, vault.ErrBundleItemNotFound):
		jsonError(w, http.StatusNotFound, "bundle item not found")
	case errors.Is(err, vault.ErrSecretForbidden), errors.Is(err, vault.ErrBundleItemForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, vault.ErrBundleWikiUnavailable):
		jsonError(w, http.StatusBadGateway, "wiki integration is not available")
	default:
		jsonServerError(w, r, "reissue share bundle", err)
	}
}

type renewBundleRequest struct {
	TTLSeconds int  `json:"ttl_seconds"`
	MaxViews   *int `json:"max_views"`
}

// handleRenew extends a bundle's expiry (and reactivates it if revoked) without
// changing the token, so a previously-issued URL keeps working. Owner-only;
// optionally adjusts max_views.
//
//	@Summary		Renew a share bundle
//	@Description	Any role; owner or admin (others get 404). Extends expiry from now (ttl_seconds < 0 = never expires, 0 = 24h) and reactivates a revoked bundle, keeping the token. max_views > 0 sets the cap, <= 0 clears it, omitted keeps it.
//	@Tags			share-bundles
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Bundle ID"
//	@Param			body	body		renewBundleRequest	true	"New lifetime and optional view cap"
//	@Success		200		{object}	vault.BundleView
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/{id} [patch]
func (h *bundleHandlers) handleRenew(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid bundle id", err)
		return
	}
	var req renewBundleRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	view, err := h.repo.RenewBundle(r.Context(), actor, id, vault.RenewBundleOpts{
		TTL:      time.Duration(req.TTLSeconds) * time.Second,
		MaxViews: req.MaxViews,
		NoExpiry: req.TTLSeconds < 0,
	})
	switch {
	case err == nil:
		jsonOK(w, view)
	case errors.Is(err, vault.ErrBundleNotFound):
		jsonError(w, http.StatusNotFound, "share bundle not found")
	default:
		jsonServerError(w, r, "renew share bundle", err)
	}
}

type updateBundleItemsRequest struct {
	Items []vault.BundleItemInput `json:"items"`
}

// handleUpdateItems replaces a bundle's item set in place (owner-only), keeping
// the same token/URL. Lets an owner add/remove secrets, API docs, or wiki
// content on an already-shared link without re-issuing it. Every new item is
// re-validated for access; expiry/passphrase/view_count are preserved.
//
//	@Summary		Replace a share bundle's items
//	@Description	Any role; owner or admin (others get 404). Same token and URL; expiry, passphrase and view count are kept. Every item is re-validated for access.
//	@Tags			share-bundles
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Bundle ID"
//	@Param			body	body		updateBundleItemsRequest	true	"New item set"
//	@Success		200		{object}	vault.BundleView
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/{id}/items [put]
func (h *bundleHandlers) handleUpdateItems(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid bundle id", err)
		return
	}
	var req updateBundleItemsRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	view, err := h.repo.UpdateBundleItems(r.Context(), actor, id, req.Items)
	switch {
	case err == nil:
		jsonOK(w, view)
	case errors.Is(err, vault.ErrBundleNotFound):
		jsonError(w, http.StatusNotFound, "share bundle not found")
	case errors.Is(err, vault.ErrBundleEmpty):
		jsonError(w, http.StatusBadRequest, "a share bundle must contain at least one item")
	case errors.Is(err, vault.ErrBundleInvalidItem):
		jsonError(w, http.StatusBadRequest, "invalid bundle item type")
	case errors.Is(err, vault.ErrBundleRecipientInvalid):
		jsonError(w, http.StatusBadRequest, "recipient contact not found")
	case errors.Is(err, vault.ErrSecretNotFound), errors.Is(err, vault.ErrBundleItemNotFound):
		jsonError(w, http.StatusNotFound, "bundle item not found")
	case errors.Is(err, vault.ErrSecretForbidden), errors.Is(err, vault.ErrBundleItemForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, vault.ErrBundleWikiUnavailable):
		jsonError(w, http.StatusBadGateway, "wiki integration is not available")
	default:
		jsonServerError(w, r, "update share bundle items", err)
	}
}

// handleRevoke godoc
//
//	@Summary		Revoke a share bundle
//	@Description	Any role; owner or admin (others get 404).
//	@Tags			share-bundles
//	@Produce		json
//	@Param			id	path	int	true	"Bundle ID"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/{id} [delete]
func (h *bundleHandlers) handleRevoke(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid bundle id", err)
		return
	}
	switch err := h.repo.RevokeBundle(r.Context(), actor, id); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, vault.ErrBundleNotFound):
		jsonError(w, http.StatusNotFound, "share bundle not found")
	default:
		jsonServerError(w, r, "revoke share bundle", err)
	}
}

// handleAccessLog returns a bundle's anonymous access log (network metadata
// only), newest-first. Owner-only: a missing or foreign bundle collapses to 404
// so a caller can't probe another owner's link state.
//
//	@Summary		A share bundle's access log
//	@Description	Any role; owner or admin (others get 404). Newest first. action "redeem" is an anonymous guest opening (time, best-effort IP, user agent, whether a passphrase was used); action "reveal" is an admin reading the link back (actor_name).
//	@Tags			share-bundles
//	@Produce		json
//	@Param			id			path		int	true	"Bundle ID"
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[vault.BundleAccessEntry]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/{id}/access-log [get]
func (h *bundleHandlers) handleAccessLog(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid bundle id", err)
		return
	}
	entries, err := h.repo.BundleAccessLog(r.Context(), actor, id)
	switch {
	case err == nil:
		jsonPaged(w, r, entries)
	case errors.Is(err, vault.ErrBundleNotFound):
		jsonError(w, http.StatusNotFound, "share bundle not found")
	default:
		jsonServerError(w, r, "share bundle access log", err)
	}
}

type bundleRevealResponse struct {
	Token      string `json:"token"`
	URL        string `json:"url"`
	Passphrase string `json:"passphrase"`
}

// handleReveal godoc
//
//	@Summary		Reveal a share bundle's link and passphrase
//	@Description	Admin. Decrypts the bundle's token and passphrase ("" when it has none) and records the reveal in its access log. 409 when the bundle predates encrypted storage and cannot be recovered.
//	@Tags			share-bundles
//	@Produce		json
//	@Param			id	path		int	true	"Bundle ID"
//	@Success		200	{object}	bundleRevealResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		409	{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/{id}/reveal [post]
func (h *bundleHandlers) handleReveal(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid bundle id", err)
		return
	}
	sec, err := h.repo.RevealBundle(r.Context(), actor, id)
	switch {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		jsonOK(w, bundleRevealResponse{Token: sec.Token, URL: "/share/" + sec.Token, Passphrase: sec.Passphrase})
	case errors.Is(err, vault.ErrBundleNotFound):
		jsonError(w, http.StatusNotFound, "share bundle not found")
	case errors.Is(err, vault.ErrBundleNotRecoverable):
		jsonError(w, http.StatusConflict, "this link was created before links were stored and cannot be recovered")
	default:
		jsonServerError(w, r, "reveal share bundle", err)
	}
}

// handleRevokeAll godoc
//
//	@Summary		Revoke every share bundle
//	@Description	Admin. Revokes every bundle not yet revoked, whoever created it; answers how many were revoked.
//	@Tags			share-bundles
//	@Produce		json
//	@Success		200	{object}	map[string]int64
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/revoke-all [post]
func (h *bundleHandlers) handleRevokeAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	n, err := h.repo.RevokeAllBundles(r.Context(), actor)
	switch {
	case err == nil:
		jsonOK(w, map[string]int64{"revoked": n})
	case errors.Is(err, vault.ErrSecretForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	default:
		jsonServerError(w, r, "revoke all share bundles", err)
	}
}

// publicBundleHandlers serves GET /api/share-bundle/{token} — the public
// redemption path. Mirrors publicShareHandlers: no auth, hardened response
// headers, and all "not redeemable" reasons collapse to 404 so a guesser
// can't probe link state. Passphrase failures stay distinct (401) so the UI
// can prompt for a retry.
type publicBundleHandlers struct {
	repo *vault.SecretRepo
	db   *database.DB // keycloak_apis settings + API catalog for key enrichment
}

// handleRedeem godoc
//
//	@Summary		Redeem a share bundle
//	@Description	Public, no auth; the token is the capability. Returns the resolved contents, including decrypted secret payloads, and counts a view. Unknown, expired, revoked and exhausted links all answer 404; a missing or wrong passphrase answers 401. Sent with Cache-Control: no-store and Referrer-Policy: no-referrer.
//	@Tags			share-bundles
//	@Produce		json
//	@Param			token		path		string	true	"Raw bundle token"
//	@Param			passphrase	query		string	false	"Passphrase, when the bundle has one"
//	@Success		200			{object}	vault.BundlePayload
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/share-bundle/{token} [get]
func (h *publicBundleHandlers) handleRedeem(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	if token == "" {
		jsonError(w, http.StatusNotFound, "share bundle not found")
		return
	}
	passphrase := r.URL.Query().Get("passphrase")
	payload, err := h.repo.RedeemBundle(r.Context(), token, passphrase, vault.RedeemMeta{
		RemoteIP:  auth.ClientIP(r),
		UserAgent: r.UserAgent(),
	})
	switch {
	case err == nil:
		h.enrichKeys(r.Context(), payload)
		jsonOK(w, payload)
	case errors.Is(err, vault.ErrShareLinkPassphraseBad):
		jsonError(w, http.StatusUnauthorized, "passphrase required or incorrect")
	case errors.Is(err, vault.ErrShareLinkNotFound),
		errors.Is(err, vault.ErrShareLinkExpired),
		errors.Is(err, vault.ErrShareLinkRevoked),
		errors.Is(err, vault.ErrShareLinkMaxViews):
		jsonError(w, http.StatusNotFound, "share bundle not found")
	default:
		jsonServerError(w, r, "redeem share bundle", err)
	}
}
