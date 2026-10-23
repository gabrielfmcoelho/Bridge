package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/endoflife"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/nvd"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/osv"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// fakeAdvisories serves OSV, NVD and endoflife.date from one test server.
// fail flips every endpoint to HTTP 500.
func fakeAdvisories(t *testing.T, fail *atomic.Bool) (*service.DependencyClients, *atomic.Int32) {
	var nvdCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "down", 500)
			return
		}
		switch {
		case r.URL.Path == "/osv/querybatch":
			var body struct {
				Queries []struct {
					Package struct{ Ecosystem, Name string } `json:"package"`
					Version string `json:"version"`
				} `json:"queries"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			out := `{"results":[`
			for i, q := range body.Queries {
				if i > 0 {
					out += ","
				}
				if q.Package.Name == "openssl" && q.Version == "3.0.11-1~deb12u2" {
					out += `{"vulns":[{"id":"DEBIAN-CVE-2024-0727"},{"id":"DEBIAN-CVE-2023-5678"}]}`
				} else {
					out += `{"vulns":[]}`
				}
			}
			w.Write([]byte(out + `]}`))
		case strings.HasPrefix(r.URL.Path, "/osv/vulns/"):
			id := strings.TrimPrefix(r.URL.Path, "/osv/vulns/")
			vec := "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L" // 5.3 medium
			if id == "DEBIAN-CVE-2024-0727" {
				vec = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" // 9.8 critical
			}
			w.Write([]byte(`{"id":"` + id + `","details":"x","upstream":["` + strings.TrimPrefix(id, "DEBIAN-") + `"],
				"severity":[{"type":"CVSS_V3","score":"` + vec + `"}],
				"affected":[{"package":{"name":"openssl","ecosystem":"Debian:12"},"ranges":[{"events":[{"introduced":"0"},{"fixed":"3.0.13-1~deb12u1"}]}]}]}`))
		case r.URL.Path == "/nvd":
			nvdCalls.Add(1)
			if r.URL.Query().Get("virtualMatchString") == "cpe:2.3:a:php:php:7.4.33" {
				w.Write([]byte(`{"totalResults":1,"vulnerabilities":[{"cve":{"id":"CVE-2024-4577","descriptions":[{"lang":"en","value":"argument injection"}],"metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":9.8}}]}}}]}`))
				return
			}
			w.Write([]byte(`{"totalResults":0,"vulnerabilities":[]}`))
		case r.URL.Path == "/eol/php.json":
			w.Write([]byte(`[{"cycle":"8.3","eol":"2027-12-31"},{"cycle":"7.4","eol":"2022-11-28"}]`))
		case r.URL.Path == "/eol/debian.json":
			w.Write([]byte(`[{"cycle":"12","eol":"2028-06-10"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return &service.DependencyClients{
		OSV:       &osv.Client{BaseURL: srv.URL + "/osv", HTTP: srv.Client()},
		NVD:       &nvd.Client{BaseURL: srv.URL + "/nvd", HTTP: srv.Client()},
		EOL:       &endoflife.Client{BaseURL: srv.URL + "/eol", HTTP: srv.Client()},
		NVDBudget: 30,
	}, &nvdCalls
}

func TestEvaluateDependencies_EndToEnd(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	h := &models.Host{Nickname: "vm-php", OficialSlug: "vm-php", Hostname: "10.0.0.8", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	web := sshtest.ContainerInfo{ID: "0123456789ab", Name: "portal", Image: "php:7.4-fpm", Status: "Up 1 hour"}
	if err := store.NewServiceRepo(d.SQL).ReconcileDiscovered(ctx, h.ID, store.DiscoveredInventory{Containers: []sshtest.ContainerInfo{web}, ContainersKnown: true}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	scan := func(sslVersion string) sshtest.VMInfo {
		return sshtest.VMInfo{ParsedContainers: []sshtest.ContainerInfo{web}, ContainersKnown: true, Dependencies: []sshtest.DepScope{{
			Container: "portal", ContainerID: web.ID, Image: web.Image,
			OS:       sshtest.OSRelease{ID: "debian", VersionID: "12"},
			ImageEnv: map[string]string{"PHP_VERSION": "7.4.33"},
			Packages: []sshtest.DepPackage{{Name: "libssl3", Version: sslVersion, SrcName: "openssl", SrcVersion: sslVersion}},
		}}}
	}
	if err := service.RecordDependencies(ctx, d.SQL, h.ID, scan("3.0.11-1~deb12u2")); err != nil {
		t.Fatalf("record: %v", err)
	}
	var fail atomic.Bool
	clients, nvdCalls := fakeAdvisories(t, &fail)
	res, err := service.EvaluateDependencies(ctx, d.SQL, clients)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if res.OSVQueried != 1 || res.OSVVulns != 2 || res.NVDQueried != 1 || res.EOLQueried != 2 || len(res.Errors) != 0 {
		t.Fatalf("result = %+v", res)
	}
	if res.Findings.New != 3 {
		t.Fatalf("findings = %+v; want 2 OSV + 1 NVD", res.Findings)
	}

	rows, _, _ := store.NewDependencyRepo(d.SQL).List(ctx, models.DependencyFilter{Kinds: []string{"os", "runtime", "package"}})
	by := map[string]models.Dependency{}
	for _, r := range rows {
		by[r.Name] = r
	}
	if r := by["libssl3"]; r.CVECount != 2 || r.MaxSeverity != "critical" || r.EvaluatedAt == nil {
		t.Fatalf("libssl3 = %+v", r)
	}
	if r := by["php"]; r.CVECount != 1 || !r.EOL || r.EOLDate != "2022-11-28" {
		t.Fatalf("php = %+v", r)
	}
	if r := by["debian"]; r.EOL || r.EvaluatedAt == nil {
		t.Fatalf("debian = %+v", r)
	}

	// The detail: advisories from the cache, findings linked to the
	// container's service.
	vulns, err := service.DependencyVulns(ctx, d.SQL, by["libssl3"])
	if err != nil || len(vulns) != 2 || vulns[0].ID != "CVE-2024-0727" || vulns[0].Fixed != "3.0.13-1~deb12u1" || vulns[0].OSVID != "DEBIAN-CVE-2024-0727" {
		t.Fatalf("vulns = %+v %v", vulns, err)
	}
	var svcLinks int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM vuln_finding_links l JOIN vuln_findings f ON f.id = l.finding_id
		WHERE f.source = 'inventory' AND l.entity_type = 'service'`).Scan(&svcLinks); err != nil || svcLinks != 3 {
		t.Fatalf("service links = %d %v", svcLinks, err)
	}

	// A second run within a day asks nothing again.
	before := nvdCalls.Load()
	res, err = service.EvaluateDependencies(ctx, d.SQL, clients)
	if err != nil || res.OSVQueried != 0 || res.NVDQueried != 0 || nvdCalls.Load() != before || res.Findings.Updated != 3 {
		t.Fatalf("cached run = %+v %v", res, err)
	}

	// openssl gets patched: the next scan resolves its findings at once,
	// with no network call (the new version simply isn't evaluated yet).
	if err := service.RecordDependencies(ctx, d.SQL, h.ID, scan("3.0.13-1~deb12u1")); err != nil {
		t.Fatalf("record 2: %v", err)
	}
	var open, resolved int
	d.SQL.QueryRow(`SELECT COUNT(*) FILTER (WHERE status = 'open'), COUNT(*) FILTER (WHERE status = 'resolved') FROM vuln_findings WHERE source = 'inventory'`).Scan(&open, &resolved)
	if open != 1 || resolved != 2 {
		t.Fatalf("after patch open=%d resolved=%d; want php open, openssl resolved", open, resolved)
	}

	// Sources down: errors are reported, the cached answers survive.
	fail.Store(true)
	if _, err := d.SQL.Exec(`UPDATE dependency_advisories SET fetched_at = fetched_at - interval '2 days'`); err != nil {
		t.Fatal(err)
	}
	res, err = service.EvaluateDependencies(ctx, d.SQL, clients)
	if err != nil || len(res.Errors) == 0 {
		t.Fatalf("down run = %+v %v", res, err)
	}
	if v, _ := service.DependencyVulns(ctx, d.SQL, by["php"]); len(v) != 1 {
		t.Fatalf("php CVEs lost after a failed refresh: %+v", v)
	}
}

func TestApplyImport_ManyInventoryFindings(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := store.WithSystemScope(context.Background())
	h := &models.Host{Nickname: "big", OficialSlug: "big", Hostname: "10.0.0.2", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	const n = 5000
	fs := make([]models.VulnFinding, n)
	for i := range fs {
		fs[i] = models.VulnFinding{Fingerprint: strings.Repeat("f", 1) + string(rune('a'+i%26)) + itoa(i), Title: "t", Severity: "low", Target: "big",
			Links: []models.VulnLink{{EntityType: "host", EntityID: h.ID}}}
	}
	repo := store.NewVulnRepo(d.SQL)
	res, err := repo.ApplyImport(ctx, "inventory", fs, nil, false)
	if err != nil || res.New != n || res.Unmatched != 0 {
		t.Fatalf("first = %+v %v", res, err)
	}
	res, err = repo.ApplyImport(ctx, "inventory", fs[:n-10], nil, false)
	if err != nil || res.Updated != n-10 || res.Resolved != 10 {
		t.Fatalf("second = %+v %v", res, err)
	}
	res, err = repo.ApplyImport(ctx, "inventory", fs, nil, false)
	if err != nil || res.Reopened != 10 {
		t.Fatalf("third = %+v %v", res, err)
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
