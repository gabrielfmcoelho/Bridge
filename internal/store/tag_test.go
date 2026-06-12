package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestTagRepo_SetGetDistinctRemove(t *testing.T) {
	ctx := context.Background()
	repo := store.NewTagRepo(openDB(t).SQL)

	// Set replaces; tags come back sorted.
	if err := repo.Set(ctx, "host", 1, []string{"prod", "db", ""}); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := repo.Get(ctx, "host", 1)
	if err != nil || len(got) != 2 || got[0] != "db" || got[1] != "prod" {
		t.Fatalf("get = %+v, %v (sorted, empty skipped)", got, err)
	}

	// Different entity type/id is isolated.
	_ = repo.Set(ctx, "host", 2, []string{"staging"})
	_ = repo.Set(ctx, "service", 1, []string{"api"})

	all, _ := repo.GetAll(ctx, "host")
	if len(all) != 2 || len(all[1]) != 2 || all[2][0] != "staging" {
		t.Fatalf("getall = %+v", all)
	}

	distinct, _ := repo.Distinct(ctx, "host")
	if len(distinct) != 3 { // db, prod, staging
		t.Fatalf("distinct host = %+v, want 3", distinct)
	}
	allDistinct, _ := repo.AllDistinct(ctx)
	if len(allDistinct) != 4 { // + api
		t.Fatalf("alldistinct = %+v, want 4", allDistinct)
	}

	// Add + Remove single tags.
	if err := repo.Add(ctx, "host", 1, "edge"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := repo.Add(ctx, "host", 1, "edge"); err != nil { // idempotent
		t.Fatalf("add dup: %v", err)
	}
	got, _ = repo.Get(ctx, "host", 1)
	if len(got) != 3 {
		t.Fatalf("after add = %+v, want 3", got)
	}
	if err := repo.Remove(ctx, "host", 1, "edge"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, _ = repo.Get(ctx, "host", 1)
	if len(got) != 2 {
		t.Fatalf("after remove = %+v, want 2", got)
	}

	// Delete clears the entity.
	if err := repo.Delete(ctx, "host", 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = repo.Get(ctx, "host", 1)
	if len(got) != 0 {
		t.Fatalf("after delete = %+v, want empty", got)
	}
}
