package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestProjectRepo_EntidadeScope(t *testing.T) {
	d := openDB(t)
	bg := context.Background()
	sga := entidadeID(t, d, "sga")
	projects := store.NewProjectRepo(d.SQL)
	releases := store.NewReleaseRepo(d.SQL)

	visible := &models.Project{Name: "visible"}
	hidden := &models.Project{Name: "hidden"}
	for _, p := range []*models.Project{visible, hidden} {
		if err := projects.Create(bg, p); err != nil {
			t.Fatalf("create project: %v", err)
		}
		if err := releases.Create(bg, &models.Release{ProjectID: &p.ID, Title: "rel-" + p.Name, Status: "pending"}); err != nil {
			t.Fatalf("create release: %v", err)
		}
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(bg, d.SQL, store.AssetProject, visible.ID, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})

	// Scoped: only the granted project (and its release) is visible.
	if list, err := projects.List(scoped); err != nil || len(list) != 1 || list[0].ID != visible.ID {
		t.Fatalf("scoped List = %+v, %v; want only visible", list, err)
	}
	if list, err := projects.ListFiltered(scoped, models.ProjectFilter{}); err != nil || len(list) != 1 {
		t.Fatalf("scoped ListFiltered = %+v, %v; want 1", list, err)
	}
	if n, err := projects.Count(scoped); err != nil || n != 1 {
		t.Fatalf("scoped Count = %d, %v; want 1", n, err)
	}
	if got, err := projects.Get(scoped, hidden.ID); got != nil || err != nil {
		t.Fatalf("scoped Get(hidden) = %+v, %v; want nil,nil", got, err)
	}
	if got, err := projects.Get(scoped, visible.ID); got == nil || err != nil {
		t.Fatalf("scoped Get(visible) = %+v, %v; want row", got, err)
	}
	hidden.Name = "renamed"
	if err := projects.Update(scoped, hidden); err != nil {
		t.Fatalf("scoped Update: %v", err)
	}
	if err := projects.Delete(scoped, hidden.ID); err != nil {
		t.Fatalf("scoped Delete: %v", err)
	}
	if got, err := projects.Get(bg, hidden.ID); err != nil || got == nil || got.Name != "hidden" {
		t.Fatalf("hidden after scoped update/delete = %+v, %v; want untouched", got, err)
	}
	if rels, err := releases.List(scoped); err != nil || len(rels) != 1 || *rels[0].ProjectID != visible.ID {
		t.Fatalf("scoped releases.List = %+v, %v; want only visible's release", rels, err)
	}

	// Unscoped: everything.
	if list, err := projects.List(bg); err != nil || len(list) != 2 {
		t.Fatalf("unscoped List = %+v, %v; want 2", list, err)
	}
	if rels, err := releases.List(bg); err != nil || len(rels) != 2 {
		t.Fatalf("unscoped releases.List = %+v, %v; want 2", rels, err)
	}
}
