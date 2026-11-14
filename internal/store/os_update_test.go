package store_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A run's lifecycle: pending → running → finished per host, counts per
// status, cancel skips what's pending, a restart interrupts what's left,
// and the latest counted result per host feeds the "pending updates" column.
func TestOSUpdateRepo_Lifecycle(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hosts []int64
	for _, n := range []string{"vm-a", "vm-b", "vm-c"} {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ($1, $1) RETURNING id`, n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		hosts = append(hosts, id)
	}
	repo := store.NewOSUpdateRepo(d.SQL)
	run := &models.OSUpdateRun{Kind: "check", Concurrency: 2}
	if err := repo.CreateRun(ctx, run, hosts); err != nil {
		t.Fatal(err)
	}
	if run.ID == 0 || run.Status != models.OSRunRunning {
		t.Fatalf("run = %+v", run)
	}
	pending, _ := repo.PendingHosts(ctx, run.ID)
	if len(pending) != 3 {
		t.Fatalf("pending = %v", pending)
	}

	if ok, err := repo.StartResult(ctx, run.ID, hosts[0], "ubuntu 22.04"); !ok || err != nil {
		t.Fatalf("start = %v %v", ok, err)
	}
	if ok, _ := repo.StartResult(ctx, run.ID, hosts[0], "x"); ok {
		t.Fatal("a running host can't start twice")
	}
	three, one := 3, 1
	if err := repo.FinishResult(ctx, &models.OSUpdateResult{RunID: run.ID, HostID: hosts[0], Status: models.OSResultSuccess,
		PkgManager: "apt", OSAfter: "ubuntu 22.04", Upgradable: &three, SecurityUpgradable: &one, Output: "@@pm apt"}); err != nil {
		t.Fatal(err)
	}
	repo.StartResult(ctx, run.ID, hosts[1], "")

	// Cancel: what's still pending is skipped.
	if err := repo.SkipPending(ctx, run.ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetRun(ctx, run.ID)
	if err != nil || got == nil {
		t.Fatalf("get = %v %v", got, err)
	}
	if got.Counts["success"] != 1 || got.Counts["running"] != 1 || got.Counts["skipped"] != 1 || len(got.Results) != 3 {
		t.Fatalf("counts = %v results = %d", got.Counts, len(got.Results))
	}
	if r := got.Results[0]; r.HostNickname != "vm-a" || *r.Upgradable != 3 || r.OSBefore != "ubuntu 22.04" || r.FinishedAt == nil || r.Report != nil {
		t.Fatalf("vm-a = %+v", r)
	}

	// A restart: the running host fails with an explanation, the run is interrupted.
	if err := repo.MarkInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetRun(ctx, run.ID)
	if got.Status != models.OSRunInterrupted || got.FinishedAt == nil || got.Results[1].Status != models.OSResultFailed ||
		!strings.Contains(got.Results[1].Reason, "restart") {
		t.Fatalf("interrupted = %+v / %+v", got, got.Results[1])
	}
	if ok, _ := repo.FinishRun(ctx, run.ID, models.OSRunDone); ok {
		t.Fatal("an interrupted run can't be finished again")
	}

	latest, err := repo.LatestByHost(ctx)
	if err != nil || len(latest) != 1 || latest[0].HostID != hosts[0] || *latest[0].SecurityUpgradable != 1 || latest[0].Output != "" {
		t.Fatalf("latest = %+v %v", latest, err)
	}
	runs, err := repo.ListRuns(ctx, 10, 0)
	if err != nil || len(runs) != 1 || runs[0].Counts["failed"] != 1 {
		t.Fatalf("runs = %+v %v", runs, err)
	}
	if missing, err := repo.GetRun(ctx, 999); missing != nil || err != nil {
		t.Fatalf("missing = %v %v", missing, err)
	}

	// Backup fields and the report round-trip; a backup can later be marked deleted.
	brun := &models.OSUpdateRun{Kind: "packages", Concurrency: 1, Backup: true, KeepBackup: true}
	if err := repo.CreateRun(ctx, brun, hosts[:1]); err != nil {
		t.Fatal(err)
	}
	repo.StartResult(ctx, brun.ID, hosts[0], "")
	size := int64(1 << 30)
	if err := repo.FinishResult(ctx, &models.OSUpdateResult{RunID: brun.ID, HostID: hosts[0], Status: models.OSResultSuccess,
		BackupKey: "bridge/os-backups/vm-a/x.tar.zst", BackupSize: &size, BackupStatus: models.OSBackupDone,
		Report: json.RawMessage(`{"new_failed_units":["fail2ban.service"]}`)}); err != nil {
		t.Fatal(err)
	}
	x, err := repo.GetResult(ctx, brun.ID, hosts[0])
	if err != nil || x == nil || x.BackupKey != "bridge/os-backups/vm-a/x.tar.zst" || *x.BackupSize != size || x.BackupStatus != models.OSBackupDone {
		t.Fatalf("result = %+v %v", x, err)
	}
	var report struct {
		NewFailedUnits []string `json:"new_failed_units"`
	}
	if err := json.Unmarshal(x.Report, &report); err != nil || strings.Join(report.NewFailedUnits, " ") != "fail2ban.service" {
		t.Fatalf("report = %s %v", x.Report, err)
	}
	repo.SetBackupStatus(ctx, brun.ID, hosts[0], models.OSBackupDeleted)
	if got, _ := repo.GetRun(ctx, brun.ID); !got.Backup || !got.KeepBackup || got.Results[0].BackupStatus != models.OSBackupDeleted {
		t.Fatalf("run = %+v", got)
	}
	if none, err := repo.GetResult(ctx, brun.ID, hosts[2]); none != nil || err != nil {
		t.Fatalf("no result = %v %v", none, err)
	}

	// A backup-only run with the run's own paths; its result keeps the
	// effective ones. Kept backups: per host, and the newest of each host.
	krun := &models.OSUpdateRun{Kind: "backup", Concurrency: 1, Backup: true, KeepBackup: true, BackupInclude: []string{"/etc", "/opt"}}
	if err := repo.CreateRun(ctx, krun, hosts[1:2]); err != nil {
		t.Fatal(err)
	}
	repo.StartResult(ctx, krun.ID, hosts[1], "")
	repo.FinishResult(ctx, &models.OSUpdateResult{RunID: krun.ID, HostID: hosts[1], Status: models.OSResultSuccess,
		BackupKey: "k", BackupSize: &size, BackupStatus: models.OSBackupDone,
		BackupInclude: []string{"/etc", "/opt"}, BackupExclude: []string{"/tmp"}})
	if got, _ := repo.GetRun(ctx, krun.ID); strings.Join(got.BackupInclude, " ") != "/etc /opt" || got.BackupExclude != nil ||
		strings.Join(got.Results[0].BackupExclude, " ") != "/tmp" {
		t.Fatalf("backup run = %+v / %+v", got, got.Results[0])
	}
	kept, err := repo.KeptBackups(ctx, hosts[1])
	if err != nil || len(kept) != 1 || kept[0].RunKind != "backup" || kept[0].BackupKey != "k" || kept[0].Report != nil {
		t.Fatalf("kept = %+v %v", kept, err)
	}
	if all, _ := repo.KeptBackups(ctx, 0); len(all) != 1 || all[0].HostID != hosts[1] {
		t.Fatalf("latest kept = %+v", all)
	}

	// A chained release remembers that it updates the packages first.
	rrun := &models.OSUpdateRun{Kind: "release", Concurrency: 1, Reboot: true, UpdateFirst: true}
	if err := repo.CreateRun(ctx, rrun, hosts[:1]); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetRun(ctx, rrun.ID); got == nil || !got.UpdateFirst {
		t.Fatalf("release run = %+v", got)
	}
	if none, _ := repo.KeptBackups(ctx, hosts[0]); len(none) != 0 {
		t.Fatalf("a deleted backup is not kept: %+v", none)
	}
	if runs, _ := repo.ListRuns(ctx, 10, hosts[1]); len(runs) != 2 || runs[0].ID != krun.ID {
		t.Fatalf("host runs = %+v", runs)
	}
}

// A host without a profile inherits everything; an empty list goes back to
// inheriting; the host's deletion takes its profile.
func TestHostBackupProfileRepo(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var host int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('db-1', 'db-1') RETURNING id`).Scan(&host); err != nil {
		t.Fatal(err)
	}
	repo := store.NewHostBackupProfileRepo(d.SQL)
	p, err := repo.Get(ctx, host)
	if err != nil || p.Include != nil || p.Exclude != nil || p.UpdatedAt != nil {
		t.Fatalf("none = %+v %v", p, err)
	}
	if err := repo.Upsert(ctx, &models.HostBackupProfile{HostID: host, Exclude: []string{"/tmp", "/var/lib/postgresql"}}); err != nil {
		t.Fatal(err)
	}
	p, _ = repo.Get(ctx, host)
	if p.Include != nil || strings.Join(p.Exclude, " ") != "/tmp /var/lib/postgresql" || p.UpdatedAt == nil {
		t.Fatalf("saved = %+v", p)
	}
	repo.Upsert(ctx, &models.HostBackupProfile{HostID: host, Include: []string{"/etc"}})
	if p, _ = repo.Get(ctx, host); strings.Join(p.Include, " ") != "/etc" || p.Exclude != nil {
		t.Fatalf("replaced = %+v", p)
	}
	d.SQL.Exec(`DELETE FROM hosts WHERE id = $1`, host)
	var n int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM host_backup_profiles`).Scan(&n)
	if n != 0 {
		t.Fatal("the profile goes with its host")
	}
}

// Scans share the run tables but never show among the OS update runs; the
// scan list filters by requester, host and still running; a restart tells a
// scan's host it was cut, not that an update may still be running.
func TestOSUpdateRepo_ScanRuns(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var users []int64
	for _, n := range []string{"A", "B"} {
		u := &models.User{Username: n, DisplayName: n, Role: "editor", Email: n + "@example.com"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		users = append(users, u.ID)
	}
	a, b := users[0], users[1]
	var h1, h2 int64
	for n, id := range map[string]*int64{"vm1": &h1, "vm2": &h2} {
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ($1, $1) RETURNING id`, n).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	repo := store.NewOSUpdateRepo(d.SQL)
	check := &models.OSUpdateRun{Kind: "check", Concurrency: 1, RequestedBy: &a}
	scanA := &models.OSUpdateRun{Kind: models.RunKindScan, Concurrency: 10, RequestedBy: &a, AuthMethod: "key"}
	doneA := &models.OSUpdateRun{Kind: models.RunKindScan, Concurrency: 1, RequestedBy: &a}
	scanB := &models.OSUpdateRun{Kind: models.RunKindScan, Concurrency: 1, RequestedBy: &b}
	for _, x := range []struct { // in this order: the lists are id DESC
		run   *models.OSUpdateRun
		hosts []int64
	}{{check, []int64{h1}}, {scanA, []int64{h1, h2}}, {doneA, []int64{h1}}, {scanB, []int64{h2}}} {
		if err := repo.CreateRun(ctx, x.run, x.hosts); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.FinishRun(ctx, doneA.ID, models.OSRunDone); err != nil {
		t.Fatal(err)
	}
	ids := func(runs []models.OSUpdateRun, err error) []int64 {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		out := []int64{}
		for _, r := range runs {
			out = append(out, r.ID)
		}
		return out
	}
	eq := func(name string, got []int64, want ...int64) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s = %v, want %v", name, got, want)
			}
		}
	}
	eq("os update runs", ids(repo.ListRuns(ctx, 50, 0)), check.ID)
	eq("os update runs of vm2", ids(repo.ListRuns(ctx, 50, h2)))
	eq("all scans", ids(repo.ListScanRuns(ctx, store.ScanRunFilter{})), scanB.ID, doneA.ID, scanA.ID)
	eq("A's scans", ids(repo.ListScanRuns(ctx, store.ScanRunFilter{RequestedBy: a})), doneA.ID, scanA.ID)
	eq("A's active scans", ids(repo.ListScanRuns(ctx, store.ScanRunFilter{RequestedBy: a, Active: true})), scanA.ID)
	eq("scans of vm2", ids(repo.ListScanRuns(ctx, store.ScanRunFilter{HostID: h2})), scanB.ID, scanA.ID)
	eq("limit", ids(repo.ListScanRuns(ctx, store.ScanRunFilter{Limit: 1})), scanB.ID)

	got, err := repo.GetRun(ctx, scanA.ID)
	if err != nil || got.AuthMethod != "key" || got.Kind != models.RunKindScan || got.RequestedName != "A" {
		t.Fatalf("scan run = %+v %v", got, err)
	}

	repo.StartResult(ctx, scanA.ID, h1, "")
	repo.StartResult(ctx, check.ID, h1, "")
	if err := repo.MarkInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	for run, want := range map[int64]string{scanA.ID: "interrupted by a Bridge restart", check.ID: "the update may still be running"} {
		x, err := repo.GetResult(ctx, run, h1)
		if err != nil || x.Status != models.OSResultFailed || !strings.Contains(x.Reason, want) {
			t.Fatalf("run %d = %+v %v, want %q", run, x, err, want)
		}
	}
	if x, _ := repo.GetResult(ctx, scanA.ID, h1); strings.Contains(x.Reason, "update") {
		t.Fatalf("a scan's reason talks about an update: %q", x.Reason)
	}
}
