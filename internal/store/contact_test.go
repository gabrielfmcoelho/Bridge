package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newContactRepo(t *testing.T) *store.ContactRepo {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewContactRepo(d.SQL)
}

func TestContactRepo_CRUDRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := newContactRepo(t)

	// Create
	c := &models.Contact{Name: "Ada", Phone: "555-1", Role: "dev", Entity: "eng", Notes: "n", IsExternal: false}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == 0 {
		t.Fatal("create did not set ID")
	}

	// List
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Ada" || got[0].Role != "dev" {
		t.Fatalf("list = %+v, want one Ada/dev", got)
	}

	// Update
	c.Role = "lead"
	c.IsExternal = true
	if err := repo.Update(ctx, c); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.List(ctx)
	if got[0].Role != "lead" || !got[0].IsExternal {
		t.Fatalf("after update = %+v, want role=lead is_external=true", got[0])
	}

	// Delete
	if err := repo.Delete(ctx, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = repo.List(ctx)
	if len(got) != 0 {
		t.Fatalf("after delete len = %d, want 0", len(got))
	}
}

// Create upserts on (name, phone): a second create with the same key must not
// error and must return the existing id rather than duplicating.
func TestContactRepo_CreateUpsertsOnConflict(t *testing.T) {
	ctx := context.Background()
	repo := newContactRepo(t)

	a := &models.Contact{Name: "Grace", Phone: "555-9"}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("create a: %v", err)
	}
	b := &models.Contact{Name: "Grace", Phone: "555-9", Role: "admiral"}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("create b (conflict): %v", err)
	}
	if b.ID != a.ID {
		t.Fatalf("conflict id = %d, want existing %d", b.ID, a.ID)
	}
	got, _ := repo.List(ctx)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (upsert, not duplicate)", len(got))
	}
}
