package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// fakeSSHD is an in-process SSH server whose host key a test can swap. It
// takes the password "pw" and answers every command with exit 0, no output.
// conns counts the connections it accepted; onExec, when set, runs before
// each command is answered (a test may block it, or change the database).
type fakeSSHD struct {
	host, port string
	mu         sync.Mutex
	key        ssh.Signer
	conns      atomic.Int32
	onExec     func()
}

func newFakeSSHD(t *testing.T, key ssh.Signer) *fakeSSHD {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s := &fakeSSHD{key: key}
	s.host, s.port, _ = net.SplitHostPort(ln.Addr().String())
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s.conns.Add(1)
			go s.serve(c)
		}
	}()
	return s
}

func (s *fakeSSHD) setKey(k ssh.Signer) { s.mu.Lock(); s.key = k; s.mu.Unlock() }

func (s *fakeSSHD) serve(c net.Conn) {
	cfg := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
		if string(pw) != "pw" {
			return nil, errors.New("denied")
		}
		return nil, nil
	}}
	s.mu.Lock()
	cfg.AddHostKey(s.key)
	s.mu.Unlock()
	_, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "")
			continue
		}
		ch, chReqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range chReqs {
				req.Reply(true, nil)
				if req.Type == "exec" || req.Type == "shell" {
					if s.onExec != nil {
						s.onExec()
					}
					ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					ch.Close()
				}
			}
		}()
	}
}

func testSigner(t *testing.T, kind string) ssh.Signer {
	t.Helper()
	var priv any
	if kind == "ecdsa" {
		priv, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	} else {
		_, priv, _ = ed25519.GenerateKey(rand.Reader)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Pin on the first login, a changed key refused (on the test path too),
// the duplicates list, reset and re-pin. Two key types, so the refusal of
// a host that lost the pinned type is "host key changed" as well.
func TestSSHHostKeyPinning(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	admin := &models.User{Username: "admin", DisplayName: "Admin", Role: "admin", Email: "a@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	keyA, keyB := testSigner(t, "ed25519"), testSigner(t, "ecdsa")
	fpA, fpB := ssh.FingerprintSHA256(keyA.PublicKey()), ssh.FingerprintSHA256(keyB.PublicKey())
	srv := newFakeSSHD(t, keyA)

	var hostID int64
	// The last test failed: a scan that now succeeds must say so.
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, port, ssh_user, has_password, password_test_status, connections_failed)
		VALUES ('vm1', 'vm1', ?, ?, 'root', TRUE, 'failed', 3) RETURNING id`, srv.host, srv.port).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	if err := vault.HostSetPassword(ctx, d, hostID, admin.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	h := &sshHandlers{db: d}
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
	pinned := func() string {
		t.Helper()
		k, _, err := store.NewHostRepo(d.SQL).SSHHostKey(ctx, hostID)
		if err != nil {
			t.Fatal(err)
		}
		fp, _ := service.HostKeyFingerprint(k)
		return fp
	}
	status := func() (string, int) {
		t.Helper()
		var s string
		var n int
		if err := d.SQL.QueryRow(`SELECT password_test_status, connections_failed FROM hosts WHERE id = ?`, hostID).Scan(&s, &n); err != nil {
			t.Fatal(err)
		}
		return s, n
	}
	host, _ := store.NewHostRepo(d.SQL).GetByID(ctx, hostID)

	// A login that fails pins nothing.
	if _, err := service.DialHost(ctx, d, host, "root", sshtest.PasswordAuth("bad")); err == nil || pinned() != "" {
		t.Fatalf("bad password: err %v, pin %q", err, pinned())
	}

	// A scan pins the key it logged in with, and saves the test's success.
	if code, body := call("POST", "/api/ssh/test/vm1", `{"method":"password","capture":true}`); code != http.StatusOK || !strings.Contains(body, `"success":true`) {
		t.Fatalf("scan = %d %s", code, body)
	}
	if got := pinned(); got != fpA {
		t.Fatalf("pin = %q, want %q", got, fpA)
	}
	if s, n := status(); s != "success" || n != 0 {
		t.Fatalf("after scan: status %q, failures %d", s, n)
	}
	var cfg struct {
		HostKey *hostKeyView `json:"host_key"`
	}
	_, body := call("GET", "/api/ssh/host-config/vm1", "")
	if json.Unmarshal([]byte(body), &cfg); cfg.HostKey == nil || cfg.HostKey.Fingerprint != fpA || cfg.HostKey.KeyType != ssh.KeyAlgoED25519 || cfg.HostKey.PinnedAt == nil {
		t.Fatalf("host-config = %s", body)
	}

	// Another key at the same address: refused before the login.
	srv.setKey(keyB)
	code, body := call("POST", "/api/ssh/test/vm1", `{"method":"password"}`)
	want := "host key changed (expected " + fpA + ", got " + fpB + ")"
	if code != http.StatusOK || !strings.Contains(body, `"success":false`) || !strings.Contains(body, want) {
		t.Fatalf("changed key = %d %s, want %q", code, body, want)
	}
	if s, n := status(); s != "failed" || n != 1 {
		t.Fatalf("after mismatch: status %q, failures %d", s, n)
	}
	var changed *service.HostKeyChangedError
	if _, err := service.DialHost(ctx, d, host, "root", sshtest.PasswordAuth("pw")); !errors.As(err, &changed) || pinned() != fpA {
		t.Fatalf("dial = %v, pin %q", err, pinned())
	}

	// A clone holding the same key shows up in the duplicates list.
	if _, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug, ssh_host_key, ssh_host_key_at)
		SELECT 'vm2', 'vm2', ssh_host_key, now() FROM hosts WHERE id = ?`, hostID); err != nil {
		t.Fatal(err)
	}
	var dups []struct {
		Fingerprint string
		Hosts       []struct{ OficialSlug string `json:"oficial_slug"` }
	}
	_, body = call("GET", "/api/ssh/host-keys/duplicates", "")
	if json.Unmarshal([]byte(body), &dups); len(dups) != 1 || dups[0].Fingerprint != fpA || len(dups[0].Hosts) != 2 ||
		dups[0].Hosts[0].OficialSlug != "vm1" || dups[0].Hosts[1].OficialSlug != "vm2" {
		t.Fatalf("duplicates = %s", body)
	}

	// Reset (logged), then the next login pins the new key.
	if code, body := call("POST", "/api/hosts/vm1/ssh-host-key/reset", ""); code != http.StatusOK || pinned() != "" {
		t.Fatalf("reset = %d %s, pin %q", code, body, pinned())
	}
	var logged string
	if err := d.SQL.QueryRow(`SELECT output FROM host_operation_logs WHERE host_id = ? AND operation_type = 'host-key-reset'`, hostID).Scan(&logged); err != nil || !strings.Contains(logged, fpA) {
		t.Fatalf("operation log = %q, %v", logged, err)
	}
	if code, body := call("POST", "/api/ssh/test/vm1", `{"method":"password"}`); code != http.StatusOK || !strings.Contains(body, `"success":true`) {
		t.Fatalf("re-pin = %d %s", code, body)
	}
	if got := pinned(); got != fpB {
		t.Fatalf("re-pinned = %q, want %q", got, fpB)
	}
	if _, body = call("GET", "/api/ssh/host-keys/duplicates", ""); strings.TrimSpace(body) != "[]" {
		t.Fatalf("duplicates after re-pin = %s", body)
	}
	if code, _ := call("POST", "/api/hosts/nope/ssh-host-key/reset", ""); code != http.StatusNotFound {
		t.Fatalf("unknown host reset = %d", code)
	}
}
