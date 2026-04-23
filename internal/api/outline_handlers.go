package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	outlineclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/outline"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type outlineHandlers struct {
	db *database.DB
}

// wikiEnvelope is the shape every "list documents" response carries.
// The UI differentiates its empty states from the boolean + nil fields.
type wikiEnvelope struct {
	Enabled     bool                    `json:"enabled"`
	Configured  bool                    `json:"configured"`
	Collection  *outlineclient.Collection `json:"collection"`
	CollectionBrowseURL string          `json:"collection_browse_url,omitempty"`
	Documents   []docSummary            `json:"documents"`
	Warning     string                  `json:"warning,omitempty"`
}

// docSummary flattens the Outline document into what the UI actually needs —
// enough to render a row without forcing the frontend to mirror Outline's full type.
type docSummary struct {
	ID          string    `json:"id"`
	URLID       string    `json:"url_id"`
	Title       string    `json:"title"`
	Emoji       string    `json:"emoji,omitempty"`
	Excerpt     string    `json:"excerpt"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by,omitempty"`
	BrowseURL   string    `json:"browse_url"`
}

// handleListProjectWiki returns recent docs in the project's linked Outline collection.
// Status surface (via the envelope flags) lets the UI pick the right empty state
// without having to introspect HTTP errors.
func (h *outlineHandlers) handleListProjectWiki(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	project, err := models.GetProject(h.db.SQL, projectID)
	if err != nil {
		jsonServerError(w, r, "project lookup failed", err)
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	env := wikiEnvelope{Documents: []docSummary{}}
	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	env.Enabled = settings.Enabled
	env.Configured = settings.APIToken != "" && settings.BaseURL != ""

	if !env.Enabled || !env.Configured {
		jsonOK(w, env)
		return
	}
	if project.OutlineCollectionID == "" {
		env.Warning = "no_collection_linked"
		jsonOK(w, env)
		return
	}

	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonOK(w, env)
		return
	}

	h.populateWikiEnvelope(r.Context(), &env, client, project.OutlineCollectionID, settings.BaseURL)
	jsonOK(w, env)
}

// commonWikiSection is one slice of the /wiki surface — a single Outline collection
// and its most-recently-updated documents. The frontend renders one card per section.
type commonWikiSection struct {
	CollectionID        string                    `json:"collection_id"`
	Collection          *outlineclient.Collection `json:"collection"`
	CollectionBrowseURL string                    `json:"collection_browse_url,omitempty"`
	Documents           []docSummary              `json:"documents"`
	Warning             string                    `json:"warning,omitempty"`
}

// commonWikiEnvelope is the multi-collection shape returned by /api/wiki/documents.
// Sections appear in the order admins listed them in settings.
type commonWikiEnvelope struct {
	Enabled    bool                `json:"enabled"`
	Configured bool                `json:"configured"`
	Sections   []commonWikiSection `json:"sections"`
	Warning    string              `json:"warning,omitempty"`
}

// handleListCommonWiki targets the site-wide common collections from admin settings.
// Each configured collection becomes its own section. Failed lookups surface a
// per-section warning rather than failing the whole call.
func (h *outlineHandlers) handleListCommonWiki(w http.ResponseWriter, r *http.Request) {
	env := commonWikiEnvelope{Sections: []commonWikiSection{}}
	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	env.Enabled = settings.Enabled
	env.Configured = settings.APIToken != "" && settings.BaseURL != ""
	if !env.Enabled || !env.Configured {
		jsonOK(w, env)
		return
	}
	if len(settings.CommonCollectionIDs) == 0 {
		env.Warning = "no_common_collection"
		jsonOK(w, env)
		return
	}

	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonOK(w, env)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	sections := make([]commonWikiSection, len(settings.CommonCollectionIDs))
	// Bounded concurrency: cap in-flight Outline requests at 5 so large configured
	// lists don't hammer the instance. Order is preserved via the indexed slice.
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for i, id := range settings.CommonCollectionIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			sections[i] = h.buildCommonSection(ctx, client, id, settings.BaseURL)
		}()
	}
	wg.Wait()

	env.Sections = sections
	jsonOK(w, env)
}

// buildCommonSection fetches one collection + its recent docs. Failures degrade
// gracefully: we still return the section, with the error surfaced in Warning.
func (h *outlineHandlers) buildCommonSection(
	ctx context.Context,
	client *outlineclient.Client,
	collectionID, baseURL string,
) commonWikiSection {
	sec := commonWikiSection{CollectionID: collectionID, Documents: []docSummary{}}

	coll, err := client.CollectionInfo(ctx, collectionID)
	if err != nil {
		sec.Warning = "collection_lookup_failed: " + err.Error()
		return sec
	}
	sec.Collection = coll
	sec.CollectionBrowseURL = coll.BrowseURL(baseURL)

	docs, err := client.DocumentsList(ctx, collectionID, 20)
	if err != nil {
		sec.Warning = "documents_lookup_failed: " + err.Error()
		return sec
	}
	sec.Documents = make([]docSummary, 0, len(docs))
	for _, d := range docs {
		sec.Documents = append(sec.Documents, docSummary{
			ID:        d.ID,
			URLID:     d.URLID,
			Title:     d.Title,
			Emoji:     d.Emoji,
			Excerpt:   buildExcerpt(d.Text, 160),
			UpdatedAt: d.UpdatedAt,
			UpdatedBy: userName(d.UpdatedBy),
			BrowseURL: d.BrowseURL(baseURL),
		})
	}
	return sec
}

// populateWikiEnvelope fetches the collection + its recent documents in parallel,
// flattens them into the UI-facing summary, and attaches browse URLs.
// On partial failure (collection fetch succeeded, documents call failed) we still
// return the header info — the frontend shows a warning under the header.
func (h *outlineHandlers) populateWikiEnvelope(
	ctx context.Context,
	env *wikiEnvelope,
	client *outlineclient.Client,
	collectionID, baseURL string,
) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	coll, err := client.CollectionInfo(ctx, collectionID)
	if err != nil {
		env.Warning = "collection_lookup_failed: " + err.Error()
		return
	}
	env.Collection = coll
	env.CollectionBrowseURL = coll.BrowseURL(baseURL)

	docs, err := client.DocumentsList(ctx, collectionID, 20)
	if err != nil {
		env.Warning = "documents_lookup_failed: " + err.Error()
		return
	}
	env.Documents = make([]docSummary, 0, len(docs))
	for _, d := range docs {
		env.Documents = append(env.Documents, docSummary{
			ID:        d.ID,
			URLID:     d.URLID,
			Title:     d.Title,
			Emoji:     d.Emoji,
			Excerpt:   buildExcerpt(d.Text, 160),
			UpdatedAt: d.UpdatedAt,
			UpdatedBy: userName(d.UpdatedBy),
			BrowseURL: d.BrowseURL(baseURL),
		})
	}
}

// handleCreateProjectDocument makes a new doc in the project's collection.
// Body: {title, text?}. The collection id is picked server-side — the client never sees it.
func (h *outlineHandlers) handleCreateProjectDocument(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	project, err := models.GetProject(h.db.SQL, projectID)
	if err != nil {
		jsonServerError(w, r, "project lookup failed", err)
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	if project.OutlineCollectionID == "" {
		jsonError(w, http.StatusBadRequest, "project has no outline_collection_id linked")
		return
	}
	h.createDoc(w, r, project.OutlineCollectionID)
}

// handleCreateCommonDocument makes a new doc in one of the configured common
// collections. Body: {title, text?, collection_id?}. When collection_id is omitted
// we default to the first configured id (preserves the pre-multi-collection UX).
// When present we validate it against the admin-configured set so callers can't
// write into arbitrary collections.
func (h *outlineHandlers) handleCreateCommonDocument(w http.ResponseWriter, r *http.Request) {
	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	if !settings.Enabled || len(settings.CommonCollectionIDs) == 0 {
		jsonError(w, http.StatusBadRequest, "common wiki not configured")
		return
	}

	var req struct {
		Title        string `json:"title"`
		Text         string `json:"text"`
		CollectionID string `json:"collection_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	collectionID := strings.TrimSpace(req.CollectionID)
	if collectionID == "" {
		collectionID = settings.CommonCollectionIDs[0]
	} else {
		allowed := false
		for _, id := range settings.CommonCollectionIDs {
			if id == collectionID {
				allowed = true
				break
			}
		}
		if !allowed {
			jsonError(w, http.StatusBadRequest, "collection_id is not in the configured common collections")
			return
		}
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}

	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonError(w, http.StatusBadRequest, "Outline integration not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	doc, err := client.DocumentCreate(ctx, outlineclient.DocumentCreateInput{
		Title:        req.Title,
		Text:         req.Text,
		CollectionID: collectionID,
		Publish:      true,
	})
	if err != nil {
		jsonError(w, http.StatusBadGateway, "create failed: "+err.Error())
		return
	}

	jsonCreated(w, map[string]any{
		"id":            doc.ID,
		"url_id":        doc.URLID,
		"title":         doc.Title,
		"browse_url":    doc.BrowseURL(settings.BaseURL),
		"collection_id": collectionID,
	})
}

// createDoc is the shared body for the two create-document endpoints.
func (h *outlineHandlers) createDoc(w http.ResponseWriter, r *http.Request, collectionID string) {
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}

	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonError(w, http.StatusBadRequest, "Outline integration not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	doc, err := client.DocumentCreate(ctx, outlineclient.DocumentCreateInput{
		Title:        req.Title,
		Text:         req.Text,
		CollectionID: collectionID,
		Publish:      true,
	})
	if err != nil {
		jsonError(w, http.StatusBadGateway, "create failed: "+err.Error())
		return
	}

	jsonCreated(w, map[string]any{
		"id":         doc.ID,
		"url_id":     doc.URLID,
		"title":      doc.Title,
		"browse_url": doc.BrowseURL(settings.BaseURL),
	})
}

// handleSearchProjectWiki runs Outline's search scoped to this project's collection.
func (h *outlineHandlers) handleSearchProjectWiki(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	project, err := models.GetProject(h.db.SQL, projectID)
	if err != nil {
		jsonServerError(w, r, "project lookup failed", err)
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	h.runSearch(w, r, project.OutlineCollectionID)
}

// handleSearchCommonWiki searches the whole workspace (no collection filter).
func (h *outlineHandlers) handleSearchCommonWiki(w http.ResponseWriter, r *http.Request) {
	h.runSearch(w, r, "")
}

type searchResult struct {
	Context      string    `json:"context"`
	ID           string    `json:"id"`
	URLID        string    `json:"url_id"`
	Title        string    `json:"title"`
	CollectionID string    `json:"collection_id"`
	UpdatedAt    time.Time `json:"updated_at"`
	BrowseURL    string    `json:"browse_url"`
}

func (h *outlineHandlers) runSearch(w http.ResponseWriter, r *http.Request, collectionID string) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		jsonOK(w, map[string]any{"results": []searchResult{}})
		return
	}

	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonOK(w, map[string]any{"results": []searchResult{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hits, err := client.DocumentsSearch(ctx, query, collectionID, 25)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}

	out := make([]searchResult, 0, len(hits))
	for _, hit := range hits {
		out = append(out, searchResult{
			Context:      hit.Context,
			ID:           hit.Document.ID,
			URLID:        hit.Document.URLID,
			Title:        hit.Document.Title,
			CollectionID: hit.Document.CollectionID,
			UpdatedAt:    hit.Document.UpdatedAt,
			BrowseURL:    hit.Document.BrowseURL(settings.BaseURL),
		})
	}
	jsonOK(w, map[string]any{"results": out})
}

// buildExcerpt strips minimal markdown (leading #s for headings, trailing whitespace)
// and truncates to `n` runes with an ellipsis. Keeps list rows compact without pulling
// in a full markdown parser.
func buildExcerpt(text string, n int) string {
	t := strings.TrimSpace(text)
	// Drop leading markdown heading markers.
	for strings.HasPrefix(t, "#") || strings.HasPrefix(t, " ") {
		t = strings.TrimLeft(t, "# ")
	}
	t = strings.ReplaceAll(t, "\n", " ")
	if len(t) <= n {
		return t
	}
	runes := []rune(t)
	if len(runes) <= n {
		return t
	}
	return string(runes[:n]) + "…"
}

func userName(u *outlineclient.User) string {
	if u == nil {
		return ""
	}
	return u.Name
}

// ────────────────────────────────────────────────────────────────────────────
// Outline-like navigation surface (tree + single-doc viewer + collection picker)
// ────────────────────────────────────────────────────────────────────────────

type workspaceCollectionSummary struct {
	ID          string `json:"id"`
	URLID       string `json:"url_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// handleListWorkspaceCollections powers the admin picker in Settings.
// Returns every collection the service token can see; the frontend chooses
// which ones to expose as "common".
func (h *outlineHandlers) handleListWorkspaceCollections(w http.ResponseWriter, r *http.Request) {
	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	if !settings.Enabled || settings.APIToken == "" || settings.BaseURL == "" {
		jsonError(w, http.StatusBadRequest, "outline integration not configured")
		return
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonError(w, http.StatusBadRequest, "outline integration not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Page through collections.list until a short page signals end-of-list. Cap
	// iterations so a misbehaving server can't hang the handler.
	const pageSize = 100
	const maxPages = 20
	out := make([]workspaceCollectionSummary, 0, pageSize)
	for page := 0; page < maxPages; page++ {
		chunk, err := client.CollectionsList(ctx, pageSize, page*pageSize)
		if err != nil {
			jsonError(w, http.StatusBadGateway, "outline api error: "+err.Error())
			return
		}
		for _, c := range chunk {
			out = append(out, workspaceCollectionSummary{
				ID:          c.ID,
				URLID:       c.URLID,
				Name:        c.Name,
				Description: c.Description,
				Color:       c.Color,
				Icon:        c.Icon,
			})
		}
		if len(chunk) < pageSize {
			break
		}
	}
	jsonOK(w, map[string]any{"collections": out})
}

// commonWikiTreeSection mirrors commonWikiSection but carries a nested tree of
// DocumentNode instead of a flat recent-docs list.
type commonWikiTreeSection struct {
	CollectionID        string                       `json:"collection_id"`
	Collection          *outlineclient.Collection    `json:"collection"`
	CollectionBrowseURL string                       `json:"collection_browse_url,omitempty"`
	Nodes               []outlineclient.DocumentNode `json:"nodes"`
	Warning             string                       `json:"warning,omitempty"`
}

type commonWikiTreeEnvelope struct {
	Enabled    bool                    `json:"enabled"`
	Configured bool                    `json:"configured"`
	BaseURL    string                  `json:"base_url,omitempty"`
	Sections   []commonWikiTreeSection `json:"sections"`
	Warning    string                  `json:"warning,omitempty"`
}

// handleCommonWikiTree returns the full nested nav for every configured common
// collection. Used by the /wiki page's left sidebar.
func (h *outlineHandlers) handleCommonWikiTree(w http.ResponseWriter, r *http.Request) {
	env := commonWikiTreeEnvelope{Sections: []commonWikiTreeSection{}}
	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	env.Enabled = settings.Enabled
	env.Configured = settings.APIToken != "" && settings.BaseURL != ""
	env.BaseURL = settings.BaseURL
	if !env.Enabled || !env.Configured {
		jsonOK(w, env)
		return
	}
	if len(settings.CommonCollectionIDs) == 0 {
		env.Warning = "no_common_collection"
		jsonOK(w, env)
		return
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonOK(w, env)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	sections := make([]commonWikiTreeSection, len(settings.CommonCollectionIDs))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for i, id := range settings.CommonCollectionIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			sections[i] = h.buildCommonTreeSection(ctx, client, id, settings.BaseURL)
		}()
	}
	wg.Wait()

	env.Sections = sections
	jsonOK(w, env)
}

func (h *outlineHandlers) buildCommonTreeSection(
	ctx context.Context,
	client *outlineclient.Client,
	collectionID, baseURL string,
) commonWikiTreeSection {
	sec := commonWikiTreeSection{CollectionID: collectionID, Nodes: []outlineclient.DocumentNode{}}
	coll, err := client.CollectionInfo(ctx, collectionID)
	if err != nil {
		sec.Warning = "collection_lookup_failed: " + err.Error()
		return sec
	}
	sec.Collection = coll
	sec.CollectionBrowseURL = coll.BrowseURL(baseURL)

	nodes, err := client.CollectionDocuments(ctx, collectionID)
	if err != nil {
		sec.Warning = "tree_lookup_failed: " + err.Error()
		return sec
	}
	sec.Nodes = nodes
	return sec
}

type wikiDocumentResponse struct {
	ID           string    `json:"id"`
	URLID        string    `json:"url_id"`
	Title        string    `json:"title"`
	Emoji        string    `json:"emoji,omitempty"`
	Text         string    `json:"text"`
	CollectionID string    `json:"collection_id"`
	UpdatedAt    time.Time `json:"updated_at"`
	UpdatedBy    string    `json:"updated_by,omitempty"`
	BrowseURL    string    `json:"browse_url"`
}

// handleGetWikiDocument returns one document with its full markdown body.
// Enforces that the doc lives inside an admin-configured common collection —
// otherwise any authenticated user could fetch arbitrary docs by guessing ids
// (the service token's visibility is typically wider than what admins wired).
func (h *outlineHandlers) handleGetWikiDocument(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		jsonError(w, http.StatusBadRequest, "document id is required")
		return
	}
	settings, err := outlineclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load outline settings", err)
		return
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		jsonError(w, http.StatusBadRequest, "outline integration not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	doc, err := client.DocumentInfo(ctx, id)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "outline api error: "+err.Error())
		return
	}

	allowed := false
	for _, cid := range settings.CommonCollectionIDs {
		if cid == doc.CollectionID {
			allowed = true
			break
		}
	}
	if !allowed {
		jsonError(w, http.StatusForbidden, "document is not inside a configured common collection")
		return
	}

	jsonOK(w, wikiDocumentResponse{
		ID:           doc.ID,
		URLID:        doc.URLID,
		Title:        doc.Title,
		Emoji:        doc.Emoji,
		Text:         doc.Text,
		CollectionID: doc.CollectionID,
		UpdatedAt:    doc.UpdatedAt,
		UpdatedBy:    userName(doc.UpdatedBy),
		BrowseURL:    doc.BrowseURL(settings.BaseURL),
	})
}
