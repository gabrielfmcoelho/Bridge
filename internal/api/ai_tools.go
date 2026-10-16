package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	glpiclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/glpi"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	outlineclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/outline"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// The assistant's tools. Read tools search the inventory through the same
// repos the list pages use, on the request context, so the caller's entidade
// scope applies (a context without scope is unscoped: never run these on
// context.Background()). Proposal tools never write: they validate a draft
// and hand it to the UI, where the user confirms and the browser calls the
// regular create endpoint with the user's own permissions.

// maxToolItems caps what one tool call returns to the model.
const maxToolItems = 10

// aiRef is a Bridge page a tool result points to; the model cites it as a
// markdown link to Href, and the UI lists the cited ones as sources.
type aiRef struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
	Href  string `json:"href"`
}

// aiProposal is an action the model drafted for the user to confirm.
type aiProposal struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // "issue" | "chamado"
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority,omitempty"`
	Parent      *aiRef `json:"parent,omitempty"`
	// ParentType/ParentID: the issue's entity_type/entity_id for POST /api/issues.
	ParentType string        `json:"parent_type,omitempty"`
	ParentID   int64         `json:"parent_id,omitempty"`
	Profile    *aiGlpiTarget `json:"profile,omitempty"`
	HostSlug   string        `json:"host_slug,omitempty"`
}

// aiGlpiTarget names the GLPI profile a chamado would be opened with (never its token).
type aiGlpiTarget struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// toolOutput is one tool's answer: items for the model, refs for citations,
// or a proposal for the user.
type toolOutput struct {
	Total    int
	Items    []map[string]any
	Refs     []aiRef
	Proposal *aiProposal
}

type aiToolEnv struct {
	db   *database.DB
	user *models.User
}

type aiTool struct {
	def     llm.Tool
	minRole string // "" = anyone with ai.use; proposals follow their create route (editor)
	run     func(ctx context.Context, env aiToolEnv, args json.RawMessage) (toolOutput, error)
}

// errToolNotFound tells the model the thing it named isn't there (or isn't visible).
var errToolNotFound = errors.New("not found or not visible to this user")

func tool(name, desc, params, minRole string, run func(context.Context, aiToolEnv, json.RawMessage) (toolOutput, error)) aiTool {
	return aiTool{
		def:     llm.Tool{Type: "function", Function: llm.ToolFunction{Name: name, Description: desc, Parameters: json.RawMessage(params)}},
		minRole: minRole,
		run:     run,
	}
}

const queryParam = `{"type":"object","properties":{"query":{"type":"string","description":"Text to search for"}},"required":["query"]}`

var aiTools = []aiTool{
	tool("search_hosts",
		"Search servers/VMs (hosts) by name, hostname or description. Returns status, hosting and active alerts. Use alert_level to list hosts with critical or warning alerts.",
		`{"type":"object","properties":{"query":{"type":"string"},"situacao":{"type":"string","description":"Status value, e.g. ativo"},"alert_level":{"type":"string","enum":["critical","warning","info"]}}}`,
		"", searchHosts),
	tool("search_services", "Search services (applications, containers) by name, description or stack.", queryParam, "", searchServices),
	tool("search_dns", "Search DNS records by domain. Returns HTTPS and certificate expiry.", queryParam, "", searchDNS),
	tool("search_projects", "Search projects by name, description or owning sector.", queryParam, "", searchProjects),
	tool("search_apis", "Search the data Atlas API catalog: APIs and their endpoints.", queryParam, "", searchAPIs),
	tool("search_issues",
		"Search issues (work items). Set assigned_to_me to list the current user's issues.",
		`{"type":"object","properties":{"query":{"type":"string"},"status":{"type":"string","enum":["backlog","todo","in_progress","review","done"]},"assigned_to_me":{"type":"boolean"}}}`,
		"", searchIssues),
	tool("search_wiki", "Full-text search in the team wiki (Outline).", queryParam, "", searchWiki),
	tool("propose_issue",
		"Draft an issue for the user to review and confirm. Does NOT create it. The parent must be a host, dns, service or project id found with a search tool.",
		`{"type":"object","properties":{"title":{"type":"string"},"description":{"type":"string","description":"Markdown"},"priority":{"type":"string","enum":["low","medium","high","critical"]},"parent_type":{"type":"string","enum":["host","dns","service","project"]},"parent_id":{"type":"integer"}},"required":["title","parent_type","parent_id"]}`,
		"editor", proposeIssue),
	tool("propose_chamado",
		"Draft a GLPI support ticket (chamado) for the user to review and confirm. Does NOT open it. Give project_id (from search_projects) when known; host_slug links it to a host.",
		`{"type":"object","properties":{"title":{"type":"string"},"description":{"type":"string"},"project_id":{"type":"integer"},"host_slug":{"type":"string"}},"required":["title","description"]}`,
		"editor", proposeChamado),
}

var roleLevel = map[string]int{"viewer": 0, "editor": 1, "admin": 2}

// toolsFor lists the tools this user may call: proposals only where the user
// could also make the real create call.
func toolsFor(u *models.User) []aiTool {
	var out []aiTool
	for _, t := range aiTools {
		if t.minRole == "" || (u != nil && roleLevel[u.Role] >= roleLevel[t.minRole]) {
			out = append(out, t)
		}
	}
	return out
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func decodeArgs(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func first[T any](xs []T) []T {
	if len(xs) > maxToolItems {
		return xs[:maxToolItems]
	}
	return xs
}

// ── Read tools ──────────────────────────────────────────────────────────

// searchStopwords are dropped from a query before searching word by word.
var searchStopwords = map[string]bool{
	"a": true, "o": true, "as": true, "os": true, "e": true, "de": true, "da": true, "do": true, "das": true, "dos": true,
	"em": true, "no": true, "na": true, "nos": true, "nas": true, "um": true, "uma": true, "para": true, "por": true,
	"com": true, "sobre": true, "the": true, "of": true, "and": true,
}

// searchTerms splits a query into its significant words (at most 5).
func searchTerms(query string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '"' || r == '?' || r == '!'
	}) {
		if len([]rune(w)) < 2 || searchStopwords[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) == 5 {
			break
		}
	}
	return out
}

// searchAny runs one (scoped) repo search per significant word and ranks the
// items by how many words they matched. The repos match a single substring,
// so "datalake sead" as one string misses a project named "Datalake" whose
// text never says "SEAD"; word by word it is the top hit. An empty query
// lists everything.
func searchAny[T any](query string, key func(T) string, fetch func(term string) ([]T, error)) ([]T, error) {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return fetch("")
	}
	hits := map[string]int{}
	var order []T
	for _, term := range terms {
		items, err := fetch(term)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			k := key(it)
			if hits[k] == 0 {
				order = append(order, it)
			}
			hits[k]++
		}
	}
	// ponytail: one query per word (≤5); a ranked full-text index if the
	// inventory grows past what that costs.
	sort.SliceStable(order, func(i, j int) bool { return hits[key(order[i])] > hits[key(order[j])] })
	return order, nil
}

func searchHosts(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct {
		Query      string `json:"query"`
		Situacao   string `json:"situacao"`
		AlertLevel string `json:"alert_level"`
	}
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	hosts, err := searchAny(a.Query, func(h service.HostListItem) string { return h.OficialSlug }, func(term string) ([]service.HostListItem, error) {
		return service.NewHostService(env.db).List(ctx, models.HostFilter{Search: term, Situacao: a.Situacao})
	})
	if err != nil {
		return toolOutput{}, err
	}
	if a.AlertLevel != "" {
		kept := hosts[:0]
		for _, h := range hosts {
			for _, al := range h.Alerts {
				if al.Level == a.AlertLevel {
					kept = append(kept, h)
					break
				}
			}
		}
		hosts = kept
	}
	out := toolOutput{Total: len(hosts)}
	for _, h := range first(hosts) {
		ref := aiRef{Type: "host", ID: h.OficialSlug, Label: h.Nickname, Href: "/hosts/" + h.OficialSlug}
		alerts := []string{}
		for _, al := range h.Alerts {
			if len(alerts) < 3 {
				alerts = append(alerts, al.Level+": "+clip(al.Message, 120))
			}
		}
		out.Items = append(out.Items, map[string]any{
			"nickname": h.Nickname, "hostname": h.Hostname, "situacao": h.Situacao, "hospedagem": h.Hospedagem,
			"entidade": h.MainEntidade, "description": clip(h.Description, 200), "alerts": alerts,
			"services": h.ServicesCount, "href": ref.Href,
		})
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

func searchServices(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct{ Query string }
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	svcs, err := searchAny(a.Query, func(s models.Service) string { return strconv.FormatInt(s.ID, 10) }, func(term string) ([]models.Service, error) {
		return store.NewServiceRepo(env.db.SQL).ListFiltered(ctx, models.ServiceFilter{Search: term})
	})
	if err != nil {
		return toolOutput{}, err
	}
	out := toolOutput{Total: len(svcs)}
	for _, s := range first(svcs) {
		ref := aiRef{Type: "service", ID: strconv.FormatInt(s.ID, 10), Label: s.Nickname, Href: fmt.Sprintf("/services/%d", s.ID)}
		out.Items = append(out.Items, map[string]any{
			"id": s.ID, "nickname": s.Nickname, "description": clip(s.Description, 200), "kind": s.ServiceKind,
			"stack": s.TechnologyStack, "environment": s.Environment, "status": s.ContainerStatus, "url": s.ExternalURL, "href": ref.Href,
		})
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

func searchDNS(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct{ Query string }
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	recs, err := searchAny(a.Query, func(d models.DNSRecord) string { return strconv.FormatInt(d.ID, 10) }, func(term string) ([]models.DNSRecord, error) {
		return store.NewDNSRepo(env.db.SQL).ListFiltered(ctx, models.DNSFilter{Search: term})
	})
	if err != nil {
		return toolOutput{}, err
	}
	out := toolOutput{Total: len(recs)}
	for _, d := range first(recs) {
		ref := aiRef{Type: "dns", ID: strconv.FormatInt(d.ID, 10), Label: d.Domain, Href: fmt.Sprintf("/dns/%d", d.ID)}
		item := map[string]any{"id": d.ID, "domain": d.Domain, "situacao": d.Situacao, "https": d.HasHTTPS, "href": ref.Href}
		if d.CertExpiresAt != nil {
			item["cert_expires"] = d.CertExpiresAt.Format("2006-01-02")
		}
		out.Items = append(out.Items, item)
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

func searchProjects(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct{ Query string }
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	projects, err := searchAny(a.Query, func(p models.Project) string { return strconv.FormatInt(p.ID, 10) }, func(term string) ([]models.Project, error) {
		return store.NewProjectRepo(env.db.SQL).ListFiltered(ctx, models.ProjectFilter{Search: term})
	})
	if err != nil {
		return toolOutput{}, err
	}
	out := toolOutput{Total: len(projects)}
	for _, p := range first(projects) {
		ref := aiRef{Type: "project", ID: strconv.FormatInt(p.ID, 10), Label: p.Name, Href: fmt.Sprintf("/projects/%d", p.ID)}
		out.Items = append(out.Items, map[string]any{
			"id": p.ID, "name": p.Name, "situacao": p.Situacao, "description": clip(p.Description, 200),
			"has_glpi_profile": p.GlpiTokenID != nil, "href": ref.Href,
		})
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

func searchAPIs(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct{ Query string }
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	repo := store.NewAPICatalogRepo(env.db.SQL)
	apis, err := searchAny(a.Query, func(x models.APICatalog) string { return strconv.FormatInt(x.ID, 10) }, func(term string) ([]models.APICatalog, error) {
		return repo.List(ctx, models.APICatalogFilter{Query: term})
	})
	if err != nil {
		return toolOutput{}, err
	}
	ops, err := searchAny(a.Query, func(o models.OperationSearchResult) string { return fmt.Sprint(o.APIID, o.OpKey) }, func(term string) ([]models.OperationSearchResult, error) {
		return repo.SearchOperations(ctx, term, 0, 0)
	})
	if err != nil {
		return toolOutput{}, err
	}
	out := toolOutput{Total: len(apis) + len(ops)}
	for _, api := range first(apis) {
		ref := aiRef{Type: "api", ID: strconv.FormatInt(api.ID, 10), Label: api.Name, Href: fmt.Sprintf("/atlas/apis/%d", api.ID)}
		out.Items = append(out.Items, map[string]any{
			"api": api.Name, "title": api.Title, "description": clip(api.Description, 200), "endpoints": api.OperationCount, "href": ref.Href,
		})
		out.Refs = append(out.Refs, ref)
	}
	for _, op := range first(ops) {
		ref := aiRef{Type: "api", ID: strconv.FormatInt(op.APIID, 10), Label: op.APIName, Href: fmt.Sprintf("/atlas/apis/%d", op.APIID)}
		out.Items = append(out.Items, map[string]any{
			"endpoint": op.Method + " " + op.Path, "api": op.APIName, "summary": clip(op.Summary, 160), "href": ref.Href,
		})
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

func searchIssues(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct {
		Query        string `json:"query"`
		Status       string `json:"status"`
		AssignedToMe bool   `json:"assigned_to_me"`
	}
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	f := models.IssueFilter{Status: a.Status, ExcludeArchived: true}
	if a.AssignedToMe && env.user != nil {
		f.AssigneeID = env.user.ID
	}
	f.VisibleSQL, f.VisibleArgs = store.VisibleExprDyn(ctx, "entity_type", "entity_id")
	issues, err := searchAny(a.Query, func(i models.Issue) string { return strconv.FormatInt(i.ID, 10) }, func(term string) ([]models.Issue, error) {
		f.Search = term
		return models.ListIssues(env.db.SQL, f)
	})
	if err != nil {
		return toolOutput{}, err
	}
	out := toolOutput{Total: len(issues)}
	for _, i := range first(issues) {
		ref := aiRef{Type: "issue", ID: strconv.FormatInt(i.ID, 10), Label: i.Title, Href: "/issues"}
		out.Items = append(out.Items, map[string]any{
			"id": i.ID, "title": i.Title, "status": i.Status, "priority": i.Priority,
			"parent": i.EntityType, "due": i.ExpectedEndDate, "href": ref.Href,
		})
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

// searchWiki runs the same workspace search as GET /api/wiki/search, which
// every signed-in user can already call.
func searchWiki(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct{ Query string }
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	settings, err := outlineclient.LoadSettings(env.db.SQL, env.db.Encryptor)
	if err != nil {
		return toolOutput{}, err
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		return toolOutput{}, errors.New("the wiki is not configured")
	}
	sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	hits, err := client.DocumentsSearch(sctx, a.Query, "", maxToolItems)
	if err != nil {
		return toolOutput{}, err
	}
	out := toolOutput{Total: len(hits)}
	for _, h := range first(hits) {
		ref := aiRef{Type: "wiki", ID: h.Document.ID, Label: h.Document.Title, Href: "/wiki?doc=" + h.Document.ID}
		out.Items = append(out.Items, map[string]any{"title": h.Document.Title, "excerpt": clip(h.Context, 300), "href": ref.Href})
		out.Refs = append(out.Refs, ref)
	}
	return out, nil
}

// ── Proposal tools ──────────────────────────────────────────────────────

// parentRef resolves an issue parent the caller can see, or errToolNotFound.
func parentRef(ctx context.Context, db *database.DB, typ string, id int64) (*aiRef, error) {
	ok, err := store.CanSee(ctx, db.SQL, store.AssetType(typ), id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errToolNotFound
	}
	sid := strconv.FormatInt(id, 10)
	switch typ {
	case "host":
		if h, err := store.NewHostRepo(db.SQL).GetByID(ctx, id); err == nil && h != nil {
			return &aiRef{Type: typ, ID: h.OficialSlug, Label: h.Nickname, Href: "/hosts/" + h.OficialSlug}, nil
		}
	case "dns":
		if d, err := store.NewDNSRepo(db.SQL).Get(ctx, id); err == nil && d != nil {
			return &aiRef{Type: typ, ID: sid, Label: d.Domain, Href: "/dns/" + sid}, nil
		}
	case "service":
		if s, err := store.NewServiceRepo(db.SQL).Get(ctx, id); err == nil && s != nil {
			return &aiRef{Type: typ, ID: sid, Label: s.Nickname, Href: "/services/" + sid}, nil
		}
	case "project":
		if p, err := store.NewProjectRepo(db.SQL).Get(ctx, id); err == nil && p != nil {
			return &aiRef{Type: typ, ID: sid, Label: p.Name, Href: "/projects/" + sid}, nil
		}
	}
	return nil, errToolNotFound
}

func proposeIssue(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		ParentType  string `json:"parent_type"`
		ParentID    int64  `json:"parent_id"`
	}
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	if strings.TrimSpace(a.Title) == "" {
		return toolOutput{}, errors.New("title is required")
	}
	switch a.Priority {
	case "low", "medium", "high", "critical":
	default:
		a.Priority = "medium"
	}
	switch a.ParentType {
	case "host", "dns", "service", "project":
	default:
		return toolOutput{}, errors.New("parent_type must be host, dns, service or project")
	}
	parent, err := parentRef(ctx, env.db, a.ParentType, a.ParentID)
	if err != nil {
		return toolOutput{}, err
	}
	return toolOutput{Proposal: &aiProposal{
		Kind: "issue", Title: clip(a.Title, 200), Description: clip(a.Description, 4000), Priority: a.Priority,
		Parent: parent, ParentType: a.ParentType, ParentID: a.ParentID,
	}}, nil
}

func proposeChamado(ctx context.Context, env aiToolEnv, raw json.RawMessage) (toolOutput, error) {
	var a struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ProjectID   int64  `json:"project_id"`
		HostSlug    string `json:"host_slug"`
	}
	if err := decodeArgs(raw, &a); err != nil {
		return toolOutput{}, err
	}
	if strings.TrimSpace(a.Title) == "" {
		return toolOutput{}, errors.New("title is required")
	}
	settings, err := glpiclient.LoadSettings(env.db.SQL, env.db.Encryptor)
	if err != nil {
		return toolOutput{}, err
	}
	if !settings.Enabled {
		return toolOutput{}, errors.New("GLPI integration is disabled; tickets cannot be opened from Bridge")
	}
	tokens := store.NewGlpiTokenRepo(env.db.SQL)
	var profileID int64
	if a.ProjectID != 0 {
		p, err := store.NewProjectRepo(env.db.SQL).Get(ctx, a.ProjectID)
		if err != nil {
			return toolOutput{}, err
		}
		if p == nil {
			return toolOutput{}, errToolNotFound
		}
		if p.GlpiTokenID != nil {
			profileID = *p.GlpiTokenID
		}
	}
	if profileID == 0 {
		all, err := tokens.List(ctx)
		if err != nil {
			return toolOutput{}, err
		}
		if len(all) != 1 {
			return toolOutput{}, errors.New("no GLPI profile for this ticket: ask the user which project it belongs to (search_projects), or suggest the chamados page")
		}
		profileID = all[0].ID
	}
	prof, err := tokens.Get(ctx, profileID)
	if err != nil || prof == nil {
		return toolOutput{}, errors.New("GLPI profile not found")
	}
	if a.HostSlug != "" {
		if h, err := store.NewHostRepo(env.db.SQL).GetBySlug(ctx, a.HostSlug); err != nil || h == nil {
			return toolOutput{}, errToolNotFound
		}
	}
	return toolOutput{Proposal: &aiProposal{
		Kind: "chamado", Title: clip(a.Title, 200), Description: clip(a.Description, 4000),
		Profile: &aiGlpiTarget{ID: prof.ID, Name: prof.Name}, HostSlug: a.HostSlug,
	}}, nil
}

// ── Chat loop ───────────────────────────────────────────────────────────

// maxToolRounds bounds how many times the model may call tools before it must answer.
const maxToolRounds = 4

type aiStep struct {
	Tool  string `json:"tool"`
	Query string `json:"query,omitempty"`
	Count int    `json:"count"`
	Error string `json:"error,omitempty"`
}

type aiChatResult struct {
	Response  string       `json:"response"`
	Tools     bool         `json:"tools"`
	Steps     []aiStep     `json:"steps"`
	Refs      []aiRef      `json:"refs"`
	Proposals []aiProposal `json:"proposals"`
}

type chatModel interface {
	Chat(ctx context.Context, messages []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error)
}

// runChat answers one user turn, letting the model call the user's tools
// (up to maxToolRounds rounds).
func runChat(ctx context.Context, model chatModel, env aiToolEnv, messages []llm.ChatMessage) (aiChatResult, error) {
	return runToolLoop(ctx, model, env, messages, toolsFor(env.user), maxToolRounds)
}

// runToolLoop lets the model call the given tools for up to maxRounds
// rounds, then makes it answer (the last round offers no tools). A gateway
// that rejects tools (4xx on the first call) gets the plain conversation
// instead, and Tools reports false.
func runToolLoop(ctx context.Context, model chatModel, env aiToolEnv, messages []llm.ChatMessage, available []aiTool, maxRounds int) (aiChatResult, error) {
	byName := map[string]aiTool{}
	defs := make([]llm.Tool, 0, len(available))
	for _, t := range available {
		byName[t.def.Function.Name] = t
		defs = append(defs, t.def)
	}
	res := aiChatResult{Tools: len(defs) > 0, Steps: []aiStep{}, Refs: []aiRef{}, Proposals: []aiProposal{}}
	seen := map[string]bool{}

	for round := 0; round <= maxRounds; round++ {
		var tools []llm.Tool
		if res.Tools && round < maxRounds {
			tools = defs // the last round has none, so the model must answer
		}
		msg, err := model.Chat(ctx, messages, tools)
		var apiErr *llm.APIError
		if err != nil && round == 0 && tools != nil && errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			log.Printf("[ai-tool] tools rejected by the model gateway (%d), answering without them", apiErr.Status)
			res.Tools = false
			msg, err = model.Chat(ctx, messages, nil)
		}
		if err != nil {
			return res, err
		}
		if len(msg.ToolCalls) == 0 || tools == nil {
			res.Response = msg.Content
			return res, nil
		}
		messages = append(messages, llm.ChatMessage{Role: "assistant", Content: msg.Content, ToolCalls: msg.ToolCalls})
		for _, call := range msg.ToolCalls {
			messages = append(messages, llm.ChatMessage{Role: "tool", ToolCallID: call.ID, Content: runToolCall(ctx, env, byName, call, &res, seen)})
		}
	}
	return res, nil // unreachable: the last round has no tools
}

// runToolCall executes one call and returns the JSON the model reads back.
func runToolCall(ctx context.Context, env aiToolEnv, byName map[string]aiTool, call llm.ToolCall, res *aiChatResult, seen map[string]bool) string {
	name := call.Function.Name
	step := aiStep{Tool: name}
	// The step label is whatever the call targets: a search, a draft's
	// title, or (code tools) a repo / host and a path or name.
	var q struct {
		Query string `json:"query"`
		Title string `json:"title"`
		Repo  string `json:"repo"`
		Host  string `json:"host"`
		Path  string `json:"path"`
		Name  string `json:"name"`
	}
	_ = json.Unmarshal([]byte(call.Function.Arguments), &q)
	target := strings.Trim(q.Repo+q.Host+":"+q.Path+q.Name, ":")
	step.Query = clip(strings.TrimSpace(target+" "+q.Query+q.Title), 120)

	reply := func(v any) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	t, ok := byName[name]
	if !ok {
		step.Error = "unknown tool"
		res.Steps = append(res.Steps, step)
		return reply(map[string]string{"error": "unknown tool " + name})
	}
	out, err := t.run(ctx, env, json.RawMessage(call.Function.Arguments))
	userID := int64(0)
	if env.user != nil {
		userID = env.user.ID
	}
	log.Printf("[ai-tool] user=%d tool=%s results=%d err=%v", userID, name, out.Total, err)
	if err != nil {
		step.Error = err.Error()
		res.Steps = append(res.Steps, step)
		return reply(map[string]string{"error": err.Error()})
	}
	if out.Proposal != nil {
		out.Proposal.ID = fmt.Sprintf("p%d", len(res.Proposals)+1)
		res.Proposals = append(res.Proposals, *out.Proposal)
		step.Count = 1
		res.Steps = append(res.Steps, step)
		return reply(map[string]string{
			"proposal_id": out.Proposal.ID,
			"status":      "shown to the user as a draft to review and confirm; it has NOT been created yet",
		})
	}
	step.Count = out.Total
	res.Steps = append(res.Steps, step)
	for _, r := range out.Refs {
		if !seen[r.Href+"|"+r.ID] {
			seen[r.Href+"|"+r.ID] = true
			res.Refs = append(res.Refs, r)
		}
	}
	items := out.Items
	if items == nil {
		items = []map[string]any{}
	}
	// Tool output is data the user's inventory holds, not instructions.
	return reply(map[string]any{"total": out.Total, "shown": len(items), "items": items})
}
