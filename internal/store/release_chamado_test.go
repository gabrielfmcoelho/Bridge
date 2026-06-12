package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestReleaseRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := store.NewReleaseRepo(openDB(t).SQL)

	rel := &models.Release{Title: "v1", Description: "first", Status: "pending"}
	if err := repo.Create(ctx, rel); err != nil {
		t.Fatalf("create: %v", err)
	}
	if rel.ID == 0 {
		t.Fatal("create did not set ID")
	}

	got, err := repo.Get(ctx, rel.ID)
	if err != nil || got == nil || got.Title != "v1" {
		t.Fatalf("get = %+v, %v", got, err)
	}

	// No linked issues yet.
	ids, err := repo.IssueIDs(ctx, rel.ID)
	if err != nil || len(ids) != 0 {
		t.Fatalf("issueIDs = %+v, %v, want empty", ids, err)
	}

	rel.Status = "live"
	if err := repo.Update(ctx, rel); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, _ := repo.List(ctx)
	if len(list) != 1 || list[0].Status != "live" {
		t.Fatalf("list = %+v", list)
	}

	if err := repo.Delete(ctx, rel.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, rel.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestHostChamadoRepo_CRUDSyncCache(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	var userID int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role, display_name) VALUES ('u','x','admin','User U') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := store.NewHostChamadoRepo(d.SQL)

	id, err := repo.Create(ctx, hostID, &models.HostChamadoInput{ChamadoID: "T-1", Title: "fix", UserID: userID, Date: "01/01/2026"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, id)
	if err != nil || got == nil || got.Title != "fix" || got.Status != "in_execution" || got.UserDisplayName != "User U" {
		t.Fatalf("get = %+v, %v", got, err)
	}

	if err := repo.Update(ctx, id, &models.HostChamadoInput{ChamadoID: "T-1", Title: "fixed", Status: "done", UserID: userID, Date: "02/01/2026"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.Get(ctx, id)
	if got.Title != "fixed" || got.Status != "done" {
		t.Fatalf("after update = %+v", got)
	}

	if err := repo.UpdateCache(ctx, id, "glpi", "http://x", "Cached", "open"); err != nil {
		t.Fatalf("updatecache: %v", err)
	}
	got, _ = repo.Get(ctx, id)
	if got.CachedTitle != "Cached" || got.ExternalSource != "glpi" || got.CachedAt == nil {
		t.Fatalf("after cache = %+v", got)
	}

	list, _ := repo.ListByHost(ctx, hostID)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}

	// CreateExternal pre-populates cache.
	eid, err := repo.CreateExternal(ctx, hostID, userID, "T-2", "ext", "open", "03/01/2026", "glpi", "http://t2")
	if err != nil {
		t.Fatalf("createexternal: %v", err)
	}
	ext, _ := repo.Get(ctx, eid)
	if ext.CachedTitle != "ext" || ext.CachedAt == nil {
		t.Fatalf("external = %+v", ext)
	}

	// Sync replaces everything with one row.
	if err := repo.Sync(ctx, hostID, []models.HostChamadoInput{{ChamadoID: "T-9", Title: "only", UserID: userID, Date: "04/01/2026"}}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	list, _ = repo.ListByHost(ctx, hostID)
	if len(list) != 1 || list[0].ChamadoID != "T-9" {
		t.Fatalf("after sync = %+v", list)
	}

	if err := repo.Delete(ctx, list[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if l, _ := repo.ListByHost(ctx, hostID); len(l) != 0 {
		t.Fatalf("after delete = %+v", l)
	}
}
