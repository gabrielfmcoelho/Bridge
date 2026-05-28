package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// secretHandlers serves the unified /api/secrets/* surface introduced in
// Plans.md Task 1.6 / spec §6. The two ACL matrices (spec §5.1 + §5.2) are
// fully enforced inside SecretRepo via decideAccess, so these handlers stay
// thin: parse → call repo → map errors → render JSON.
//
// Auth is intentionally not wrapped here. Production wires authenticated()
// via register's wrap parameter; tests wire identity + a per-request actor
// injector. Either way the actor reaches the handler through the request
// context (auth.UserFromContext) and is converted to vault.ActorContext.
type secretHandlers struct {
	db   *database.DB
	repo *vault.SecretRepo
}

// register wires all secret routes onto mux. The wrap callback adapts auth
// for the target environment (authenticated in prod, identity in tests).
// Centralising routes here keeps prod + tests in lockstep.
func (h *secretHandlers) register(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	mux.Handle("GET /api/secrets", wrap(http.HandlerFunc(h.handleList)))
	mux.Handle("POST /api/secrets", wrap(http.HandlerFunc(h.handleCreate)))
	// Literal sub-paths (mine, trash, env) must be registered before the
	// catch-all /{id} pattern. Go 1.22+ mux specificity picks the literal
	// match automatically, but listing them first keeps the intent obvious.
	mux.Handle("GET /api/secrets/mine", wrap(http.HandlerFunc(h.handleMine)))
	mux.Handle("GET /api/secrets/trash", wrap(http.HandlerFunc(h.handleTrash)))
	// Env-var bundle endpoints (Phase 2 — Tasks 2.2 + 2.3).
	mux.Handle("POST /api/secrets/env/bulk", wrap(http.HandlerFunc(h.handleEnvBulk)))
	mux.Handle("GET /api/secrets/env", wrap(http.HandlerFunc(h.handleEnvList)))
	// Share-link endpoints — owner-only mgmt (Phase 3 Task 3.3). The
	// public redemption GET /api/share/{token} is wired in router.go
	// outside the auth wrapper.
	mux.Handle("POST /api/secrets/{id}/share-links", wrap(http.HandlerFunc(h.handleCreateShareLink)))
	mux.Handle("GET /api/secrets/{id}/share-links", wrap(http.HandlerFunc(h.handleListShareLinks)))
	mux.Handle("DELETE /api/secrets/{id}/share-links/{linkId}", wrap(http.HandlerFunc(h.handleRevokeShareLink)))
	mux.Handle("GET /api/secrets/{id}", wrap(http.HandlerFunc(h.handleGetMetadata)))
	mux.Handle("GET /api/secrets/{id}/reveal", wrap(http.HandlerFunc(h.handleReveal)))
	mux.Handle("GET /api/secrets/{id}/history", wrap(http.HandlerFunc(h.handleHistory)))
	mux.Handle("PUT /api/secrets/{id}", wrap(http.HandlerFunc(h.handleUpdate)))
	mux.Handle("DELETE /api/secrets/{id}", wrap(http.HandlerFunc(h.handleDelete)))
	mux.Handle("POST /api/secrets/{id}/restore", wrap(http.HandlerFunc(h.handleRestore)))
}

// actor reads the authenticated user from the request context and converts
// to the ACL-aware vault.ActorContext. Returns nil if no user is present —
// that condition is a programmer error (auth middleware should have rejected
// the request) so handlers below treat it as 401.
func (h *secretHandlers) actor(r *http.Request) (vault.ActorContext, bool) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return vault.ActorContext{}, false
	}
	return vault.ActorContext{UserID: u.ID, Role: u.Role}, true
}

// writeRepoErr maps vault repo sentinels to HTTP status codes. Anything else
// is a 500 (logged via jsonServerError).
func writeRepoErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, vault.ErrSecretNotFound):
		jsonError(w, http.StatusNotFound, "secret not found")
	case errors.Is(err, vault.ErrSecretForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	default:
		jsonServerError(w, r, "secret repo error", err)
	}
}

// --- handlers ---------------------------------------------------------------

func (h *secretHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	filter := vault.SecretFilter{
		Scope:      models.SecretScope(r.URL.Query().Get("scope")),
		Type:       models.SecretType(r.URL.Query().Get("type")),
		Visibility: models.SecretVisibility(r.URL.Query().Get("visibility")),
		GroupLabel: r.URL.Query().Get("group_label"),
	}
	if v := r.URL.Query().Get("parent_id"); v != "" {
		if pid, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.ParentID = &pid
		}
	}
	if r.URL.Query().Get("include_deleted") == "1" {
		filter.IncludeDeleted = true
	}
	views, err := h.repo.List(r.Context(), actor, filter)
	if err != nil {
		writeRepoErr(w, r, err)
		return
	}
	if views == nil {
		views = []vault.SecretView{}
	}
	jsonOK(w, views)
}

type createSecretRequest struct {
	Type        string  `json:"type"`
	Scope       string  `json:"scope"`
	Visibility  string  `json:"visibility"`
	ParentID    *int64  `json:"parent_id,omitempty"`
	OwnerUserID *int64  `json:"owner_user_id,omitempty"` // optional; defaults to caller
	Name        string  `json:"name"`
	GroupLabel  *string `json:"group_label,omitempty"`
	Description *string `json:"description,omitempty"`
	Payload     string  `json:"payload"`
}

func (h *secretHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req createSecretRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Payload == "" {
		jsonError(w, http.StatusBadRequest, "payload required")
		return
	}
	owner := actor.UserID
	if req.OwnerUserID != nil {
		owner = *req.OwnerUserID
	}
	s := &models.Secret{
		Type:        models.SecretType(req.Type),
		Scope:       models.SecretScope(req.Scope),
		Visibility:  models.SecretVisibility(req.Visibility),
		ParentID:    req.ParentID,
		OwnerUserID: owner,
		Name:        req.Name,
		GroupLabel:  req.GroupLabel,
		Description: req.Description,
		KeyVersion:  1,
		CreatedBy:   actor.UserID,
	}
	id, err := h.repo.Create(r.Context(), actor, s, req.Payload)
	if err != nil {
		// Validation errors (invalid type/scope/visibility, missing fields,
		// CHECK-constraint violations) surface as plain errors from
		// Secret.Validate; map those to 400.
		if errors.Is(err, vault.ErrSecretForbidden) {
			jsonError(w, http.StatusForbidden, "forbidden")
			return
		}
		if errors.Is(err, vault.ErrSecretNotFound) {
			jsonError(w, http.StatusNotFound, "secret not found")
			return
		}
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	jsonCreated(w, map[string]any{"id": id})
}

func (h *secretHandlers) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	v, err := h.repo.GetMetadata(r.Context(), actor, id)
	if err != nil {
		writeRepoErr(w, r, err)
		return
	}
	jsonOK(w, v)
}

func (h *secretHandlers) handleReveal(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	plain, err := h.repo.Reveal(r.Context(), actor, id)
	if err != nil {
		writeRepoErr(w, r, err)
		return
	}
	jsonOK(w, map[string]any{"payload": plain})
}

// updateSecretRequest accepts the patchable fields and explicitly captures
// `visibility` so the handler can reject any attempt to change it with a
// 422. Per spec §6 visibility is immutable on PUT; silently dropping the
// field would be surprising to clients.
type updateSecretRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	GroupLabel  *string         `json:"group_label,omitempty"`
	Payload     *string         `json:"payload,omitempty"`
	Visibility  json.RawMessage `json:"visibility,omitempty"` // sentinel — must not be present
}

func (h *secretHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	var req updateSecretRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if len(req.Visibility) > 0 && string(req.Visibility) != "null" {
		jsonError(w, http.StatusUnprocessableEntity, "visibility is immutable; create a new secret with the desired visibility instead")
		return
	}
	patch := vault.SecretPatch{
		Name:        req.Name,
		Description: req.Description,
		GroupLabel:  req.GroupLabel,
		Payload:     req.Payload,
	}
	if err := h.repo.Update(r.Context(), actor, id, patch); err != nil {
		writeRepoErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func (h *secretHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	if err := h.repo.SoftDelete(r.Context(), actor, id); err != nil {
		writeRepoErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *secretHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	if err := h.repo.Restore(r.Context(), actor, id); err != nil {
		writeRepoErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *secretHandlers) handleHistory(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	rows, err := h.repo.History(r.Context(), actor, id)
	if err != nil {
		writeRepoErr(w, r, err)
		return
	}
	if rows == nil {
		rows = []models.SecretAuditLog{}
	}
	jsonOK(w, rows)
}

func (h *secretHandlers) handleTrash(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	views, err := h.repo.ListTrash(r.Context(), actor)
	if err != nil {
		writeRepoErr(w, r, err)
		return
	}
	if views == nil {
		views = []vault.SecretView{}
	}
	jsonOK(w, views)
}

func (h *secretHandlers) handleMine(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	views, err := h.repo.List(r.Context(), actor, vault.SecretFilter{
		Visibility:  models.SecretVisibilityPersonal,
		OwnerUserID: &actor.UserID,
	})
	if err != nil {
		writeRepoErr(w, r, err)
		return
	}
	if views == nil {
		views = []vault.SecretView{}
	}
	jsonOK(w, views)
}

