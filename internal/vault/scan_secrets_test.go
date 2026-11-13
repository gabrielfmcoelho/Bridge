package vault_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

const scanDesc = "-DcacheServerPassword · java · /opt/compras1"

func TestUpsertScanSecretCreatesThenUpdates(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	count := func(id int64, action string) int {
		t.Helper()
		var n int
		if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM secret_audit_log WHERE secret_id = ? AND action = ?`, id, action).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "abc123", scanDesc)
	if err != nil || id == 0 {
		t.Fatalf("create: id=%d err=%v", id, err)
	}
	var typ, scope, vis, name, group, desc string
	var parent int64
	if err := d.SQL.QueryRow(`SELECT type, scope, visibility, parent_id, name, group_label, description FROM secrets WHERE id = ?`, id).
		Scan(&typ, &scope, &vis, &parent, &name, &group, &desc); err != nil {
		t.Fatal(err)
	}
	if typ != "env_var" || scope != "host" || vis != "shared" || parent != hid ||
		name != "CACHE_SERVER_PASSWORD" || group != "scan-opt-compras1" || desc != scanDesc {
		t.Fatalf("row = %s %s %s %d %s %s %q", typ, scope, vis, parent, name, group, desc)
	}
	if got, err := repo.Reveal(ctx, admin, id); err != nil || got != `{"value":"abc123"}` {
		t.Fatalf("reveal = %q, %v", got, err)
	}

	// The next scan finds the same value: same row, nothing written to the audit.
	again, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "abc123", scanDesc)
	if err != nil || again != id {
		t.Fatalf("same value: id=%d (want %d) err=%v", again, id, err)
	}
	if c, u := count(id, "create"), count(id, "update"); c != 1 || u != 0 {
		t.Fatalf("after same value: create=%d update=%d", c, u)
	}

	// A changed value updates the payload in place, audited.
	rotated, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "xyz789", scanDesc)
	if err != nil || rotated != id {
		t.Fatalf("new value: id=%d (want %d) err=%v", rotated, id, err)
	}
	var meta string
	if err := d.SQL.QueryRow(`SELECT metadata FROM secret_audit_log WHERE secret_id = ? AND action = 'update'`, id).Scan(&meta); err != nil {
		t.Fatalf("update audit: %v", err)
	}
	if meta != `{"changed_fields":["payload"]}` {
		t.Fatalf("update metadata = %s", meta)
	}
	if c := count(id, "create"); c != 1 {
		t.Fatalf("create audits = %d", c)
	}
	if got, _ := repo.Reveal(ctx, admin, id); got != `{"value":"xyz789"}` {
		t.Fatalf("reveal after update = %q", got)
	}

	// No actor (a backfill): the system actor owns the row.
	sys, err := vault.UpsertScanSecret(ctx, d, hid, 0, "scan-cron-root", "PASSWORD_2", "p2", "")
	if err != nil {
		t.Fatalf("system actor: %v", err)
	}
	var owner int64
	_ = d.SQL.QueryRow(`SELECT owner_user_id FROM secrets WHERE id = ?`, sys).Scan(&owner)
	if owner != uid {
		t.Fatalf("owner = %d, want %d", owner, uid)
	}
}

func TestUpsertScanSecretRejectsBadNames(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	for _, c := range []struct{ group, name, value string }{
		{"scan-opt-compras1", "cacheServerPassword", "v"}, // not an env var name
		{"scan_x", "CACHE_SERVER_PASSWORD", "v"},          // not a kebab group
		{"opt-compras1", "CACHE_SERVER_PASSWORD", "v"},    // not a scan group: would be revealable by any viewer
		{"scan-opt-compras1", "CACHE_SERVER_PASSWORD", ""},
	} {
		if _, err := vault.UpsertScanSecret(ctx, d, hid, uid, c.group, c.name, c.value, ""); err == nil {
			t.Errorf("group %q name %q value %q: want error", c.group, c.name, c.value)
		}
	}
	var n int
	_ = d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d rows written", n)
	}
}

func TestRevealScanSecretNeedsPermission(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	actor := func(name, role string) vault.ActorContext {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`, name, "x", role).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return vault.ActorContext{UserID: id, Role: role}
	}
	viewer, editor := actor("vic", "viewer"), actor("eddie", "editor")

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "abc123", scanDesc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reveal(ctx, viewer, id); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("viewer reveal: err = %v, want forbidden", err)
	}
	if _, _, err := repo.CreateBundle(ctx, viewer, []vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: id}},
		vault.CreateBundleOpts{Title: "x", TTL: time.Hour}); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("viewer bundle: err = %v, want forbidden", err)
	}
	if got, err := repo.Reveal(ctx, editor, id); err != nil || got != `{"value":"abc123"}` {
		t.Fatalf("editor reveal = %q, %v", got, err)
	}

	// The host's SSH password keeps today's rule: any viewer of the host.
	if err := vault.HostSetPassword(ctx, d, hid, uid, "s3cret"); err != nil {
		t.Fatal(err)
	}
	var pid int64
	if err := d.SQL.QueryRow(`SELECT id FROM secrets WHERE scope = 'host' AND parent_id = ? AND name = 'password'`, hid).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.Reveal(ctx, viewer, pid); err != nil || got != "s3cret" {
		t.Fatalf("viewer reveal of host password = %q, %v", got, err)
	}
}

// The reveal gate is keyed on the group label, so only an admin may move a host
// secret into or out of a scan- group; an editor renaming it would expose it.
func TestRenameScanSecretGroupNeedsAdmin(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	var eid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('eddie','x','editor') RETURNING id`).Scan(&eid); err != nil {
		t.Fatal(err)
	}
	editor := vault.ActorContext{UserID: eid, Role: "editor"}
	group := func(id int64) string {
		var g string
		_ = d.SQL.QueryRow(`SELECT group_label FROM secrets WHERE id = ?`, id).Scan(&g)
		return g
	}
	str := func(s string) *string { return &s }

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-x", "CACHE_SERVER_PASSWORD", "abc123", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, editor, id, vault.SecretPatch{GroupLabel: str("x")}); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor scan-x → x: err = %v, want forbidden", err)
	}
	if g := group(id); g != "scan-x" {
		t.Fatalf("group after refused rename = %q", g)
	}
	// Other fields stay editable by the editor.
	if err := repo.Update(ctx, editor, id, vault.SecretPatch{Description: str("found in /opt/compras1")}); err != nil {
		t.Fatalf("editor description: %v", err)
	}
	if err := repo.Update(ctx, admin, id, vault.SecretPatch{GroupLabel: str("x")}); err != nil {
		t.Fatalf("admin scan-x → x: %v", err)
	}
	if g := group(id); g != "x" {
		t.Fatalf("group after admin rename = %q", g)
	}
	if err := repo.Update(ctx, editor, id, vault.SecretPatch{GroupLabel: str("scan-y")}); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor x → scan-y: err = %v, want forbidden", err)
	}
	if g := group(id); g != "x" {
		t.Fatalf("group after refused move in = %q", g)
	}
}

// Bridge never recreates what a user trashed: a scan finding a trashed
// identity again writes nothing until the row is restored.
func TestUpsertScanSecretTrashedNotRecreated(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-x", "TOKEN", "abc123", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	got, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-x", "TOKEN", "xyz789", "")
	if !errors.Is(err, vault.ErrScanSecretTrashed) || got != 0 {
		t.Fatalf("trashed identity: id=%d err=%v, want ErrScanSecretTrashed", got, err)
	}
	var n int
	_ = d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE name = 'TOKEN'`).Scan(&n)
	if n != 1 {
		t.Fatalf("%d rows, want only the trashed one", n)
	}

	if err := repo.Restore(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if got, err := vault.UpsertScanSecret(ctx, d, hid, uid, "scan-x", "TOKEN", "xyz789", ""); err != nil || got != id {
		t.Fatalf("after restore: id=%d (want %d) err=%v", got, id, err)
	}
}

// Two scans of the same host finding a new secret at once: the second insert
// hits the unique index and carries on as an update of the first one's row.
func TestUpsertScanSecretConcurrentFirstInsert(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("TOKEN_%d", i)
		start := make(chan struct{})
		var wg sync.WaitGroup
		ids, errs := make([]int64, 2), make([]error, 2)
		for g := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ids[g], errs[g] = vault.UpsertScanSecret(ctx, d, hid, uid, "scan-x", name, "abc123", "")
			}()
		}
		close(start)
		wg.Wait()
		if errs[0] != nil || errs[1] != nil || ids[0] == 0 || ids[0] != ids[1] {
			t.Fatalf("%s: ids=%v errs=%v", name, ids, errs)
		}
	}
	var n int
	_ = d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE group_label = 'scan-x'`).Scan(&n)
	if n != 20 {
		t.Fatalf("%d rows, want 20", n)
	}
}
