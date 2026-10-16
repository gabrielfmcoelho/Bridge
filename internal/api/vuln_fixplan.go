package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

const (
	fixPlanMaxRounds = 10 // tool rounds before the model must answer
	fixPlanMaxRepos  = 5
	fixPlanPerType   = 10 // hosts / services / DNS / projects in the topology
	fixReadLines     = 200
	fixReadChars     = 12_000
)

// fixRepo is one repository the fix plan may read, keyed by its GitLab path.
type fixRepo struct {
	Key  string // path_with_namespace
	ID   int
	Name string
	Ref  string
}

// fixScope is what one fix plan may look at: the visible topology around the
// finding as text, and the repos / host scans its tools are allowed to read.
// Everything in it went through the caller's visibility.
type fixScope struct {
	topology string
	repos    []fixRepo
	hosts    map[string]*sshtest.VMInfo // lower-cased host nickname → last scan
	notes    []string                   // what could not be gathered
	trees    map[int][]gitlabclient.TreeEntry
	read     strings.Builder // every tool output, to check the plan's versions against
}

// buildFixScope walks one hop out from the finding's (visible) links. Every
// entity is loaded through its scoped Get, so a hidden one is dropped; the
// relation helpers only return ids. gl is nil when GitLab is off.
func buildFixScope(ctx context.Context, db *database.DB, f *models.VulnFinding, gl *gitlabclient.Client, gls gitlabclient.Settings) *fixScope {
	sc := &fixScope{hosts: map[string]*sshtest.VMInfo{}, trees: map[int][]gitlabclient.TreeEntry{}}
	hostsR, svcR, dnsR, projR := store.NewHostRepo(db.SQL), store.NewServiceRepo(db.SQL), store.NewDNSRepo(db.SQL), store.NewProjectRepo(db.SQL)

	var hosts []*models.Host
	var services []*models.Service
	var records []*models.DNSRecord
	var projects []*models.Project
	seen := map[string]bool{}
	first := func(kind string, id int64, n int) bool {
		k := fmt.Sprintf("%s:%d", kind, id)
		if seen[k] || n >= fixPlanPerType {
			return false
		}
		seen[k] = true
		return true
	}
	addHost := func(id int64) {
		if first("host", id, len(hosts)) {
			if h, _ := hostsR.GetByID(ctx, id); h != nil {
				hosts = append(hosts, h)
			}
		}
	}
	addService := func(id int64) {
		if first("service", id, len(services)) {
			if s, _ := svcR.Get(ctx, id); s != nil {
				services = append(services, s)
			}
		}
	}
	addDNS := func(id int64) {
		if first("dns", id, len(records)) {
			if d, _ := dnsR.Get(ctx, id); d != nil {
				records = append(records, d)
			}
		}
	}
	addProject := func(id int64) {
		if first("project", id, len(projects)) {
			if p, _ := projR.Get(ctx, id); p != nil {
				projects = append(projects, p)
			}
		}
	}
	for _, l := range f.Links {
		switch l.EntityType {
		case "host":
			addHost(l.EntityID)
			if ss, err := svcR.ListByHost(ctx, l.EntityID); err == nil {
				for _, s := range ss {
					addService(s.ID)
				}
			}
			if ds, err := dnsR.RecordsByHost(ctx, l.EntityID); err == nil {
				for _, d := range ds {
					addDNS(d.ID)
				}
			}
			if ps, err := projR.ProjectsByHost(ctx, l.EntityID); err == nil {
				for _, p := range ps {
					addProject(p.ID)
				}
			}
		case "service":
			addService(l.EntityID)
			each(svcR.HostIDs(ctx, l.EntityID))(addHost)
			each(svcR.DNSIDs(ctx, l.EntityID))(addDNS)
		case "dns":
			addDNS(l.EntityID)
			each(dnsR.HostIDs(ctx, l.EntityID))(addHost)
			each(dnsR.ServiceIDs(ctx, l.EntityID))(addService)
			each(dnsR.ProjectIDs(ctx, l.EntityID))(addProject)
		case "project":
			addProject(l.EntityID)
			if ss, err := svcR.ListByProject(ctx, l.EntityID); err == nil {
				for _, s := range ss {
					addService(s.ID)
				}
			}
			each(projR.HostIDs(ctx, l.EntityID))(addHost)
			each(projR.DirectDNSIDs(ctx, l.EntityID))(addDNS)
		}
	}
	// A service's project holds its repos.
	for _, s := range services {
		if s.ProjectID != nil {
			addProject(*s.ProjectID)
		}
	}

	var b strings.Builder
	if len(hosts)+len(services)+len(records)+len(projects) == 0 {
		b.WriteString("Nenhum ativo visível ligado a este achado.\n")
	}
	if len(hosts) > 0 {
		b.WriteString("Hosts:\n")
		scans := store.NewHostScanRepo(db.SQL)
		for _, h := range hosts {
			fmt.Fprintf(&b, "- %s (hostname %s)", h.Nickname, orDash(h.Hostname))
			if scan, _ := scans.GetLatest(ctx, h.ID); scan != nil {
				var vm sshtest.VMInfo
				if json.Unmarshal([]byte(scan.Data), &vm) == nil {
					sc.hosts[strings.ToLower(h.Nickname)] = &vm
					fmt.Fprintf(&b, " · SO: %s · kernel %s · última varredura %s", orDash(vm.OS), orDash(vm.Kernel), scan.ScannedAt.Format("2006-01-02"))
					if len(vm.ParsedContainers) > 0 {
						var cs []string
						for i, c := range vm.ParsedContainers {
							if i == 15 {
								cs = append(cs, "…")
								break
							}
							cs = append(cs, c.Name+" ("+c.Image+")")
						}
						b.WriteString(" · containers: " + strings.Join(cs, ", "))
					}
					if len(vm.ServiceInventory) > 0 {
						var ss []string
						for i, s := range vm.ServiceInventory {
							if i == 15 {
								ss = append(ss, "…")
								break
							}
							ss = append(ss, strings.TrimSpace(s.Name+" "+s.Version))
						}
						b.WriteString(" · serviços detectados: " + strings.Join(ss, ", "))
					}
				}
			} else {
				b.WriteString(" · sem varredura")
			}
			b.WriteString("\n")
		}
	}
	if len(services) > 0 {
		b.WriteString("Serviços:\n")
		for _, s := range services {
			fmt.Fprintf(&b, "- %s", s.Nickname)
			for _, kv := range [][2]string{
				{"stack", s.TechnologyStack}, {"versão", s.Version}, {"imagem", s.ContainerImage},
				{"deploy", s.DeployApproach}, {"orquestrador", s.OrchestratorTool}, {"ambiente", s.Environment},
				{"coolify", s.CoolifyProject}, {"repo", strings.TrimSpace(s.GitRepository + " " + s.GitBranch)},
			} {
				if kv[1] != "" {
					fmt.Fprintf(&b, " · %s: %s", kv[0], kv[1])
				}
			}
			b.WriteString("\n")
		}
	}
	if len(records) > 0 {
		b.WriteString("DNS:\n")
		for _, d := range records {
			fmt.Fprintf(&b, "- %s · HTTPS %t", d.Domain, d.HasHTTPS)
			if d.CertIssuer != "" {
				fmt.Fprintf(&b, " · cert %s", d.CertIssuer)
			}
			b.WriteString("\n")
		}
	}
	if len(projects) > 0 {
		b.WriteString("Projetos:\n")
		for _, p := range projects {
			fmt.Fprintf(&b, "- %s\n", p.Name)
		}
	}
	sc.topology = b.String()

	if gl == nil {
		sc.notes = append(sc.notes, "GitLab Code Management desativado ou sem token: nenhum repositório pode ser lido.")
		return sc
	}
	addRepo := func(r fixRepo) {
		if len(sc.repos) >= fixPlanMaxRepos {
			return
		}
		for _, x := range sc.repos {
			if x.ID == r.ID {
				return
			}
		}
		sc.repos = append(sc.repos, r)
	}
	// A service's own repo first: its Coolify branch is what is deployed.
	for _, s := range services {
		for _, raw := range []string{s.GitRepository, s.GitlabURL, s.RepositoryURL} {
			path, ok := gitlabRepoPath(raw, gls.BaseURL)
			if !ok {
				continue
			}
			p, err := gl.SearchProjectByPath(path)
			if err != nil {
				sc.notes = append(sc.notes, "repo "+path+" do serviço "+s.Nickname+" não acessível")
				continue
			}
			addRepo(fixRepo{Key: p.PathWithNamespace, ID: p.ID, Name: p.Name, Ref: strOr(s.GitBranch, gls.DefaultRef)})
			break
		}
	}
	links := store.NewProjectGitLabLinkRepo(db.SQL)
	for _, p := range projects {
		ls, err := links.List(ctx, p.ID)
		if err != nil || len(ls) == 0 {
			continue
		}
		targets, warnings := gitlabTargets(ls, gl, gls.DefaultRef)
		sc.notes = append(sc.notes, warnings...)
		for _, t := range targets {
			addRepo(fixRepo{Key: t.Path, ID: t.ID, Name: t.Name, Ref: t.Ref})
		}
	}
	if len(sc.repos) == 0 {
		sc.notes = append(sc.notes, "Nenhum repositório GitLab ligado aos projetos ou serviços deste achado.")
	}
	return sc
}

// each adapts a relation helper's (ids, err) to a loop over add; an error
// just contributes nothing.
func each(ids []int64, _ error) func(func(int64)) {
	return func(add func(int64)) {
		for _, id := range ids {
			add(id)
		}
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// gitlabRepoPath extracts "group/repo" from a clone or web URL (https, ssh://
// or scp-style git@host:path) when it points at the configured GitLab host.
func gitlabRepoPath(raw, baseURL string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" {
		return "", false
	}
	var host, path string
	if !strings.Contains(raw, "://") && strings.Contains(raw, "@") && strings.Contains(raw, ":") {
		// scp-style: git@host:group/repo.git
		at := raw[strings.Index(raw, "@")+1:]
		host, path, _ = strings.Cut(at, ":")
	} else {
		u, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		host, path = u.Host, u.Path
	}
	if !strings.EqualFold(strings.Split(host, ":")[0], base.Hostname()) {
		return "", false
	}
	path = strings.Trim(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	// Web URLs may point inside the repo: …/group/repo/-/tree/main
	if i := strings.Index(path, "/-/"); i >= 0 {
		path = path[:i]
	}
	if !strings.Contains(path, "/") {
		return "", false
	}
	return path, true
}

// repo resolves a tool's repo argument against the allowlist.
func (sc *fixScope) repo(key string) (fixRepo, error) {
	for _, r := range sc.repos {
		if strings.EqualFold(r.Key, strings.TrimSpace(key)) {
			return r, nil
		}
	}
	keys := make([]string, 0, len(sc.repos))
	for _, r := range sc.repos {
		keys = append(keys, r.Key)
	}
	if len(keys) == 0 {
		return fixRepo{}, errors.New("no repository is available for this finding")
	}
	return fixRepo{}, fmt.Errorf("repo %q is not available; use one of: %s", key, strings.Join(keys, ", "))
}

func (sc *fixScope) tree(gl *gitlabclient.Client, r fixRepo) ([]gitlabclient.TreeEntry, error) {
	if t, ok := sc.trees[r.ID]; ok {
		return t, nil
	}
	t, err := gl.ListTree(r.ID, r.Ref, "", true)
	if err != nil {
		return nil, err
	}
	sc.trees[r.ID] = t
	return t, nil
}

// fixTools are the read-only tools of one fix plan. Every repo / host
// argument is checked against the scope, so the model can't read beyond the
// finding's topology. gl may be nil: then only host_packages works.
func fixTools(sc *fixScope, gl *gitlabclient.Client) []aiTool {
	type args struct {
		Repo      string `json:"repo"`
		Path      string `json:"path"`
		Name      string `json:"name"`
		Query     string `json:"query"`
		Host      string `json:"host"`
		StartLine int    `json:"start_line"`
	}
	parse := func(raw json.RawMessage) args {
		var a args
		_ = json.Unmarshal(raw, &a)
		return a
	}
	repoOf := func(a args) (fixRepo, error) {
		if gl == nil {
			return fixRepo{}, errors.New("GitLab is not available")
		}
		return sc.repo(a.Repo)
	}
	out := func(items []map[string]any) (toolOutput, error) {
		b, _ := json.Marshal(items)
		sc.read.Write(b)
		return toolOutput{Total: len(items), Items: items}, nil
	}

	return []aiTool{
		tool("list_files", "List one directory of a repository (path \"\" = root).",
			`{"type":"object","properties":{"repo":{"type":"string","description":"Repository path, from the list given"},"path":{"type":"string"}},"required":["repo"]}`,
			"", func(_ context.Context, _ aiToolEnv, raw json.RawMessage) (toolOutput, error) {
				a := parse(raw)
				r, err := repoOf(a)
				if err != nil {
					return toolOutput{}, err
				}
				entries, err := gl.ListTree(r.ID, r.Ref, strings.Trim(a.Path, "/"), false)
				if err != nil {
					return toolOutput{}, err
				}
				var items []map[string]any
				for i, e := range entries {
					if i == 200 {
						break
					}
					items = append(items, map[string]any{"path": e.Path, "type": e.Type})
				}
				return out(items)
			}),
		tool("find_files", "Find files in a repository whose path contains name (e.g. Dockerfile, package.json, nginx).",
			`{"type":"object","properties":{"repo":{"type":"string"},"name":{"type":"string"}},"required":["repo","name"]}`,
			"", func(_ context.Context, _ aiToolEnv, raw json.RawMessage) (toolOutput, error) {
				a := parse(raw)
				r, err := repoOf(a)
				if err != nil {
					return toolOutput{}, err
				}
				if strings.TrimSpace(a.Name) == "" {
					return toolOutput{}, errors.New("name is required")
				}
				entries, err := sc.tree(gl, r)
				if err != nil {
					return toolOutput{}, err
				}
				needle := strings.ToLower(strings.TrimSpace(a.Name))
				var items []map[string]any
				for _, e := range entries {
					if e.Type == "blob" && strings.Contains(strings.ToLower(e.Path), needle) {
						items = append(items, map[string]any{"path": e.Path})
						if len(items) == 50 {
							break
						}
					}
				}
				return out(items)
			}),
		tool("read_file", "Read a repository file, up to 200 numbered lines from start_line (default 1).",
			`{"type":"object","properties":{"repo":{"type":"string"},"path":{"type":"string"},"start_line":{"type":"integer"}},"required":["repo","path"]}`,
			"", func(_ context.Context, _ aiToolEnv, raw json.RawMessage) (toolOutput, error) {
				a := parse(raw)
				r, err := repoOf(a)
				if err != nil {
					return toolOutput{}, err
				}
				body, err := gl.GetRawFile(r.ID, strings.Trim(a.Path, "/"), r.Ref)
				if err != nil {
					return toolOutput{}, err
				}
				if strings.ContainsRune(string(body[:min(len(body), 8000)]), 0) {
					return toolOutput{}, errors.New("binary file")
				}
				return out([]map[string]any{readLines(a.Path, string(body), a.StartLine)})
			}),
		tool("search_code", "Search file contents of a repository (GitLab code search).",
			`{"type":"object","properties":{"repo":{"type":"string"},"query":{"type":"string"}},"required":["repo","query"]}`,
			"", func(_ context.Context, _ aiToolEnv, raw json.RawMessage) (toolOutput, error) {
				a := parse(raw)
				r, err := repoOf(a)
				if err != nil {
					return toolOutput{}, err
				}
				hits, err := gl.SearchBlobs(r.ID, a.Query, r.Ref)
				if err != nil {
					return toolOutput{}, err
				}
				var items []map[string]any
				for i, h := range hits {
					if i == 10 {
						break
					}
					items = append(items, map[string]any{"path": h.Path, "line": h.Startline, "snippet": clip(h.Data, 400)})
				}
				return out(items)
			}),
		tool("host_packages", "Search a host's last scan: installed packages, detected services and containers whose name matches query.",
			`{"type":"object","properties":{"host":{"type":"string","description":"Host nickname, from the list given"},"query":{"type":"string"}},"required":["host","query"]}`,
			"", func(_ context.Context, _ aiToolEnv, raw json.RawMessage) (toolOutput, error) {
				a := parse(raw)
				vm, ok := sc.hosts[strings.ToLower(strings.TrimSpace(a.Host))]
				if !ok {
					return toolOutput{}, fmt.Errorf("host %q has no scan available", a.Host)
				}
				q := strings.ToLower(strings.TrimSpace(a.Query))
				if q == "" {
					return toolOutput{}, errors.New("query is required")
				}
				var items []map[string]any
				add := func(m map[string]any) bool {
					items = append(items, m)
					return len(items) < 30
				}
				for _, p := range vm.InstalledPackages {
					if strings.Contains(strings.ToLower(p.Name), q) && !add(map[string]any{"package": p.Name, "version": p.Version, "source": p.Source}) {
						return out(items)
					}
				}
				for _, s := range vm.ServiceInventory {
					if strings.Contains(strings.ToLower(s.Name+" "+s.Package), q) && !add(map[string]any{"service": s.Name, "version": s.Version, "package": s.Package, "state": s.State}) {
						return out(items)
					}
				}
				for _, c := range vm.ParsedContainers {
					if strings.Contains(strings.ToLower(c.Name+" "+c.Image), q) && !add(map[string]any{"container": c.Name, "image": c.Image, "status": c.Status}) {
						return out(items)
					}
				}
				return out(items)
			}),
	}
}

var versionRe = regexp.MustCompile(`\bv?(\d+\.\d+\.\d+)\b`)

// unsupportedVersions lists the x.y.z versions the plan cites that appear
// nowhere in what the model was given or read. Qwen names "fixed" versions
// from memory even when told not to; this catches it without trusting it.
func unsupportedVersions(plan, evidence string) []string {
	known := map[string]bool{}
	for _, m := range versionRe.FindAllStringSubmatch(evidence, -1) {
		known[m[1]] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range versionRe.FindAllStringSubmatch(plan, -1) {
		if !known[m[1]] && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// readLines returns up to fixReadLines numbered lines from start (1-based),
// capped at fixReadChars, and whether the file goes on.
func readLines(path, body string, start int) map[string]any {
	lines := strings.Split(body, "\n")
	if start < 1 {
		start = 1
	}
	var b strings.Builder
	end := start - 1
	for i := start - 1; i < len(lines) && i < start-1+fixReadLines; i++ {
		line := fmt.Sprintf("%d: %s\n", i+1, lines[i])
		if b.Len()+len(line) > fixReadChars {
			break
		}
		b.WriteString(line)
		end = i + 1
	}
	return map[string]any{"path": path, "from": start, "to": end, "total_lines": len(lines), "more": end < len(lines), "content": b.String()}
}

// fixPlanMessage is the user turn: the finding, its CVE when there is one,
// the topology, and what the tools may read.
func fixPlanMessage(f *models.VulnFinding, rec *cveRecordText, sc *fixScope) string {
	var b strings.Builder
	b.WriteString("## Achado\n")
	fmt.Fprintf(&b, "Título: %s\nSeveridade: %s\nAlvo: %s\n", f.Title, f.Severity, orDash(f.Target))
	for _, kv := range [][2]string{{"VM", f.VMHint}, {"Scanner", f.Scanner + " · " + f.Source}, {"CWE", f.CWE}, {"Risco", f.Risk}, {"Remediação sugerida pelo scanner", f.Remediation}, {"Referência", f.Reference}} {
		if strings.Trim(kv[1], " ·") != "" {
			fmt.Fprintf(&b, "%s: %s\n", kv[0], clip(kv[1], 1500))
		}
	}
	if f.Evidence != "" {
		fmt.Fprintf(&b, "Evidência:\n%s\n", clip(f.Evidence, 2000))
	}
	if rec != nil {
		b.WriteString("\n## CVE\n" + rec.text)
	}
	b.WriteString("\n## Topologia visível\n" + sc.topology)
	b.WriteString("\n## O que as ferramentas podem ler\n")
	if len(sc.repos) == 0 {
		b.WriteString("Repositórios: nenhum.\n")
	} else {
		b.WriteString("Repositórios (use o caminho como repo):\n")
		for _, r := range sc.repos {
			fmt.Fprintf(&b, "- %s (%s, ref %s)\n", r.Key, r.Name, orDash(r.Ref))
		}
	}
	var hosts []string
	for h := range sc.hosts {
		hosts = append(hosts, h)
	}
	if len(hosts) == 0 {
		b.WriteString("Hosts com varredura: nenhum.\n")
	} else {
		sort.Strings(hosts)
		b.WriteString("Hosts com varredura (host_packages): " + strings.Join(hosts, ", ") + "\n")
	}
	for _, n := range sc.notes {
		b.WriteString("Obs.: " + n + "\n")
	}
	return b.String()
}

// cveRecordText is the CVE block of the prompt. analysis is the catalog's
// LLM-written text inside it: shown to the model, but not a source for
// version numbers.
type cveRecordText struct{ text, analysis string }

// cveForFixPlan renders the finding's CVE from NVD (live) plus the catalog's
// generic pt-BR analysis when one exists. nil when the finding has no CVE.
func (h *vulnHandlers) cveForFixPlan(ctx context.Context, f *models.VulnFinding) *cveRecordText {
	id := strings.ToUpper(strings.TrimSpace(f.CVE))
	if !cveIDRe.MatchString(id) {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", id)
	if h.nvd != nil {
		if rec, err := h.nvd.CVE(ctx, id); err == nil {
			fmt.Fprintf(&b, "Descrição (NVD): %s\n", rec.Description)
			if rec.CVSS != nil {
				fmt.Fprintf(&b, "CVSS: %.1f (%s)\n", *rec.CVSS, rec.CVSSVector)
			}
			if len(rec.Affected) > 0 {
				b.WriteString("Versões afetadas (NVD CPE; em \"<X\", X é a primeira versão corrigida):\n- " + strings.Join(rec.Affected, "\n- ") + "\n")
			}
			if len(rec.References) > 0 {
				b.WriteString("Referências:\n- " + strings.Join(rec.References[:min(len(rec.References), 10)], "\n- ") + "\n")
			}
		} else {
			b.WriteString("Registro da NVD indisponível: " + err.Error() + "\n")
		}
	}
	rec := &cveRecordText{}
	if e, err := h.cveRepo().Get(ctx, id); err == nil && e != nil && e.AnalysisPT != "" {
		rec.analysis = clip(e.AnalysisPT, 4000)
		b.WriteString("Análise genérica (catálogo):\n" + rec.analysis + "\n")
	}
	rec.text = b.String()
	return rec
}

func (h *vulnHandlers) fixPlanRepo() *store.VulnFixPlanRepo {
	return store.NewVulnFixPlanRepo(h.db.SQL)
}

// handleGetFixPlan godoc
//
//	@Summary		Get a finding's fix plan
//	@Description	The case-specific fix plan, or a null body when none was generated. Never calls the LLM. 404 when the finding is absent or invisible. Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			id	path		int	true	"Finding ID"
//	@Success		200	{object}	models.VulnFixPlan
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/{id}/fix-plan [get]
func (h *vulnHandlers) handleGetFixPlan(w http.ResponseWriter, r *http.Request) {
	f, ok := h.load(w, r)
	if !ok {
		return
	}
	p, err := h.fixPlanRepo().Get(r.Context(), f.ID)
	if err != nil {
		jsonServerError(w, r, "failed to load fix plan", err)
		return
	}
	if p == nil {
		jsonOK(w, nil)
		return
	}
	jsonOK(w, p)
}

// handleGenerateFixPlan godoc
//
//	@Summary		Generate a finding's fix plan
//	@Description	The LLM explores, with read-only tools, the GitLab repos linked to the finding's projects / services and its hosts' last scans, then writes a case-specific fix plan in pt-BR. Replaces the previous plan. Without GitLab it works from the topology and host scans alone. Can take minutes. Requires ai.use.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			id	path		int	true	"Finding ID"
//	@Success		200	{object}	models.VulnFixPlan
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse	"LLM failed"
//	@Failure		503	{object}	httpx.ErrorResponse	"AI not configured"
//	@Router			/api/vulnerabilities/{id}/fix-plan [post]
func (h *vulnHandlers) handleGenerateFixPlan(w http.ResponseWriter, r *http.Request) {
	f, ok := h.load(w, r)
	if !ok {
		return
	}
	client, err := service.LLMClient(r.Context(), h.db)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}
	var gl *gitlabclient.Client
	gls, err := gitlabclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err == nil && gls.Enabled {
		gl = gitlabclient.NewServiceClient(gls)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Minute)
	defer cancel()
	sc := buildFixScope(ctx, h.db, f, gl, gls)
	user := auth.UserFromContext(r.Context())
	rec := h.cveForFixPlan(ctx, f)
	prompt := fixPlanMessage(f, rec, sc)
	res, err := runToolLoop(ctx, client, aiToolEnv{db: h.db, user: user}, []llm.ChatMessage{
		{Role: "system", Content: llm.FixPlanSystem},
		{Role: "user", Content: prompt},
	}, fixTools(sc, gl), fixPlanMaxRounds, true)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}
	content := strings.TrimSpace(res.Response)
	if content == "" {
		jsonError(w, http.StatusBadGateway, "AI returned an empty plan")
		return
	}
	// Evidence is what the model was given plus what its tools read (so
	// after the loop), minus the catalog analysis: LLM-written, not a source.
	evidence := prompt + sc.read.String()
	if rec != nil && rec.analysis != "" {
		evidence = strings.Replace(evidence, rec.analysis, "", 1)
	}
	if v := unsupportedVersions(content, evidence); len(v) > 0 {
		content = "> ⚠️ Versões citadas sem fonte no que o modelo leu: " + strings.Join(v, ", ") +
			". Confirme a versão corrigida no advisory antes de aplicar.\n\n" + content
	}
	if !res.Tools {
		content = "> O modelo não aceitou ferramentas: este plano foi feito sem ler código nem pacotes.\n\n" + content
	}
	steps, _ := json.Marshal(res.Steps)
	repos := make([]string, 0, len(sc.repos))
	for _, rp := range sc.repos {
		repos = append(repos, rp.Key)
	}
	if err := h.fixPlanRepo().Upsert(r.Context(), &models.VulnFixPlan{
		FindingID: f.ID, Content: content, Steps: steps, Repos: repos, Model: client.Model(), GeneratedBy: &user.ID,
	}); err != nil {
		jsonServerError(w, r, "failed to save fix plan", err)
		return
	}
	saved, err := h.fixPlanRepo().Get(r.Context(), f.ID)
	if err != nil || saved == nil {
		jsonServerError(w, r, "failed to reload fix plan", err)
		return
	}
	jsonOK(w, saved)
}
