package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/apicatalog"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// apiCatalogHandlers serves the Atlas REST API catalog surface
// (/api/api-catalog/*). Browsing is authenticated; importing / mutating
// requires the editor role; deletion and restore require admin. Visibility is
// the entidades contract; an API links to services and projects through
// api_service_links / api_project_links. Handlers stay thin: parse → repo →
// render, following the project_handlers.go convention.
type apiCatalogHandlers struct {
	db *database.DB
	// allowPrivateFetch controls whether import-from-URL may target private,
	// loopback, or link-local addresses. Bridge is an internal infrastructure
	// tool whose primary use case is cataloging INTERNAL APIs (often behind
	// split-horizon DNS resolving to RFC1918 addresses), and import is already
	// editor-gated — so we DEFAULT TO ALLOW. Hardened/internet-exposed
	// deployments can re-enable the strict SSRF guard by setting
	// ATLAS_BLOCK_PRIVATE_SPEC_FETCH=1 (or true).
	allowPrivateFetch bool
}

// atlasAllowPrivateFetch reports whether private-address spec fetches are
// permitted. Default true; ATLAS_BLOCK_PRIVATE_SPEC_FETCH in {1,true,yes}
// flips on strict SSRF blocking.
func atlasAllowPrivateFetch() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ATLAS_BLOCK_PRIVATE_SPEC_FETCH"))) {
	case "1", "true", "yes":
		return false
	default:
		return true
	}
}

// registerRoutes wires the catalog routes (self-registration, R2). Browsing is
// authenticated; import/mutate is editor; delete is admin. Handler tests pass a
// registrar without a db (no auth middleware) and inject a fixed identity.
func (h *apiCatalogHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/api-catalog", h.handleList)
	rr.auth("GET /api/api-catalog/search", h.handleSearchOperations)
	rr.auth("GET /api/api-catalog/trash", h.handleListTrash)
	rr.role("admin", "POST /api/api-catalog/{id}/restore", h.handleRestore)
	rr.role("editor", "POST /api/api-catalog/{id}/spec", h.handleReplaceSpec)
	rr.role("editor", "POST /api/api-catalog/import/upload", h.handleImportUpload)
	rr.role("editor", "POST /api/api-catalog/import/url", h.handleImportURL)
	rr.auth("GET /api/api-catalog/{id}", h.handleGet)
	rr.auth("GET /api/api-catalog/{id}/spec", h.handleGetSpec)
	rr.auth("POST /api/api-catalog/{id}/spec/filter", h.handleFilterSpec)
	rr.role("editor", "POST /api/api-catalog/{id}/refetch", h.handleRefetch)
	rr.role("editor", "PUT /api/api-catalog/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/api-catalog/{id}", h.handleDelete)
}

// --- list / search ----------------------------------------------------------

// handleList godoc
//
//	@Summary		List catalogued APIs
//	@Description	Any role. Only APIs visible to the caller (entidade scoping).
//	@Tags			atlas
//	@Produce		json
//	@Param			q			query		string	false	"Search text"
//	@Param			service_id	query		int		false	"Only APIs linked to this service"
//	@Param			project_id	query		int		false	"Only APIs linked to this project, directly or through its services"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.APICatalog]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog [get]
func (h *apiCatalogHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	f := models.APICatalogFilter{Query: r.URL.Query().Get("q")}
	f.ServiceID, f.ProjectID = linkFilterParams(r)
	list, err := store.NewAPICatalogRepo(h.db.SQL).List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "list api catalog", err)
		return
	}
	if list == nil {
		list = []models.APICatalog{}
	}
	jsonPaged(w, r, list)
}

// handleSearchOperations godoc
//
//	@Summary		Search operations across catalogued APIs
//	@Description	Any role. Searches the operation index of visible APIs.
//	@Tags			atlas
//	@Produce		json
//	@Param			q			query		string	false	"Search text"
//	@Param			service_id	query		int		false	"Only APIs linked to this service"
//	@Param			project_id	query		int		false	"Only APIs linked to this project, directly or through its services"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.OperationSearchResult]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/search [get]
func (h *apiCatalogHandlers) handleSearchOperations(w http.ResponseWriter, r *http.Request) {
	serviceID, projectID := linkFilterParams(r)
	hits, err := store.NewAPICatalogRepo(h.db.SQL).SearchOperations(r.Context(), r.URL.Query().Get("q"), serviceID, projectID)
	if err != nil {
		jsonServerError(w, r, "search operations", err)
		return
	}
	if hits == nil {
		hits = []models.OperationSearchResult{}
	}
	jsonPaged(w, r, hits)
}

// --- get --------------------------------------------------------------------

// handleGet godoc
//
//	@Summary		Get a catalogued API
//	@Description	Any role; invisible APIs answer 404. Includes its operations, links, responsáveis and entidade grants.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{object}	models.APICatalog
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id} [get]
func (h *apiCatalogHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	a, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get api catalog", err)
		return
	}
	if a == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	h.attachDetail(r.Context(), a)
	jsonOK(w, a)
}

// attachDetail loads the row's entidade grants and responsáveis (best effort:
// a failure leaves them empty).
func (h *apiCatalogHandlers) attachDetail(ctx context.Context, a *models.APICatalog) {
	if g, err := store.NewAssetEntidadeRepo(h.db.SQL).Get(ctx, store.AssetAPICatalog, a.ID); err == nil {
		a.Entidades = &g
	}
	a.Responsaveis = []models.Responsavel{}
	if rs, err := store.NewResponsavelRepo(h.db.SQL).List(ctx, string(store.AssetAPICatalog), a.ID); err == nil && rs != nil {
		a.Responsaveis = rs
	}
}

// handleGetSpec returns the canonical spec JSON verbatim for the renderer.
//
//	@Summary		Get an API's spec
//	@Description	Any role; invisible APIs answer 404. The canonical OpenAPI/Swagger JSON document, verbatim.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int						true	"API catalog ID"
//	@Success		200	{object}	map[string]interface{}	"OpenAPI/Swagger document"
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/spec [get]
func (h *apiCatalogHandlers) handleGetSpec(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	spec, err := store.NewAPICatalogRepo(h.db.SQL).GetSpec(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get api spec", err)
		return
	}
	if spec == "" {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, spec)
}

type filterSpecRequest struct {
	Mode   string   `json:"mode"`
	OpKeys []string `json:"op_keys,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// handleFilterSpec returns a spec reduced to the selected operations/tags —
// a preview of what a partial share bundle (Phase D) would expose.
//
//	@Summary		Preview a filtered spec
//	@Description	Any role; invisible APIs answer 404. The spec reduced to the selected operations or tags — what a partial share bundle would expose.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"API catalog ID"
//	@Param			body	body		filterSpecRequest		true	"Selector: mode plus op_keys or tags"
//	@Success		200		{object}	map[string]interface{}	"Filtered OpenAPI/Swagger document"
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/spec/filter [post]
func (h *apiCatalogHandlers) handleFilterSpec(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	var req filterSpecRequest
	if !decodeBody(w, r, &req) {
		return
	}
	spec, err := store.NewAPICatalogRepo(h.db.SQL).GetSpec(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get api spec", err)
		return
	}
	if spec == "" {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	filtered, err := apicatalog.Filter([]byte(spec), apicatalog.Selector{Mode: req.Mode, OpKeys: req.OpKeys, Tags: req.Tags})
	if err != nil {
		jsonServerError(w, r, "filter spec", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(filtered)
}

// --- import -----------------------------------------------------------------

// handleImportUpload godoc
//
//	@Summary		Import an API from an uploaded spec
//	@Description	Editor+. Multipart upload of an OpenAPI/Swagger file; name falls back to the spec's title. Linked services/projects must be visible (404 otherwise).
//	@Tags			atlas
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			spec						formData	file	true	"OpenAPI/Swagger spec file (JSON or YAML)"
//	@Param			name						formData	string	false	"Name (defaults to the spec title)"
//	@Param			description					formData	string	false	"Description"
//	@Param			service_ids					formData	string	false	"Comma-separated linked service IDs"
//	@Param			project_ids					formData	string	false	"Comma-separated linked project IDs"
//	@Param			base_url					formData	string	false	"Base URL of the running API"
//	@Param			docs_url					formData	string	false	"Human docs URL"
//	@Param			creator_entidade_id			formData	int		false	"Creator entidade ID"
//	@Param			responsible_entidade_ids	formData	string	false	"Comma-separated responsible entidade IDs"
//	@Param			is_global					formData	string	false	"true or 1 to make it globally visible"
//	@Success		201							{object}	models.APICatalog
//	@Failure		400							{object}	httpx.ErrorResponse
//	@Failure		401							{object}	httpx.ErrorResponse
//	@Failure		403							{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/import/upload [post]
func (h *apiCatalogHandlers) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	owner := actor.UserID
	raw, ok := readSpecUpload(w, r)
	if !ok {
		return
	}

	meta, err := h.formMeta(r, models.APICatalogSourceUpload, "")
	if err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	h.createFromSpec(w, r, raw, meta, owner)
}

type importURLRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ServiceIDs  []int64 `json:"service_ids"`
	ProjectIDs  []int64 `json:"project_ids"`
	SourceURL   string  `json:"source_url"`
	BaseURL     string  `json:"base_url"`
	DocsURL     string  `json:"docs_url"`
	models.AssetGrantsInput
}

// handleImportURL godoc
//
//	@Summary		Import an API from a spec URL
//	@Description	Editor+. Fetches source_url (private addresses are allowed unless ATLAS_BLOCK_PRIVATE_SPEC_FETCH is set); a fetch or parse failure answers 400. Linked services/projects must be visible (404 otherwise).
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			body	body		importURLRequest	true	"Spec URL, metadata and entidade grants"
//	@Success		201		{object}	models.APICatalog
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/import/url [post]
func (h *apiCatalogHandlers) handleImportURL(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	owner := actor.UserID
	var req importURLRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"source_url": req.SourceURL}) {
		return
	}
	raw, err := apicatalog.FetchSpec(r.Context(), req.SourceURL, h.allowPrivateFetch)
	if err != nil {
		// Fetch / SSRF failures are user-facing input problems, not 500s.
		jsonBadRequest(w, r, "could not fetch spec: "+err.Error(), err)
		return
	}
	meta := catalogMeta{
		Name:        req.Name,
		Description: req.Description,
		ServiceIDs:  req.ServiceIDs,
		ProjectIDs:  req.ProjectIDs,
		SourceType:  models.APICatalogSourceURL,
		SourceURL:   req.SourceURL,
		BaseURL:     strings.TrimSpace(req.BaseURL),
		DocsURL:     strings.TrimSpace(req.DocsURL),
		Grants:      req.AssetGrantsInput,
	}
	h.createFromSpec(w, r, raw, meta, owner)
}

// --- update / refetch / delete ----------------------------------------------

// updateCatalogRequest edits an API's metadata. service_ids, project_ids and
// responsaveis are replaced when sent (send [] to clear) and kept when omitted.
type updateCatalogRequest struct {
	Name         string                     `json:"name"`
	Description  string                     `json:"description"`
	BaseURL      string                     `json:"base_url"`
	DocsURL      string                     `json:"docs_url"`
	ServiceIDs   *[]int64                   `json:"service_ids"`
	ProjectIDs   *[]int64                   `json:"project_ids"`
	Responsaveis *[]models.ResponsavelInput `json:"responsaveis"`
	models.AssetGrantsInput
}

// handleUpdate godoc
//
//	@Summary		Update a catalogued API's metadata
//	@Description	Editor+; invisible APIs answer 404. Grants change only when a grant field is sent.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"API catalog ID"
//	@Param			body	body		updateCatalogRequest	true	"Metadata and optional entidade grants"
//	@Success		200		{object}	models.APICatalog
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id} [put]
func (h *apiCatalogHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	var req updateCatalogRequest
	if !decodeBody(w, r, &req) {
		return
	}
	repo := store.NewAPICatalogRepo(h.db.SQL)
	if existing, err := repo.Get(r.Context(), id); err != nil {
		jsonServerError(w, r, "get api catalog", err)
		return
	} else if existing == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	var serviceIDs, projectIDs []int64
	if req.ServiceIDs != nil {
		serviceIDs = append([]int64{}, *req.ServiceIDs...)
	}
	if req.ProjectIDs != nil {
		projectIDs = append([]int64{}, *req.ProjectIDs...)
	}
	if !h.linksVisible(w, r, serviceIDs, projectIDs) {
		return
	}
	if err := repo.UpdateMeta(r.Context(), id, req.Name, req.Description, strings.TrimSpace(req.BaseURL), strings.TrimSpace(req.DocsURL)); err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	if err := repo.SetLinks(r.Context(), id, serviceIDs, projectIDs); err != nil {
		jsonServerError(w, r, "failed to set api links", err)
		return
	}
	if req.Responsaveis != nil {
		if err := store.NewResponsavelRepo(h.db.SQL).Sync(r.Context(), string(store.AssetAPICatalog), id, *req.Responsaveis); err != nil {
			jsonBadRequest(w, r, err.Error(), err)
			return
		}
	}
	if req.AssetGrantsInput.Present() {
		grants := store.NewAssetEntidadeRepo(h.db.SQL)
		existing, _ := grants.Get(r.Context(), store.AssetAPICatalog, id)
		g, ok := resolveGrants(w, r, req.AssetGrantsInput, &existing)
		if !ok {
			return
		}
		if err := grants.Replace(r.Context(), h.db.SQL, store.AssetAPICatalog, id, g); err != nil {
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
	}
	a, err := repo.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "reload api catalog", err)
		return
	}
	if a == nil { // grants just moved it out of the caller's sight
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	h.attachDetail(r.Context(), a)
	jsonOK(w, a)
}

// handleRefetch re-downloads a URL-sourced spec and replaces the stored spec
// + operation index.
//
//	@Summary		Re-fetch a URL-imported spec
//	@Description	Editor+; invisible APIs answer 404. Replaces the stored spec and operation index; 400 when the API was not imported from a URL or the fetch/parse fails.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{object}	models.APICatalog
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/refetch [post]
func (h *apiCatalogHandlers) handleRefetch(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	a, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get api catalog", err)
		return
	}
	if a == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	if a.SourceType != models.APICatalogSourceURL || a.SourceURL == "" {
		jsonError(w, http.StatusBadRequest, "api was not imported from a URL; nothing to refetch")
		return
	}
	raw, err := apicatalog.FetchSpec(r.Context(), a.SourceURL, h.allowPrivateFetch)
	if err != nil {
		jsonBadRequest(w, r, "could not fetch spec: "+err.Error(), err)
		return
	}
	ps, err := apicatalog.Parse(raw)
	if err != nil {
		jsonBadRequest(w, r, "could not parse spec: "+err.Error(), err)
		return
	}
	h.storeSpec(w, r, id, ps)
}

// storeSpec replaces an API's spec + operation index and answers the reloaded
// API (404 when it vanished or left the caller's sight meanwhile).
func (h *apiCatalogHandlers) storeSpec(w http.ResponseWriter, r *http.Request, id int64, ps *apicatalog.ParsedSpec) {
	repo := store.NewAPICatalogRepo(h.db.SQL)
	err := repo.UpdateSpec(r.Context(), id, string(ps.SpecJSON), ps.SpecHash, ps.SpecVersion,
		ps.Title, ps.VersionLabel, ps.ExternalURL, toModelOps(ps.Operations))
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	if err != nil {
		jsonServerError(w, r, "update api spec", err)
		return
	}
	reloaded, err := repo.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "reload api catalog", err)
		return
	}
	if reloaded == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	h.attachDetail(r.Context(), reloaded)
	jsonOK(w, reloaded)
}

// handleReplaceSpec godoc
//
//	@Summary		Replace an API's spec with an uploaded file
//	@Description	Editor+. Re-parses the file and replaces the stored spec and operation index; metadata, links and keys are kept. Works for APIs imported either way (a URL import keeps its source URL for later refetches).
//	@Tags			atlas
//	@Accept			mpfd
//	@Produce		json
//	@Param			id		path		int		true	"API catalog ID"
//	@Param			spec	formData	file	true	"OpenAPI / Swagger document (JSON or YAML)"
//	@Success		200		{object}	models.APICatalog
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/spec [post]
func (h *apiCatalogHandlers) handleReplaceSpec(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	raw, ok := readSpecUpload(w, r)
	if !ok {
		return
	}
	ps, err := apicatalog.Parse(raw)
	if err != nil {
		jsonBadRequest(w, r, "could not parse spec: "+err.Error(), err)
		return
	}
	h.storeSpec(w, r, id, ps)
}

// readSpecUpload reads the multipart "spec" file (bounded by MaxSpecBytes).
func readSpecUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, apicatalog.MaxSpecBytes+1<<20)
	if err := r.ParseMultipartForm(apicatalog.MaxSpecBytes + 1<<20); err != nil {
		jsonBadRequest(w, r, "spec file too large or malformed form", err)
		return nil, false
	}
	file, _, err := r.FormFile("spec")
	if err != nil {
		jsonBadRequest(w, r, "missing spec file", err)
		return nil, false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, apicatalog.MaxSpecBytes+1))
	if err != nil {
		jsonServerError(w, r, "read spec file", err)
		return nil, false
	}
	return raw, true
}

// handleDelete godoc
//
//	@Summary		Delete a catalogued API
//	@Description	Admin. Soft delete.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path	int	true	"API catalog ID"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id} [delete]
func (h *apiCatalogHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	if a, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), id); err != nil {
		jsonServerError(w, r, "get api catalog", err)
		return
	} else if a == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	// Soft-deletes the API and its key secrets in one transaction; restore
	// brings both back.
	actor, _ := actorFrom(r)
	if err := store.DeleteParent(r.Context(), h.db.SQL, actor, models.SecretScopeAPICatalog, id); err != nil {
		jsonServerError(w, r, "delete api catalog", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListTrash godoc
//
//	@Summary		List trashed APIs
//	@Description	Any role. Soft-deleted APIs visible to the caller, newest first.
//	@Tags			atlas
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.APICatalog]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/trash [get]
func (h *apiCatalogHandlers) handleListTrash(w http.ResponseWriter, r *http.Request) {
	items, err := store.NewAPICatalogRepo(h.db.SQL).ListTrash(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list api trash", err)
		return
	}
	jsonPaged(w, r, items)
}

// handleRestore godoc
//
//	@Summary		Restore an API from the trash
//	@Description	Admin. The API comes back with its operations, links and key secrets.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/restore [post]
func (h *apiCatalogHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	found, err := store.NewAPICatalogRepo(h.db.SQL).Restore(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to restore api", err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "api not found in trash")
		return
	}
	actor, _ := actorFrom(r)
	if err := store.RestoreParent(r.Context(), h.db.SQL, actor, models.SecretScopeAPICatalog, id); err != nil {
		jsonServerError(w, r, "failed to restore api keys", err)
		return
	}
	jsonOK(w, StatusResponse{Status: "restored"})
}

// --- helpers ----------------------------------------------------------------

type catalogMeta struct {
	Name        string
	Description string
	ServiceIDs  []int64
	ProjectIDs  []int64
	SourceType  string
	SourceURL   string
	BaseURL     string
	DocsURL     string
	Grants      models.AssetGrantsInput
}

// formMeta extracts catalog metadata from a multipart form (upload path).
// Grant fields: creator_entidade_id (int), responsible_entidade_ids
// (comma-separated ints), is_global ("true"/"1") — each only set when present.
func (h *apiCatalogHandlers) formMeta(r *http.Request, sourceType, sourceURL string) (catalogMeta, error) {
	m := catalogMeta{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		SourceType:  sourceType,
		SourceURL:   sourceURL,
		BaseURL:     strings.TrimSpace(r.FormValue("base_url")),
		DocsURL:     strings.TrimSpace(r.FormValue("docs_url")),
	}
	var err error
	if m.ServiceIDs, err = formIDList(r, "service_ids"); err != nil {
		return m, err
	}
	if m.ProjectIDs, err = formIDList(r, "project_ids"); err != nil {
		return m, err
	}
	if v := strings.TrimSpace(r.FormValue("creator_entidade_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return m, fmt.Errorf("invalid creator_entidade_id")
		}
		m.Grants.CreatorEntidadeID = &id
	}
	if v := strings.TrimSpace(r.FormValue("responsible_entidade_ids")); v != "" {
		ids := []int64{}
		for _, s := range strings.Split(v, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return m, fmt.Errorf("invalid responsible_entidade_ids")
			}
			ids = append(ids, id)
		}
		m.Grants.ResponsibleEntidadeIDs = &ids
	}
	if v := strings.TrimSpace(r.FormValue("is_global")); v != "" {
		g := v == "true" || v == "1"
		m.Grants.IsGlobal = &g
	}
	return m, nil
}

// createFromSpec parses raw, checks the linked services/projects are
// visible, and persists the catalog + operation index + links.
func (h *apiCatalogHandlers) createFromSpec(w http.ResponseWriter, r *http.Request, raw []byte, meta catalogMeta, owner int64) {
	if !h.linksVisible(w, r, meta.ServiceIDs, meta.ProjectIDs) {
		return
	}
	ps, err := apicatalog.Parse(raw)
	if err != nil {
		jsonBadRequest(w, r, "could not parse spec: "+err.Error(), err)
		return
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = ps.Title // fall back to the spec's info.title
	}
	if name == "" {
		jsonError(w, http.StatusBadRequest, "name is required (spec has no title to fall back to)")
		return
	}
	g, ok := resolveGrants(w, r, meta.Grants, nil)
	if !ok {
		return
	}
	a := &models.APICatalog{
		Name:         name,
		Description:  meta.Description,
		SourceType:   meta.SourceType,
		SourceURL:    meta.SourceURL,
		ExternalURL:  ps.ExternalURL,
		BaseURL:      meta.BaseURL,
		DocsURL:      meta.DocsURL,
		SpecVersion:  ps.SpecVersion,
		SpecJSON:     string(ps.SpecJSON),
		SpecHash:     ps.SpecHash,
		Title:        ps.Title,
		VersionLabel: ps.VersionLabel,
		OwnerUserID:  owner,
		CreatedBy:    owner,
	}
	if err := store.NewAPICatalogRepo(h.db.SQL).Create(r.Context(), a, toModelOps(ps.Operations)); err != nil {
		jsonBadRequest(w, r, "could not save api: "+err.Error(), err)
		return
	}
	// Grants must land before the reload: without them a scoped caller can't
	// see the row it just created.
	if err := store.NewAssetEntidadeRepo(h.db.SQL).Replace(r.Context(), h.db.SQL, store.AssetAPICatalog, a.ID, g); err != nil {
		jsonServerError(w, r, "failed to set entidades", err)
		return
	}
	if err := store.NewAPICatalogRepo(h.db.SQL).SetLinks(r.Context(), a.ID, dedupeOrEmpty(meta.ServiceIDs), dedupeOrEmpty(meta.ProjectIDs)); err != nil {
		jsonServerError(w, r, "failed to set api links", err)
		return
	}
	reloaded, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), a.ID)
	if err != nil {
		jsonServerError(w, r, "reload api catalog", err)
		return
	}
	if reloaded != nil {
		h.attachDetail(r.Context(), reloaded)
	}
	jsonCreated(w, reloaded)
}

// linksVisible rejects links to services or projects the caller can't see,
// missing or trashed ones included: 404, like the asset itself would answer.
func (h *apiCatalogHandlers) linksVisible(w http.ResponseWriter, r *http.Request, serviceIDs, projectIDs []int64) bool {
	for _, id := range serviceIDs {
		if s, err := store.NewServiceRepo(h.db.SQL).Get(r.Context(), id); err != nil || s == nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("service %d not found", id))
			return false
		}
	}
	for _, id := range projectIDs {
		if p, err := store.NewProjectRepo(h.db.SQL).Get(r.Context(), id); err != nil || p == nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("project %d not found", id))
			return false
		}
	}
	return true
}

// dedupeOrEmpty turns a nil list into [] so SetLinks writes (clears) it.
func dedupeOrEmpty(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

// linkFilterParams reads the optional service_id / project_id list filters.
func linkFilterParams(r *http.Request) (serviceID, projectID int64) {
	serviceID, _ = strconv.ParseInt(r.URL.Query().Get("service_id"), 10, 64)
	projectID, _ = strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	return serviceID, projectID
}

// formIDList parses a comma-separated id list form field ("" → nil).
func formIDList(r *http.Request, field string) ([]int64, error) {
	v := strings.TrimSpace(r.FormValue(field))
	if v == "" {
		return nil, nil
	}
	ids := []int64{}
	for _, part := range strings.Split(v, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s", field)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func toModelOps(ops []apicatalog.Operation) []models.APIOperation {
	out := make([]models.APIOperation, len(ops))
	for i, o := range ops {
		out[i] = models.APIOperation{
			Method:      o.Method,
			Path:        o.Path,
			OperationID: o.OperationID,
			Summary:     o.Summary,
			Description: o.Description,
			Tags:        o.Tags,
			OpKey:       o.OpKey,
		}
	}
	return out
}
