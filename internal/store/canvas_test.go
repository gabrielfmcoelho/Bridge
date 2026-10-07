package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestCanvasRepo_ScopeAndVersion(t *testing.T) {
	d := openDB(t)
	bg := context.Background()
	govpi := entidadeID(t, d, "govpi")
	etipi := entidadeID(t, d, "etipi") // child of govpi
	sga := entidadeID(t, d, "sga")
	repo := store.NewCanvasRepo(d.SQL)

	c := &models.Canvas{EntidadeID: etipi, Title: "Ideias"}
	if err := repo.Create(bg, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	seen := func(ids ...int64) bool {
		t.Helper()
		got, err := repo.Get(store.WithScope(bg, store.Scope{EntidadeIDs: ids}), c.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got != nil
	}
	if !seen(etipi) || !seen(govpi, etipi) {
		t.Fatal("own / ancestor scope should see the canvas")
	}
	if seen(sga) {
		t.Fatal("sibling entidade must not see the canvas")
	}
	if list, _ := repo.List(store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}}), 0); len(list) != 0 {
		t.Fatalf("sibling list = %d rows, want 0", len(list))
	}

	got, _ := repo.Get(bg, c.ID)
	if string(got.Content) != `{"nodes":[],"edges":[]}` || got.EntidadeName == "" {
		t.Fatalf("new canvas = %+v", got)
	}

	content := `{"nodes":[{"id":"n1"}],"edges":[]}`
	v, err := repo.Update(bg, c.ID, 1, nil, &content)
	if err != nil || v != 2 {
		t.Fatalf("update v1: v=%d err=%v", v, err)
	}
	if _, err := repo.Update(bg, c.ID, 1, nil, &content); !errors.Is(err, store.ErrVersionConflict) {
		t.Fatalf("stale update err = %v, want ErrVersionConflict", err)
	}
	sgaCtx := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})
	if _, err := repo.Update(sgaCtx, c.ID, 2, nil, &content); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("invisible update err = %v, want sql.ErrNoRows (not a conflict leak)", err)
	}
	got, _ = repo.Get(bg, c.ID)
	if string(got.Content) != content || got.Title != "Ideias" {
		t.Fatalf("after update = %+v (title must be kept when omitted)", got)
	}
}

func TestCanSee_EntidadeParent(t *testing.T) {
	d := openDB(t)
	bg := context.Background()
	etipi := entidadeID(t, d, "etipi")
	sga := entidadeID(t, d, "sga")
	cases := []struct {
		ids  []int64
		want bool
	}{{[]int64{etipi}, true}, {[]int64{sga}, false}, {nil, false}}
	for _, tc := range cases {
		ok, err := store.CanSee(store.WithScope(bg, store.Scope{EntidadeIDs: tc.ids}), d.SQL, store.AssetEntidade, etipi)
		if err != nil || ok != tc.want {
			t.Fatalf("CanSee(entidade etipi) with %v = %v, %v; want %v", tc.ids, ok, err, tc.want)
		}
	}
}
