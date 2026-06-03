package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestEnumOptionRepo_CRUDAndAutoOrder(t *testing.T) {
	ctx := context.Background()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	repo := store.NewEnumOptionRepo(d.SQL)

	// Two creates in the same category with SortOrder unset must auto-assign
	// 0 then 1 (max+1). Use a fresh category to avoid migration-seeded rows.
	a := &models.EnumOption{Category: "zz_test", Value: "alpha"}
	b := &models.EnumOption{Category: "zz_test", Value: "beta"}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("create b: %v", err)
	}
	if a.SortOrder != 0 || b.SortOrder != 1 {
		t.Fatalf("sort orders = %d,%d, want 0,1", a.SortOrder, b.SortOrder)
	}

	got, err := repo.List(ctx, "zz_test")
	if err != nil || len(got) != 2 || got[0].Value != "alpha" {
		t.Fatalf("list = %+v, %v", got, err)
	}

	if err := repo.Update(ctx, "zz_test", "alpha", "alpha2", "#10b981"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.List(ctx, "zz_test")
	if got[0].Value != "alpha2" || got[0].Color != "#10b981" {
		t.Fatalf("after update = %+v", got[0])
	}

	all, err := repo.ListAll(ctx)
	if err != nil || len(all["zz_test"]) != 2 {
		t.Fatalf("listall[zz_test] = %+v, %v", all["zz_test"], err)
	}

	if err := repo.Delete(ctx, "zz_test", "beta"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = repo.List(ctx, "zz_test")
	if len(got) != 1 {
		t.Fatalf("after delete len = %d, want 1", len(got))
	}
}
