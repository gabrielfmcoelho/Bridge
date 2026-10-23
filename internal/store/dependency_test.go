package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestDependencyRepo_ReplaceAndVisibility(t *testing.T) {
	d := openDB(t)
	ctx := context.Background()
	h := &models.Host{Nickname: "vm-app", OficialSlug: "vm-app", Hostname: "10.0.0.9", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	svc := &models.Service{Nickname: "web"}
	if err := store.NewServiceRepo(d.SQL).Create(ctx, svc); err != nil {
		t.Fatalf("service: %v", err)
	}
	repo := store.NewDependencyRepo(d.SQL)
	row := func(container, kind, name, version string, service *int64) models.Dependency {
		return models.Dependency{ServiceID: service, Container: container, Kind: kind, Name: name, Version: version, VersionSource: models.DepSourceProbe}
	}

	// Scan 1: host + two containers, all probed.
	first := []models.Dependency{
		row("", models.DepKindOS, "ubuntu", "22.04", nil),
		row("", models.DepKindRuntime, "php", "8.1.2", nil),
		row("web", models.DepKindRuntime, "nginx", "1.25.3", &svc.ID),
		row("old", models.DepKindRuntime, "node", "16.20.0", nil),
	}
	if err := repo.ReplaceForHost(ctx, h.ID, first, store.HostScopes{Probed: []string{"", "web", "old"}, Running: []string{"web", "old"}, RunningKnown: true}); err != nil {
		t.Fatalf("replace 1: %v", err)
	}
	// Scan 2: php upgraded on the host, "old" container gone, "web" not
	// probed this time (exec failed) — its rows stay.
	second := []models.Dependency{
		row("", models.DepKindOS, "ubuntu", "22.04", nil),
		row("", models.DepKindRuntime, "php", "8.1.27", nil),
	}
	if err := repo.ReplaceForHost(ctx, h.ID, store.DedupeDependencies(append(second, second...)), store.HostScopes{Probed: []string{""}, Running: []string{"web"}, RunningKnown: true}); err != nil {
		t.Fatalf("replace 2: %v", err)
	}
	all, total, err := repo.List(ctx, models.DependencyFilter{Kinds: []string{"os", "runtime", "package"}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]bool{}
	for _, r := range all {
		got[r.Container+"/"+r.Name+"@"+r.Version] = true
	}
	want := []string{"/ubuntu@22.04", "/php@8.1.27", "web/nginx@1.25.3"}
	if total != len(want) {
		t.Fatalf("rows = %v (total %d); want %v", got, total, want)
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("missing %s in %v", w, got)
		}
	}

	// Visibility: a scoped caller sees the service's rows once the service
	// is granted, the host's rows only once the host is.
	var sga int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug='sga'`).Scan(&sga); err != nil {
		t.Fatalf("sga: %v", err)
	}
	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}})
	if rows, n, err := repo.List(scoped, models.DependencyFilter{}); err != nil || n != 0 || len(rows) != 0 {
		t.Fatalf("ungranted list = %d %v", n, err)
	}
	grants := store.NewAssetEntidadeRepo(d.SQL)
	if err := grants.Replace(ctx, d.SQL, store.AssetService, svc.ID, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant service: %v", err)
	}
	rows, n, err := repo.List(scoped, models.DependencyFilter{})
	if err != nil || n != 1 || rows[0].Container != "web" || rows[0].HostNickname != "vm-app" {
		t.Fatalf("service-granted list = %+v %d %v", rows, n, err)
	}
	if g, err := repo.Get(scoped, rows[0].ID); err != nil || g == nil {
		t.Fatalf("get visible = %+v %v", g, err)
	}
	var hostRow int64
	for _, r := range all {
		if r.Container == "" && r.Name == "php" {
			hostRow = r.ID
		}
	}
	if g, err := repo.Get(scoped, hostRow); err != nil || g != nil {
		t.Fatalf("get host row without host grant = %+v %v; want nil", g, err)
	}
	if err := grants.Replace(ctx, d.SQL, store.AssetHost, h.ID, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant host: %v", err)
	}
	if _, n, _ := repo.List(scoped, models.DependencyFilter{Search: "PHP"}); n != 1 {
		t.Fatalf("host-granted search = %d; want 1", n)
	}
	page, n, err := repo.List(ctx, models.DependencyFilter{Page: 2, PerPage: 2})
	if err != nil || n != 3 || len(page) != 1 {
		t.Fatalf("page 2 = %d rows of %d (%v)", len(page), n, err)
	}
}

func TestDependencyMigration_AcceptsInventorySource(t *testing.T) {
	d := openDB(t)
	var def string
	if err := d.SQL.QueryRow(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'vuln_findings_source_check'`).Scan(&def); err != nil {
		t.Fatalf("constraint: %v", err)
	}
	if !strings.Contains(def, "inventory") || !strings.Contains(def, "wazuh") {
		t.Fatalf("source check = %s", def)
	}
}
