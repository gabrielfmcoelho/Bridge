package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// DNSRepo honours the entidade scope on ctx: a scoped caller only sees /
// touches records granted to its entidades; an unscoped ctx sees everything.
func TestDNSRepo_EntidadeScope(t *testing.T) {
	bg := context.Background()
	d := openDB(t)
	repo := store.NewDNSRepo(d.SQL)
	sga := entidadeID(t, d, "sga")

	seed := func(domain string) int64 {
		t.Helper()
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES (?) RETURNING id`, domain).Scan(&id); err != nil {
			t.Fatalf("seed %s: %v", domain, err)
		}
		return id
	}
	mine := seed("mine.example")
	other := seed("other.example")
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(bg, d.SQL, store.AssetDNS, mine, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})

	// List / ListFiltered / Count: only the granted record.
	if got, err := repo.List(scoped); err != nil || len(got) != 1 || got[0].ID != mine {
		t.Fatalf("scoped List = %+v, %v; want only %d", got, err, mine)
	}
	if got, err := repo.ListFiltered(scoped, models.DNSFilter{}); err != nil || len(got) != 1 || got[0].ID != mine {
		t.Fatalf("scoped ListFiltered = %+v, %v; want only %d", got, err, mine)
	}
	if n, _ := repo.CountFiltered(scoped, models.DNSFilter{}); n != 1 {
		t.Fatalf("scoped CountFiltered = %d, want 1", n)
	}
	if n, _ := repo.Count(scoped); n != 1 {
		t.Fatalf("scoped Count = %d, want 1", n)
	}

	// Get: invisible ⇒ (nil, nil); visible ⇒ row.
	if got, err := repo.Get(scoped, other); got != nil || err != nil {
		t.Fatalf("scoped Get(other) = %+v, %v; want nil, nil", got, err)
	}
	if got, err := repo.Get(scoped, mine); got == nil || err != nil {
		t.Fatalf("scoped Get(mine) = %+v, %v; want row", got, err)
	}

	// Update / Delete on the invisible record are no-ops.
	if err := repo.Update(scoped, &models.DNSRecord{ID: other, Domain: "hijacked.example"}); err != nil {
		t.Fatalf("scoped Update(other): %v", err)
	}
	if err := repo.Delete(scoped, other); err != nil {
		t.Fatalf("scoped Delete(other): %v", err)
	}
	if got, err := repo.Get(bg, other); err != nil || got == nil || got.Domain != "other.example" {
		t.Fatalf("unscoped Get(other) after scoped write = %+v, %v; want untouched row", got, err)
	}

	// Unscoped ctx sees both.
	if got, err := repo.List(bg); err != nil || len(got) != 2 {
		t.Fatalf("unscoped List = %d rows, %v; want 2", len(got), err)
	}
}
