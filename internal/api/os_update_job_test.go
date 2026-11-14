package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// The run handlers with a fake host runner: validation, a run through to
// done (concurrency respected, results and audit rows written), and a cancel
// that skips the hosts not started while the running one finishes.
func TestOSUpdateRuns_Handlers(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	admin := &models.User{Username: "admin", DisplayName: "Admin", Role: "admin", Email: "a@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	var hostIDs []int64
	for i := range 4 {
		var id int64
		n := fmt.Sprintf("vm%d", i)
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES ($1, $1, '10.0.0.1', 'root') RETURNING id`, n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		hostIDs = append(hostIDs, id)
	}

	var running, peak atomic.Int32
	release := make(chan struct{})
	h := &sshHandlers{db: d}
	h.osHostRunner = func(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
		if n := running.Add(1); n > peak.Load() {
			peak.Store(n)
		}
		defer running.Add(-1)
		if run.Kind == sshtest.OSUpdatePackages {
			<-release // held until the test cancels
		}
		two, zero := 2, 0
		return models.OSUpdateResult{Status: models.OSResultSuccess, PkgManager: "apt", Upgradable: &two, SecurityUpgradable: &zero, OSAfter: "Ubuntu 24.04"}
	}
	mux := newEcho()
	h.registerRoutes(routeRegistrar{e: mux})
	call := func(method, path, body string) (int, string) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), admin)))
		b, _ := io.ReadAll(w.Result().Body)
		return w.Code, string(b)
	}
	waitRun := func(id int64, status string) models.OSUpdateRun {
		t.Helper()
		var run models.OSUpdateRun
		for range 100 {
			_, body := call("GET", fmt.Sprintf("/api/ssh/os-updates/runs/%d", id), "")
			json.Unmarshal([]byte(body), &run)
			if run.Status == status {
				return run
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("run %d never reached %s: %+v", id, status, run)
		return run
	}
	ids := func(xs ...int64) string { b, _ := json.Marshal(xs); return string(b) }

	for body, want := range map[string]int{
		`{"kind":"bogus","host_ids":[1]}`:                                   http.StatusBadRequest,
		`{"kind":"release","host_ids":` + ids(hostIDs[0], hostIDs[1]) + `}`: http.StatusBadRequest,
		`{"kind":"repair","host_ids":` + ids(hostIDs[0], hostIDs[1]) + `}`:  http.StatusBadRequest,
		`{"kind":"check","host_ids":[]}`:                                    http.StatusBadRequest,
		`{"kind":"check","host_ids":[1],"concurrency":11}`:                  http.StatusBadRequest,
		`{"kind":"check","host_ids":` + ids(hostIDs[0], 9999) + `}`:         http.StatusNotFound,
	} {
		if code, _ := call("POST", "/api/ssh/os-updates/runs", body); code != want {
			t.Errorf("%s = %d, want %d", body, code, want)
		}
	}

	// A release upgrade: one host, one at a time, always rebooting.
	code, body := call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","reboot":false,"concurrency":5,"host_ids":`+ids(hostIDs[0])+`}`)
	var rel models.OSUpdateRun
	json.Unmarshal([]byte(body), &rel)
	if code != http.StatusAccepted || !rel.Reboot || rel.Concurrency != 1 {
		t.Fatalf("release = %d %+v", code, rel)
	}
	waitRun(rel.ID, models.OSRunDone)

	// A repair: one host, one at a time, rebooting only when asked.
	code, body = call("POST", "/api/ssh/os-updates/runs", `{"kind":"repair","reboot":false,"concurrency":5,"host_ids":`+ids(hostIDs[0])+`}`)
	var fix models.OSUpdateRun
	json.Unmarshal([]byte(body), &fix)
	if code != http.StatusAccepted || fix.Reboot || fix.Concurrency != 1 || fix.Kind != sshtest.OSUpdateRepair {
		t.Fatalf("repair = %d %s", code, body)
	}
	waitRun(fix.ID, models.OSRunDone)

	// A check on every host, two at a time; reboot is ignored for a check.
	code, body = call("POST", "/api/ssh/os-updates/runs", `{"kind":"check","reboot":true,"concurrency":2,"host_ids":`+ids(hostIDs...)+`}`)
	if code != http.StatusAccepted {
		t.Fatalf("start = %d %s", code, body)
	}
	var run models.OSUpdateRun
	json.Unmarshal([]byte(body), &run)
	if run.Reboot {
		t.Error("a check never reboots")
	}
	done := waitRun(run.ID, models.OSRunDone)
	if done.Counts["success"] != 4 || len(done.Results) != 4 || done.Results[0].OSAfter != "Ubuntu 24.04" || done.Results[0].FinishedAt == nil {
		t.Fatalf("done = %+v", done)
	}
	if peak.Load() > 2 {
		t.Errorf("peak concurrency = %d, want ≤ 2", peak.Load())
	}
	var logs int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM host_operation_logs WHERE operation_type = 'os-update:check' AND status = 'success'`).Scan(&logs)
	if logs != 4 {
		t.Errorf("audit rows = %d, want 4", logs)
	}
	if _, body := call("GET", "/api/ssh/os-updates/latest", ""); !strings.Contains(body, `"upgradable":2`) {
		t.Errorf("latest = %s", body)
	}

	// Packages one at a time; cancel while the first host is held.
	_, body = call("POST", "/api/ssh/os-updates/runs", `{"kind":"packages","concurrency":1,"host_ids":`+ids(hostIDs...)+`}`)
	json.Unmarshal([]byte(body), &run)
	for running.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if code, body := call("POST", fmt.Sprintf("/api/ssh/os-updates/runs/%d/cancel", run.ID), ""); code != http.StatusOK || !strings.Contains(body, `"skipped":3`) {
		t.Fatalf("cancel = %d %s", code, body)
	}
	close(release)
	cancelled := waitRun(run.ID, models.OSRunCancelled)
	if cancelled.Counts["success"] != 1 || cancelled.Counts["skipped"] != 3 {
		t.Fatalf("cancelled = %+v", cancelled.Counts)
	}
	if code, _ := call("GET", "/api/ssh/os-updates/runs/9999", ""); code != http.StatusNotFound {
		t.Errorf("missing run = %d", code)
	}
	if _, body := call("GET", "/api/ssh/os-updates/runs", ""); !strings.Contains(body, `"kind":"packages"`) {
		t.Errorf("list = %s", body)
	}
}

func TestUpgradedCount(t *testing.T) {
	n := func(v int) *int { return &v }
	cases := []struct {
		kind string
		rep  sshtest.OSUpdateReport
		want *int
	}{
		{sshtest.OSUpdateCheck, sshtest.OSUpdateReport{Upgradable: n(5), Remaining: n(0)}, nil},
		{sshtest.OSUpdatePackages, sshtest.OSUpdateReport{Upgradable: n(5), Remaining: n(1)}, n(4)},
		{sshtest.OSUpdatePackages, sshtest.OSUpdateReport{Upgradable: n(5)}, nil},
		{sshtest.OSUpdateSecurity, sshtest.OSUpdateReport{Upgradable: n(5), Security: n(2), RemainingSecurity: n(0)}, n(2)},
		{sshtest.OSUpdatePackages, sshtest.OSUpdateReport{Upgradable: n(1), Remaining: n(3)}, n(0)}, // new packages appeared
	}
	for _, c := range cases {
		got := upgradedCount(c.kind, c.rep)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s %+v = %v, want %v", c.kind, c.rep, got, c.want)
		}
	}
}

// streamTo pipes produce into consume without holding the whole stream, and
// either side failing fails the backup.
func TestStreamTo(t *testing.T) {
	n, err := streamTo(
		func(r io.Reader) (int64, error) { b, err := io.ReadAll(r); return int64(len(b)), err },
		func(w io.Writer) error { _, err := io.WriteString(w, strings.Repeat("x", 1<<20)); return err },
	)
	if err != nil || n != 1<<20 {
		t.Fatalf("ok = %d %v", n, err)
	}
	// The producer fails halfway: the consumer sees the error, nothing is kept.
	_, err = streamTo(
		func(r io.Reader) (int64, error) { b, err := io.ReadAll(r); return int64(len(b)), err },
		func(w io.Writer) error { io.WriteString(w, "partial"); return errors.New("tar exit 2") },
	)
	if err == nil || err.Error() != "tar exit 2" {
		t.Fatalf("producer failure = %v", err)
	}
	// The consumer (MinIO) gives up: the producer is unblocked and the
	// consumer's error is reported.
	_, err = streamTo(
		func(r io.Reader) (int64, error) { return 0, errors.New("bucket full") },
		func(w io.Writer) error {
			for range 100 {
				if _, err := io.WriteString(w, strings.Repeat("y", 1<<16)); err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "bucket full") {
		t.Fatalf("consumer failure = %v", err)
	}
}

// Backup options: refused without MinIO, dropped for a check; with MinIO set
// up a release always backs up, and a kept backup gets a download link.
func TestOSUpdateRuns_Backup(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	admin := &models.User{Username: "admin", DisplayName: "Admin", Role: "admin", Email: "a@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES ('vm1', 'vm1', '10.0.0.1', 'root') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	h := &sshHandlers{db: d, osHostRunner: func(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
		size := int64(2048)
		x := models.OSUpdateResult{Status: models.OSResultFailed, Reason: "apt broke"}
		if run.Backup {
			x.BackupKey, x.BackupSize, x.BackupStatus = "bridge/os-backups/vm1/x-run1.tar.zst", &size, models.OSBackupDone
		}
		return x
	}}
	mux := newEcho()
	h.registerRoutes(routeRegistrar{e: mux})
	call := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), admin)))
		var out map[string]any
		json.NewDecoder(w.Result().Body).Decode(&out)
		return w.Code, out
	}
	hosts := fmt.Sprintf("[%d]", hostID)

	if _, cfg := call("GET", "/api/ssh/os-updates/config", ""); cfg["backup_available"] != false {
		t.Fatalf("config without MinIO = %v", cfg)
	}
	if code, out := call("POST", "/api/ssh/os-updates/runs", `{"kind":"packages","backup":true,"host_ids":`+hosts+`}`); code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), "MinIO") {
		t.Fatalf("backup without MinIO = %d %v", code, out)
	}
	if code, out := call("POST", "/api/ssh/os-updates/runs", `{"kind":"check","backup":true,"keep_backup":true,"host_ids":`+hosts+`}`); code != http.StatusAccepted || out["backup"] != false || out["keep_backup"] != false {
		t.Fatalf("check ignores backup = %d %v", code, out)
	}

	// MinIO set up on a port that refuses: links are signed offline, and the
	// lifecycle rule a backup run re-applies fails fast and is only logged.
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"minio_enabled": "true", "minio_endpoint": "127.0.0.1:1", "minio_access_key": "ak", "minio_bucket": "ops"} {
		settings.Set(ctx, k, v)
	}
	if err := store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "minio_secret_key", "sk"); err != nil {
		t.Fatal(err)
	}
	if _, cfg := call("GET", "/api/ssh/os-updates/config", ""); cfg["backup_available"] != true {
		t.Fatalf("config with MinIO = %v", cfg)
	}
	// A release with no backup to go back to can't skip its own.
	if code, out := call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":false,"host_ids":`+hosts+`}`); code != http.StatusBadRequest {
		t.Fatalf("release without any backup = %d %v", code, out)
	}
	code, run := call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":true,"host_ids":`+hosts+`}`)
	if code != http.StatusAccepted || run["backup"] != true {
		t.Fatalf("release with backup = %d %v", code, run)
	}
	id := int64(run["id"].(float64))
	for range 100 {
		if x, _ := store.NewOSUpdateRepo(d.SQL).GetResult(ctx, id, hostID); x != nil && x.FinishedAt != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	link := fmt.Sprintf("/api/ssh/os-updates/runs/%d/hosts/%d/backup", id, hostID)
	code, out := call("GET", link, "")
	if code != http.StatusOK || !strings.Contains(fmt.Sprint(out["url"]), "127.0.0.1:1/ops/bridge/os-backups/vm1/") || out["expires_in"] != float64(3600) {
		t.Fatalf("link = %d %v", code, out)
	}
	// Marked deleted (as after a manual delete): no more link.
	store.NewOSUpdateRepo(d.SQL).SetBackupStatus(ctx, id, hostID, models.OSBackupDeleted)
	if code, _ := call("GET", link, ""); code != http.StatusNotFound {
		t.Fatalf("deleted backup link = %d", code)
	}
	if code, _ := call("GET", fmt.Sprintf("/api/ssh/os-updates/runs/%d/hosts/%d/backup", id, 9999), ""); code != http.StatusNotFound {
		t.Fatalf("other host = %d", code)
	}
}

// Restores: validated, only from a kept backup, onto its host; in place
// they back the chosen paths up first and keep that copy.
func TestOSRestore_Handler(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	admin := &models.User{Username: "admin", DisplayName: "Admin", Role: "admin", Email: "a@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES ('vm1', 'vm1', '10.0.0.1', 'root') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	repo := store.NewOSUpdateRepo(d.SQL)
	// A finished package run that kept vm1's backup.
	src := &models.OSUpdateRun{Kind: "packages", Concurrency: 1, Backup: true, KeepBackup: true}
	repo.CreateRun(ctx, src, []int64{hostID})
	repo.StartResult(ctx, src.ID, hostID, "")
	size := int64(10)
	repo.FinishResult(ctx, &models.OSUpdateResult{RunID: src.ID, HostID: hostID, Status: models.OSResultFailed,
		BackupKey: "bridge/os-backups/vm1/x.tar.zst", BackupSize: &size, BackupStatus: models.OSBackupDone})

	ran := make(chan models.OSUpdateRun, 4)
	h := &sshHandlers{db: d, osHostRunner: func(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
		ran <- run
		return models.OSUpdateResult{Status: models.OSResultSuccess}
	}}
	mux := newEcho()
	h.registerRoutes(routeRegistrar{e: mux})
	call := func(body string) (int, map[string]any) {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/ssh/os-updates/restores", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), admin)))
		var out map[string]any
		json.NewDecoder(w.Result().Body).Decode(&out)
		return w.Code, out
	}
	body := func(mode string, paths string) string {
		return fmt.Sprintf(`{"run_id":%d,"host_id":%d,"mode":%q,"paths":%s}`, src.ID, hostID, mode, paths)
	}

	// MinIO not configured yet.
	if code, out := call(body("staging", "[]")); code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), "MinIO") {
		t.Fatalf("no MinIO = %d %v", code, out)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"minio_enabled": "true", "minio_endpoint": "127.0.0.1:1", "minio_access_key": "ak", "minio_bucket": "ops"} {
		settings.Set(ctx, k, v)
	}
	store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "minio_secret_key", "sk")

	for b, want := range map[string]int{
		body("full", "[]"):                  http.StatusBadRequest,
		body("paths", "[]"):                 http.StatusBadRequest,
		body("paths", `["/"]`):              http.StatusBadRequest,
		body("paths", `["etc"]`):            http.StatusBadRequest,
		body("staging", `["/etc/../root"]`): http.StatusBadRequest,
		fmt.Sprintf(`{"run_id":%d,"host_id":%d,"mode":"staging"}`, src.ID+99, hostID): http.StatusNotFound,
	} {
		if code, out := call(b); code != want {
			t.Errorf("%s = %d %v, want %d", b, code, out, want)
		}
	}

	// Review folder: no safety backup.
	code, out := call(body("staging", "[]"))
	if code != http.StatusAccepted || out["kind"] != "restore" || out["restore_mode"] != "staging" || out["backup"] != false {
		t.Fatalf("staging = %d %v", code, out)
	}
	if got := <-ran; *got.SourceRunID != src.ID || *got.SourceHostID != hostID || got.Concurrency != 1 {
		t.Fatalf("runner got %+v", got)
	}

	// In place: the chosen paths are backed up first and that copy is kept.
	code, out = call(body("paths", `["/etc/nginx", " /opt/app "]`))
	if code != http.StatusAccepted || out["backup"] != true || out["keep_backup"] != true {
		t.Fatalf("paths = %d %v", code, out)
	}
	if got := <-ran; strings.Join(got.RestorePaths, "|") != "/etc/nginx|/opt/app" || got.RestoreMode != "paths" {
		t.Fatalf("runner got %+v", got)
	}
	if run, _ := repo.GetRun(ctx, int64(out["id"].(float64))); strings.Join(run.RestorePaths, "|") != "/etc/nginx|/opt/app" {
		t.Fatalf("stored paths = %q", run.RestorePaths)
	}

	// A backup no longer kept can't be restored.
	repo.SetBackupStatus(ctx, src.ID, hostID, models.OSBackupDeleted)
	if code, _ := call(body("staging", "[]")); code != http.StatusNotFound {
		t.Fatalf("deleted backup = %d", code)
	}
}

// Host backups: the profile (partial, [] inherits), the run's override over
// it, backup-only runs (kept, no reboot), the kept list and the settings
// groups (the connection apart from the feature).
func TestHostBackups_Handlers(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	admin := &models.User{Username: "admin", DisplayName: "Admin", Role: "admin", Email: "a@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES ('db1', 'db1', '10.0.0.2', 'root') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	ran := make(chan models.OSUpdateRun, 4)
	h := &sshHandlers{db: d, osHostRunner: func(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
		ran <- run
		size := int64(4096)
		return models.OSUpdateResult{Status: models.OSResultSuccess, BackupKey: "bridge/os-backups/db1/x.tar.zst", BackupSize: &size,
			BackupStatus: models.OSBackupDone, BackupInclude: run.BackupInclude}
	}}
	mux := newEcho()
	h.registerRoutes(routeRegistrar{e: mux})
	call := func(method, path, body string) (int, string) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), admin)))
		return w.Code, w.Body.String()
	}
	type view struct {
		Profile   models.HostBackupProfile `json:"profile"`
		Effective struct{ Include, Exclude []string }
	}
	profile := func(method, body string) (int, view) {
		t.Helper()
		code, raw := call(method, "/api/hosts/db1/backup-profile", body)
		var v view
		json.Unmarshal([]byte(raw), &v)
		return code, v
	}

	// No profile: everything inherited.
	if code, v := profile("GET", ""); code != http.StatusOK || v.Profile.Exclude != nil || strings.Join(v.Effective.Include, " ") != "/" ||
		strings.Join(v.Effective.Exclude, " ") != service.DefaultBackupExclude {
		t.Fatalf("no profile = %d %+v", code, v)
	}
	if code, v := profile("PUT", `{"exclude":["/tmp /var/lib/postgresql"]}`); code != http.StatusOK ||
		strings.Join(v.Effective.Exclude, " ") != "/tmp /var/lib/postgresql" || strings.Join(v.Effective.Include, " ") != "/" {
		t.Fatalf("exclude = %d %+v", code, v)
	}
	// Partial: include set, exclude kept.
	if _, v := profile("PUT", `{"include":["/etc","/srv"]}`); strings.Join(v.Profile.Include, " ") != "/etc /srv" ||
		strings.Join(v.Profile.Exclude, " ") != "/tmp /var/lib/postgresql" {
		t.Fatalf("partial = %+v", v)
	}
	// [] goes back to the default.
	if _, v := profile("PUT", `{"include":[]}`); v.Profile.Include != nil || strings.Join(v.Effective.Include, " ") != "/" {
		t.Fatalf("reset = %+v", v)
	}
	if code, _ := profile("PUT", `{"include":["etc"]}`); code != http.StatusBadRequest {
		t.Fatalf("relative = %d", code)
	}
	if code, _ := call("GET", "/api/hosts/nope/backup-profile", ""); code != http.StatusNotFound {
		t.Fatalf("unknown host = %d", code)
	}

	// The run's override wins over the profile, list by list.
	defaults := service.LoadBackupDefaults(ctx, d.SQL)
	paths, err := h.backupPaths(ctx, models.OSUpdateRun{BackupInclude: []string{"/opt"}}, hostID, defaults)
	if err != nil || strings.Join(paths.Include, " ") != "/opt" || strings.Join(paths.Exclude, " ") != "/tmp /var/lib/postgresql" {
		t.Fatalf("effective = %+v %v", paths, err)
	}

	hosts := fmt.Sprintf("[%d]", hostID)
	if code, raw := call("POST", "/api/ssh/os-updates/runs", `{"kind":"backup","host_ids":`+hosts+`}`); code != http.StatusBadRequest || !strings.Contains(raw, "MinIO") {
		t.Fatalf("backup without MinIO = %d %s", code, raw)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"minio_enabled": "true", "minio_endpoint": "127.0.0.1:1", "minio_access_key": "ak", "minio_bucket": "ops", "osbackup_retention_days": "7"} {
		settings.Set(ctx, k, v)
	}
	store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "minio_secret_key", "sk")
	// No kept backup yet: a version change must take one.
	if code, raw := call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":false,"host_ids":`+hosts+`}`); code != http.StatusBadRequest || !strings.Contains(raw, "no kept backup") {
		t.Fatalf("release without any backup = %d %s", code, raw)
	}
	if code, _ := call("POST", "/api/ssh/os-updates/runs", `{"kind":"backup","backup_include":["etc"],"host_ids":`+hosts+`}`); code != http.StatusBadRequest {
		t.Fatalf("relative override = %d", code)
	}
	code, raw := call("POST", "/api/ssh/os-updates/runs", `{"kind":"backup","reboot":true,"backup_include":["/etc /opt"],"host_ids":`+hosts+`}`)
	var run models.OSUpdateRun
	json.Unmarshal([]byte(raw), &run)
	if code != http.StatusAccepted || !run.Backup || !run.KeepBackup || run.Reboot || strings.Join(run.BackupInclude, " ") != "/etc /opt" {
		t.Fatalf("backup run = %d %s", code, raw)
	}
	<-ran
	for range 100 {
		if x, _ := store.NewOSUpdateRepo(d.SQL).GetResult(ctx, run.ID, hostID); x != nil && x.FinishedAt != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Kept backups: newest per host with its expiry; one host's list keeps
	// what MinIO can't confirm (unreachable here).
	var kept []models.OSUpdateResult
	_, raw = call("GET", "/api/ssh/os-updates/backups", "")
	json.Unmarshal([]byte(raw), &kept)
	if len(kept) != 1 || kept[0].RunKind != "backup" || kept[0].ExpiresAt == nil ||
		kept[0].ExpiresAt.Sub(*kept[0].FinishedAt) != 7*24*time.Hour || strings.Join(kept[0].BackupInclude, " ") != "/etc /opt" {
		t.Fatalf("latest = %s", raw)
	}
	_, raw = call("GET", fmt.Sprintf("/api/ssh/os-updates/backups?host_id=%d", hostID), "")
	if json.Unmarshal([]byte(raw), &kept); len(kept) != 1 {
		t.Fatalf("host backups = %s", raw)
	}
	if code, _ := call("GET", "/api/ssh/os-updates/backups?host_id=9999", ""); code != http.StatusNotFound {
		t.Fatalf("unknown host = %d", code)
	}
	var runs []models.OSUpdateRun
	_, raw = call("GET", fmt.Sprintf("/api/ssh/os-updates/runs?host_id=%d", hostID), "")
	if json.Unmarshal([]byte(raw), &runs); len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("host runs = %s", raw)
	}
	if _, raw = call("GET", "/api/ssh/os-updates/runs?host_id=9999", ""); strings.TrimSpace(raw) != "[]" {
		t.Fatalf("other host runs = %s", raw)
	}

	// With a kept backup, a version change may go without a new one, and
	// update the packages first. update_first is a release's only.
	code, raw = call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":false,"update_first":true,"host_ids":`+hosts+`}`)
	var rel models.OSUpdateRun
	if json.Unmarshal([]byte(raw), &rel); code != http.StatusAccepted || rel.Backup || !rel.UpdateFirst || !rel.Reboot {
		t.Fatalf("release on the kept backup = %d %s", code, raw)
	}
	if got := <-ran; !got.UpdateFirst || got.Backup {
		t.Fatalf("runner got %+v", got)
	}
	code, raw = call("POST", "/api/ssh/os-updates/runs", `{"kind":"packages","update_first":true,"host_ids":`+hosts+`}`)
	rel = models.OSUpdateRun{}
	if json.Unmarshal([]byte(raw), &rel); code != http.StatusAccepted || rel.UpdateFirst {
		t.Fatalf("packages with update_first = %d %s", code, raw)
	}
	<-ran
	for range 100 {
		if x, _ := store.NewOSUpdateRepo(d.SQL).GetResult(ctx, rel.ID, hostID); x != nil && x.FinishedAt != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// A host with no kept backup: a version change without a new one is
	// refused, unless the caller says to go without any backup.
	var bare int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES ('db2', 'db2', '10.0.0.3', 'root') RETURNING id`).Scan(&bare); err != nil {
		t.Fatal(err)
	}
	bareHosts := fmt.Sprintf("[%d]", bare)
	if code, raw := call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":false,"host_ids":`+bareHosts+`}`); code != http.StatusBadRequest || !strings.Contains(raw, "no kept backup") {
		t.Fatalf("release without any backup = %d %s", code, raw)
	}
	code, raw = call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":false,"allow_no_backup":true,"host_ids":`+bareHosts+`}`)
	rel = models.OSUpdateRun{}
	if json.Unmarshal([]byte(raw), &rel); code != http.StatusAccepted || rel.Backup || rel.KeepBackup || !rel.Reboot {
		t.Fatalf("release without a backup, allowed = %d %s", code, raw)
	}
	if got := <-ran; got.Backup || got.Kind != sshtest.OSUpdateRelease {
		t.Fatalf("runner got %+v", got)
	}
	for range 100 {
		if x, _ := store.NewOSUpdateRepo(d.SQL).GetResult(ctx, rel.ID, bare); x != nil && x.FinishedAt != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The connection's group holds no feature settings, and the reverse.
	for _, k := range integrationGroups["minio"] {
		if strings.HasPrefix(k, "osbackup_") {
			t.Fatalf("%s belongs to the osbackup group", k)
		}
	}
	if len(integrationGroups["osbackup"]) != 4 {
		t.Fatalf("osbackup group = %v", integrationGroups["osbackup"])
	}
}

// A failed run's reason names what dpkg failed on.
func TestExitReason(t *testing.T) {
	rep := sshtest.ParseOSUpdateLog("@@error do-release-upgrade failed\n@@dpkg_broken proftpd-core\n" +
		"@@dpkg_error proftpd-core: installed proftpd-core package post-installation script subprocess returned error exit status 1\n" +
		"@@dpkg_error proftpd-mod-crypto: dependency problems - leaving unconfigured\n")
	want := "do-release-upgrade failed; dpkg could not configure: proftpd-core (installed proftpd-core package post-installation script " +
		"subprocess returned error exit status 1); proftpd-mod-crypto (dependency problems - leaving unconfigured)"
	if got := exitReason(rep, 1); got != want {
		t.Fatalf("got %q", got)
	}
	if got := exitReason(sshtest.ParseOSUpdateLog("@@dpkg_broken proftpd-core\n@@dpkg_broken fail2ban\n"), 100); got != "exit 100; dpkg left half configured: proftpd-core, fail2ban" {
		t.Fatalf("broken only = %q", got)
	}
	if got := exitReason(sshtest.OSUpdateReport{}, 0); got != "the script ended without a result" {
		t.Fatalf("no result = %q", got)
	}
}
