package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
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

// registerRoutes wires all secret routes (self-registration, R2). Auth is at
// the perimeter only; per-row ACL (RBAC for shared, ownership for personal)
// lives in vault. Handler tests pass a registrar without a db, so no auth
// middleware, and inject the actor themselves.
func (h *secretHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/secrets", h.handleList)
	rr.auth("POST /api/secrets", h.handleCreate)
	rr.auth("GET /api/secrets/mine", h.handleMine)
	rr.auth("GET /api/secrets/trash", h.handleTrash)
	// Env-var bundle endpoints (Phase 2 — Tasks 2.2 + 2.3).
	rr.auth("POST /api/secrets/env/bulk", h.handleEnvBulk)
	rr.auth("GET /api/secrets/env", h.handleEnvList)
	// Consolidate the same credential repeated across hosts (admin).
	rr.role("admin", "GET /api/secrets/consolidation", h.handleConsolidationPlan)
	rr.role("admin", "POST /api/secrets/consolidation", h.handleConsolidationApply)
	// Per-secret public sharing was retired in R3: a single-secret share is now
	// a one-item share bundle (POST /api/share-bundles). The owner-only
	// management UI lists/revokes via /api/share-bundles?secret_id=.
	rr.auth("GET /api/secrets/{id}", h.handleGetMetadata)
	rr.auth("GET /api/secrets/{id}/reveal", h.handleReveal)
	rr.auth("GET /api/secrets/{id}/history", h.handleHistory)
	rr.auth("PUT /api/secrets/{id}", h.handleUpdate)
	rr.auth("DELETE /api/secrets/{id}", h.handleDelete)
	rr.auth("POST /api/secrets/{id}/restore", h.handleRestore)
	// Shared-credential host links: reuse one avulso password credential across
	// N hosts via host_remote_users.secret_id (secret_host_link_handlers.go).
	rr.auth("GET /api/secrets/{id}/hosts", h.handleListLinkedHosts)
	rr.auth("POST /api/secrets/{id}/hosts", h.handleLinkHosts)
	rr.auth("DELETE /api/secrets/{id}/hosts/{host_id}", h.handleUnlinkHost)
}

// --- handlers ---------------------------------------------------------------

func (h *secretHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	filter := vault.SecretFilter{
		Scope:      models.SecretScope(r.URL.Query().Get("scope")),
		Type:       models.SecretType(r.URL.Query().Get("type")),
		Visibility: models.SecretVisibility(r.URL.Query().Get("visibility")),
		GroupLabel: r.URL.Query().Get("group_label"),
		Query:      strings.TrimSpace(r.URL.Query().Get("q")),
		Kind:       r.URL.Query().Get("kind"),
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
		writeErr(w, r, err)
		return
	}
	if views == nil {
		views = []vault.SecretView{}
	}
	jsonPaged(w, r, views)
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
	// Username: the login a password is for (JSON payloads carry their own).
	Username string `json:"username,omitempty"`
	Payload  string `json:"payload"`
	// Entidade grants — honoured only for shared avulso secrets (root assets);
	// every other scope/visibility inherits from its parent or is owner-only.
	models.AssetGrantsInput
}

// ownsGrants reports whether a secret carries its own entidade grants
// (asset_type 'secret'): only shared avulso ones do.
func ownsGrants(scope models.SecretScope, vis models.SecretVisibility) bool {
	return scope == models.SecretScopeAvulso && vis == models.SecretVisibilityShared
}

func (h *secretHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
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
		Username:    strings.TrimSpace(req.Username),
		KeyVersion:  1,
		CreatedBy:   actor.UserID,
	}
	var (
		grants    models.AssetGrants
		hasGrants = ownsGrants(s.Scope, s.Visibility)
	)
	if hasGrants {
		g, ok := resolveGrants(w, r, req.AssetGrantsInput, nil)
		if !ok {
			return
		}
		grants = g
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
	if hasGrants {
		if err := store.NewAssetEntidadeRepo(h.db.SQL).Replace(r.Context(), h.db.SQL, store.AssetSecret, id, grants); err != nil {
			// Row exists but is admin-only until grants are set.
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
	}
	jsonCreated(w, map[string]any{"id": id})
}

func (h *secretHandlers) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
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
	v, err := h.repo.GetMetadata(r.Context(), actor, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if ownsGrants(v.Scope, v.Visibility) {
		if g, err := store.NewAssetEntidadeRepo(h.db.SQL).Get(r.Context(), store.AssetSecret, id); err == nil {
			v.Entidades = &g
		}
	}
	jsonOK(w, v)
}

func (h *secretHandlers) handleReveal(w http.ResponseWriter, r *http.Request) {
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
	plain, err := h.repo.Reveal(r.Context(), actor, id)
	if err != nil {
		writeErr(w, r, err)
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

	// Entidade grants — shared avulso only; ignored otherwise.
	models.AssetGrantsInput
}

func (h *secretHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
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
	// Resolve grants before mutating so a bad entidade fails the whole PUT;
	// Replace only after Update passed the repo's ACL (editor+ on shared).
	var newGrants *models.AssetGrants
	if req.AssetGrantsInput.Present() {
		v, err := h.repo.GetMetadata(r.Context(), actor, id)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if ownsGrants(v.Scope, v.Visibility) {
			grantsRepo := store.NewAssetEntidadeRepo(h.db.SQL)
			existing, err := grantsRepo.Get(r.Context(), store.AssetSecret, id)
			if err != nil {
				jsonServerError(w, r, "failed to load entidades", err)
				return
			}
			g, ok := resolveGrants(w, r, req.AssetGrantsInput, &existing)
			if !ok {
				return
			}
			newGrants = &g
		}
	}
	if err := h.repo.Update(r.Context(), actor, id, patch); err != nil {
		writeErr(w, r, err)
		return
	}
	if newGrants != nil {
		if err := store.NewAssetEntidadeRepo(h.db.SQL).Replace(r.Context(), h.db.SQL, store.AssetSecret, id, *newGrants); err != nil {
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func (h *secretHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.repo.SoftDelete(r.Context(), actor, id); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *secretHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
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
	if err := h.repo.Restore(r.Context(), actor, id); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *secretHandlers) handleHistory(w http.ResponseWriter, r *http.Request) {
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
	rows, err := h.repo.History(r.Context(), actor, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if rows == nil {
		rows = []models.SecretAuditLog{}
	}
	jsonPaged(w, r, rows)
}

func (h *secretHandlers) handleTrash(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	views, err := h.repo.ListTrash(r.Context(), actor)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if views == nil {
		views = []vault.SecretView{}
	}
	jsonPaged(w, r, views)
}

func (h *secretHandlers) handleMine(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	views, err := h.repo.List(r.Context(), actor, vault.SecretFilter{
		Visibility:  models.SecretVisibilityPersonal,
		OwnerUserID: &actor.UserID,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if views == nil {
		views = []vault.SecretView{}
	}
	jsonPaged(w, r, views)
}
