package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestGlpiTokenRepo_CRUDAndTokenPreservation(t *testing.T) {
	ctx := context.Background()
	repo := store.NewGlpiTokenRepo(openDB(t).SQL)

	tok := &models.GlpiToken{Name: "team-a", UserTokenCipher: []byte{1, 2}, UserTokenNonce: []byte{3}, DefaultEntityID: 7}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, tok.ID)
	if err != nil || got == nil || !got.HasToken || got.DefaultEntityID != 7 {
		t.Fatalf("get = %+v, %v (HasToken should be true)", got, err)
	}

	// Update with nil cipher preserves the stored token.
	upd := &models.GlpiToken{ID: tok.ID, Name: "team-a2", DefaultEntityID: 9}
	if err := repo.Update(ctx, upd); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.Get(ctx, tok.ID)
	if got.Name != "team-a2" || got.DefaultEntityID != 9 || !got.HasToken {
		t.Fatalf("after update = %+v, want name/entity changed + token preserved", got)
	}
	if string(got.UserTokenCipher) != string([]byte{1, 2}) {
		t.Fatalf("token cipher changed: %v", got.UserTokenCipher)
	}

	list, _ := repo.List(ctx)
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if err := repo.Delete(ctx, tok.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, tok.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestGlpiDropdownCatalogueRepo_UpsertGetDelete(t *testing.T) {
	ctx := context.Background()
	repo := store.NewGlpiDropdownCatalogueRepo(openDB(t).SQL)

	if c, _ := repo.Get(ctx, "Entity"); c != nil {
		t.Fatalf("get empty = %+v, want nil", c)
	}

	// Insert.
	if err := repo.Upsert(ctx, "Entity", []byte(`[{"id":1}]`), 1, nil); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	c, err := repo.Get(ctx, "Entity")
	if err != nil || c == nil || c.OptionCount != 1 || string(c.Options) != `[{"id":1}]` {
		t.Fatalf("get = %+v, %v", c, err)
	}

	// Update (same itemtype) -> still one row, new payload.
	if err := repo.Upsert(ctx, "Entity", []byte(`[{"id":1},{"id":2}]`), 2, nil); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	c, _ = repo.Get(ctx, "Entity")
	if c.OptionCount != 2 {
		t.Fatalf("after update count = %d, want 2", c.OptionCount)
	}

	list, _ := repo.List(ctx)
	if len(list) != 1 || list[0].Itemtype != "Entity" || list[0].OptionCount != 2 {
		t.Fatalf("list = %+v", list)
	}

	if err := repo.Delete(ctx, "Entity"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if c, _ := repo.Get(ctx, "Entity"); c != nil {
		t.Fatalf("after delete = %+v, want nil", c)
	}
}
