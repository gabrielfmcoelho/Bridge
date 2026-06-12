package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newProjectRepo(t *testing.T) (*store.ProjectRepo, *database.DB) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewProjectRepo(d.SQL), d
}

func TestProjectRepo_CRUDRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo, _ := newProjectRepo(t)

	// Create
	p := &models.Project{Name: "Atlas", Description: "infra catalog", Situacao: "active", SetorResponsavel: "eng"}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("create did not set ID")
	}

	// Get
	got, err := repo.Get(ctx, p.ID)
	if err != nil || got == nil {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if got.Name != "Atlas" || got.Description != "infra catalog" || got.SetorResponsavel != "eng" {
		t.Fatalf("get = %+v, want Atlas/infra catalog/eng", got)
	}

	// Get on a missing id returns (nil, nil).
	if got, err := repo.Get(ctx, 9999); got != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", got, err)
	}

	// List
	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Atlas" {
		t.Fatalf("list = %+v, %v", list, err)
	}

	// Count
	if n, err := repo.Count(ctx); err != nil || n != 1 {
		t.Fatalf("count = %d, %v; want 1", n, err)
	}

	// Update
	p.Name = "Atlas v2"
	p.Situacao = "archived"
	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.Get(ctx, p.ID)
	if got.Name != "Atlas v2" || got.Situacao != "archived" {
		t.Fatalf("after update = %+v, want Atlas v2/archived", got)
	}

	// Delete
	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, p.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

// ListFiltered / CountFiltered: server-side search, situacao, tag filters plus
// sort + window. Mirrors the host filter test shape.
func TestProjectRepo_ListFiltered(t *testing.T) {
	ctx := context.Background()
	repo, d := newProjectRepo(t)

	// Seed 4 projects with distinct names / situacao.
	seed := []models.Project{
		{Name: "Alpha", Description: "first project", Situacao: "active", SetorResponsavel: "eng"},
		{Name: "Beta", Description: "second", Situacao: "archived", SetorResponsavel: "ops"},
		{Name: "Gamma", Description: "third alpha-ish", Situacao: "active", SetorResponsavel: "eng"},
		{Name: "Delta", Description: "fourth", Situacao: "active", SetorResponsavel: "sec"},
	}
	for i := range seed {
		if err := repo.Create(ctx, &seed[i]); err != nil {
			t.Fatalf("create %s: %v", seed[i].Name, err)
		}
	}
	// Tag exactly one project (Beta) with 'x'.
	if _, err := d.SQL.Exec(`INSERT INTO tags (entity_type, entity_id, tag) VALUES ('project', ?, 'x')`, seed[1].ID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	names := func(ps []models.Project) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = p.Name
		}
		return out
	}

	// Search matches name OR description (case-insensitive ILIKE). "alpha"
	// hits Alpha (name) and Gamma (description "third alpha-ish").
	got, err := repo.ListFiltered(ctx, models.ProjectFilter{Search: "alpha", SortBy: "name"})
	if err != nil {
		t.Fatalf("ListFiltered(search): %v", err)
	}
	if g := names(got); len(g) != 2 || g[0] != "Alpha" || g[1] != "Gamma" {
		t.Fatalf("search 'alpha' = %v, want [Alpha Gamma]", g)
	}
	if n, err := repo.CountFiltered(ctx, models.ProjectFilter{Search: "alpha"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(search) = %d, %v; want 2", n, err)
	}

	// Situacao filter.
	if n, err := repo.CountFiltered(ctx, models.ProjectFilter{Situacao: "active"}); err != nil || n != 3 {
		t.Fatalf("CountFiltered(situacao=active) = %d, %v; want 3", n, err)
	}

	// Tag filter returns only the tagged project.
	got, err = repo.ListFiltered(ctx, models.ProjectFilter{Tag: "x"})
	if err != nil {
		t.Fatalf("ListFiltered(tag): %v", err)
	}
	if g := names(got); len(g) != 1 || g[0] != "Beta" {
		t.Fatalf("tag 'x' = %v, want [Beta]", g)
	}

	// Sort by name desc.
	got, err = repo.ListFiltered(ctx, models.ProjectFilter{SortBy: "name", SortDir: "desc"})
	if err != nil {
		t.Fatalf("ListFiltered(sort desc): %v", err)
	}
	if g := names(got); len(g) != 4 || g[0] != "Gamma" || g[3] != "Alpha" {
		t.Fatalf("sort name desc = %v, want [Gamma Delta Beta Alpha]", g)
	}

	// Pagination: name asc → [Alpha Beta Delta Gamma]; PerPage=2 Page=2 → [Delta Gamma].
	got, err = repo.ListFiltered(ctx, models.ProjectFilter{SortBy: "name", PerPage: 2, Page: 2})
	if err != nil {
		t.Fatalf("ListFiltered(page2): %v", err)
	}
	if g := names(got); len(g) != 2 || g[0] != "Delta" || g[1] != "Gamma" {
		t.Fatalf("page2 = %v, want [Delta Gamma]", g)
	}
	// CountFiltered ignores the window: full match count.
	if n, err := repo.CountFiltered(ctx, models.ProjectFilter{SortBy: "name", PerPage: 2, Page: 2}); err != nil || n != 4 {
		t.Fatalf("CountFiltered(page2) = %d, %v; want 4", n, err)
	}
}

// Host-link round trip. This also guards against the column-count bug in the
// legacy models.ListProjectsByHost (SELECT had 14 cols, Scan read 18): the
// repo's ProjectsByHost must return fully-populated project rows.
func TestProjectRepo_HostLinks(t *testing.T) {
	ctx := context.Background()
	repo, d := newProjectRepo(t)

	// project_host_links.host_id FKs hosts; seed one host.
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed hosts: %v", err)
	}

	p := &models.Project{Name: "Linked", Description: "has a host"}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.SetProjectsForHost(ctx, hostID, []int64{p.ID}); err != nil {
		t.Fatalf("set links: %v", err)
	}

	// project -> hosts
	hostIDs, err := repo.HostIDs(ctx, p.ID)
	if err != nil || len(hostIDs) != 1 || hostIDs[0] != hostID {
		t.Fatalf("HostIDs = %v, %v; want [%d]", hostIDs, err, hostID)
	}

	// host -> projects, fully populated (the bug-catch: Description must survive).
	projects, err := repo.ProjectsByHost(ctx, hostID)
	if err != nil {
		t.Fatalf("ProjectsByHost: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != p.ID || projects[0].Description != "has a host" {
		t.Fatalf("ProjectsByHost = %+v, want one fully-populated Linked", projects)
	}

	// Re-setting links replaces (not appends): empty slice clears.
	if err := repo.SetProjectsForHost(ctx, hostID, nil); err != nil {
		t.Fatalf("clear links: %v", err)
	}
	if projects, _ := repo.ProjectsByHost(ctx, hostID); len(projects) != 0 {
		t.Fatalf("after clear = %+v, want none", projects)
	}
}
