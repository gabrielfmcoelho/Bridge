package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// service_kind: written by the scan, backfilled for older rows, filterable;
// and source can't be rewritten through Update.
func TestServiceRepo_Kind(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}

	pg := sshtest.DiscoveredService{Name: "postgresql", Label: "PostgreSQL", Kind: "database", HostRunning: true}
	if err := repo.ReconcileDiscovered(ctx, hostID, store.DiscoveredInventory{
		ContainersKnown: true,
		Containers:      []sshtest.ContainerInfo{{ID: "c1", Name: "a1rwyy-1433", Image: "a1rwyy:d7fff1"}},
		Services:        []sshtest.DiscoveredService{pg},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	kinds := map[string]string{}
	svcs, err := repo.ListDiscoveredByHost(ctx, hostID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range svcs {
		kinds[s.DiscoveryKey] = s.ServiceKind
	}
	if kinds["postgresql"] != "database" || kinds["a1rwyy-1433"] != "app" {
		t.Fatalf("scan kinds = %v, want postgresql=database, unknown image=app", kinds)
	}

	// A row from before v87: scan-owned, unclassified → BackfillKinds fills it.
	var oldID int64
	if err := d.SQL.QueryRow(`INSERT INTO services (nickname, source, discovery_kind, discovery_key, container_image)
		VALUES ('old', 'auto', 'container', 'r1', 'redis:7') RETURNING id`).Scan(&oldID); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	n, err := repo.BackfillKinds(ctx)
	if err != nil || n != 1 {
		t.Fatalf("BackfillKinds = %d, %v; want 1", n, err)
	}
	old, _ := repo.Get(ctx, oldID)
	if want := sshtest.InferFromImage("redis:7", "r1").Kind; old.ServiceKind != want || want == "" {
		t.Fatalf("backfilled kind = %q, want %q", old.ServiceKind, want)
	}
	if n, _ := repo.BackfillKinds(ctx); n != 0 {
		t.Fatalf("second BackfillKinds = %d, want 0 (idempotent)", n)
	}

	// Filters: kind, source, discovery kind.
	manual := &models.Service{Nickname: "m", ServiceKind: "queue"}
	if err := repo.Create(ctx, manual); err != nil {
		t.Fatalf("create manual: %v", err)
	}
	count := func(f models.ServiceFilter) int {
		t.Helper()
		c, err := repo.CountFiltered(ctx, f)
		if err != nil {
			t.Fatalf("count %+v: %v", f, err)
		}
		return c
	}
	if c := count(models.ServiceFilter{Kind: "database"}); c != 1 {
		t.Fatalf("kind=database → %d, want 1", c)
	}
	if c := count(models.ServiceFilter{Source: "manual"}); c != 1 {
		t.Fatalf("source=manual → %d, want 1", c)
	}
	if c := count(models.ServiceFilter{Source: "auto", DiscoveryKind: "container"}); c != 2 {
		t.Fatalf("auto containers → %d, want 2", c)
	}

	// Update can edit the kind but not the source.
	manual.Source = "auto"
	manual.ServiceKind = "cache"
	if err := repo.Update(ctx, manual); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.Get(ctx, manual.ID)
	if got.Source != "manual" || got.ServiceKind != "cache" {
		t.Fatalf("after update source=%q kind=%q, want manual/cache", got.Source, got.ServiceKind)
	}
}
