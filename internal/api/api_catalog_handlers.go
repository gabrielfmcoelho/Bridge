package api

import (
	"context"
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
// requires the editor role; deletion requires admin. The catalog's ACL is
// simple (scope projeto/avulso + owner), so these handlers stay thin: parse →
// model func → render, following the project_handlers.go convention.
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

// register wires the catalog routes. wrap applies auth for the target
// environment: given a required role ("" = any authenticated user, else a
// minimum role), it returns the wrapped handler. Production passes a wrap
// backed by authenticated()/authedRole(); tests pass one that injects a fake
// identity. Centralising routes here keeps prod + tests in lockstep, and
// literal sub-paths precede the catch-all /{id} for clarity.
func (h *apiCatalogHandlers) register(mux *http.ServeMux, wrap func(role string, next http.Handler) http.Handler) {
	mux.Handle("GET /api/api-catalog", wrap("", http.HandlerFunc(h.handleList)))
	mux.Handle("GET /api/api-catalog/search", wrap("", http.HandlerFunc(h.handleSearchOperations)))
	mux.Handle("POST /api/api-catalog/import/upload", wrap("editor", http.HandlerFunc(h.handleImportUpload)))
	mux.Handle("POST /api/api-catalog/import/url", wrap("editor", http.HandlerFunc(h.handleImportURL)))
	mux.Handle("GET /api/api-catalog/{id}", wrap("", http.HandlerFunc(h.handleGet)))
	mux.Handle("GET /api/api-catalog/{id}/spec", wrap("", http.HandlerFunc(h.handleGetSpec)))
	mux.Handle("POST /api/api-catalog/{id}/spec/filter", wrap("", http.HandlerFunc(h.handleFilterSpec)))
	mux.Handle("POST /api/api-catalog/{id}/refetch", wrap("editor", http.HandlerFunc(h.handleRefetch)))
	mux.Handle("PUT /api/api-catalog/{id}", wrap("editor", http.HandlerFunc(h.handleUpdate)))
	mux.Handle("DELETE /api/api-catalog/{id}", wrap("admin", http.HandlerFunc(h.handleDelete)))
}

// --- list / search ----------------------------------------------------------

func (h *apiCatalogHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	f := models.APICatalogFilter{
		Scope: r.URL.Query().Get("scope"),
		Query: r.URL.Query().Get("q"),
	}
	if v := r.URL.Query().Get("parent_id"); v != "" {
		if pid, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ParentID = &pid
		}
	}
	list, err := store.NewAPICatalogRepo(h.db.SQL).List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "list api catalog", err)
		return
	}
	if list == nil {
		list = []models.APICatalog{}
	}
	jsonOK(w, list)
}

func (h *apiCatalogHandlers) handleSearchOperations(w http.ResponseWriter, r *http.Request) {
	var parentID *int64
	if v := r.URL.Query().Get("parent_id"); v != "" {
		if pid, err := strconv.ParseInt(v, 10, 64); err == nil {
			parentID = &pid
		}
	}
	hits, err := store.NewAPICatalogRepo(h.db.SQL).SearchOperations(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("scope"), parentID)
	if err != nil {
		jsonServerError(w, r, "search operations", err)
		return
	}
	if hits == nil {
		hits = []models.OperationSearchResult{}
	}
	jsonOK(w, hits)
}

// --- get --------------------------------------------------------------------

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
	jsonOK(w, a)
}

// handleGetSpec returns the canonical spec JSON verbatim for the renderer.
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
func (h *apiCatalogHandlers) handleFilterSpec(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	var req filterSpecRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
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

func (h *apiCatalogHandlers) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	owner := actor.UserID
	r.Body = http.MaxBytesReader(w, r.Body, apicatalog.MaxSpecBytes+1<<20)
	if err := r.ParseMultipartForm(apicatalog.MaxSpecBytes + 1<<20); err != nil {
		jsonBadRequest(w, r, "spec file too large or malformed form", err)
		return
	}
	file, _, err := r.FormFile("spec")
	if err != nil {
		jsonBadRequest(w, r, "missing spec file", err)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, apicatalog.MaxSpecBytes+1))
	if err != nil {
		jsonServerError(w, r, "read spec file", err)
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
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	ParentID    *int64 `json:"parent_id"`
	SourceURL   string `json:"source_url"`
	BaseURL     string `json:"base_url"`
	DocsURL     string `json:"docs_url"`
}

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
		Scope:       defaultScope(req.Scope),
		ParentID:    req.ParentID,
		SourceType:  models.APICatalogSourceURL,
		SourceURL:   req.SourceURL,
		BaseURL:     strings.TrimSpace(req.BaseURL),
		DocsURL:     strings.TrimSpace(req.DocsURL),
	}
	h.createFromSpec(w, r, raw, meta, owner)
}

// --- update / refetch / delete ----------------------------------------------

type updateCatalogRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	BaseURL     string `json:"base_url"`
	DocsURL     string `json:"docs_url"`
}

func (h *apiCatalogHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	var req updateCatalogRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if err := store.NewAPICatalogRepo(h.db.SQL).UpdateMeta(r.Context(), id, req.Name, req.Description, strings.TrimSpace(req.BaseURL), strings.TrimSpace(req.DocsURL)); err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	a, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "reload api catalog", err)
		return
	}
	if a == nil {
		jsonError(w, http.StatusNotFound, "api not found")
		return
	}
	jsonOK(w, a)
}

// handleRefetch re-downloads a URL-sourced spec and replaces the stored spec
// + operation index.
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
	if err := store.NewAPICatalogRepo(h.db.SQL).UpdateSpec(r.Context(), id, string(ps.SpecJSON), ps.SpecHash, ps.SpecVersion,
		ps.Title, ps.VersionLabel, ps.ExternalURL, toModelOps(ps.Operations)); err != nil {
		jsonServerError(w, r, "update api spec", err)
		return
	}
	reloaded, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "reload api catalog", err)
		return
	}
	jsonOK(w, reloaded)
}

func (h *apiCatalogHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	if err := store.NewAPICatalogRepo(h.db.SQL).SoftDelete(r.Context(), id); err != nil {
		jsonServerError(w, r, "delete api catalog", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ----------------------------------------------------------------

type catalogMeta struct {
	Name        string
	Description string
	Scope       string
	ParentID    *int64
	SourceType  string
	SourceURL   string
	BaseURL     string
	DocsURL     string
}

// formMeta extracts catalog metadata from a multipart form (upload path).
func (h *apiCatalogHandlers) formMeta(r *http.Request, sourceType, sourceURL string) (catalogMeta, error) {
	m := catalogMeta{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Scope:       defaultScope(r.FormValue("scope")),
		SourceType:  sourceType,
		SourceURL:   sourceURL,
		BaseURL:     strings.TrimSpace(r.FormValue("base_url")),
		DocsURL:     strings.TrimSpace(r.FormValue("docs_url")),
	}
	if v := strings.TrimSpace(r.FormValue("parent_id")); v != "" {
		pid, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return m, fmt.Errorf("invalid parent_id")
		}
		m.ParentID = &pid
	}
	return m, nil
}

// createFromSpec parses raw, validates the parent (for projeto scope), and
// persists the catalog + operation index.
func (h *apiCatalogHandlers) createFromSpec(w http.ResponseWriter, r *http.Request, raw []byte, meta catalogMeta, owner int64) {
	if err := h.validateParent(r.Context(), meta.Scope, meta.ParentID); err != nil {
		jsonBadRequest(w, r, err.Error(), err)
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
	a := &models.APICatalog{
		Scope:        meta.Scope,
		ParentID:     meta.ParentID,
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
	reloaded, err := store.NewAPICatalogRepo(h.db.SQL).Get(r.Context(), a.ID)
	if err != nil {
		jsonServerError(w, r, "reload api catalog", err)
		return
	}
	jsonCreated(w, reloaded)
}

func (h *apiCatalogHandlers) validateParent(ctx context.Context, scope string, parentID *int64) error {
	if scope != models.APICatalogScopeProjeto {
		return nil
	}
	if parentID == nil {
		return fmt.Errorf("parent_id is required for projeto scope")
	}
	p, err := store.NewProjectRepo(h.db.SQL).Get(ctx, *parentID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %d not found", *parentID)
	}
	return nil
}

func defaultScope(s string) string {
	if s == "" {
		return models.APICatalogScopeAvulso
	}
	return s
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
