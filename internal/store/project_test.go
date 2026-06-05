package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newProjectRepo(t *testing.T) (*store.ProjectRepo, *database.DB) {
	t.Helper()
	d, err := database.Open(t.TempDir())
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

// Host-link round trip. This also guards against the column-count bug in the
// legacy models.ListProjectsByHost (SELECT had 14 cols, Scan read 18): the
// repo's ProjectsByHost must return fully-populated project rows.
func TestProjectRepo_HostLinks(t *testing.T) {
	ctx := context.Background()
	repo, d := newProjectRepo(t)

	// project_host_links.host_id FKs hosts; seed one host.
	hr, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1')`)
	hostID, _ := hr.LastInsertId()

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
