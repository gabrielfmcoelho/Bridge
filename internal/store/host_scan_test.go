package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostScanRepo_WriteAndReadAggregates(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('web-1', 'web-1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}

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

// TestHostScanRepo_PopulatesNumericMetrics asserts that Create derives the
// sortable numeric columns (P2) from the scan JSON's display strings, and that
// missing/non-numeric values store as NULL (not 0).
func TestHostScanRepo_PopulatesNumericMetrics(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('m1','m1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	repo := store.NewHostScanRepo(d.SQL)
	if err := repo.Create(ctx, hostID,
		`{"cpu_usage":"45%","ram_percent":"50%","disk_percent":"40%","containers":["c1","c2","c3"]}`); err != nil {
		t.Fatalf("create: %v", err)
	}

	var cpu, ram, disk *float64
	var containers int
	if err := d.SQL.QueryRow(
		`SELECT cpu_pct, ram_pct, disk_pct, containers_count FROM host_scans WHERE host_id = ?`, hostID,
	).Scan(&cpu, &ram, &disk, &containers); err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if cpu == nil || *cpu != 45 || ram == nil || *ram != 50 || disk == nil || *disk != 40 {
		t.Fatalf("metrics = cpu:%v ram:%v disk:%v, want 45/50/40", cpu, ram, disk)
	}
	if containers != 3 {
		t.Fatalf("containers_count = %d, want 3", containers)
	}

	// Missing percentages → NULL (not 0).
	if err := repo.Create(ctx, hostID, `{"cpu":"4 cores"}`); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	var cpu2 *float64
	if err := d.SQL.QueryRow(
		`SELECT cpu_pct FROM host_scans WHERE host_id = ? ORDER BY id DESC LIMIT 1`, hostID,
	).Scan(&cpu2); err != nil {
		t.Fatalf("read cpu2: %v", err)
	}
	if cpu2 != nil {
		t.Fatalf("absent cpu_usage should be NULL, got %v", *cpu2)
	}
}
