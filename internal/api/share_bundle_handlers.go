package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
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
	Title      string                  `json:"title"`
	TTLSeconds int                     `json:"ttl_seconds"`
	MaxViews   int                     `json:"max_views"`
	Passphrase string                  `json:"passphrase"`
	Items      []vault.BundleItemInput `json:"items"`
}

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
		Title:      req.Title,
		TTL:        time.Duration(req.TTLSeconds) * time.Second,
		MaxViews:   req.MaxViews,
		Passphrase: req.Passphrase,
	})
	switch {
	case err == nil:
		jsonCreated(w, map[string]any{
			"id":             view.ID,
			"title":          view.Title,
			"expires_at":     view.ExpiresAt,
			"max_views":      view.MaxViews,
			"has_passphrase": view.HasPassphrase,
			"items":          view.Items,
			"token":          tok,
			"url":            fmt.Sprintf("/share/%s", tok),
		})
	case errors.Is(err, vault.ErrBundleEmpty):
		jsonError(w, http.StatusBadRequest, "a share bundle must contain at least one item")
	case errors.Is(err, vault.ErrShareTargetNotPersonal):
		jsonError(w, http.StatusBadRequest, "only personal secrets may be shared")
	case errors.Is(err, vault.ErrBundleInvalidItem):
		jsonError(w, http.StatusBadRequest, "invalid bundle item type")
	case errors.Is(err, vault.ErrSecretNotFound), errors.Is(err, vault.ErrBundleItemNotFound):
		jsonError(w, http.StatusNotFound, "bundle item not found")
	case errors.Is(err, vault.ErrSecretForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	default:
		jsonServerError(w, r, "create share bundle", err)
	}
}

func (h *bundleHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
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
	if bundles == nil {
		bundles = []vault.BundleView{}
	}
	jsonOK(w, bundles)
}

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

// publicBundleHandlers serves GET /api/share-bundle/{token} — the public
// redemption path. Mirrors publicShareHandlers: no auth, hardened response
// headers, and all "not redeemable" reasons collapse to 404 so a guesser
// can't probe link state. Passphrase failures stay distinct (401) so the UI
// can prompt for a retry.
type publicBundleHandlers struct {
	repo *vault.SecretRepo
}

func (h *publicBundleHandlers) handleRedeem(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	if token == "" {
		jsonError(w, http.StatusNotFound, "share bundle not found")
		return
	}
	passphrase := r.URL.Query().Get("passphrase")
	payload, err := h.repo.RedeemBundle(r.Context(), token, passphrase)
	switch {
	case err == nil:
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
