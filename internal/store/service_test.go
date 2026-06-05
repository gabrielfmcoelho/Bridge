package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newServiceRepo(t *testing.T) (*store.ServiceRepo, *database.DB) {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewServiceRepo(d.SQL), d
}

func TestServiceRepo_CRUDRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo, _ := newServiceRepo(t)

	// Create defaults Source to "manual".
	s := &models.Service{Nickname: "api", Description: "the api", Environment: "prod"}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.ID == 0 {
		t.Fatal("create did not set ID")
	}
	if s.Source != "manual" {
		t.Fatalf("source = %q, want manual default", s.Source)
	}

	got, err := repo.Get(ctx, s.ID)
	if err != nil || got == nil || got.Nickname != "api" || got.Environment != "prod" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if got, err := repo.Get(ctx, 9999); got != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", got, err)
	}

	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 || list[0].Nickname != "api" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if n, err := repo.Count(ctx); err != nil || n != 1 {
		t.Fatalf("count = %d, %v; want 1", n, err)
	}

	s.Nickname = "api-v2"
	s.Environment = "staging"
	if err := repo.Update(ctx, s); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.Get(ctx, s.ID)
	if got.Nickname != "api-v2" || got.Environment != "staging" {
		t.Fatalf("after update = %+v", got)
	}

	if err := repo.Delete(ctx, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, s.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestServiceRepo_LinksAndDeps(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)

	pr, _ := d.SQL.Exec(`INSERT INTO projects (name) VALUES ('proj')`)
	projID, _ := pr.LastInsertId()
	hr, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1')`)
	hostID, _ := hr.LastInsertId()
	dr, _ := d.SQL.Exec(`INSERT INTO dns_records (domain) VALUES ('x.com')`)
	dnsID, _ := dr.LastInsertId()

	s := &models.Service{Nickname: "svc", ProjectID: &projID}
	dep := &models.Service{Nickname: "dep"}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("create s: %v", err)
	}
	if err := repo.Create(ctx, dep); err != nil {
		t.Fatalf("create dep: %v", err)
	}

	// ListByProject
	byProj, err := repo.ListByProject(ctx, projID)
	if err != nil || len(byProj) != 1 || byProj[0].ID != s.ID {
		t.Fatalf("ListByProject = %+v, %v", byProj, err)
	}

	// Host links (service -> host, and host -> services).
	if err := repo.SetHostLinks(ctx, s.ID, []int64{hostID}); err != nil {
		t.Fatalf("SetHostLinks: %v", err)
	}
	if ids, _ := repo.HostIDs(ctx, s.ID); len(ids) != 1 || ids[0] != hostID {
		t.Fatalf("HostIDs = %v", ids)
	}
	if byHost, _ := repo.ListByHost(ctx, hostID); len(byHost) != 1 || byHost[0].ID != s.ID {
		t.Fatalf("ListByHost = %+v", byHost)
	}

	// DNS links.
	if err := repo.SetDNSLinks(ctx, s.ID, []int64{dnsID}); err != nil {
		t.Fatalf("SetDNSLinks: %v", err)
	}
	if ids, _ := repo.DNSIDs(ctx, s.ID); len(ids) != 1 || ids[0] != dnsID {
		t.Fatalf("DNSIDs = %v", ids)
	}

	// Dependencies (s depends on dep): forward + reverse.
	if err := repo.SetDependencies(ctx, s.ID, []int64{dep.ID}); err != nil {
		t.Fatalf("SetDependencies: %v", err)
	}
	if ids, _ := repo.DependencyIDs(ctx, s.ID); len(ids) != 1 || ids[0] != dep.ID {
		t.Fatalf("DependencyIDs = %v", ids)
	}
	if ids, _ := repo.DependentIDs(ctx, dep.ID); len(ids) != 1 || ids[0] != s.ID {
		t.Fatalf("DependentIDs = %v", ids)
	}

	// Counts by host (service + project).
	if m, _ := repo.CountsByHost(ctx); m[hostID] != 1 {
		t.Fatalf("CountsByHost[%d] = %d, want 1", hostID, m[hostID])
	}
	if m, _ := repo.ProjectCountsByHost(ctx); m[hostID] != 1 {
		t.Fatalf("ProjectCountsByHost[%d] = %d, want 1 (via service)", hostID, m[hostID])
	}

	// SetServicesForHost replaces the host's service set.
	if err := repo.SetServicesForHost(ctx, hostID, []int64{dep.ID}); err != nil {
		t.Fatalf("SetServicesForHost: %v", err)
	}
	if byHost, _ := repo.ListByHost(ctx, hostID); len(byHost) != 1 || byHost[0].ID != dep.ID {
		t.Fatalf("after SetServicesForHost = %+v", byHost)
	}
}

func TestServiceRepo_FixateAndContainerBinding(t *testing.T) {
	ctx := context.Background()
	repo, _ := newServiceRepo(t)

	auto := &models.Service{Nickname: "auto", Source: "auto"}
	if err := repo.Create(ctx, auto); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Fixate(ctx, auto.ID); err != nil {
		t.Fatalf("fixate: %v", err)
	}
	got, _ := repo.Get(ctx, auto.ID)
	if got.Source != "fixed" {
		t.Fatalf("source after fixate = %q, want fixed", got.Source)
	}

	if err := repo.UpdateContainerBinding(ctx, auto.ID, "newname", "newid"); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	got, _ = repo.Get(ctx, auto.ID)
	if got.ContainerName != "newname" || got.ContainerID != "newid" {
		t.Fatalf("after rebind = %+v", got)
	}
}

func TestServiceRepo_ReconcileContainers(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	hr, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1')`)
	hostID, _ := hr.LastInsertId()

	// First sweep: one container → creates an online auto service linked to host.
	c := sshtest.ContainerInfo{ID: "c1", Name: "web", Image: "nginx:latest", Ports: "0.0.0.0:8080->80/tcp"}
	if err := repo.ReconcileContainers(ctx, hostID, []sshtest.ContainerInfo{c}); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	svcs, _ := repo.ListContainerServicesByHost(ctx, hostID)
	if len(svcs) != 1 || svcs[0].ContainerName != "web" || svcs[0].ContainerStatus != "online" {
		t.Fatalf("after reconcile 1 = %+v", svcs)
	}
	if svcs[0].Source != "auto" || svcs[0].Port != "8080" {
		t.Fatalf("auto service shape = %+v (want source=auto port=8080)", svcs[0])
	}

	// Second sweep: same container, new image → updates in place (no new row).
	c.Image = "nginx:1.27"
	if err := repo.ReconcileContainers(ctx, hostID, []sshtest.ContainerInfo{c}); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	svcs, _ = repo.ListContainerServicesByHost(ctx, hostID)
	if len(svcs) != 1 || svcs[0].ContainerImage != "nginx:1.27" {
		t.Fatalf("after reconcile 2 = %+v", svcs)
	}

	// Third sweep: empty → the unseen service is marked offline.
	if err := repo.ReconcileContainers(ctx, hostID, nil); err != nil {
		t.Fatalf("reconcile 3: %v", err)
	}
	svcs, _ = repo.ListContainerServicesByHost(ctx, hostID)
	if len(svcs) != 1 || svcs[0].ContainerStatus != "offline" {
		t.Fatalf("after reconcile 3 = %+v, want offline", svcs)
	}
}
