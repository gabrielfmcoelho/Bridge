package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A CVE entry is visible to a scoped caller only through a finding they can
// see; admin sees every entry.
func TestCVECatalogRepo(t *testing.T) {
	bg := context.Background()
	d := openDB(t)
	vulns := store.NewVulnRepo(d.SQL)
	repo := store.NewCVECatalogRepo(d.SQL)
	sga := entidadeID(t, d, "sga")
	etipi := entidadeID(t, d, "etipi")
	grants := store.NewAssetEntidadeRepo(d.SQL)
	mine, theirs := seedHost(t, d, "mine"), seedHost(t, d, "theirs")
	if err := grants.Replace(bg, d.SQL, store.AssetHost, mine, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatal(err)
	}
	if err := grants.Replace(bg, d.SQL, store.AssetHost, theirs, models.AssetGrants{CreatorEntidadeID: &etipi}); err != nil {
		t.Fatal(err)
	}

	withCVE := func(fp, target, cve string) models.VulnFinding {
		f := vf(fp, target, "high")
		f.CVE = cve
		return f
	}
	if _, err := vulns.ApplyImport(bg, "etipi", []models.VulnFinding{
		withCVE("m1", "mine", "CVE-2024-0001"),
		withCVE("m2", "mine", "CVE-2024-0001"),
		withCVE("t1", "theirs", "CVE-2024-0002"),
	}, nil, false); err != nil {
		t.Fatal(err)
	}

	cvss := 9.8
	for _, id := range []string{"CVE-2024-0001", "CVE-2024-0002"} {
		if err := repo.Upsert(bg, &models.CVECatalogEntry{CVEID: id, Description: "desc " + id, CVSS: &cvss,
			References: []string{"https://a", "https://b"}, AnalysisPT: "pt", AnalysisEN: "en", Model: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	// Re-analysis replaces in place.
	if err := repo.Upsert(bg, &models.CVECatalogEntry{CVEID: "CVE-2024-0001", Description: "desc", AnalysisPT: "pt2",
		AnalysisEN: "en2", References: []string{}}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(bg, "CVE-2024-0001")
	if err != nil || got == nil {
		t.Fatalf("admin get = %+v, %v", got, err)
	}
	if got.AnalysisPT != "pt2" || got.AnalysisEN != "en2" || len(got.References) != 0 || got.OpenFindings != 2 {
		t.Fatalf("get after re-analysis = %+v", got)
	}
	if all, n, _ := repo.List(bg, "", 1, 50); n != 2 || all[0].AnalysisPT != "" {
		t.Fatalf("admin list = %d %+v, want 2 entries without analysis text", n, all)
	}
	if _, n, _ := repo.List(bg, "0002", 1, 50); n != 1 {
		t.Fatalf("search = %d, want 1", n)
	}

	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})
	if e, _ := repo.Get(scoped, "CVE-2024-0002"); e != nil {
		t.Fatal("CVE only on an invisible host must be nil (404)")
	}
	all, n, err := repo.List(scoped, "", 1, 50)
	if err != nil || n != 1 || all[0].CVEID != "CVE-2024-0001" {
		t.Fatalf("scoped list = %d %+v, %v", n, all, err)
	}
}
