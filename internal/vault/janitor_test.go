package vault_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestShareLinkJanitor_DeletesPastGrace builds a tiny fixture with three
// share-link rows at different expiry distances and asserts only the one
// past grace gets removed.
func TestShareLinkJanitor_DeletesPastGrace(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	// Seed: one user (FK target for created_by + the personal-secret owner),
	// one personal secret (FK target for share_links.secret_id), three
	// share_links rows at different expiry offsets.
	res, _ := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`, "alice", "x", "admin")
	uid, _ := res.LastInsertId()

	res2, _ := d.SQL.Exec(
		`INSERT INTO secrets
		    (type, scope, visibility, owner_user_id, name,
		     payload_ciphertext, payload_nonce, key_version, created_by)
		 VALUES ('password', 'avulso', 'personal', ?, 'secret-1', ?, ?, 1, ?)`,
		uid, []byte{0x01}, []byte{0x02}, uid,
	)
	secretID, _ := res2.LastInsertId()

	mkLink := func(expires time.Time) int64 {
		r, err := d.SQL.Exec(
			`INSERT INTO secret_share_links (secret_id, token_hash, expires_at, created_by)
			 VALUES (?, ?, ?, ?)`,
			secretID, []byte{0xaa}, expires.UTC().Format(time.RFC3339Nano), uid,
		)
		if err != nil {
			t.Fatalf("seed link: %v", err)
		}
		// SQLite has a tiny race where multiple inserts in the same
		// millisecond can collide on the unique idx_secret_share_links_token
		// constraint we built on token_hash. Vary the hash so seeds succeed.
		id, _ := r.LastInsertId()
		_, _ = d.SQL.Exec(`UPDATE secret_share_links SET token_hash = ? WHERE id = ?`,
			[]byte{byte(id), byte(id + 1)}, id)
		return id
	}

	now := time.Now().UTC()
	// 1. Still live (expires in 1 hour) — never delete.
	live := mkLink(now.Add(1 * time.Hour))
	// 2. Expired 1 day ago — within 7d grace, keep.
	expiringWithinGrace := mkLink(now.Add(-1 * 24 * time.Hour))
	// 3. Expired 30 days ago — well past 7d grace, DELETE.
	expiredPastGrace := mkLink(now.Add(-30 * 24 * time.Hour))

	j := vault.NewShareLinkJanitor(d.SQL) // 7d grace default
	deleted, err := j.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted: got %d, want 1", deleted)
	}

	// The live + within-grace rows should still exist; only the past-grace
	// row should be gone.
	for _, c := range []struct {
		id   int64
		want bool // true = should exist
		desc string
	}{
		{live, true, "live link"},
		{expiringWithinGrace, true, "expired within grace"},
		{expiredPastGrace, false, "expired past grace"},
	} {
		var n int
		_ = d.SQL.QueryRow(`SELECT COUNT(*) FROM secret_share_links WHERE id = ?`, c.id).Scan(&n)
		got := n == 1
		if got != c.want {
			t.Errorf("%s (id=%d): exists=%v, want %v", c.desc, c.id, got, c.want)
		}
	}
}

// TestShareLinkJanitor_EmptyIsNoOp guards against a janitor that throws
// when there's nothing to clean up — common pattern on fresh deploys.
func TestShareLinkJanitor_EmptyIsNoOp(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	j := vault.NewShareLinkJanitor(d.SQL)
	n, err := j.RunOnce(ctx)
	if err != nil {
		t.Errorf("empty RunOnce: %v", err)
	}
	if n != 0 {
		t.Errorf("empty deleted: got %d, want 0", n)
	}
}
