package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newServiceRepo(t *testing.T) (*store.ServiceRepo, *database.DB) {
	t.Helper()
	d, err := dbtest.Open(t)
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

// ListFiltered / CountFiltered: server-side search, tag, developed_by, and the
// tri-state boolean filters (is_external_dependency / orchestrator_managed) plus
// sort + window. Mirrors the project filter test shape.
func TestServiceRepo_ListFiltered(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)

	// Seed 4 services with varied nickname / technology_stack and booleans.
	seed := []models.Service{
		{Nickname: "alpha-api", Description: "first", TechnologyStack: "go", DevelopedBy: "team-a", IsExternalDependency: true, OrchestratorManaged: true},
		{Nickname: "beta-worker", Description: "second alpha-ish", TechnologyStack: "python", DevelopedBy: "team-b", IsExternalDependency: false, OrchestratorManaged: false},
		{Nickname: "gamma-db", Description: "third", TechnologyStack: "postgres", DevelopedBy: "team-a", IsExternalDependency: true, OrchestratorManaged: false},
		{Nickname: "delta-cache", Description: "fourth", TechnologyStack: "redis", DevelopedBy: "team-c", IsExternalDependency: false, OrchestratorManaged: true},
	}
	for i := range seed {
		if err := repo.Create(ctx, &seed[i]); err != nil {
			t.Fatalf("create %s: %v", seed[i].Nickname, err)
		}
	}
	// Tag exactly one service (beta-worker) with 'x'.
	if _, err := d.SQL.Exec(`INSERT INTO tags (entity_type, entity_id, tag) VALUES ('service', ?, 'x')`, seed[1].ID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	nicks := func(ss []models.Service) []string {
		out := make([]string, len(ss))
		for i, s := range ss {
			out[i] = s.Nickname
		}
		return out
	}

	// Search matches nickname OR description OR technology_stack (ILIKE). "alpha"
	// hits alpha-api (nickname) and beta-worker (description "second alpha-ish").
	got, err := repo.ListFiltered(ctx, models.ServiceFilter{Search: "alpha", SortBy: "nickname"})
	if err != nil {
		t.Fatalf("ListFiltered(search): %v", err)
	}
	if g := nicks(got); len(g) != 2 || g[0] != "alpha-api" || g[1] != "beta-worker" {
		t.Fatalf("search 'alpha' = %v, want [alpha-api beta-worker]", g)
	}
	if n, err := repo.CountFiltered(ctx, models.ServiceFilter{Search: "alpha"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(search) = %d, %v; want 2", n, err)
	}

	// Tag filter returns only the tagged service.
	got, err = repo.ListFiltered(ctx, models.ServiceFilter{Tag: "x"})
	if err != nil {
		t.Fatalf("ListFiltered(tag): %v", err)
	}
	if g := nicks(got); len(g) != 1 || g[0] != "beta-worker" {
		t.Fatalf("tag 'x' = %v, want [beta-worker]", g)
	}

	// DevelopedBy filter.
	if n, err := repo.CountFiltered(ctx, models.ServiceFilter{DevelopedBy: "team-a"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(developed_by=team-a) = %d, %v; want 2", n, err)
	}

	// IsExternalDependency tri-state: yes (alpha-api, gamma-db) / no (beta-worker, delta-cache).
	if n, err := repo.CountFiltered(ctx, models.ServiceFilter{IsExternalDependency: "yes"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(is_external=yes) = %d, %v; want 2", n, err)
	}
	if n, err := repo.CountFiltered(ctx, models.ServiceFilter{IsExternalDependency: "no"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(is_external=no) = %d, %v; want 2", n, err)
	}

	// OrchestratorManaged tri-state: yes (alpha-api, delta-cache) / no (beta-worker, gamma-db).
	got, err = repo.ListFiltered(ctx, models.ServiceFilter{OrchestratorManaged: "yes", SortBy: "nickname"})
	if err != nil {
		t.Fatalf("ListFiltered(orchestrator=yes): %v", err)
	}
	if g := nicks(got); len(g) != 2 || g[0] != "alpha-api" || g[1] != "delta-cache" {
		t.Fatalf("orchestrator=yes = %v, want [alpha-api delta-cache]", g)
	}
	if n, err := repo.CountFiltered(ctx, models.ServiceFilter{OrchestratorManaged: "no"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(orchestrator=no) = %d, %v; want 2", n, err)
	}

	// Sort by nickname desc → [gamma-db delta-cache beta-worker alpha-api].
	got, err = repo.ListFiltered(ctx, models.ServiceFilter{SortBy: "nickname", SortDir: "desc"})
	if err != nil {
		t.Fatalf("ListFiltered(sort desc): %v", err)
	}
	if g := nicks(got); len(g) != 4 || g[0] != "gamma-db" || g[3] != "alpha-api" {
		t.Fatalf("sort nickname desc = %v, want [gamma-db delta-cache beta-worker alpha-api]", g)
	}

	// Pagination: nickname asc → [alpha-api beta-worker delta-cache gamma-db];
	// PerPage=2 Page=2 → [delta-cache gamma-db].
	got, err = repo.ListFiltered(ctx, models.ServiceFilter{SortBy: "nickname", PerPage: 2, Page: 2})
	if err != nil {
		t.Fatalf("ListFiltered(page2): %v", err)
	}
	if g := nicks(got); len(g) != 2 || g[0] != "delta-cache" || g[1] != "gamma-db" {
		t.Fatalf("page2 = %v, want [delta-cache gamma-db]", g)
	}
	// CountFiltered ignores the window: full match count.
	if n, err := repo.CountFiltered(ctx, models.ServiceFilter{SortBy: "nickname", PerPage: 2, Page: 2}); err != nil || n != 4 {
		t.Fatalf("CountFiltered(page2) = %d, %v; want 4", n, err)
	}
}

func TestServiceRepo_LinksAndDeps(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)

	var projID int64
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('proj') RETURNING id`).Scan(&projID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	var dnsID int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('x.com') RETURNING id`).Scan(&dnsID); err != nil {
		t.Fatalf("seed dns_record: %v", err)
	}

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

func TestServiceRepo_ReconcileDiscovered(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	byKey := func(t *testing.T) map[string]models.Service {
		t.Helper()
		svcs, err := repo.ListDiscoveredByHost(ctx, hostID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		m := map[string]models.Service{}
		for _, s := range svcs {
			m[s.DiscoveryKind+"/"+s.DiscoveryKey] = s
		}
		return m
	}

	// First sweep: one container + one host service → two online auto services.
	c := sshtest.ContainerInfo{ID: "c1", Name: "web", Image: "nginx:latest", Ports: "0.0.0.0:8080->80/tcp"}
	pg := sshtest.DiscoveredService{
		Name: "postgresql", Label: "PostgreSQL", Kind: "database",
		Unit: "postgresql", State: "active", Version: "15.4",
		Ports: []int{5432}, HostPorts: []int{5432}, HostRunning: true,
		Sources: []string{"systemd", "package", "port"},
	}
	inv := store.DiscoveredInventory{
		Containers: []sshtest.ContainerInfo{c}, ContainersKnown: true,
		Services: []sshtest.DiscoveredService{pg},
	}
	if err := repo.ReconcileDiscovered(ctx, hostID, inv); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	got := byKey(t)
	if len(got) != 2 {
		t.Fatalf("after reconcile 1 = %+v, want 2 rows", got)
	}
	if s := got["container/web"]; s.Source != "auto" || s.ContainerStatus != "online" || s.Port != "8080" {
		t.Fatalf("container row = %+v (want auto/online/8080)", s)
	}
	if s := got["host/postgresql"]; s.Source != "auto" || s.ContainerStatus != "online" ||
		s.Port != "5432" || s.ServiceType != "database" || s.Version != "15.4" || s.ContainerName != "" {
		t.Fatalf("host row = %+v (want auto/online/5432/database/15.4, no container)", s)
	}

	// Second sweep: container renamed (same engine ID) → row moves, no duplicate.
	c.Name = "web-1"
	c.Image = "nginx:1.27"
	inv.Containers = []sshtest.ContainerInfo{c}
	if err := repo.ReconcileDiscovered(ctx, hostID, inv); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	got = byKey(t)
	if len(got) != 2 {
		t.Fatalf("after rename = %+v, want 2 rows (no orphan)", got)
	}
	if s := got["container/web-1"]; s.ContainerName != "web-1" || s.ContainerImage != "nginx:1.27" || s.ContainerStatus != "online" {
		t.Fatalf("renamed row = %+v", s)
	}

	// Third sweep: docker unreadable → container row must NOT go offline, but
	// the host service (still reported) stays online.
	if err := repo.ReconcileDiscovered(ctx, hostID, store.DiscoveredInventory{
		ContainersKnown: false,
		Services:        []sshtest.DiscoveredService{pg},
	}); err != nil {
		t.Fatalf("reconcile 3: %v", err)
	}
	got = byKey(t)
	if s := got["container/web-1"]; s.ContainerStatus != "online" {
		t.Fatalf("docker-unreadable sweep offlined the container row: %+v", s)
	}

	// Fourth sweep: docker answered with nothing and postgres stopped → both
	// go offline. This is the sweep the old container-count guard could never
	// reach.
	if err := repo.ReconcileDiscovered(ctx, hostID, store.DiscoveredInventory{ContainersKnown: true}); err != nil {
		t.Fatalf("reconcile 4: %v", err)
	}
	got = byKey(t)
	for k, s := range got {
		if s.ContainerStatus != "offline" {
			t.Fatalf("row %s = %+v, want offline", k, s)
		}
	}
}

// Only catalog hits with a live host-side instance become host rows: the scan
// sets HostRunning after attributing every process by cgroup, so a container's
// postgres seen through the shared PID namespace never lands here.
func TestServiceRepo_ReconcileDiscovered_SkipsNonLiveHostServices(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h2','h2') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	if err := repo.ReconcileDiscovered(ctx, hostID, store.DiscoveredInventory{
		ContainersKnown: true,
		Services: []sshtest.DiscoveredService{
			// installed but never started
			{Name: "redis", Label: "Redis", State: "stopped", Sources: []string{"package"}},
			// unit file present, not loaded
			{Name: "nginx", Label: "Nginx", Sources: []string{"systemd"}},
			// running, but every process and port belongs to a container
			{Name: "mongodb", Label: "MongoDB", State: "running", Ports: []int{27017},
				ContainerID: "abc", Sources: []string{"container"}},
		},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	svcs, _ := repo.ListDiscoveredByHost(ctx, hostID)
	if len(svcs) != 0 {
		t.Fatalf("created %d rows, want 0: %+v", len(svcs), svcs)
	}
}

// The case per-PID attribution exists for: a native database and a
// containerized one on the same host produce two independent rows.
func TestServiceRepo_ReconcileDiscovered_NativeAndContainerCoexist(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h3','h3') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	if err := repo.ReconcileDiscovered(ctx, hostID, store.DiscoveredInventory{
		ContainersKnown: true,
		Containers: []sshtest.ContainerInfo{
			{ID: "c1", Name: "db", Image: "postgres:16", Ports: "0.0.0.0:15432->5432/tcp"},
		},
		Services: []sshtest.DiscoveredService{{
			Name: "postgresql", Label: "PostgreSQL", Unit: "postgresql", State: "active",
			Ports: []int{5432, 15432}, HostPorts: []int{5432}, HostRunning: true,
			ContainerID: "c1", Sources: []string{"systemd", "process", "port", "container"},
		}},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	svcs, _ := repo.ListDiscoveredByHost(ctx, hostID)
	if len(svcs) != 2 {
		t.Fatalf("got %d rows, want 2 (native + container): %+v", len(svcs), svcs)
	}
	for _, s := range svcs {
		switch s.DiscoveryKind {
		case "host":
			if s.Port != "5432" || s.ContainerName != "" {
				t.Errorf("host row = %+v, want port 5432 and no container", s)
			}
		case "container":
			if s.Port != "15432" || s.ContainerName != "db" {
				t.Errorf("container row = %+v, want port 15432 on container db", s)
			}
		default:
			t.Errorf("unexpected discovery_kind %q", s.DiscoveryKind)
		}
	}
}
