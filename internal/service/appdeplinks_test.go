package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAppDepLinks(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	const base = "https://gl.example"
	insert := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := d.SQL.QueryRow(q+" RETURNING id", args...).Scan(&id); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return id
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Three catalogs: shop-api (g/api, id 10), shop-web (g/web, 11), x (other/x, 12).
	repo := store.NewAppDepRepo(d.SQL)
	cat := func(id int, path, ci string) models.AppDepSource {
		c, err := models.ParseAppDepCatalog([]byte("ci: " + ci + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		return models.AppDepSource{GitLabProjectID: id, ProjectPath: path, Ref: "main", Catalog: c, SyncedAt: time.Now()}
	}
	if _, err := repo.Apply(ctx, []models.AppDepSource{cat(10, "g/api", "shop-api"), cat(11, "g/web", "shop-web"), cat(12, "other/x", "x")}, nil); err != nil {
		t.Fatal(err)
	}

	hRepo := insert(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h-repo','h-repo')`)
	hScan := insert(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h-scan','h-scan')`)
	hGroup := insert(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h-group','h-group')`)
	hOther := insert(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h-other','h-other')`)
	hTrash := insert(`INSERT INTO hosts (nickname, oficial_slug, deleted_at) VALUES ('h-trash','h-trash', now())`)
	dnsID := insert(`INSERT INTO dns_records (domain) VALUES ('web.example.gov.br')`)
	proj := insert(`INSERT INTO projects (name) VALUES ('Loja')`)
	projGroup := insert(`INSERT INTO projects (name) VALUES ('Outros')`)
	svc := insert(`INSERT INTO services (nickname, repository_url) VALUES ('web', 'git@gl.example:G/Web.git')`)

	repoLink := `INSERT INTO gitlab_repo_links (entity_type, entity_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind, source) VALUES (?, ?, ?, ?, ?, ?, ?)`
	exec(repoLink, "host", hRepo, 10, base, "g/api", "project", "manual")
	exec(repoLink, "host", hScan, 0, base, "G/API", "project", "scan")                     // keyed by path, any case
	exec(repoLink, "host", hGroup, 99, base, "g", "group", "manual")                       // every repo under g/
	exec(repoLink, "host", hOther, 10, "https://gitlab.com", "g/api", "project", "manual") // another GitLab: no match
	exec(repoLink, "dns", dnsID, 11, base+"/", "g/web", "project", "manual")
	exec(`INSERT INTO project_gitlab_links (project_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind) VALUES (?, ?, ?, ?, ?)`, proj, 10, base, "g/api", "project")
	exec(`INSERT INTO project_gitlab_links (project_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind) VALUES (?, ?, ?, ?, ?)`, projGroup, 50, base, "other", "group")
	for _, l := range []models.AppDepManualLink{
		{CI: "x", EntityType: "host", EntityID: hRepo},
		{CI: "shop-api", EntityType: "host", EntityID: hRepo}, // also derived: one link, two sources
		{CI: "x", EntityType: "host", EntityID: hTrash},       // trashed: dropped on read
		{CI: "gone", EntityType: "host", EntityID: hRepo},     // no catalog declares it: waits
	} {
		if err := repo.AddLink(ctx, l, 0); err != nil {
			t.Fatal(err)
		}
	}

	links, err := service.AppDepLinks(ctx, d.SQL, base, service.AppDepLinkFilter{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range links {
		got[fmt.Sprintf("%s/%s/%d", l.CI, l.EntityType, l.EntityID)] = strings.Join(l.Sources, ",") + "|" + l.Via
	}
	want := map[string]string{
		fmt.Sprintf("shop-api/host/%d", hRepo):   "manual,repo|g/api",
		fmt.Sprintf("shop-api/host/%d", hScan):   "scan|G/API",
		fmt.Sprintf("shop-api/host/%d", hGroup):  "group|g",
		fmt.Sprintf("shop-web/host/%d", hGroup):  "group|g",
		fmt.Sprintf("shop-web/dns/%d", dnsID):    "repo|g/web",
		fmt.Sprintf("shop-api/project/%d", proj): "repo|g/api",
		fmt.Sprintf("x/project/%d", projGroup):   "group|other",
		fmt.Sprintf("shop-web/service/%d", svc):  "repo|G/Web",
		fmt.Sprintf("x/host/%d", hRepo):          "manual|",
	}
	if len(got) != len(want) {
		t.Errorf("got %d links, want %d:\n%v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	for _, l := range links {
		if l.CI == "shop-api" && l.CIID != 10 || l.CI == "x" && l.CIID != 12 {
			t.Errorf("ci id of %s = %d", l.CI, l.CIID)
		}
	}

	// Filters.
	byHost, _ := service.AppDepLinks(ctx, d.SQL, base, service.AppDepLinkFilter{EntityType: "host", EntityID: hRepo})
	if len(byHost) != 2 || byHost[0].CI != "shop-api" || byHost[1].CI != "x" {
		t.Errorf("by host = %+v", byHost)
	}
	byCI, _ := service.AppDepLinks(ctx, d.SQL, base, service.AppDepLinkFilter{CI: "shop-web"})
	if len(byCI) != 3 {
		t.Errorf("by ci = %+v", byCI)
	}

	// A caller who sees no asset gets no link.
	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{}})
	if none, err := service.AppDepLinks(scoped, d.SQL, base, service.AppDepLinkFilter{}); err != nil || len(none) != 0 {
		t.Errorf("scoped caller: %+v, %v", none, err)
	}

	// Removing the manual link keeps the derived one.
	if ok, err := repo.RemoveLink(ctx, models.AppDepManualLink{CI: "shop-api", EntityType: "host", EntityID: hRepo}); err != nil || !ok {
		t.Fatalf("remove: %v %v", ok, err)
	}
	after, _ := service.AppDepLinks(ctx, d.SQL, base, service.AppDepLinkFilter{CI: "shop-api", EntityType: "host", EntityID: hRepo})
	if len(after) != 1 || strings.Join(after[0].Sources, ",") != "repo" {
		t.Errorf("after remove = %+v", after)
	}
	if ok, _ := repo.RemoveLink(ctx, models.AppDepManualLink{CI: "shop-api", EntityType: "host", EntityID: hRepo}); ok {
		t.Error("removing twice must report nothing removed")
	}
	if ok, _ := repo.EntityLiveVisible(ctx, "host", hTrash); ok {
		t.Error("a trashed host is not linkable")
	}
	if ok, _ := repo.EntityLiveVisible(scoped, "host", hRepo); ok {
		t.Error("an invisible host is not linkable")
	}
}
