package vault_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestShareLinkJanitor_DeletesPastGrace builds a tiny fixture with three
// share-link rows at different expiry distances and asserts only the one
// past grace gets removed.
func TestShareLinkJanitor_DeletesPastGrace(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	// Seed: one user (FK target for created_by) and three share_bundles rows at
	// different expiry offsets. (Per-secret share_links were retired in R3; a
	// share is now a bundle, which the janitor sweeps.)
	var uid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`, "alice", "x", "admin").Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	mkLink := func(expires time.Time) int64 {
		var id int64
		if err := d.SQL.QueryRow(
			`INSERT INTO share_bundles (token_hash, expires_at, created_by) VALUES (?, ?, ?) RETURNING id`,
			[]byte{0xaa}, expires.UTC().Format(time.RFC3339Nano), uid,
		).Scan(&id); err != nil {
			t.Fatalf("seed bundle: %v", err)
		}
		// Vary token_hash so the unique idx_share_bundles_token constraint
		// doesn't collide across same-millisecond inserts.
		_, _ = d.SQL.Exec(`UPDATE share_bundles SET token_hash = ? WHERE id = ?`,
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
	swept, err := j.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if swept != 1 {
		t.Errorf("swept: got %d, want 1", swept)
	}

	// Soft-delete, NOT hard-delete: every row still exists. Only the
	// past-grace row carries a deleted_at; live + within-grace rows stay
	// active (deleted_at IS NULL) so the public link keeps working until
	// its own expiry. Retaining the row keeps the token revivable via Renew.
	for _, c := range []struct {
		id      int64
		wantDel bool // true = deleted_at should be set
		desc    string
	}{
		{live, false, "live link"},
		{expiringWithinGrace, false, "expired within grace"},
		{expiredPastGrace, true, "expired past grace"},
	} {
		var exists int
		_ = d.SQL.QueryRow(`SELECT COUNT(*) FROM share_bundles WHERE id = ?`, c.id).Scan(&exists)
		if exists != 1 {
			t.Errorf("%s (id=%d): row was hard-deleted; want soft-delete (row retained)", c.desc, c.id)
			continue
		}
		var deletedAt sql.NullTime
		if err := d.SQL.QueryRow(`SELECT deleted_at FROM share_bundles WHERE id = ?`, c.id).Scan(&deletedAt); err != nil {
			t.Fatalf("%s: read deleted_at: %v", c.desc, err)
		}
		if deletedAt.Valid != c.wantDel {
			t.Errorf("%s (id=%d): deleted_at set=%v, want %v", c.desc, c.id, deletedAt.Valid, c.wantDel)
		}
	}
}

// TestShareLinkJanitor_EmptyIsNoOp guards against a janitor that throws
// when there's nothing to clean up — common pattern on fresh deploys.
func TestShareLinkJanitor_EmptyIsNoOp(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
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
