package store_test

import (
	"context"
	"encoding/json"
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

	bulk, err := repo.LatestListSummaryBulk(ctx)
	if err != nil || bulk[hostID] != `{"cpu": "4"}` {
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

// The host list reads only a few fields of the latest scan; the summary keeps
// those, drops the rest, uses the newest scan, and skips a row that isn't JSON.
func TestHostScanRepo_LatestListSummaryBulk(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	seed := func(slug string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ($1, $1) RETURNING id`, slug).Scan(&id); err != nil {
			t.Fatalf("seed host: %v", err)
		}
		return id
	}
	full, bad := seed("full"), seed("bad")
	repo := store.NewHostScanRepo(d.SQL)
	if err := repo.Create(ctx, full, `{"cpu":"2"}`); err != nil {
		t.Fatal(err)
	}
	// Newer scan of the same host, with fields the list never reads.
	if _, err := d.SQL.Exec(`INSERT INTO host_scans (host_id, data, scanned_at) VALUES ($1, $2, now() + interval '1 minute')`, full,
		`{"cpu":"4","cpu_usage":"45%","ram":"8G","ram_percent":"50%","storage":"100G","disk_percent":"40%",
		  "services":["nginx"],"containers":["c1","c2"],"os":"Ubuntu 22.04.4 LTS","arch":"x86_64","reboot_required":true,
		  "profile":{"idle":true,"reasons":["no traffic"],"counterfacts":["x"],"signals":{"big":1}},
		  "storage_detail":{"filesystems":[{"mount":"/"}]},"web_servers":[{"kind":"nginx"}]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO host_scans (host_id, data) VALUES ($1, 'not json')`, bad); err != nil {
		t.Fatal(err)
	}
	// The host's own os row feeds os_info; a container's os row doesn't.
	if _, err := d.SQL.Exec(`INSERT INTO dependency_inventory (host_id, container, kind, name, version, version_source, eol, eol_date)
		VALUES ($1, '', 'os', 'ubuntu', '22.04', 'os_release', false, '2027-04-01'),
		       ($1, 'web', 'os', 'alpine', '3.12', 'os_release', true, '2022-05-01')`, full); err != nil {
		t.Fatal(err)
	}

	bulk, err := repo.LatestListSummaryBulk(ctx)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if _, ok := bulk[bad]; ok {
		t.Errorf("invalid scan JSON should be skipped, got %q", bulk[bad])
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(bulk[full]), &got); err != nil {
		t.Fatalf("summary not JSON: %v (%q)", err, bulk[full])
	}
	for _, k := range []string{"cpu", "cpu_usage", "ram", "ram_percent", "storage", "disk_percent", "services", "containers", "profile"} {
		if _, ok := got[k]; !ok {
			t.Errorf("summary missing %q: %v", k, got)
		}
	}
	if got["cpu"] != "4" {
		t.Errorf("cpu = %v, want the newest scan's 4", got["cpu"])
	}
	for _, k := range []string{"storage_detail", "web_servers"} {
		if _, ok := got[k]; ok {
			t.Errorf("summary should drop %q", k)
		}
	}
	prof, _ := got["profile"].(map[string]any)
	if prof["idle"] != true || prof["signals"] != nil {
		t.Errorf("profile = %v, want idle/reasons/counterfacts only", prof)
	}
	osInfo, _ := got["os_info"].(map[string]any)
	if osInfo["id"] != "ubuntu" || osInfo["version"] != "22.04" || osInfo["pretty"] != "Ubuntu 22.04.4 LTS" ||
		osInfo["arch"] != "x86_64" || osInfo["reboot_required"] != true || osInfo["eol"] != false || osInfo["eol_date"] != "2027-04-01" {
		t.Errorf("os_info = %v", osInfo)
	}
}

func TestHostScanRepo_ListAfterAndUpdateData(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('web-1', 'web-1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	repo := store.NewHostScanRepo(d.SQL)
	for _, data := range []string{`{"cpu":"1"}`, `{"cpu":"2"}`, `{"cpu":"3"}`} {
		if err := repo.Create(ctx, hostID, data); err != nil {
			t.Fatal(err)
		}
	}

	first, err := repo.ListAfter(ctx, 0, 2)
	if err != nil || len(first) != 2 || first[0].Data != `{"cpu":"1"}` || first[1].Data != `{"cpu":"2"}` || first[0].HostID != hostID {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	rest, err := repo.ListAfter(ctx, first[1].ID, 2)
	if err != nil || len(rest) != 1 || rest[0].Data != `{"cpu":"3"}` {
		t.Fatalf("second page = %+v, %v", rest, err)
	}
	if none, err := repo.ListAfter(ctx, rest[0].ID, 2); err != nil || len(none) != 0 {
		t.Fatalf("past the end = %+v, %v", none, err)
	}

	if err := repo.UpdateData(ctx, first[0].ID, `{"cpu":"9"}`); err != nil {
		t.Fatal(err)
	}
	again, _ := repo.ListAfter(ctx, 0, 1)
	if len(again) != 1 || again[0].Data != `{"cpu":"9"}` || !again[0].ScannedAt.Equal(first[0].ScannedAt) {
		t.Fatalf("after update = %+v (scanned_at was %v)", again, first[0].ScannedAt)
	}
}
