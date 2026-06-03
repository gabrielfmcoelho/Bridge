package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostAlertRepo_CRUDResolveAndExternalUpsert(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	hres, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h')`)
	hostID, _ := hres.LastInsertId()
	repo := store.NewHostAlertRepo(d.SQL)

	a := &models.HostAlert{HostID: hostID, Type: "resource_cpu", Level: "warning", Message: "high"}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Source != "manual" {
		t.Fatalf("source = %q, want manual default", a.Source)
	}

	got, _ := repo.Get(ctx, a.ID)
	if got == nil || got.Message != "high" {
		t.Fatalf("get = %+v", got)
	}

	a.Message = "very high"
	if err := repo.Update(ctx, a); err != nil {
		t.Fatalf("update: %v", err)
	}
	if l, _ := repo.ListByHost(ctx, hostID); len(l) != 1 || l[0].Message != "very high" {
		t.Fatalf("listbyhost = %+v", l)
	}

	if err := repo.Resolve(ctx, a.ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got, _ = repo.Get(ctx, a.ID)
	if got.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", got.Status)
	}

	// UpsertExternal: insert then update by (source, external_id) — no dup.
	ext := &models.HostAlert{HostID: hostID, Type: "grafana", Level: "critical", Message: "m1", Source: "grafana", Status: "active", ExternalSource: "grafana", ExternalID: "G-1"}
	r1, err := repo.UpsertExternal(ctx, ext)
	if err != nil || r1.ID == 0 {
		t.Fatalf("upsert insert = %+v, %v", r1, err)
	}
	ext.Message = "m2"
	r2, err := repo.UpsertExternal(ctx, ext)
	if err != nil || r2.ID != r1.ID {
		t.Fatalf("upsert update id = %d, want %d (%v)", r2.ID, r1.ID, err)
	}
	gotExt, _ := repo.GetExternal(ctx, "grafana", "G-1")
	if gotExt == nil || gotExt.Message != "m2" {
		t.Fatalf("external = %+v, want m2", gotExt)
	}

	bulk, _ := repo.ListBulk(ctx)
	if len(bulk[hostID]) != 2 {
		t.Fatalf("bulk[host] = %d alerts, want 2", len(bulk[hostID]))
	}

	if err := repo.Delete(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
