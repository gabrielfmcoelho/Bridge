package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A deleted contact leaves the list and the responsáveis it held (the link
// stays for the restore); adding the same name+phone again brings it back.
func TestContactRepo_TrashAndRestore(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	contacts := store.NewContactRepo(d.SQL)
	resp := store.NewResponsavelRepo(d.SQL)
	c := &models.Contact{Name: "Ana", Phone: "86 9"}
	if err := contacts.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := resp.Sync(ctx, "host", 1, []models.ResponsavelInput{{ContactID: c.ID, IsMain: true}}); err != nil {
		t.Fatal(err)
	}
	if err := contacts.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := contacts.List(ctx); len(list) != 0 {
		t.Fatalf("list = %+v", list)
	}
	if rs, _ := resp.List(ctx, "host", 1); len(rs) != 0 {
		t.Fatalf("responsáveis still show the trashed contact: %+v", rs)
	}
	if names, _ := resp.MainNamesBulk(ctx, "host"); names[1] != "" {
		t.Fatalf("main name = %q", names[1])
	}
	if trash, _ := contacts.ListTrash(ctx); len(trash) != 1 || trash[0].DeletedAt == nil {
		t.Fatalf("trash = %+v", trash)
	}
	if ok, err := contacts.Restore(ctx, c.ID); err != nil || !ok {
		t.Fatalf("restore = %v, %v", ok, err)
	}
	if rs, _ := resp.List(ctx, "host", 1); len(rs) != 1 {
		t.Fatalf("responsável after restore = %+v", rs)
	}

	// Re-adding a trashed contact takes it out of the trash.
	contacts.Delete(ctx, c.ID)
	again := &models.Contact{Name: "Ana", Phone: "86 9"}
	if err := contacts.Create(ctx, again); err != nil || again.ID != c.ID {
		t.Fatalf("re-create = %d, %v; want id %d", again.ID, err, c.ID)
	}
	if list, _ := contacts.List(ctx); len(list) != 1 {
		t.Fatalf("list after re-create = %+v", list)
	}
}
