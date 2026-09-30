package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A deleted contact leaves the list and the responsáveis it held (the link
// stays for the restore); a duplicate name+phone is refused, never merged.
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

	// Adding the same name+phone again never touches the existing contact.
	c.Role = "kept"
	contacts.Update(ctx, c)
	dup := &models.Contact{Name: "Ana", Phone: "86 9", Role: "overwritten"}
	var exists store.ErrContactExists
	if err := contacts.Create(ctx, dup); !errors.As(err, &exists) || exists.ID != c.ID || exists.Trashed {
		t.Fatalf("duplicate create = %v", err)
	}
	contacts.Delete(ctx, c.ID)
	if err := contacts.Create(ctx, dup); !errors.As(err, &exists) || !exists.Trashed {
		t.Fatalf("duplicate of trashed = %v; want Trashed", err)
	}
	contacts.Restore(ctx, c.ID)
	if got, _ := contacts.Get(ctx, c.ID); got == nil || got.Role != "kept" {
		t.Fatalf("existing contact changed: %+v", got)
	}

	// Usage: the host it's responsável for, only while the host is live.
	var hid int64
	d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('web','web-01') RETURNING id`).Scan(&hid)
	resp.Sync(ctx, "host", hid, []models.ResponsavelInput{{ContactID: c.ID, IsMain: true}})
	if uses, _ := contacts.Usage(ctx, c.ID); len(uses) != 1 || uses[0].Slug != "web-01" || !uses[0].IsMain {
		t.Fatalf("usage = %+v", uses)
	}
	if counts, _ := contacts.UsageCounts(ctx); counts[c.ID]["host"] != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	d.SQL.Exec(`UPDATE hosts SET deleted_at = now() WHERE id = ?`, hid)
	if uses, _ := contacts.Usage(ctx, c.ID); len(uses) != 0 {
		t.Fatalf("usage counts a trashed host: %+v", uses)
	}
}
