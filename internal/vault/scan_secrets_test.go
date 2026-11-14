package vault_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
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

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "abc123", scanDesc)
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
	again, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "abc123", scanDesc)
	if err != nil || again != id {
		t.Fatalf("same value: id=%d (want %d) err=%v", again, id, err)
	}
	if c, u := count(id, "create"), count(id, "update"); c != 1 || u != 0 {
		t.Fatalf("after same value: create=%d update=%d", c, u)
	}

	// A changed value updates the payload in place, audited.
	rotated, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "xyz789", scanDesc)
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
	sys, err := vault.UpsertScanSecret(ctx, d, hid, 0, models.SecretTypeEnvVar, "scan-cron-root", "PASSWORD_2", "p2", "")
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
		if _, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, c.group, c.name, c.value, ""); err == nil {
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

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-opt-compras1", "CACHE_SERVER_PASSWORD", "abc123", scanDesc)
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
	bundle := func(a vault.ActorContext) error {
		_, _, err := repo.CreateBundle(ctx, a, []vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: id}},
			vault.CreateBundleOpts{Title: "x", TTL: time.Hour})
		return err
	}
	if err := bundle(editor); err != nil {
		t.Fatalf("editor bundle: %v", err)
	}
	// Revoked from editors (Settings › Permissions): reveal and bundling both refused.
	if _, err := d.SQL.Exec(`DELETE FROM role_permissions WHERE role = 'editor' AND permission = ?`, vault.PermHostSecretsReveal); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reveal(ctx, editor, id); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor reveal after revoke: err = %v, want forbidden", err)
	}
	if err := bundle(editor); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor bundle after revoke: err = %v, want forbidden", err)
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

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", "CACHE_SERVER_PASSWORD", "abc123", "")
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

	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", "TOKEN", "abc123", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	got, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", "TOKEN", "xyz789", "")
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
	if got, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", "TOKEN", "xyz789", ""); err != nil || got != id {
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
				ids[g], errs[g] = vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-x", name, "abc123", "")
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

// ScanSecretNames reads names and value fingerprints only: which of the
// candidate values a name of the family (base, base_<n>, live or trashed)
// already holds, and the family's names in suffix order.
func TestScanSecretNames(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	put := func(name, value string) int64 {
		t.Helper()
		id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-manage-py", name, value, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	put("PASSWORD_10", "ten")
	put("PASSWORD", "one")
	trashed := put("PASSWORD_2", "two")
	put("PASSWORD_X", "x")    // not of the family
	put("PASSWORD_2_3", "y")  // not of the family either
	put("DB_PASSWORD", "two") // another family
	if _, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-other", "PASSWORD", "one", ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(ctx, admin, trashed); err != nil {
		t.Fatal(err)
	}

	held, names, err := vault.ScanSecretNames(ctx, d, hid, "scan-manage-py", "PASSWORD", []string{"two", "three", "ten"})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"two": "PASSWORD_2", "ten": "PASSWORD_10"}; !reflect.DeepEqual(held, want) {
		t.Errorf("held = %v, want %v", held, want)
	}
	if want := []string{"PASSWORD", "PASSWORD_2", "PASSWORD_10"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

// A bulk env-var write onto a scan secret's row restamps its fingerprint: the
// next scan finding the old value again writes it back instead of taking the
// stale fingerprint for "unchanged". A row the bulk write inserts is stamped too.
func TestBulkEnvVarsStampFingerprint(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	desc := "--token · foo · /usr/local/bin/foo"
	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-foo", "TOKEN", "abc123", desc)
	if err != nil {
		t.Fatal(err)
	}
	target := []vault.EnvVarTarget{{Scope: models.SecretScopeHost, ParentID: &hid}}
	if _, err := repo.BulkUpsertEnvVarsMulti(ctx, admin, target, models.SecretVisibilityShared, "scan-foo",
		[]vault.EnvVarUpsert{{Name: "TOKEN", Value: "edited", Description: &desc}, {Name: "OTHER", Value: "v"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-foo", "TOKEN", "abc123", desc); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.Reveal(ctx, admin, id); err != nil || got != `{"value":"abc123"}` {
		t.Fatalf("after the scan saw abc123 again: reveal = %q, %v", got, err)
	}
	var unstamped int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE value_fingerprint IS NULL AND deleted_at IS NULL`).Scan(&unstamped); err != nil || unstamped != 0 {
		t.Fatalf("rows without a fingerprint = %d, %v", unstamped, err)
	}
}
