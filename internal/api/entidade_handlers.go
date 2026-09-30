package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// entidadeHandlers owns the entidade tree (admin CRUD, read for pickers), the
// generic per-asset grants endpoint and the admin triage endpoints
// (unassigned list + bulk assign). Per-asset visibility itself is enforced in
// the repos via store.VisibleExpr — nothing here decides who sees what.
type entidadeHandlers struct {
	db        *database.DB
	entidades *store.EntidadeRepo
	grants    *store.AssetEntidadeRepo
}

func newEntidadeHandlers(db *database.DB) *entidadeHandlers {
	return &entidadeHandlers{db: db, entidades: store.NewEntidadeRepo(db.SQL), grants: store.NewAssetEntidadeRepo(db.SQL)}
}

// handleList godoc
//
//	@Summary		List entidades
//	@Description	Any role. The whole entidade tree, flat (for pickers).
//	@Tags			entidades
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.Entidade]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/entidades [get]
func (h *entidadeHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	items, err := h.entidades.List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list entidades", err)
		return
	}
	jsonPaged(w, r, items)
}

// handleGet godoc
//
//	@Summary		Get an entidade
//	@Description	Any role.
//	@Tags			entidades
//	@Produce		json
//	@Param			id	path		int	true	"Entidade ID"
//	@Success		200	{object}	models.Entidade
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/entidades/{id} [get]
func (h *entidadeHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	e, err := h.entidades.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load entidade", err)
		return
	}
	if e == nil {
		jsonError(w, http.StatusNotFound, "entidade not found")
		return
	}
	jsonOK(w, e)
}

func (h *entidadeHandlers) decodeEntidade(w http.ResponseWriter, r *http.Request) (*models.Entidade, bool) {
	var req models.Entidade
	if !decodeBody(w, r, &req) {
		return nil, false
	}
	req.Name = strings.TrimSpace(req.Name)
	if !requireFields(w, map[string]string{"name": req.Name}) {
		return nil, false
	}
	req.Slug = store.Slugify(req.Slug)
	if req.Slug == "" {
		req.Slug = store.Slugify(req.Name)
	}
	if req.Slug == "" {
		jsonError(w, http.StatusBadRequest, "slug is required")
		return nil, false
	}
	return &req, true
}

// handleCreate godoc
//
//	@Summary		Create an entidade
//	@Description	Admin. name is required; slug defaults to the slugified name. A taken slug or invalid parent answers 409.
//	@Tags			entidades
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.Entidade	true	"Entidade"
//	@Success		201		{object}	models.Entidade
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/entidades [post]
func (h *entidadeHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeEntidade(w, r)
	if !ok {
		return
	}
	if err := h.entidades.Create(r.Context(), req); err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "slug already exists or parent is invalid", err)
		return
	}
	jsonCreated(w, req)
}

// handleUpdate godoc
//
//	@Summary		Update an entidade
//	@Description	Admin. A rejected update (taken slug, cyclic parent, ...) answers 409.
//	@Tags			entidades
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"Entidade ID"
//	@Param			body	body		models.Entidade	true	"Entidade"
//	@Success		200		{object}	models.Entidade
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/entidades/{id} [put]
func (h *entidadeHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	req, ok := h.decodeEntidade(w, r)
	if !ok {
		return
	}
	req.ID = id
	if err := h.entidades.Update(r.Context(), req); err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "update rejected: "+err.Error(), err)
		return
	}
	e, err := h.entidades.Get(r.Context(), id)
	if err != nil || e == nil {
		jsonError(w, http.StatusNotFound, "entidade not found")
		return
	}
	jsonOK(w, e)
}

// handleDelete godoc
//
//	@Summary		Delete an entidade
//	@Description	Admin. An entidade that still has children, members or assets answers 409.
//	@Tags			entidades
//	@Produce		json
//	@Param			id	path		int	true	"Entidade ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		409	{object}	httpx.ErrorResponse
//	@Router			/api/entidades/{id} [delete]
func (h *entidadeHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	err := h.entidades.Delete(r.Context(), id)
	if errors.Is(err, store.ErrEntidadeInUse) {
		jsonError(w, http.StatusConflict, "entidade still has children, members or assets")
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to delete entidade", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// assetTypeParam validates ?asset_type= / {type} against the registry.
func assetTypeParam(w http.ResponseWriter, raw string) (store.AssetType, bool) {
	t := store.AssetType(raw)
	if _, ok := store.AssetOf(t); !ok {
		jsonError(w, http.StatusBadRequest, "unknown asset_type")
		return "", false
	}
	return t, true
}

// handleUnassigned godoc
//
//	@Summary		List assets without entidade grants
//	@Description	Admin. Assets of one type with no grant rows (admin-only today). Defaults to 50 per page.
//	@Tags			entidades
//	@Produce		json
//	@Param			asset_type	query		string	true	"Asset type"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200, default 50)"
//	@Success		200			{object}	ListEnvelope[store.UnassignedRow]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/entidades/unassigned [get]
func (h *entidadeHandlers) handleUnassigned(w http.ResponseWriter, r *http.Request) {
	t, ok := assetTypeParam(w, r.URL.Query().Get("asset_type"))
	if !ok {
		return
	}
	pp := parsePageParams(r)
	if pp.Unbounded() {
		pp.PerPage = 50
	}
	rows, total, err := h.grants.ListUnassigned(r.Context(), t, pp.Page, pp.PerPage)
	if err != nil {
		jsonServerError(w, r, "failed to list unassigned assets", err)
		return
	}
	jsonList(w, rows, metaFor(pp, total))
}

// entidadeBulkAssignRequest sets the same grants on many assets of one type.
type entidadeBulkAssignRequest struct {
	AssetType string  `json:"asset_type"`
	AssetIDs  []int64 `json:"asset_ids"`
	models.AssetGrantsInput
}

// handleBulkAssign godoc
//
//	@Summary		Assign entidades to many assets
//	@Description	Admin. Replaces the grants of every listed asset; answers {"status": "ok", "count": N}. An unknown entidade answers 409.
//	@Tags			entidades
//	@Accept			json
//	@Produce		json
//	@Param			body	body		entidadeBulkAssignRequest	true	"Asset type, IDs and grants"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/entidades/bulk-assign [post]
func (h *entidadeHandlers) handleBulkAssign(w http.ResponseWriter, r *http.Request) {
	var req entidadeBulkAssignRequest
	if !decodeBody(w, r, &req) {
		return
	}
	t, ok := assetTypeParam(w, req.AssetType)
	if !ok {
		return
	}
	if len(req.AssetIDs) == 0 {
		jsonError(w, http.StatusBadRequest, "asset_ids is required")
		return
	}
	g, err := store.ResolveGrants(r.Context(), req.AssetGrantsInput, nil)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.grants.BulkReplace(r.Context(), t, req.AssetIDs, g); err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "bulk assign rejected (unknown entidade?)", err)
		return
	}
	jsonOK(w, map[string]any{"status": "ok", "count": len(req.AssetIDs)})
}

// assetRef resolves {type}/{id} from the path and enforces visibility: an
// invisible asset is a 404, never a 403, so existence isn't leaked.
func (h *entidadeHandlers) assetRef(w http.ResponseWriter, r *http.Request) (store.AssetType, int64, bool) {
	t, ok := assetTypeParam(w, r.PathValue("type"))
	if !ok {
		return "", 0, false
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return "", 0, false
	}
	visible, err := store.CanSee(r.Context(), h.db.SQL, t, id)
	if err != nil {
		jsonServerError(w, r, "failed to check visibility", err)
		return "", 0, false
	}
	if !visible {
		jsonError(w, http.StatusNotFound, "not found")
		return "", 0, false
	}
	return t, id, true
}

// handleGetAssetGrants godoc
//
//	@Summary		Get an asset's entidade grants
//	@Description	Any role. Invisible assets answer 404.
//	@Tags			entidades
//	@Produce		json
//	@Param			type	path		string	true	"Asset type (host, service, dns, project, ...)"
//	@Param			id		path		int		true	"Asset ID"
//	@Success		200		{object}	models.AssetGrants
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/assets/{type}/{id}/entidades [get]
func (h *entidadeHandlers) handleGetAssetGrants(w http.ResponseWriter, r *http.Request) {
	t, id, ok := h.assetRef(w, r)
	if !ok {
		return
	}
	g, err := h.grants.Get(r.Context(), t, id)
	if err != nil {
		jsonServerError(w, r, "failed to load grants", err)
		return
	}
	jsonOK(w, g)
}

// handlePutAssetGrants godoc
//
//	@Summary		Replace an asset's entidade grants
//	@Description	Editor+. Invisible assets answer 404; granting an entidade outside the caller's visible set answers 403; an unknown entidade answers 409.
//	@Tags			entidades
//	@Accept			json
//	@Produce		json
//	@Param			type	path		string					true	"Asset type (host, service, dns, project, ...)"
//	@Param			id		path		int						true	"Asset ID"
//	@Param			body	body		models.AssetGrantsInput	true	"Grants"
//	@Success		200		{object}	models.AssetGrants
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/assets/{type}/{id}/entidades [put]
func (h *entidadeHandlers) handlePutAssetGrants(w http.ResponseWriter, r *http.Request) {
	t, id, ok := h.assetRef(w, r)
	if !ok {
		return
	}
	var req models.AssetGrantsInput
	if !decodeBody(w, r, &req) {
		return
	}
	existing, err := h.grants.Get(r.Context(), t, id)
	if err != nil {
		jsonServerError(w, r, "failed to load grants", err)
		return
	}
	g, err := store.ResolveGrants(r.Context(), req, &existing)
	if errors.Is(err, store.ErrEntidadeForbidden) {
		jsonError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.grants.Replace(r.Context(), h.db.SQL, t, id, g); err != nil {
		jsonErrorLogged(w, r, http.StatusConflict, "grants rejected (unknown entidade?)", err)
		return
	}
	jsonOK(w, g)
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *entidadeHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/entidades", h.handleList)
	rr.role("admin", "POST /api/entidades", h.handleCreate)
	rr.role("admin", "GET /api/entidades/unassigned", h.handleUnassigned)
	rr.role("admin", "POST /api/entidades/bulk-assign", h.handleBulkAssign)
	rr.auth("GET /api/entidades/{id}", h.handleGet)
	rr.role("admin", "PUT /api/entidades/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/entidades/{id}", h.handleDelete)
	rr.auth("GET /api/assets/{type}/{id}/entidades", h.handleGetAssetGrants)
	rr.role("editor", "PUT /api/assets/{type}/{id}/entidades", h.handlePutAssetGrants)
}
