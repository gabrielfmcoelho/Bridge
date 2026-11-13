package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// scanEnv is a database, its users and an HTTP caller acting as one of them
// (with that user's entidade scope, as RequireAuth would attach it).
type scanEnv struct {
	t               *testing.T
	d               *database.DB
	h               *sshHandlers
	mux             http.Handler
	admin, edA, edB *models.User
	entidade        int64
}

func newScanEnv(t *testing.T) *scanEnv {
	t.Helper()
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	e := &scanEnv{t: t, d: d, h: &sshHandlers{db: d}}
	for _, u := range []**models.User{&e.admin, &e.edA, &e.edB} {
		n := map[**models.User]string{&e.admin: "admin", &e.edA: "ed-a", &e.edB: "ed-b"}[u]
		role := "editor"
		if n == "admin" {
			role = "admin"
		}
		*u = &models.User{Username: n, DisplayName: n, Role: role, Email: n + "@example.com"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, *u); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.SQL.QueryRow(`INSERT INTO entidades (name, slug) VALUES ('E', 'e') RETURNING id`).Scan(&e.entidade); err != nil {
		t.Fatal(err)
	}
	mux := newEcho()
	e.h.registerRoutes(routeRegistrar{e: mux})
	e.mux = mux
	return e
}

// host adds a host the editors' entidade sees (visible) or only admins do.
func (e *scanEnv) host(name, address, port string, visible bool) int64 {
	e.t.Helper()
	var id int64
	if err := e.d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, port, ssh_user) VALUES ($1, $1, $2, $3, 'root') RETURNING id`,
		name, address, port).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	if visible {
		if _, err := e.d.SQL.Exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES ('host', ?, ?, 'creator')`, id, e.entidade); err != nil {
			e.t.Fatal(err)
		}
	}
	return id
}

func (e *scanEnv) call(u *models.User, method, path, body string) (int, string) {
	e.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := auth.WithUser(r.Context(), u)
	scope := store.Scope{Admin: true}
	if u.Role != "admin" {
		scope = store.Scope{EntidadeIDs: []int64{e.entidade}, PrimaryEntidadeID: e.entidade}
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r.WithContext(store.WithScope(ctx, scope)))
	return w.Code, w.Body.String()
}

func (e *scanEnv) start(u *models.User, body string) models.OSUpdateRun {
	e.t.Helper()
	code, out := e.call(u, "POST", "/api/ssh/scans", body)
	var run models.OSUpdateRun
	if code != http.StatusAccepted || json.Unmarshal([]byte(out), &run) != nil {
		e.t.Fatalf("start %s = %d %s", body, code, out)
	}
	return run
}

// wait polls the run (as admin) until it leaves running.
func (e *scanEnv) wait(id int64) models.OSUpdateRun {
	e.t.Helper()
	var run models.OSUpdateRun
	for range 200 {
		_, out := e.call(e.admin, "GET", fmt.Sprintf("/api/ssh/scans/%d", id), "")
		json.Unmarshal([]byte(out), &run)
		if run.Status != "" && run.Status != models.OSRunRunning {
			return run
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.t.Fatalf("run %d never finished: %+v", id, run)
	return run
}

func idsJSON(xs ...int64) string { b, _ := json.Marshal(xs); return string(b) }

// The routes with a fake runner: validation, visibility of hosts (404) and of
// runs (only the requester and admins), the list, cancel, and that scans stay
// out of the OS update runs.
func TestScanRuns_Handlers(t *testing.T) {
	e := newScanEnv(t)
	vm0, vm1, hidden := e.host("vm0", "10.0.0.1", "22", true), e.host("vm1", "10.0.0.2", "22", true), e.host("vm2", "10.0.0.3", "22", false)

	hold := make(chan struct{})
	var mu sync.Mutex
	held := false
	e.h.osHostRunner = func(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
		mu.Lock()
		h := held
		mu.Unlock()
		if h && host.ID == vm0 {
			<-hold
		}
		return models.OSUpdateResult{Status: models.OSResultSuccess, OSAfter: "Ubuntu 24.04", Output: `{"scan_saved":true}`}
	}

	many := make([]int64, osUpdateMaxHosts+1)
	for body, want := range map[string]int{
		`{"host_ids":[]}`:                                     http.StatusBadRequest,
		`{"host_ids":` + idsJSON(many...) + `}`:               http.StatusBadRequest,
		`{"host_ids":` + idsJSON(vm0) + `,"concurrency":11}`:  http.StatusBadRequest,
		`{"host_ids":` + idsJSON(vm0) + `,"method":"telnet"}`: http.StatusBadRequest,
		`{"host_ids":` + idsJSON(vm0, hidden) + `}`:           http.StatusNotFound, // invisible: 404, not 403
		`{"host_ids":` + idsJSON(vm0, 99999) + `}`:            http.StatusNotFound,
	} {
		if code, out := e.call(e.edA, "POST", "/api/ssh/scans", body); code != want {
			t.Errorf("%.60s = %d %s, want %d", body, code, out, want)
		}
	}

	run := e.start(e.edA, `{"host_ids":`+idsJSON(vm0, vm1)+`,"method":"key"}`)
	if run.Kind != models.RunKindScan || run.Concurrency != scanDefaultConcurrency || run.AuthMethod != "key" ||
		run.RequestedBy == nil || *run.RequestedBy != e.edA.ID {
		t.Fatalf("run = %+v", run)
	}
	done := e.wait(run.ID)
	if done.Status != models.OSRunDone || len(done.Results) != 2 || done.Counts["success"] != 2 || done.Results[0].OSAfter != "Ubuntu 24.04" {
		t.Fatalf("done = %+v", done)
	}

	// Someone else's run doesn't exist for an editor; it does for an admin.
	path := fmt.Sprintf("/api/ssh/scans/%d", run.ID)
	for _, c := range []struct {
		u            *models.User
		method, path string
		want         int
	}{
		{e.edA, "GET", path, http.StatusOK},
		{e.edB, "GET", path, http.StatusNotFound},
		{e.edB, "POST", path + "/cancel", http.StatusNotFound},
		{e.admin, "GET", path, http.StatusOK},
		{e.edA, "GET", "/api/ssh/scans/999999", http.StatusNotFound},
	} {
		if code, out := e.call(c.u, c.method, c.path, ""); code != c.want {
			t.Errorf("%s %s %s = %d %s, want %d", c.u.Username, c.method, c.path, code, out, c.want)
		}
	}
	// An OS update run isn't a scan, even for its admin.
	var osRun int64
	e.d.SQL.QueryRow(`INSERT INTO os_update_runs (kind, requested_by) VALUES ('check', ?) RETURNING id`, e.admin.ID).Scan(&osRun)
	if code, _ := e.call(e.admin, "GET", fmt.Sprintf("/api/ssh/scans/%d", osRun), ""); code != http.StatusNotFound {
		t.Errorf("an OS update run read as a scan = %d", code)
	}

	// A run that holds vm0, one host at a time: cancel skips vm1, vm0 finishes.
	mu.Lock()
	held = true
	mu.Unlock()
	running := e.start(e.edA, `{"host_ids":`+idsJSON(vm0, vm1)+`,"concurrency":1}`)
	list := func(u *models.User, q string) []int64 {
		t.Helper()
		_, out := e.call(u, "GET", "/api/ssh/scans"+q, "")
		var runs []models.OSUpdateRun
		if err := json.Unmarshal([]byte(out), &runs); err != nil {
			t.Fatalf("list %s = %s", q, out)
		}
		ids := []int64{}
		for _, r := range runs {
			ids = append(ids, r.ID)
		}
		return ids
	}
	if got := list(e.edA, "?active=1"); len(got) != 1 || got[0] != running.ID {
		t.Fatalf("A's active runs = %v", got)
	}
	if got := list(e.edA, fmt.Sprintf("?active=1&host_id=%d", vm1)); len(got) != 1 {
		t.Fatalf("A's active runs of vm1 = %v", got)
	}
	if got := list(e.edB, ""); len(got) != 0 {
		t.Fatalf("B sees %v", got)
	}
	if got := list(e.admin, ""); len(got) != 2 {
		t.Fatalf("admin sees %v", got)
	}
	time.Sleep(100 * time.Millisecond) // vm0 is running
	if code, out := e.call(e.edA, "POST", fmt.Sprintf("/api/ssh/scans/%d/cancel", running.ID), ""); code != http.StatusOK {
		t.Fatalf("cancel = %d %s", code, out)
	}
	close(hold)
	cancelled := e.wait(running.ID)
	if cancelled.Status != models.OSRunCancelled || cancelled.Counts["success"] != 1 || cancelled.Counts["skipped"] != 1 {
		t.Fatalf("cancelled = %+v", cancelled)
	}
	if got := list(e.edA, "?active=1"); len(got) != 0 {
		t.Fatalf("still active: %v", got)
	}

	// Scans stay out of the OS update views, and write no os-update audit row.
	_, out := e.call(e.admin, "GET", "/api/ssh/os-updates/runs", "")
	if strings.Contains(out, `"kind":"scan"`) {
		t.Fatalf("OS update runs list scans: %s", out)
	}
	var n int
	e.d.SQL.QueryRow(`SELECT COUNT(*) FROM host_operation_logs WHERE operation_type LIKE 'os-update:%'`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d os-update audit rows for scans", n)
	}
}

// The real runner against an in-process sshd: the scan's own bookkeeping
// (test status, operation log, pinned key, stored scan), a changed host key
// refused once, no credentials skipped, a refused connection retried.
func TestScanJob_Runner(t *testing.T) {
	e := newScanEnv(t)
	ctx := context.Background()
	scanRetryWait = 10 * time.Millisecond
	t.Cleanup(func() { scanRetryWait = 10 * time.Second })
	key, other := testSigner(t, "ed25519"), testSigner(t, "ecdsa")
	srv := newFakeSSHD(t, key)

	ok := e.host("ok", srv.host, srv.port, true)
	changed := e.host("changed", srv.host, srv.port, true)
	bare := e.host("bare", srv.host, srv.port, true)
	// A port that accepts and hangs up at once: retryable, so three tries.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var dropped atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			dropped.Add(1)
			c.Close()
		}
	}()
	dh, dp, _ := net.SplitHostPort(ln.Addr().String())
	drop := e.host("drop", dh, dp, true)
	for _, id := range []int64{ok, changed, drop} {
		if _, err := e.d.SQL.Exec(`UPDATE hosts SET has_password = TRUE WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
		if err := vault.HostSetPassword(ctx, e.d, id, e.admin.ID, "pw"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.d.SQL.Exec(`UPDATE hosts SET ssh_host_key = ?, ssh_host_key_at = now() WHERE id = ?`,
		strings.TrimSpace(string(ssh.MarshalAuthorizedKey(other.PublicKey()))), changed); err != nil {
		t.Fatal(err)
	}

	run := e.wait(e.start(e.edA, `{"host_ids":`+idsJSON(ok, changed, bare, drop)+`,"concurrency":1}`).ID)
	res := map[int64]models.OSUpdateResult{}
	for _, x := range run.Results {
		res[x.HostID] = x
	}

	if x := res[ok]; x.Status != models.OSResultSuccess || !strings.Contains(x.Output, `"scan_saved":true`) {
		t.Errorf("ok = %+v", x)
	}
	if x := res[changed]; x.Status != models.OSResultFailed || !strings.Contains(x.Reason, "host key changed") {
		t.Errorf("changed = %+v", x)
	}
	if x := res[bare]; x.Status != models.OSResultSkipped || !strings.Contains(x.Reason, "no password or key") {
		t.Errorf("bare = %+v", x)
	}
	if x := res[drop]; x.Status != models.OSResultFailed || dropped.Load() != scanAttempts {
		t.Errorf("drop = %+v after %d tries, want %d", x, dropped.Load(), scanAttempts)
	}
	// ok, then changed (refused at the handshake, never retried): two connections.
	if n := srv.conns.Load(); n != 2 {
		t.Errorf("sshd saw %d connections, want 2", n)
	}

	type hostState struct {
		status       string
		failures     int
		pinned, logs int
		scans        int
	}
	state := func(id int64) hostState {
		t.Helper()
		var s hostState
		if err := e.d.SQL.QueryRow(`SELECT COALESCE(password_test_status, ''), connections_failed, (ssh_host_key IS NOT NULL)::int,
			(SELECT COUNT(*) FROM host_operation_logs l WHERE l.host_id = h.id AND l.operation_type = 'test' AND l.user_id = ?),
			(SELECT COUNT(*) FROM host_scans s WHERE s.host_id = h.id)
			FROM hosts h WHERE h.id = ?`, e.edA.ID, id).Scan(&s.status, &s.failures, &s.pinned, &s.logs, &s.scans); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if s := state(ok); s != (hostState{"success", 0, 1, 1, 1}) {
		t.Errorf("ok host = %+v", s)
	}
	if s := state(changed); s != (hostState{"failed", 1, 1, 1, 0}) {
		t.Errorf("changed host = %+v", s)
	}
	var logStatus string
	e.d.SQL.QueryRow(`SELECT status FROM host_operation_logs WHERE host_id = ? AND operation_type = 'test'`, changed).Scan(&logStatus)
	if logStatus != "failed" {
		t.Errorf("changed host's log = %q", logStatus)
	}
}

// A scan takes minutes now: an edit made to the host meanwhile survives the
// job's own writes (test status, specs).
func TestScanJob_KeepsEditsMadeDuringTheScan(t *testing.T) {
	e := newScanEnv(t)
	srv := newFakeSSHD(t, testSigner(t, "ed25519"))
	id := e.host("vm", srv.host, srv.port, true)
	e.d.SQL.Exec(`UPDATE hosts SET has_password = TRUE, description = 'before' WHERE id = ?`, id)
	if err := vault.HostSetPassword(context.Background(), e.d, id, e.admin.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	srv.onExec = func() {
		once.Do(func() { e.d.SQL.Exec(`UPDATE hosts SET description = 'edited during the scan' WHERE id = ?`, id) })
	}
	if run := e.wait(e.start(e.edA, `{"host_ids":`+idsJSON(id)+`}`).ID); run.Counts["success"] != 1 {
		t.Fatalf("run = %+v", run)
	}
	var desc, status string
	e.d.SQL.QueryRow(`SELECT description, password_test_status FROM hosts WHERE id = ?`, id).Scan(&desc, &status)
	if desc != "edited during the scan" || status != "success" {
		t.Fatalf("description %q, status %q", desc, status)
	}
}

// A host that stops answering mid-scan: past scanHostTimeout the client is
// closed and the host fails. What a closed client leaves (every command
// empty) is never recorded: it would mark the host's services offline.
func TestScanJob_HungHostFailsWithoutRecording(t *testing.T) {
	e := newScanEnv(t)
	scanHostTimeout = 500 * time.Millisecond
	t.Cleanup(func() { scanHostTimeout = 20 * time.Minute })
	srv := newFakeSSHD(t, testSigner(t, "ed25519"))
	id := e.host("vm", srv.host, srv.port, true)
	e.d.SQL.Exec(`UPDATE hosts SET has_password = TRUE WHERE id = ?`, id)
	if err := vault.HostSetPassword(context.Background(), e.d, id, e.admin.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	srv.onExec = func() { <-hang }

	run := e.wait(e.start(e.edA, `{"host_ids":`+idsJSON(id)+`}`).ID)
	if x := run.Results[0]; x.Status != models.OSResultFailed || !strings.Contains(x.Reason, "longer than") {
		t.Fatalf("result = %+v", x)
	}
	var scans int
	var status string
	e.d.SQL.QueryRow(`SELECT (SELECT COUNT(*) FROM host_scans WHERE host_id = ?), password_test_status FROM hosts WHERE id = ?`, id, id).Scan(&scans, &status)
	if scans != 0 || status != "failed" {
		t.Fatalf("%d scans stored, status %q", scans, status)
	}
}

// The method a scan dials with, as the browser's batch scan picked it.
func TestScanMethod(t *testing.T) {
	for _, c := range []struct {
		pw, key   bool
		preferred string
		want      string
		method    string
	}{
		{true, true, "", "", "key"},
		{true, true, "password", "", "password"},
		{true, true, "key", "", "key"},
		{false, true, "", "", "key"},
		{true, false, "", "", "password"},
		{false, false, "", "", ""},
		{true, true, "key", "password", "password"},
	} {
		h := &models.Host{HasPassword: c.pw, HasKey: c.key, PreferredAuth: c.preferred}
		if got := scanMethod(h, c.want); got != c.method {
			t.Errorf("%+v = %q", c, got)
		}
	}
}
