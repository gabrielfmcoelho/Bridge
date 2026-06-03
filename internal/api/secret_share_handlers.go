package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Share-link handlers — three authenticated owner-only routes plus the
// public GET /api/share/{token} redemption endpoint. The public endpoint
// is registered separately via registerPublicShare() because it must
// NOT be wrapped with the authenticated middleware.

type createShareLinkRequest struct {
	TTLSeconds int    `json:"ttl_seconds,omitempty"` // 0 = 24h default
	MaxViews   int    `json:"max_views,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

func (h *secretHandlers) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	var req createShareLinkRequest
	// Body is optional — all fields have defaults — so a missing body or
	// empty payload is fine; decode errors fall through to the rejection
	// below only when the body is malformed JSON.
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			jsonBadRequest(w, r, "invalid request body", err)
			return
		}
	}

	opts := vault.CreateShareLinkOpts{
		TTL:        time.Duration(req.TTLSeconds) * time.Second,
		MaxViews:   req.MaxViews,
		Passphrase: req.Passphrase,
	}
	tok, view, err := h.repo.CreateShareLink(r.Context(), actor, id, opts)
	switch {
	case err == nil:
		// Token is emitted ONLY here. The handler caller is responsible for
		// surfacing it to the operator (copy-to-clipboard etc) — it cannot
		// be retrieved later.
		jsonCreated(w, map[string]any{
			"id":             view.ID,
			"expires_at":     view.ExpiresAt,
			"max_views":      view.MaxViews,
			"has_passphrase": view.HasPassphrase,
			"token":          tok,
			// Human-facing link: points at the frontend /share/{token} page
			// (passphrase prompt + reveal UI), NOT the raw /api/share JSON
			// redemption endpoint. The page fetches /api/share/{token} itself.
			"url":            fmt.Sprintf("/share/%s", tok),
		})
	case errors.Is(err, vault.ErrShareTargetNotPersonal):
		jsonError(w, http.StatusBadRequest, "share links may only be created against personal secrets")
	case errors.Is(err, vault.ErrSecretNotFound):
		jsonError(w, http.StatusNotFound, "secret not found")
	case errors.Is(err, vault.ErrSecretForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	default:
		jsonServerError(w, r, "create share link", err)
	}
}

func (h *secretHandlers) handleListShareLinks(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	links, err := h.repo.ListShareLinks(r.Context(), actor, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if links == nil {
		links = []vault.SecretShareLinkView{}
	}
	jsonOK(w, links)
}

func (h *secretHandlers) handleRevokeShareLink(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	secretID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	linkID, err := pathInt64(r, "linkId")
	if err != nil {
		jsonBadRequest(w, r, "invalid link id", err)
		return
	}
	switch err := h.repo.RevokeShareLink(r.Context(), actor, secretID, linkID); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, vault.ErrShareLinkNotFound):
		jsonError(w, http.StatusNotFound, "share link not found")
	default:
		writeErr(w, r, err)
	}
}

// publicShareHandlers serves GET /api/share/{token} — the public redemption
// path. NOT wrapped with auth. Lives on its own type so the registration
// surface is unambiguous: this is the only endpoint we expose without
// authentication, and it has its own hardened response middleware (Task 3.5).
type publicShareHandlers struct {
	repo *vault.SecretRepo
}

func (h *publicShareHandlers) handleRedeem(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		jsonError(w, http.StatusNotFound, "share link not found")
		return
	}
	passphrase := r.URL.Query().Get("passphrase")
	plain, err := h.repo.RedeemShareLink(r.Context(), token, passphrase)

	// Hardening (Task 3.5 preview): no caching, no referrer leak — applied
	// regardless of outcome so failure responses don't leak presence via
	// cache or referer either.
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")

	switch {
	case err == nil:
		jsonOK(w, map[string]any{"payload": plain})
	case errors.Is(err, vault.ErrShareLinkPassphraseBad):
		// Distinct from 404 so the caller's UI can prompt for a retry.
		jsonError(w, http.StatusUnauthorized, "passphrase required or incorrect")
	case errors.Is(err, vault.ErrShareLinkNotFound),
		errors.Is(err, vault.ErrShareLinkExpired),
		errors.Is(err, vault.ErrShareLinkRevoked),
		errors.Is(err, vault.ErrShareLinkMaxViews):
		// Collapse the four "not redeemable" reasons to a single 404 in
		// public responses to avoid leaking link state to a guesser. The
		// distinct sentinels remain available internally for owner views.
		jsonError(w, http.StatusNotFound, "share link not found")
	default:
		jsonServerError(w, r, "redeem share link", err)
	}
}
