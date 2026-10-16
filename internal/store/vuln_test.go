package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func vf(fp, target, sev string) models.VulnFinding {
	return models.VulnFinding{Fingerprint: fp, Title: "t-" + fp, Severity: sev, Target: target, Scanner: "Nikto"}
}

func vulnByFP(t *testing.T, repo *store.VulnRepo, fp string) models.VulnFinding {
	t.Helper()
	all, _, err := repo.List(context.Background(), models.VulnFilter{PerPage: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all {
		if f.Fingerprint == fp {
			return f
		}
	}
	t.Fatalf("finding %s not found", fp)
	return models.VulnFinding{}
}

func TestVulnRepo_ApplyImportLifecycle(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	repo := store.NewVulnRepo(d.SQL)
	var dnsID int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('wiki.example.test') RETURNING id`).Scan(&dnsID); err != nil {
		t.Fatal(err)
	}
	hostID := seedHost(t, d, "VM-PROXY-01")

	a := vf("a", "wiki.example.test:443", "high")
	a.VMHint = "vm-proxy-01"
	b := vf("b", "wiki.example.test:443", "high")
	c := vf("c", "other.example.test", "high")
	x := vf("x", "absent.example.test", "high") // target the next report won't cover
	crit := vf("k", "wiki.example.test:443", "critical")

	// Dry run writes nothing.
	res, err := repo.ApplyImport(ctx, "etipi", []models.VulnFinding{a, b, c, x, crit}, nil, true)
	if err != nil || res.New != 5 || !res.DryRun {
		t.Fatalf("dry run = %+v, %v", res, err)
	}
	if all, n, _ := repo.List(ctx, models.VulnFilter{}); n != 0 || len(all) != 0 {
		t.Fatalf("dry run persisted %d rows", n)
	}

	res, err = repo.ApplyImport(ctx, "etipi", []models.VulnFinding{a, b, c, x, crit}, nil, false)
	if err != nil || res.New != 5 || res.Unmatched != 2 {
		t.Fatalf("first import = %+v, %v (want 5 new, 2 unmatched: c and x)", res, err)
	}
	fa := vulnByFP(t, repo, "a")
	got, _ := repo.Get(ctx, fa.ID)
	if len(got.Links) != 2 {
		t.Fatalf("a links = %+v, want dns + host (VM hint)", got.Links)
	}

	// Mark b false positive: it must survive re-imports.
	fp := "false_positive"
	fb := vulnByFP(t, repo, "b")
	if err := repo.Update(ctx, fb.ID, store.VulnUpdate{Status: &fp}, 0); err != nil {
		t.Fatal(err)
	}

	// Second (partial) report covers wiki + other, high only: a and b absent.
	cover := &store.VulnCoverage{Targets: []string{"wiki.example.test:443", "other.example.test"}, Severities: []string{"high"}}
	res, err = repo.ApplyImport(ctx, "etipi", []models.VulnFinding{c}, cover, false)
	if err != nil || res.Updated != 1 || res.Resolved != 1 {
		t.Fatalf("second import = %+v, %v (want c updated, a resolved)", res, err)
	}
	if s := vulnByFP(t, repo, "a").Status; s != "resolved" {
		t.Fatalf("a = %s, want resolved", s)
	}
	if s := vulnByFP(t, repo, "b").Status; s != "false_positive" {
		t.Fatalf("b = %s, want false_positive kept", s)
	}
	if s := vulnByFP(t, repo, "x").Status; s != "open" {
		t.Fatalf("x (target not covered) = %s, want open", s)
	}
	if s := vulnByFP(t, repo, "k").Status; s != "open" {
		t.Fatalf("k (severity not covered) = %s, want open", s)
	}

	// a reappears → reopened, resolved_at cleared.
	res, err = repo.ApplyImport(ctx, "etipi", []models.VulnFinding{a, c}, cover, false)
	if err != nil || res.Reopened != 1 {
		t.Fatalf("third import = %+v, %v", res, err)
	}
	if f := vulnByFP(t, repo, "a"); f.Status != "open" || f.ResolvedAt != nil {
		t.Fatalf("a = %s resolved_at=%v, want open", f.Status, f.ResolvedAt)
	}

	// Manual link survives the auto-link rewrite.
	fc := vulnByFP(t, repo, "c")
	if err := repo.AddLink(ctx, fc.ID, "host", hostID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyImport(ctx, "etipi", []models.VulnFinding{a, c}, cover, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, fc.ID); len(got.Links) != 1 || got.Links[0].Origin != "manual" {
		t.Fatalf("c links = %+v, want the manual host link", got.Links)
	}

	// Other sources are never resolved by an ETIPI import.
	if _, err := repo.ApplyImport(ctx, "wazuh", []models.VulnFinding{vf("w", "agent-1", "medium")}, nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyImport(ctx, "etipi", nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if s := vulnByFP(t, repo, "w").Status; s != "open" {
		t.Fatalf("wazuh finding = %s after etipi import, want open", s)
	}
}

// Visibility comes only from stored links; related assets add rows to a tab
// but never make a finding visible.
func TestVulnRepo_Scope(t *testing.T) {
	bg := context.Background()
	d := openDB(t)
	repo := store.NewVulnRepo(d.SQL)
	sga := entidadeID(t, d, "sga")
	etipi := entidadeID(t, d, "etipi")
	grants := store.NewAssetEntidadeRepo(d.SQL)

	mine := seedHost(t, d, "mine")
	theirs := seedHost(t, d, "theirs")
	var dnsID, svcID int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('d.example.test') RETURNING id`).Scan(&dnsID); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO services (nickname) VALUES ('svc') RETURNING id`).Scan(&svcID); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`,
	} {
		if _, err := d.SQL.Exec(q, svcID, theirs); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.SQL.Exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dnsID, mine); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(grants.Replace(bg, d.SQL, store.AssetHost, mine, models.AssetGrants{CreatorEntidadeID: &sga}))
	must(grants.Replace(bg, d.SQL, store.AssetHost, theirs, models.AssetGrants{CreatorEntidadeID: &etipi}))
	must(grants.Replace(bg, d.SQL, store.AssetService, svcID, models.AssetGrants{CreatorEntidadeID: &sga}))
	must(grants.Replace(bg, d.SQL, store.AssetDNS, dnsID, models.AssetGrants{CreatorEntidadeID: &sga}))

	_, err := repo.ApplyImport(bg, "etipi", []models.VulnFinding{
		vf("m", "mine", "high"),           // host "mine"
		vf("t", "theirs", "high"),         // host "theirs"
		vf("u", "nowhere", "high"),        // unmatched
		vf("d", "d.example.test", "high"), // dns, which is linked to host "mine"
	}, nil, false)
	must(err)

	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})
	all, n, err := repo.List(scoped, models.VulnFilter{})
	must(err)
	if n != 2 || len(all) != 2 {
		t.Fatalf("scoped list = %d, want m + d", n)
	}
	if f, _ := repo.Get(scoped, vulnByFP(t, repo, "t").ID); f != nil {
		t.Fatal("finding on an invisible host must be nil (404)")
	}
	if f, _ := repo.Get(scoped, vulnByFP(t, repo, "u").ID); f != nil {
		t.Fatal("unmatched finding must be admin-only")
	}
	if _, n, _ := repo.List(bg, models.VulnFilter{Unmatched: true}); n != 1 {
		t.Fatalf("admin unmatched = %d, want 1", n)
	}

	// Host tab: own link + the DNS record linked to it, marked via.
	tab, _, err := repo.List(scoped, models.VulnFilter{EntityType: "host", EntityID: mine})
	must(err)
	if len(tab) != 2 {
		t.Fatalf("host tab = %+v, want m + d", tab)
	}
	for _, f := range tab {
		if f.Fingerprint == "d" && f.Via != "dns:d.example.test" {
			t.Fatalf("via = %q", f.Via)
		}
	}
	// Service tab: the service relates to host "theirs", which the caller
	// can't see — the relation must not leak t.
	if tab, n, _ := repo.List(scoped, models.VulnFilter{EntityType: "service", EntityID: svcID}); n != 0 {
		t.Fatalf("service tab leaked %+v", tab)
	}
	if _, n, _ := repo.List(bg, models.VulnFilter{EntityType: "service", EntityID: svcID}); n != 1 {
		t.Fatalf("admin service tab = %d, want t via host", n)
	}
}
