package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newResponsavelRepo(t *testing.T) (*store.ResponsavelRepo, *database.DB) {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewResponsavelRepo(d.SQL), d
}

func TestResponsavelRepo_SyncListAndMainNames(t *testing.T) {
	ctx := context.Background()
	repo, d := newResponsavelRepo(t)

	// Two contacts: one internal (main), one external.
	cr1, _ := d.SQL.Exec(`INSERT INTO contacts (name, phone, is_external) VALUES ('Ada','1',0)`)
	internalID, _ := cr1.LastInsertId()
	cr2, _ := d.SQL.Exec(`INSERT INTO contacts (name, phone, is_external) VALUES ('Ext','2',1)`)
	externalID, _ := cr2.LastInsertId()

	// Sync two responsaveis for host #7: internal is main.
	in := []models.ResponsavelInput{
		{ContactID: internalID, IsMain: true},
		{ContactID: externalID, IsMain: false},
	}
	if err := repo.Sync(ctx, "host", 7, in); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// List joins contact details, main-first.
	list, err := repo.List(ctx, "host", 7)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if !list[0].IsMain || list[0].Name != "Ada" {
		t.Fatalf("list[0] should be main Ada: %+v", list[0])
	}
	if list[1].Name != "Ext" || !list[1].IsExternal {
		t.Fatalf("list[1] should be external Ext: %+v", list[1])
	}

	// MainNamesBulk returns only the main *internal* responsável.
	names, err := repo.MainNamesBulk(ctx, "host")
	if err != nil || names[7] != "Ada" {
		t.Fatalf("MainNamesBulk = %v, %v; want [7]=Ada", names, err)
	}

	// Entity-type isolation: a "dns" query must not see host rows.
	if l, _ := repo.List(ctx, "dns", 7); len(l) != 0 {
		t.Fatalf("dns/7 = %+v, want none (entity_type isolation)", l)
	}

	// Re-sync replaces (does not append): now only the external contact, not main.
	if err := repo.Sync(ctx, "host", 7, []models.ResponsavelInput{{ContactID: externalID}}); err != nil {
		t.Fatalf("resync: %v", err)
	}
	list, _ = repo.List(ctx, "host", 7)
	if len(list) != 1 || list[0].ContactID != externalID {
		t.Fatalf("after resync = %+v, want only external", list)
	}
	if names, _ := repo.MainNamesBulk(ctx, "host"); len(names) != 0 {
		t.Fatalf("MainNamesBulk after resync = %v, want none (no internal main)", names)
	}

	// Sync rejects a zero contact id.
	if err := repo.Sync(ctx, "host", 7, []models.ResponsavelInput{{ContactID: 0}}); err == nil {
		t.Fatal("sync with contact_id=0 should error")
	}
}

// TestResponsavelRepo_MigratedFromLegacyTables proves the v70 data migration ran:
// a fresh DB has the responsaveis table and none of the four legacy tables.
func TestResponsavelRepo_MigratedFromLegacyTables(t *testing.T) {
	_, d := newResponsavelRepo(t)
	for _, legacy := range []string{"host_responsaveis", "dns_responsaveis", "service_responsaveis", "project_responsaveis"} {
		var n int
		err := d.SQL.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, legacy).Scan(&n)
		if err != nil {
			t.Fatalf("query sqlite_master: %v", err)
		}
		if n != 0 {
			t.Errorf("legacy table %s still exists after v70 migration", legacy)
		}
	}
	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='responsaveis'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("responsaveis table missing after v70 (n=%d, err=%v)", n, err)
	}
}
