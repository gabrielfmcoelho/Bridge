package api

import (
	"net/http"
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

// apiTokenCreateRequest names the token and sets its lifetime in days
// (0 = never expires).
type apiTokenCreateRequest struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expires_in_days"`
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
//	@Description	Any role, browser session only. The token acts as the caller (same role, permissions and entidades). Send it as "Authorization: Bearer brg_…". The plaintext is in this response only.
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

	token, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		jsonServerError(w, r, "failed to generate api token", err)
		return
	}
	t := models.APIToken{UserID: auth.UserFromContext(r.Context()).ID, Name: req.Name, Prefix: prefix}
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
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user := auth.UserFromContext(r.Context())
	t, err := h.tokens.Get(r.Context(), id)
	if err != nil || t == nil || (t.UserID != user.ID && user.Role != "admin") {
		jsonError(w, http.StatusNotFound, "api token not found")
		return
	}
	if err := h.tokens.Revoke(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to revoke api token", err)
		return
	}
	jsonOK(w, StatusResponse{Status: "revoked"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *apiTokenHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/auth/tokens", h.handleList)
	rr.auth("POST /api/auth/tokens", h.handleCreate)
	rr.auth("DELETE /api/auth/tokens/{id}", h.handleRevoke)
}
