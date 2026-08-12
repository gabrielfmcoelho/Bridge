package vault_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// sharedCredFixture opens an isolated DB and seeds an admin user.
func sharedCredFixture(t *testing.T) (*database.DB, int64) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	var uid int64
	if err := d.SQL.QueryRow(
		`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`,
		"admin", "x", "admin").Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return d, uid
}

// mkHostPw inserts a host with the given ssh_user and sets a per-host password.
func mkHostPw(t *testing.T, d *database.DB, uid int64, n int, sshUser, value string) int64 {
	t.Helper()
	var hid int64
	slug := fmt.Sprintf("h%d", n)
	if err := d.SQL.QueryRow(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?) RETURNING id`,
		slug, slug, "10.0.0."+slug, sshUser).Scan(&hid); err != nil {
		t.Fatalf("seed host %d: %v", n, err)
	}
	if err := vault.HostSetPassword(context.Background(), d, hid, uid, value); err != nil {
		t.Fatalf("set pw host %d: %v", n, err)
	}
	return hid
}

func countLiveHostPwRows(t *testing.T, d *database.DB, hostID int64) int {
	t.Helper()
	var n int
	_ = d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secrets WHERE scope='host' AND parent_id=? AND name='password' AND deleted_at IS NULL`,
		hostID).Scan(&n)
	return n
}

func TestMigrateSharedHostPasswords_ConsolidatesAndIsIdempotent(t *testing.T) {
	d, uid := sharedCredFixture(t)
	ctx := context.Background()

	h1 := mkHostPw(t, d, uid, 1, "deploy", "shared-pw")
	h2 := mkHostPw(t, d, uid, 2, "deploy", "shared-pw")
	h3 := mkHostPw(t, d, uid, 3, "deploy", "shared-pw")

	st, err := vault.MigrateSharedHostPasswords(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if st.Credentials != 1 || st.Links != 3 || st.Skipped != 0 {
		t.Fatalf("stats: %+v; want 1 cred / 3 links / 0 skipped", st)
	}

	// One avulso shared credential named after the ssh_user.
	var creds int
	_ = d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secrets WHERE scope='avulso' AND type='password' AND name='deploy' AND deleted_at IS NULL`).
		Scan(&creds)
	if creds != 1 {
		t.Errorf("avulso credentials=%d, want 1", creds)
	}
	// Three links.
	var links int
	_ = d.SQL.QueryRow(`SELECT COUNT(*) FROM host_remote_users WHERE secret_id IS NOT NULL`).Scan(&links)
	if links != 3 {
		t.Errorf("links=%d, want 3", links)
	}
	// Per-host rows are LEFT IN PLACE (non-destructive).
	for _, h := range []int64{h1, h2, h3} {
		if got := countLiveHostPwRows(t, d, h); got != 1 {
			t.Errorf("host %d per-host rows=%d, want 1 (non-destructive)", h, got)
		}
	}
	// Resolution now goes via the link, still returns the value.
	for _, h := range []int64{h1, h2, h3} {
		got, ok, err := vault.HostGetPassword(ctx, d, h)
		if err != nil || !ok || got != "shared-pw" {
			t.Errorf("host %d resolve: got=%q ok=%v err=%v", h, got, ok, err)
		}
	}

	// Idempotent: a second run creates/links nothing.
	st2, err := vault.MigrateSharedHostPasswords(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("migrate re-run: %v", err)
	}
	if st2.Credentials != 0 || st2.Links != 0 {
		t.Errorf("re-run not idempotent: %+v", st2)
	}
}

func TestMigrateSharedHostPasswords_DistinctValuesAndSingletons(t *testing.T) {
	d, uid := sharedCredFixture(t)
	ctx := context.Background()

	// deploy/A x2  -> credential "deploy"
	mkHostPw(t, d, uid, 1, "deploy", "A")
	mkHostPw(t, d, uid, 2, "deploy", "A")
	// deploy/B x2  -> credential "deploy-2" (same user, distinct value)
	mkHostPw(t, d, uid, 3, "deploy", "B")
	mkHostPw(t, d, uid, 4, "deploy", "B")
	// ops/C x1     -> singleton, skipped
	mkHostPw(t, d, uid, 5, "ops", "C")

	st, err := vault.MigrateSharedHostPasswords(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if st.Credentials != 2 || st.Links != 4 || st.Skipped != 1 {
		t.Fatalf("stats: %+v; want 2 creds / 4 links / 1 skipped", st)
	}
	var names int
	_ = d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secrets WHERE scope='avulso' AND type='password'
		   AND name IN ('deploy','deploy-2') AND deleted_at IS NULL`).Scan(&names)
	if names != 2 {
		t.Errorf("expected credentials 'deploy' and 'deploy-2', found %d", names)
	}
}

func TestCleanupRedundantHostPasswords(t *testing.T) {
	d, uid := sharedCredFixture(t)
	ctx := context.Background()

	h1 := mkHostPw(t, d, uid, 1, "deploy", "shared-pw")
	h2 := mkHostPw(t, d, uid, 2, "deploy", "shared-pw")
	h3 := mkHostPw(t, d, uid, 3, "deploy", "shared-pw")
	if _, err := vault.MigrateSharedHostPasswords(ctx, d.SQL, d.Encryptor, uid); err != nil {
		t.Fatalf("phase1: %v", err)
	}
	// Pre-cleanup: per-host rows still live (Phase 1 is non-destructive).
	for _, h := range []int64{h1, h2, h3} {
		if countLiveHostPwRows(t, d, h) != 1 {
			t.Fatalf("host %d: expected per-host row before cleanup", h)
		}
	}

	st, err := vault.CleanupRedundantHostPasswords(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if st.Deleted != 3 || st.Kept != 0 {
		t.Fatalf("cleanup stats: %+v; want 3 deleted / 0 kept", st)
	}
	// Per-host rows soft-deleted, but resolution still works via the link.
	for _, h := range []int64{h1, h2, h3} {
		if countLiveHostPwRows(t, d, h) != 0 {
			t.Errorf("host %d: per-host row should be soft-deleted", h)
		}
		if got, ok, _ := vault.HostGetPassword(ctx, d, h); !ok || got != "shared-pw" {
			t.Errorf("host %d after cleanup: got=%q ok=%v", h, got, ok)
		}
	}
	// Idempotent.
	st2, err := vault.CleanupRedundantHostPasswords(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("cleanup re-run: %v", err)
	}
	if st2.Deleted != 0 {
		t.Errorf("cleanup not idempotent: %+v", st2)
	}
}

func TestCleanupRedundantHostPasswords_KeepsDivergent(t *testing.T) {
	d, uid := sharedCredFixture(t)
	ctx := context.Background()

	// A host with a per-host password of "B".
	h := mkHostPw(t, d, uid, 1, "deploy", "B")
	// A shared credential holding a DIFFERENT value "A".
	ct, nonce, err := d.Encryptor.Encrypt("A")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	var credID int64
	if err := d.SQL.QueryRow(
		`INSERT INTO secrets (type, scope, visibility, parent_id, owner_user_id, name,
			payload_ciphertext, payload_nonce, key_version, created_by)
		 VALUES ('password','avulso','shared',NULL,?,?,?,?,1,?) RETURNING id`,
		uid, "cred-a", ct, nonce, uid).Scan(&credID); err != nil {
		t.Fatalf("insert cred: %v", err)
	}
	// Force the (divergent) link directly — the host's per-host value is "B"
	// but the link points at credential "A".
	if err := store.NewHostRemoteUserRepo(d.SQL).SetSecret(ctx, h, "deploy", &credID); err != nil {
		t.Fatalf("link: %v", err)
	}

	st, err := vault.CleanupRedundantHostPasswords(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if st.Deleted != 0 || st.Kept != 1 {
		t.Fatalf("stats: %+v; want 0 deleted / 1 kept (divergent value)", st)
	}
	if countLiveHostPwRows(t, d, h) != 1 {
		t.Error("divergent per-host row must NOT be deleted")
	}
}

func TestHostGetPassword_ResolvesViaDirectLink(t *testing.T) {
	d, uid := sharedCredFixture(t)
	ctx := context.Background()

	// A shared avulso password credential (as the create-and-reuse UI would make).
	ct, nonce, err := d.Encryptor.Encrypt("lib-pw")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	var credID int64
	if err := d.SQL.QueryRow(
		`INSERT INTO secrets (type, scope, visibility, parent_id, owner_user_id, name,
			payload_ciphertext, payload_nonce, key_version, created_by)
		 VALUES ('password','avulso','shared',NULL,?,?,?,?,1,?) RETURNING id`,
		uid, "sead-shared", ct, nonce, uid).Scan(&credID); err != nil {
		t.Fatalf("insert credential: %v", err)
	}

	// A host with no per-host password, linked to the shared credential.
	var hid int64
	if err := d.SQL.QueryRow(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?) RETURNING id`,
		"hlink", "hlink", "10.0.0.9", "deploy").Scan(&hid); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	repo := store.NewHostRemoteUserRepo(d.SQL)
	if err := repo.SetSecret(ctx, hid, "deploy", &credID); err != nil {
		t.Fatalf("link: %v", err)
	}

	got, ok, err := vault.HostGetPassword(ctx, d, hid)
	if err != nil || !ok || got != "lib-pw" {
		t.Fatalf("resolve via link: got=%q ok=%v err=%v", got, ok, err)
	}
	ids, err := repo.ListHostsBySecret(ctx, credID)
	if err != nil || len(ids) != 1 || ids[0] != hid {
		t.Fatalf("ListHostsBySecret: ids=%v err=%v", ids, err)
	}

	// Unlink → no per-host row exists, so resolution reports no password.
	if err := repo.SetSecret(ctx, hid, "deploy", nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, ok, _ := vault.HostGetPassword(ctx, d, hid); ok {
		t.Error("after unlink with no per-host row, get should return ok=false")
	}
}

func TestHostSetPassword_BreaksSharing(t *testing.T) {
	d, uid := sharedCredFixture(t)
	ctx := context.Background()

	h1 := mkHostPw(t, d, uid, 1, "deploy", "shared-pw")
	h2 := mkHostPw(t, d, uid, 2, "deploy", "shared-pw")
	if _, err := vault.MigrateSharedHostPasswords(ctx, d.SQL, d.Encryptor, uid); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Override h1's password: it should detach from the shared credential.
	if err := vault.HostSetPassword(ctx, d, h1, uid, "h1-only"); err != nil {
		t.Fatalf("override: %v", err)
	}
	if got, _, _ := vault.HostGetPassword(ctx, d, h1); got != "h1-only" {
		t.Errorf("h1 after override: got %q, want h1-only", got)
	}
	// h2 still resolves the shared value — sharing broken only for h1.
	if got, _, _ := vault.HostGetPassword(ctx, d, h2); got != "shared-pw" {
		t.Errorf("h2 after h1 override: got %q, want shared-pw", got)
	}
	// h1's link is cleared.
	var h1secret sql.NullInt64
	_ = d.SQL.QueryRow(`SELECT secret_id FROM host_remote_users WHERE host_id=? AND username='deploy'`, h1).Scan(&h1secret)
	if h1secret.Valid {
		t.Errorf("h1 link not cleared: secret_id=%d", h1secret.Int64)
	}
}
