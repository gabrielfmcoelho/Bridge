package vault_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

func newHostSecretFixture(t *testing.T) (*database.DB, int64, int64) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	var uid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`, "alice", "x", "admin").Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var hid int64
	if err := d.SQL.QueryRow(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?) RETURNING id`,
		"h1", "h1", "10.0.0.1", "deploy",
	).Scan(&hid); err != nil {
		t.Fatalf("seed host: %v", err)
	}

	return d, hid, uid
}

func TestHostSetGetPassword(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()

	if _, ok, err := vault.HostGetPassword(ctx, d, hid); ok || err != nil {
		t.Fatalf("pre-set: ok=%v err=%v; want ok=false", ok, err)
	}
	if err := vault.HostSetPassword(ctx, d, hid, uid, "s3cret"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok, err := vault.HostGetPassword(ctx, d, hid)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got != "s3cret" {
		t.Errorf("plaintext mismatch: got %q", got)
	}

	// Update path: re-set with a different value, verify it round-trips
	// and that no duplicate row appeared.
	if err := vault.HostSetPassword(ctx, d, hid, uid, "rotated-pw"); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	got, _, _ = vault.HostGetPassword(ctx, d, hid)
	if got != "rotated-pw" {
		t.Errorf("rotated value: got %q", got)
	}
	var n int
	_ = d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secrets WHERE scope='host' AND parent_id=? AND name='password' AND deleted_at IS NULL`,
		hid).Scan(&n)
	if n != 1 {
		t.Errorf("expected 1 live row, got %d", n)
	}

	// Clear path: empty plaintext soft-deletes.
	if err := vault.HostSetPassword(ctx, d, hid, uid, ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, _ := vault.HostGetPassword(ctx, d, hid); ok {
		t.Error("after clear, get should return ok=false")
	}
}

func TestHostSetGetSSHKey(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()

	key := vault.HostSSHKey{
		Username:      "deploy",
		PrivateKeyPEM: "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END-----",
		PublicKey:     "ssh-ed25519 AAAA deploy@h1",
	}
	if err := vault.HostSetSSHKey(ctx, d, hid, uid, key); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok, err := vault.HostGetSSHKey(ctx, d, hid)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Username != key.Username ||
		got.PrivateKeyPEM != key.PrivateKeyPEM ||
		got.PublicKey != key.PublicKey {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, key)
	}
}
