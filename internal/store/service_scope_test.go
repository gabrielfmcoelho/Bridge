package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Entidade scoping on ServiceRepo: a scoped ctx sees only granted services;
// an unscoped ctx sees everything.
func TestServiceRepo_EntidadeScope(t *testing.T) {
	d := openDB(t)
	bg := context.Background()
	repo := store.NewServiceRepo(d.SQL)
	sga := entidadeID(t, d, "sga")
	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})

	mine := &models.Service{Nickname: "mine"}
	other := &models.Service{Nickname: "other"}
	for _, s := range []*models.Service{mine, other} {
		if err := repo.Create(bg, s); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(bg, d.SQL, store.AssetService, mine.ID, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	// Unscoped: both.
	if list, err := repo.List(bg); err != nil || len(list) != 2 {
		t.Fatalf("unscoped list = %d, %v; want 2", len(list), err)
	}
	if n, _ := repo.Count(bg); n != 2 {
		t.Fatalf("unscoped count = %d, want 2", n)
	}

	// Scoped: only the granted one, across every read path.
	if list, err := repo.List(scoped); err != nil || len(list) != 1 || list[0].ID != mine.ID {
		t.Fatalf("scoped list = %+v, %v; want only mine", list, err)
	}
	if list, err := repo.ListFiltered(scoped, models.ServiceFilter{}); err != nil || len(list) != 1 {
		t.Fatalf("scoped ListFiltered = %d, %v; want 1", len(list), err)
	}
	if n, err := repo.CountFiltered(scoped, models.ServiceFilter{}); err != nil || n != 1 {
		t.Fatalf("scoped CountFiltered = %d, %v; want 1", n, err)
	}
	if n, _ := repo.Count(scoped); n != 1 {
		t.Fatalf("scoped count = %d, want 1", n)
	}
	if got, err := repo.Get(scoped, mine.ID); err != nil || got == nil {
		t.Fatalf("scoped get(mine) = %+v, %v", got, err)
	}
	if got, err := repo.Get(scoped, other.ID); err != nil || got != nil {
		t.Fatalf("scoped get(other) = %+v, %v; want nil,nil", got, err)
	}

	// Scoped Update/Delete on the invisible row: no effect.
	other.Nickname = "renamed"
	if err := repo.Update(scoped, other); err != nil {
		t.Fatalf("scoped update: %v", err)
	}
	if got, _ := repo.Get(bg, other.ID); got.Nickname != "other" {
		t.Fatalf("scoped update leaked: %+v", got)
	}
	if err := repo.Delete(scoped, other.ID); err != nil {
		t.Fatalf("scoped delete: %v", err)
	}
	if got, _ := repo.Get(bg, other.ID); got == nil {
		t.Fatal("scoped delete leaked")
	}
	if err := repo.Delete(bg, other.ID); err != nil {
		t.Fatalf("unscoped delete: %v", err)
	}
	if got, _ := repo.Get(bg, other.ID); got != nil {
		t.Fatalf("unscoped delete had no effect: %+v", got)
	}
}

// ReconcileDiscovered copies the host's grants onto each newly created
// auto-discovered service, so scan-born services are visible to whoever sees
// the host.
func TestServiceRepo_ReconcileDiscovered_CopiesHostGrants(t *testing.T) {
	d := openDB(t)
	bg := context.Background()
	repo := store.NewServiceRepo(d.SQL)
	sga := entidadeID(t, d, "sga")
	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})

	hostID := seedHost(t, d, "h-scan")
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(bg, d.SQL, store.AssetHost, hostID, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant host: %v", err)
	}
	inv := store.DiscoveredInventory{
		Containers:      []sshtest.ContainerInfo{{ID: "c1", Name: "web", Image: "nginx:latest"}},
		ContainersKnown: true,
	}
	if err := repo.ReconcileDiscovered(bg, hostID, inv); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	svcs, err := repo.ListByHost(scoped, hostID)
	if err != nil || len(svcs) != 1 || svcs[0].DiscoveryKey != "web" {
		t.Fatalf("scoped ListByHost = %+v, %v; want the discovered service", svcs, err)
	}
	g, err := store.NewAssetEntidadeRepo(d.SQL).Get(bg, store.AssetService, svcs[0].ID)
	if err != nil || g.CreatorEntidadeID == nil || *g.CreatorEntidadeID != sga {
		t.Fatalf("copied grants = %+v, %v; want creator=sga", g, err)
	}
}
