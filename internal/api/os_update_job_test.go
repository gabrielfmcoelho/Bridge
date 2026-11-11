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

	// MinIO set up (nothing is contacted: links are signed offline).
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"minio_enabled": "true", "minio_endpoint": "10.0.122.76:9000", "minio_access_key": "ak", "minio_bucket": "ops"} {
		settings.Set(ctx, k, v)
	}
	if err := store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "minio_secret_key", "sk"); err != nil {
		t.Fatal(err)
	}
	if _, cfg := call("GET", "/api/ssh/os-updates/config", ""); cfg["backup_available"] != true {
		t.Fatalf("config with MinIO = %v", cfg)
	}
	code, run := call("POST", "/api/ssh/os-updates/runs", `{"kind":"release","backup":false,"host_ids":`+hosts+`}`)
	if code != http.StatusAccepted || run["backup"] != true {
		t.Fatalf("release forces backup = %d %v", code, run)
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
	if code != http.StatusOK || !strings.Contains(fmt.Sprint(out["url"]), "10.0.122.76:9000/ops/bridge/os-backups/vm1/") || out["expires_in"] != float64(3600) {
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
