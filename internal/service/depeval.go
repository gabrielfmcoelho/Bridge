package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/endoflife"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/nvd"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/osv"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vulnimport"
)

// Cache keys of dependency_advisories (see store.AdvisoryRepo).
func osvKey(d models.Dependency) string {
	return d.Ecosystem + "|" + d.LookupName + "|" + d.LookupVersion
}
func osvVulnKey(id string) string { return "vuln|" + id }
func nvdKeys(d models.Dependency) []string {
	var out []string
	for _, cpe := range strings.Split(d.LookupName, ",") {
		if cpe = strings.TrimSpace(cpe); cpe != "" {
			out = append(out, cpe+"|"+d.LookupVersion)
		}
	}
	return out
}

var severityRank = map[string]int{"critical": 5, "high": 4, "medium": 3, "low": 2, "info": 1}

// advisoryCache is what ApplyCache and the detail endpoint read.
type advisoryCache struct {
	osv, osvVuln, nvd, eol map[string]store.Advisory
}

func loadAdvisories(ctx context.Context, db *sql.DB, deps []models.Dependency) (*advisoryCache, error) {
	repo := store.NewAdvisoryRepo(db)
	var osvKeys, nvdK, eolK []string
	for _, d := range deps {
		switch d.Lookup {
		case models.DepLookupOSV:
			osvKeys = append(osvKeys, osvKey(d))
		case models.DepLookupNVD:
			nvdK = append(nvdK, nvdKeys(d)...)
		}
		if d.EOLProduct != "" {
			eolK = append(eolK, d.EOLProduct)
		}
	}
	c := &advisoryCache{}
	var err error
	if c.osv, err = repo.Get(ctx, models.DepLookupOSV, dedupe(osvKeys)); err != nil {
		return nil, err
	}
	var vulnKeys []string
	for _, a := range c.osv {
		var ids []string
		_ = json.Unmarshal([]byte(a.Data), &ids)
		for _, id := range ids {
			vulnKeys = append(vulnKeys, osvVulnKey(id))
		}
	}
	if c.osvVuln, err = repo.Get(ctx, models.DepLookupOSV, dedupe(vulnKeys)); err != nil {
		return nil, err
	}
	if c.nvd, err = repo.Get(ctx, models.DepLookupNVD, dedupe(nvdK)); err != nil {
		return nil, err
	}
	if c.eol, err = repo.Get(ctx, "eol", dedupe(eolK)); err != nil {
		return nil, err
	}
	return c, nil
}

// vulnsFor computes a row's advisories from the cache. evaluated is false
// when its lookup was never answered.
func (c *advisoryCache) vulnsFor(d models.Dependency) (vulns []models.DependencyVuln, evaluated bool) {
	seen := map[string]bool{}
	switch d.Lookup {
	case models.DepLookupOSV:
		a, ok := c.osv[osvKey(d)]
		if !ok || (a.Data == "" && a.Error != "") {
			return nil, false
		}
		var ids []string
		_ = json.Unmarshal([]byte(a.Data), &ids)
		for _, id := range ids {
			var v osv.Vuln
			if err := json.Unmarshal([]byte(c.osvVuln[osvVulnKey(id)].Data), &v); err != nil || v.ID == "" {
				v = osv.Vuln{ID: id, Severity: "info"}
			}
			key := v.CVE
			if key == "" {
				key = v.ID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			vulns = append(vulns, models.DependencyVuln{ID: key, OSVID: v.ID, Severity: v.Severity, CVSS: v.CVSS,
				Summary: v.Summary, Fixed: v.FixedFor(d.Ecosystem, d.LookupName), Source: models.DepLookupOSV})
		}
		evaluated = true
	case models.DepLookupNVD:
		for _, k := range nvdKeys(d) {
			a, ok := c.nvd[k]
			if !ok || (a.Data == "" && a.Error != "") {
				continue
			}
			evaluated = true
			var list []nvd.VersionCVE
			_ = json.Unmarshal([]byte(a.Data), &list)
			for _, v := range list {
				if seen[v.ID] {
					continue
				}
				seen[v.ID] = true
				vulns = append(vulns, models.DependencyVuln{ID: v.ID, Severity: osv.SeverityFromScore(v.CVSS), CVSS: v.CVSS,
					Summary: v.Summary, Source: models.DepLookupNVD})
			}
		}
	}
	sort.SliceStable(vulns, func(i, j int) bool {
		if a, b := severityRank[vulns[i].Severity], severityRank[vulns[j].Severity]; a != b {
			return a > b
		}
		return vulns[i].CVSS > vulns[j].CVSS
	})
	return vulns, evaluated
}

func (c *advisoryCache) eolFor(d models.Dependency, now time.Time) (bool, string, bool) {
	a, ok := c.eol[d.EOLProduct]
	if d.EOLProduct == "" || !ok || a.Data == "" {
		return false, "", false
	}
	var cycles []endoflife.Cycle
	if json.Unmarshal([]byte(a.Data), &cycles) != nil {
		return false, "", false
	}
	v := d.Version
	if d.Kind != models.DepKindOS {
		v = cleanUpstream(v)
	}
	eol, date, matched := endoflife.Status(cycles, v, now)
	return eol, date, matched
}

// DependencyVulns returns one row's advisories from the cache (the drawer).
func DependencyVulns(ctx context.Context, db *sql.DB, d models.Dependency) ([]models.DependencyVuln, error) {
	c, err := loadAdvisories(ctx, db, []models.Dependency{d})
	if err != nil {
		return nil, err
	}
	vulns, _ := c.vulnsFor(d)
	if vulns == nil {
		vulns = []models.DependencyVuln{}
	}
	return vulns, nil
}

// ApplyCache recomputes the CVE count, worst severity and EOL flag of the
// given hosts' rows (nil = every host) from the cache, and turns every
// matched CVE into an "inventory" finding. Offline: it never calls out.
// With hostIDs, only those hosts' absent findings resolve.
func ApplyCache(ctx context.Context, db *sql.DB, hostIDs []int64) (*models.VulnImportResult, error) {
	ctx = store.WithSystemScope(ctx)
	deps, err := store.NewDependencyRepo(db).ForHosts(ctx, hostIDs)
	if err != nil {
		return nil, err
	}
	c, err := loadAdvisories(ctx, db, deps)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	derived := make([]store.DependencyDerived, 0, len(deps))
	var hits []vulnimport.InventoryHit
	slugs := map[string]bool{}
	for _, d := range deps {
		slugs[d.HostSlug] = true
		vulns, evaluated := c.vulnsFor(d)
		eol, eolDate, eolKnown := c.eolFor(d, now)
		dd := store.DependencyDerived{ID: d.ID, CVECount: len(vulns), EOL: eol, EOLDate: eolDate}
		if evaluated || eolKnown {
			dd.EvaluatedAt = &now
		}
		for _, v := range vulns {
			if severityRank[v.Severity] > severityRank[dd.MaxSeverity] {
				dd.MaxSeverity = v.Severity
			}
			hits = append(hits, vulnimport.InventoryHit{Dep: d, Vuln: v})
		}
		derived = append(derived, dd)
	}
	if err := store.NewDependencyRepo(db).SetDerived(ctx, derived); err != nil {
		return nil, err
	}
	var cover *store.VulnCoverage
	if hostIDs != nil {
		cover = &store.VulnCoverage{Severities: []string{"critical", "high", "medium", "low", "info"}}
		for s := range slugs {
			cover.Targets = append(cover.Targets, s)
		}
		if len(cover.Targets) == 0 {
			return &models.VulnImportResult{}, nil // nothing inventoried there: nothing to resolve either
		}
	}
	return store.NewVulnRepo(db).ApplyImport(ctx, vulnimport.SourceInventory, vulnimport.FromInventory(hits), cover, false)
}

// DependencyEvalResult summarises one "Evaluate CVEs" run.
type DependencyEvalResult struct {
	OSVQueried  int `json:"osv_queried"`
	OSVVulns    int `json:"osv_vulns"`
	NVDQueried  int `json:"nvd_queried"`
	NVDDeferred int `json:"nvd_deferred"`
	EOLQueried  int `json:"eol_queried"`
	// Left for the next run when the time box closed first.
	OSVVulnsDeferred int                      `json:"osv_vulns_deferred"`
	EOLDeferred      int                      `json:"eol_deferred"`
	Errors           []string                 `json:"errors"`
	Findings         *models.VulnImportResult `json:"findings"`
}

// DependencyClients are the outbound clients Evaluate uses (tests swap
// them for fakes).
type DependencyClients struct {
	OSV *osv.Client
	NVD *nvd.Client
	EOL *endoflife.Client
	// NVDBudget caps NVD calls per run (each one is rate-gated).
	NVDBudget int
	// Reserve is the time kept before the caller's deadline for writing the
	// cache and applying findings; outbound calls stop when it starts. 0 = 45s.
	Reserve time.Duration
}

const advisoryMaxAge = 24 * time.Hour

// DependencyClientsFromSettings builds the clients, or errors when the
// integration is off. The NVD API key raises its call budget.
func DependencyClientsFromSettings(ctx context.Context, db *database.DB) (*DependencyClients, error) {
	settings := store.NewAppSettingsRepo(db.SQL)
	if settings.Value(ctx, "deps_cve_enabled") != "true" {
		return nil, fmt.Errorf("dependency CVE checks are not enabled (Settings › Integrations › Dependencies)")
	}
	n := nvd.New()
	budget := 30
	if key, ok, err := store.NewAppSecretRepo(db.SQL).Reveal(ctx, db.Encryptor, "nvd_api_key"); err != nil {
		return nil, fmt.Errorf("failed to decrypt the NVD API key: %w", err)
	} else if ok && key != "" {
		n.APIKey, budget = key, 300
	}
	return &DependencyClients{OSV: osv.New(), NVD: n, EOL: endoflife.New(), NVDBudget: budget}, nil
}

// EvaluateDependencies refreshes the cache for every inventoried version
// whose answer is missing, failed or older than a day, then applies it.
// Only ecosystem / product / version leave Bridge — never host names.
func EvaluateDependencies(ctx context.Context, db *sql.DB, cl *DependencyClients) (*DependencyEvalResult, error) {
	ctx = store.WithSystemScope(ctx)
	deps, err := store.NewDependencyRepo(db).ForHosts(ctx, nil)
	if err != nil {
		return nil, err
	}
	repo := store.NewAdvisoryRepo(db)
	res := &DependencyEvalResult{Errors: []string{}}
	fail := func(format string, a ...any) {
		if len(res.Errors) < 20 {
			res.Errors = append(res.Errors, fmt.Sprintf(format, a...))
		}
	}

	// Outbound calls run on work, which closes Reserve before the caller's
	// deadline: what doesn't fit is deferred to the next run, and there is
	// always time left to write the cache and apply findings (ctx).
	work, cancelWork := workContext(ctx, cl.Reserve)
	defer cancelWork()
	cutOff := func() bool { return work.Err() != nil }

	osvQueries := map[string]osv.Query{}
	var nvdK, eolK []string
	for _, d := range deps {
		switch d.Lookup {
		case models.DepLookupOSV:
			osvQueries[osvKey(d)] = osv.Query{Ecosystem: d.Ecosystem, Name: d.LookupName, Version: d.LookupVersion}
		case models.DepLookupNVD:
			nvdK = append(nvdK, nvdKeys(d)...)
		}
		if d.EOLProduct != "" {
			eolK = append(eolK, d.EOLProduct)
		}
	}
	keys := make([]string, 0, len(osvQueries))
	for k := range osvQueries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// OSV: one batch for every stale package version.
	stale, err := repo.Stale(ctx, models.DepLookupOSV, keys, advisoryMaxAge)
	if err != nil {
		return nil, err
	}
	if len(stale) > 0 {
		qs := make([]osv.Query, len(stale))
		for i, k := range stale {
			qs[i] = osvQueries[k]
		}
		ids, err := cl.OSV.QueryBatch(work, qs)
		switch {
		case err != nil && cutOff():
			// Out of time: the versions stay stale for the next run.
		case err != nil:
			fail("osv: %v", err)
			for _, k := range stale {
				_ = repo.Put(ctx, models.DepLookupOSV, k, "", err.Error())
			}
		default:
			res.OSVQueried = len(stale)
			for i, k := range stale {
				list := ids[i]
				if list == nil {
					list = []string{}
				}
				data, _ := json.Marshal(list)
				if err := repo.Put(ctx, models.DepLookupOSV, k, string(data), ""); err != nil {
					return nil, err
				}
			}
		}
	}

	// NVD: one rate-gated call per stale product version, within the budget.
	staleNVD, err := repo.Stale(ctx, models.DepLookupNVD, dedupe(nvdK), advisoryMaxAge)
	if err != nil {
		return nil, err
	}
	for i, k := range staleNVD {
		if i >= cl.NVDBudget || cutOff() {
			res.NVDDeferred = len(staleNVD) - i
			break
		}
		cpe, version, _ := strings.Cut(k, "|")
		list, err := cl.NVD.VersionCVEs(work, cpe, version)
		if err != nil {
			if cutOff() {
				res.NVDDeferred = len(staleNVD) - i
				break
			}
			fail("nvd %s %s: %v", cpe, version, err)
			_ = repo.Put(ctx, models.DepLookupNVD, k, "", err.Error())
			continue
		}
		if list == nil {
			list = []nvd.VersionCVE{}
		}
		data, _ := json.Marshal(list)
		if err := repo.Put(ctx, models.DepLookupNVD, k, string(data), ""); err != nil {
			return nil, err
		}
		res.NVDQueried++
	}

	// endoflife.date: one call per product.
	staleEOL, err := repo.Stale(ctx, "eol", dedupe(eolK), advisoryMaxAge)
	if err != nil {
		return nil, err
	}
	for i, p := range staleEOL {
		if cutOff() {
			res.EOLDeferred = len(staleEOL) - i
			break
		}
		cycles, err := cl.EOL.Cycles(work, p)
		if err != nil {
			if cutOff() {
				res.EOLDeferred = len(staleEOL) - i
				break
			}
			fail("endoflife %s: %v", p, err)
			_ = repo.Put(ctx, "eol", p, "", err.Error())
			continue
		}
		data, _ := json.Marshal(cycles)
		if err := repo.Put(ctx, "eol", p, string(data), ""); err != nil {
			return nil, err
		}
		res.EOLQueried++
	}

	// OSV advisory details last — the bulk of a first run (one call each) and
	// only severity/summary on top of findings that already exist. Taken
	// from every cached version answer, so details deferred by an earlier
	// run are picked up even when no version was stale this time.
	versions, err := repo.Get(ctx, models.DepLookupOSV, keys)
	if err != nil {
		return nil, err
	}
	var vulnIDs []string
	for _, a := range versions {
		var list []string
		_ = json.Unmarshal([]byte(a.Data), &list)
		for _, id := range list {
			vulnIDs = append(vulnIDs, osvVulnKey(id))
		}
	}
	vulnIDs = dedupe(vulnIDs)
	have, err := repo.Get(ctx, models.DepLookupOSV, vulnIDs)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, k := range vulnIDs {
		if a, ok := have[k]; !ok || a.Data == "" {
			missing = append(missing, strings.TrimPrefix(k, "vuln|"))
		}
	}
	sort.Strings(missing)
	fetched, deferred := fetchOSVVulns(ctx, work, cutOff, cl.OSV, repo, missing, fail)
	res.OSVVulns, res.OSVVulnsDeferred = fetched, deferred

	if res.Findings, err = ApplyCache(ctx, db, nil); err != nil {
		return nil, err
	}
	return res, nil
}

// workContext ends reserve before ctx's deadline (none → just cancellable).
func workContext(ctx context.Context, reserve time.Duration) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return context.WithCancel(ctx)
	}
	if reserve <= 0 {
		reserve = 45 * time.Second
	}
	return context.WithDeadline(ctx, deadline.Add(-reserve))
}

// fetchOSVVulns fetches advisory details with 8 workers on work and caches
// them on ctx. Returns how many were fetched and how many the time box left
// for the next run; a cut-off call is neither an error nor cached.
func fetchOSVVulns(ctx, work context.Context, cutOff func() bool, cl *osv.Client, repo *store.AdvisoryRepo, ids []string, fail func(string, ...any)) (fetched, deferred int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	queue := make(chan string)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range queue {
				v, err := cl.Vuln(work, id)
				mu.Lock()
				switch {
				case err != nil && cutOff():
					deferred++
				case err != nil:
					fail("osv %s: %v", id, err)
					_ = repo.Put(ctx, models.DepLookupOSV, osvVulnKey(id), "", err.Error())
				default:
					data, _ := json.Marshal(v)
					if err := repo.Put(ctx, models.DepLookupOSV, osvVulnKey(id), string(data), ""); err != nil {
						log.Printf("[deps] cache osv %s: %v", id, err)
					}
					fetched++
				}
				mu.Unlock()
			}
		}()
	}
	for i, id := range ids {
		if cutOff() {
			mu.Lock()
			deferred += len(ids) - i
			mu.Unlock()
			break
		}
		queue <- id
	}
	close(queue)
	wg.Wait()
	return fetched, deferred
}

func dedupe(ss []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
