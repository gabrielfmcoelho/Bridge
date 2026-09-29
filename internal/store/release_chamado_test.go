package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestReleaseRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	repo := store.NewReleaseRepo(d.SQL)
	var projectID int64
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p') RETURNING id`).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// Releases live inside a project (v90): none without one.
	if err := repo.Create(ctx, &models.Release{Title: "orphan", Status: "pending"}); err == nil {
		t.Fatal("create without project succeeded, want NOT NULL violation")
	}

	rel := &models.Release{ProjectID: &projectID, Title: "v1", Description: "first", Status: "pending"}
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
	list, _ := repo.List(ctx, projectID)
	if len(list) != 1 || list[0].Status != "live" {
		t.Fatalf("list = %+v", list)
	}
	if other, _ := repo.List(ctx, projectID+1); len(other) != 0 {
		t.Fatalf("list(other project) = %+v, want empty", other)
	}
	var userID, issueID int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO issues (project_id, title, created_by) VALUES (?, 'i', ?) RETURNING id`, projectID, userID).Scan(&issueID); err != nil {
		t.Fatalf("seed issue: %v", err)
	}
	if err := repo.SetIssues(ctx, rel.ID, []int64{issueID}); err != nil {
		t.Fatalf("set issues: %v", err)
	}
	if byRel, err := repo.IssueIDsByRelease(ctx, []int64{rel.ID}); err != nil || len(byRel[rel.ID]) != 1 || byRel[rel.ID][0] != issueID {
		t.Fatalf("IssueIDsByRelease = %+v, %v", byRel, err)
	}

	// Deleting the project takes its releases along.
	if _, err := d.SQL.Exec(`DELETE FROM projects WHERE id = ?`, projectID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	if got, _ := repo.Get(ctx, rel.ID); got != nil {
		t.Fatalf("release after project delete = %+v, want gone", got)
	}
}

func TestProjectEmbedRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	repo := store.NewProjectEmbedRepo(d.SQL)
	var p1, p2 int64
	d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p1') RETURNING id`).Scan(&p1)
	d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p2') RETURNING id`).Scan(&p2)

	e := &models.ProjectEmbed{ProjectID: p1, Title: "BI", URL: "https://bi.example/x", Height: 600}
	if err := repo.Create(ctx, e); err != nil || e.ID == 0 {
		t.Fatalf("create: %v (id %d)", err, e.ID)
	}
	// Another project can't reach it.
	e2 := *e
	e2.ProjectID, e2.Title = p2, "hijack"
	if found, err := repo.Update(ctx, &e2); err != nil || found {
		t.Fatalf("update via other project = %v, %v; want not found", found, err)
	}
	if err := repo.Delete(ctx, p2, e.ID); err != nil {
		t.Fatalf("delete via other project: %v", err)
	}
	list, _ := repo.List(ctx, p1)
	if len(list) != 1 || list[0].Title != "BI" {
		t.Fatalf("list = %+v; want the untouched embed", list)
	}
	if other, _ := repo.List(ctx, p2); len(other) != 0 {
		t.Fatalf("list(p2) = %+v, want empty", other)
	}
	e.Title = "BI 2"
	if found, err := repo.Update(ctx, e); err != nil || !found {
		t.Fatalf("update = %v, %v", found, err)
	}
	if err := repo.Delete(ctx, p1, e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := repo.List(ctx, p1); len(list) != 0 {
		t.Fatalf("after delete = %+v", list)
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
