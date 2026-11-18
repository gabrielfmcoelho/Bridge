package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAppDepRepoApply(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	r := store.NewAppDepRepo(d.SQL)
	now := time.Now().UTC().Truncate(time.Second)

	parsed := func(id int, path, ci string) models.AppDepSource {
		c, err := models.ParseAppDepCatalog([]byte("ci: " + ci + "\ncomponents:\n  - {name: api, category: application, technology: rest-api, port: 443}\n"))
		if err != nil {
			t.Fatal(err)
		}
		return models.AppDepSource{GitLabProjectID: id, ProjectPath: path, WebURL: "https://gl/" + path, Ref: "main", CommitID: "c1", Catalog: c, SyncedAt: now}
	}
	if _, err := r.Apply(ctx, []models.AppDepSource{
		parsed(1, "g/a", "a"),
		parsed(2, "g/b", "b"),
		{GitLabProjectID: 3, ProjectPath: "g/broken", Ref: "main", ParseError: "ci is missing", SyncedAt: now},
	}, nil); err != nil {
		t.Fatal(err)
	}

	// 1 is re-read with a new commit, 2 is gone, 3 is untouched.
	up := parsed(1, "g/a", "a2")
	up.CommitID = "c2"
	removed, err := r.Apply(ctx, []models.AppDepSource{up}, []int{2, 99})
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v, want 1", removed, err)
	}

	all, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("rows = %+v, want 2", all)
	}
	a, broken := all[0], all[1]
	if a.GitLabProjectID != 1 || a.CommitID != "c2" || a.Catalog == nil || a.Catalog.CI != "a2" ||
		len(a.Catalog.Components) != 1 || a.Catalog.Components[0].Port != 443 ||
		a.FileURL != "https://gl/g/a/-/blob/main/catalog/dependencies.yml" || !a.SyncedAt.Equal(now) {
		t.Errorf("updated row = %+v", a)
	}
	if broken.GitLabProjectID != 3 || broken.Catalog != nil || broken.ParseError != "ci is missing" {
		t.Errorf("parse failure kept = %+v", broken)
	}
	ids, _ := r.IDs(ctx)
	if len(ids) != 2 {
		t.Errorf("ids = %v", ids)
	}
}
