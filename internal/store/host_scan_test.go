package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostScanRepo_WriteAndReadAggregates(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	res, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('web-1', 'web-1')`)
	if err != nil {
		t.Fatalf("seed host: %v", err)
	}
	hostID, _ := res.LastInsertId()

	repo := store.NewHostScanRepo(d.SQL)

	// Empty state.
	if n, _ := repo.CountHostsWithScans(ctx); n != 0 {
		t.Fatalf("count empty = %d, want 0", n)
	}
	if got, _ := repo.GetLatest(ctx, hostID); got != nil {
		t.Fatalf("latest empty = %+v, want nil", got)
	}

	if err := repo.Create(ctx, hostID, `{"cpu":"4"}`); err != nil {
		t.Fatalf("create: %v", err)
	}

	latest, err := repo.GetLatest(ctx, hostID)
	if err != nil || latest == nil || latest.Data != `{"cpu":"4"}` {
		t.Fatalf("latest = %+v, %v", latest, err)
	}

	if n, _ := repo.CountHostsWithScans(ctx); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}

	statuses, err := repo.Statuses(ctx)
	if err != nil {
		t.Fatalf("statuses: %v", err)
	}
	if _, ok := statuses[hostID]; !ok {
		t.Fatalf("statuses missing host %d: %+v", hostID, statuses)
	}

	bulk, err := repo.LatestDataBulk(ctx)
	if err != nil || bulk[hostID] != `{"cpu":"4"}` {
		t.Fatalf("bulk = %+v, %v", bulk, err)
	}

	recent, err := repo.RecentWithHost(ctx, 5)
	if err != nil || len(recent) != 1 || recent[0]["nickname"] != "web-1" {
		t.Fatalf("recent = %+v, %v", recent, err)
	}
}
