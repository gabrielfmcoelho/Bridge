package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newProjects(t *testing.T, names ...string) (*store.ProjectRepo, func(string, ...any) error, []int64) {
	t.Helper()
	d := openDB(t)
	var ids []int64
	for _, n := range names {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES (?) RETURNING id`, n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	exec := func(q string, a ...any) error { _, err := d.SQL.Exec(q, a...); return err }
	return store.NewProjectRepo(d.SQL), exec, ids
}

func TestSetGestorLinksSubstitui(t *testing.T) {
	ctx := context.Background()
	repo, _, ids := newProjects(t, "a")
	l := func(id int64, n string) models.ProjectGestorLink {
		return models.ProjectGestorLink{GestorProjetoID: id, GestorProjetoNome: n}
	}
	if got, err := repo.GestorLinksFor(ctx, ids[0]); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty = %v %v, want non-nil empty", got, err)
	}
	if err := repo.SetGestorLinks(ctx, ids[0], []models.ProjectGestorLink{l(1, "Um"), l(2, "Dois")}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGestorLinks(ctx, ids[0], []models.ProjectGestorLink{l(2, "Dois"), l(3, "Tres")}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GestorLinksFor(ctx, ids[0])
	if len(got) != 2 || got[0].GestorProjetoID != 2 || got[1].GestorProjetoID != 3 || got[0].ProjectID != ids[0] {
		t.Errorf("links = %+v", got)
	}
}

func TestGestorProjetoEmOutroProjetoDaErro(t *testing.T) {
	ctx := context.Background()
	repo, _, ids := newProjects(t, "a", "b")
	if err := repo.SetGestorLinks(ctx, ids[0], []models.ProjectGestorLink{{GestorProjetoID: 7, GestorProjetoNome: "x"}}); err != nil {
		t.Fatal(err)
	}
	err := repo.SetGestorLinks(ctx, ids[1], []models.ProjectGestorLink{{GestorProjetoID: 7, GestorProjetoNome: "x"}})
	if !errors.Is(err, store.ErrGestorProjetoEmUso) {
		t.Fatalf("err = %v, want ErrGestorProjetoEmUso", err)
	}
	if got, _ := repo.GestorLinksFor(ctx, ids[0]); len(got) != 1 {
		t.Errorf("owner lost its link: %+v", got)
	}
}

func TestApagarProjetoApagaVinculos(t *testing.T) {
	ctx := context.Background()
	repo, exec, ids := newProjects(t, "a")
	if err := repo.SetGestorLinks(ctx, ids[0], []models.ProjectGestorLink{{GestorProjetoID: 7}}); err != nil {
		t.Fatal(err)
	}
	if err := exec(`DELETE FROM projects WHERE id = ?`, ids[0]); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GestorLinksFor(ctx, ids[0]); len(got) != 0 {
		t.Errorf("links survived delete: %+v", got)
	}
}
