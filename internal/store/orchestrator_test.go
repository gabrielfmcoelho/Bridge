package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestOrchestratorRepo_CRUDRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	// orchestrators.host_id is a FK to hosts; seed a minimal host.
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1', 'h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}

	repo := store.NewOrchestratorRepo(d.SQL)

	if n, _ := repo.Count(ctx); n != 0 {
		t.Fatalf("count on empty = %d, want 0", n)
	}

	o := &models.Orchestrator{HostID: hostID, Type: "coolify", Version: "4", Observacoes: "x"}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("create: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("create did not set ID")
	}

	byHost, err := repo.GetByHost(ctx, hostID)
	if err != nil || byHost == nil || byHost.Type != "coolify" {
		t.Fatalf("GetByHost = %+v, %v", byHost, err)
	}

	o.Version = "4.1"
	if err := repo.Update(ctx, o); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.Get(ctx, o.ID)
	if got == nil || got.Version != "4.1" {
		t.Fatalf("after update = %+v, want version 4.1", got)
	}

	if n, _ := repo.Count(ctx); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}

	if err := repo.Delete(ctx, o.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n, _ := repo.Count(ctx); n != 0 {
		t.Fatalf("count after delete = %d, want 0", n)
	}

	// Get on a missing id returns (nil, nil), not an error.
	if got, err := repo.Get(ctx, 9999); got != nil || err != nil {
		t.Fatalf("Get(missing) = %+v, %v; want nil, nil", got, err)
	}
}
