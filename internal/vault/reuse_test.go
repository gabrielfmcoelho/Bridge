package vault_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Reuse counts the distinct hosts the caller can see that carry a secret with
// the same value: an admin (unscoped) sees all three; a scoped editor sees only
// the two granted hosts, never the one out of reach. A value on a single host
// is not flagged.
func TestSecretReuseAcrossHosts(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	var sga int64
	if err := env.d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga); err != nil {
		t.Fatalf("entidade sga: %v", err)
	}
	grants := store.NewAssetEntidadeRepo(env.d.SQL)
	mkHost := func(n string) int64 {
		var id int64
		if err := env.d.SQL.QueryRow(
			`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,'x','deploy') RETURNING id`, n, n,
		).Scan(&id); err != nil {
			t.Fatalf("host %s: %v", n, err)
		}
		return id
	}
	grant := func(hid int64) {
		if err := grants.Replace(ctx, env.d.SQL, store.AssetHost, hid, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
			t.Fatalf("grant host %d: %v", hid, err)
		}
	}
	hA, hB, hC := mkHost("hA"), mkHost("hB"), mkHost("hC")
	grant(hA) // visible to the scoped editor
	grant(hB) // visible to the scoped editor
	// hC is ungranted — admin-only, invisible to the scoped editor.

	put := func(hid int64, name, value string) {
		if _, err := vault.UpsertScanSecret(ctx, env.d, hid, env.alice.UserID, models.SecretTypeEnvVar, "scan-x", name, value, ""); err != nil {
			t.Fatalf("put %s on %d: %v", name, hid, err)
		}
	}
	// The same deploy password captured on all three hosts.
	put(hA, "DEPLOY_PW", "same-pw")
	put(hB, "DEPLOY_PW", "same-pw")
	put(hC, "DEPLOY_PW", "same-pw")
	// A value only hA has: never flagged as reused.
	put(hA, "LONELY", "unique-pw")

	reuseOf := func(ctx context.Context, actor vault.ActorContext, name string) int {
		t.Helper()
		views, err := env.repo.List(ctx, actor, vault.SecretFilter{Scope: models.SecretScopeHost, ParentID: &hA})
		if err != nil {
			t.Fatalf("list hA: %v", err)
		}
		for _, v := range views {
			if v.Name == name {
				return v.ReuseHosts
			}
		}
		t.Fatalf("secret %q not in hA's list", name)
		return 0
	}

	// Admin (unscoped ctx): all three hosts count.
	if n := reuseOf(ctx, env.alice, "DEPLOY_PW"); n != 3 {
		t.Errorf("admin reuse DEPLOY_PW = %d, want 3", n)
	}
	if n := reuseOf(ctx, env.alice, "LONELY"); n != 0 {
		t.Errorf("admin reuse LONELY = %d, want 0 (single host)", n)
	}

	// Scoped editor: only the two granted hosts count — hC must not leak.
	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}})
	if n := reuseOf(scoped, env.bob, "DEPLOY_PW"); n != 2 {
		t.Errorf("scoped reuse DEPLOY_PW = %d, want 2 (hC hidden)", n)
	}
	if n := reuseOf(scoped, env.bob, "LONELY"); n != 0 {
		t.Errorf("scoped reuse LONELY = %d, want 0", n)
	}

	// The vault-wide list (no single host) never computes reuse.
	views, err := env.repo.List(ctx, env.alice, vault.SecretFilter{Scope: models.SecretScopeHost})
	if err != nil {
		t.Fatalf("host-wide list: %v", err)
	}
	for _, v := range views {
		if v.ReuseHosts != 0 {
			t.Errorf("host-wide list stamped reuse on %q (%d); want 0", v.Name, v.ReuseHosts)
		}
	}
}

// Drift is what the Segredos tab reads as "changed on the VM": a scan that
// re-captures a different value updates the row in place, so updated_at moves
// past created_at. A fresh capture is not drift.
func TestScanSecretDriftTimestamps(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	ts := func(id int64) (created, updated time.Time) {
		t.Helper()
		if err := d.SQL.QueryRow(`SELECT created_at, updated_at FROM secrets WHERE id = ?`, id).Scan(&created, &updated); err != nil {
			t.Fatal(err)
		}
		return
	}

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", "PASSWORD", "abc123", "")
	if err != nil {
		t.Fatal(err)
	}
	c0, u0 := ts(id)
	if u0.After(c0) { // fresh capture: not drifted (badge hidden)
		t.Fatalf("fresh capture already drifted: updated %v after created %v", u0, c0)
	}

	// A scan finding a different value: same row, updated_at bumped = drift.
	if _, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", "PASSWORD", "rotated", ""); err != nil {
		t.Fatal(err)
	}
	c1, u1 := ts(id)
	if !c1.Equal(c0) {
		t.Fatalf("created_at moved on update: %v -> %v", c0, c1)
	}
	if !u1.After(c1) {
		t.Fatalf("drift not visible: updated %v not after created %v", u1, c1)
	}
}
