package api

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// apiTokenHandlers manage personal API tokens. Every user manages their own;
// admins also list and revoke everyone's. The routes need a browser session:
// a request authenticated by an API token is refused, so a leaked token can't
// mint itself a longer-lived successor.
type apiTokenHandlers struct {
	tokens *store.APITokenRepo
}

// apiTokenCreateRequest names the token, sets its lifetime in days (0 = never
// expires) and what it may call. user_id (admins only) issues the token for a
// service account instead of the caller.
type apiTokenCreateRequest struct {
	Name               string   `json:"name"`
	ExpiresInDays      int      `json:"expires_in_days"`
	Scopes             []string `json:"scopes" example:"hosts:read,dns:read"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute"`
	UserID             *int64   `json:"user_id"`
}

// apiTokenUsageResponse is a token's request counts.
type apiTokenUsageResponse struct {
	Lifetime int64            `json:"lifetime"`
	Daily    map[string]int64 `json:"daily"` // "YYYY-MM-DD" → requests
}

// apiTokenCreateResponse carries the plaintext token — returned only here,
// never again.
type apiTokenCreateResponse struct {
	Token    string          `json:"token"`
	APIToken models.APIToken `json:"api_token"`
}

const maxAPITokenDays = 3650

// sessionOnly refuses requests authenticated by an API token.
func sessionOnly(w http.ResponseWriter, r *http.Request) bool {
	if auth.APITokenFromContext(r.Context()) != 0 {
		jsonError(w, http.StatusForbidden, "api tokens cannot manage api tokens; sign in")
		return false
	}
	return true
}

// handleList godoc
//
//	@Summary		List API tokens
//	@Description	Any role, browser session only. The caller's own tokens, revoked ones included; admins pass all=true to see every user's.
//	@Tags			auth
//	@Produce		json
//	@Param			all			query		bool	false	"Admin only: every user's tokens"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.APIToken]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/auth/tokens [get]
func (h *apiTokenHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	if !sessionOnly(w, r) {
		return
	}
	user := auth.UserFromContext(r.Context())
	ownerID := user.ID
	if r.URL.Query().Get("all") == "true" {
		if user.Role != "admin" {
			jsonError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		ownerID = 0
	}
	tokens, err := h.tokens.List(r.Context(), ownerID)
	if err != nil {
		jsonServerError(w, r, "failed to list api tokens", err)
		return
	}
	jsonPaged(w, r, tokens)
}

// handleCreate godoc
//
//	@Summary		Create an API token
//	@Description	Any role, browser session only. The token acts as its owner (same role, permissions and entidades) narrowed to its scopes (GET /api/auth/tokens/scopes; at least one, each usable by the owner). Admins may pass user_id to issue it for a service account. Send it as "Authorization: Bearer brg_…". The plaintext is in this response only.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		apiTokenCreateRequest	true	"Name and lifetime in days (0 = never expires)"
//	@Success		201		{object}	apiTokenCreateResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/auth/tokens [post]
func (h *apiTokenHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !sessionOnly(w, r) {
		return
	}
	var req apiTokenCreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		jsonError(w, http.StatusBadRequest, "name is required (max 100 characters)")
		return
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > maxAPITokenDays {
		jsonError(w, http.StatusBadRequest, "expires_in_days must be between 0 (never) and 3650")
		return
	}
	if req.RateLimitPerMinute != nil && *req.RateLimitPerMinute < 0 {
		jsonError(w, http.StatusBadRequest, "rate_limit_per_minute must be 0 (unlimited) or more")
		return
	}
	caller := auth.UserFromContext(r.Context())
	owner := caller
	if req.UserID != nil && *req.UserID != caller.ID {
		if caller.Role != "admin" {
			jsonError(w, http.StatusForbidden, "only admins issue tokens for another account")
			return
		}
		u, err := store.NewUserRepo(h.tokens.DB()).GetByID(r.Context(), *req.UserID)
		if err != nil || u == nil || u.Kind != models.UserKindService {
			jsonError(w, http.StatusBadRequest, "user_id must be a service account")
			return
		}
		owner = u
	}
	if len(req.Scopes) == 0 {
		jsonError(w, http.StatusBadRequest, "choose at least one scope (GET /api/auth/tokens/scopes)")
		return
	}
	perms := store.NewPermissionRepo(h.tokens.DB())
	hasPerm := func(code string) bool { return perms.Has(r.Context(), owner.Role, code) }
	for _, sc := range req.Scopes {
		if !auth.ScopeUsable(owner.Role, hasPerm, sc) {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("scope %q does not exist or is beyond the owner's role", sc))
			return
		}
	}

	token, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		jsonServerError(w, r, "failed to generate api token", err)
		return
	}
	t := models.APIToken{UserID: owner.ID, Name: req.Name, Prefix: prefix, Scopes: slices.Compact(slices.Sorted(slices.Values(req.Scopes))), RateLimitPerMinute: req.RateLimitPerMinute}
	if req.ExpiresInDays > 0 {
		exp := time.Now().AddDate(0, 0, req.ExpiresInDays)
		t.ExpiresAt = &exp
	}
	if err := h.tokens.Create(r.Context(), &t, hash); err != nil {
		jsonServerError(w, r, "failed to create api token", err)
		return
	}
	jsonCreated(w, apiTokenCreateResponse{Token: token, APIToken: t})
}

// handleScopes godoc
//
//	@Summary		Scopes an API token can carry
//	@Description	Any role. "*" (full access) and one entry per scope, with the route patterns it opens and the least role (or permission) an owner needs to use it.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{array}		auth.ScopeInfo
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/auth/tokens/scopes [get]
func (h *apiTokenHandlers) handleScopes(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, auth.Catalogue())
}

// handleUsage godoc
//
//	@Summary		An API token's request counts
//	@Description	Browser session only; the token's owner or an admin (someone else's token answers 404). Per-day counts for the last days (default 30) and the lifetime total.
//	@Tags			auth
//	@Produce		json
//	@Param			id		path		int	true	"Token ID"
//	@Param			days	query		int	false	"Days back (1-365, default 30)"
//	@Success		200		{object}	apiTokenUsageResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/auth/tokens/{id}/usage [get]
func (h *apiTokenHandlers) handleUsage(w http.ResponseWriter, r *http.Request) {
	if !sessionOnly(w, r) {
		return
	}
	t, ok := h.ownedToken(w, r)
	if !ok {
		return
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 365 {
		days = 30
	}
	daily, total, err := h.tokens.Usage(r.Context(), t.ID, days)
	if err != nil {
		jsonServerError(w, r, "load api token usage", err)
		return
	}
	jsonOK(w, apiTokenUsageResponse{Lifetime: total, Daily: daily})
}

// ownedToken resolves {id} to a token of the caller (or any, for admins),
// answering 404 otherwise.
func (h *apiTokenHandlers) ownedToken(w http.ResponseWriter, r *http.Request) (*models.APIToken, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return nil, false
	}
	user := auth.UserFromContext(r.Context())
	t, err := h.tokens.Get(r.Context(), id)
	if err != nil || t == nil || (t.UserID != user.ID && user.Role != "admin") {
		jsonError(w, http.StatusNotFound, "api token not found")
		return nil, false
	}
	return t, true
}

// handleRevoke godoc
//
//	@Summary		Revoke an API token
//	@Description	Any role, browser session only. Your own tokens; admins may revoke anyone's. Someone else's token answers 404.
//	@Tags			auth
//	@Produce		json
//	@Param			id	path		int	true	"Token ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/auth/tokens/{id} [delete]
func (h *apiTokenHandlers) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if !sessionOnly(w, r) {
		return
	}
	t, ok := h.ownedToken(w, r)
	if !ok {
		return
	}
	if err := h.tokens.Revoke(r.Context(), t.ID); err != nil {
		jsonServerError(w, r, "failed to revoke api token", err)
		return
	}
	jsonOK(w, StatusResponse{Status: "revoked"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *apiTokenHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/auth/tokens", h.handleList)
	rr.auth("POST /api/auth/tokens", h.handleCreate)
	rr.auth("GET /api/auth/tokens/scopes", h.handleScopes)
	rr.auth("GET /api/auth/tokens/{id}/usage", h.handleUsage)
	rr.auth("DELETE /api/auth/tokens/{id}", h.handleRevoke)
}
