package vault_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// A config-file login is stored as a cred secret: the JSON payload is kept
// verbatim, the username column is derived from it, and an admin reveal returns
// the whole payload. A rotated password updates the row in place.
func TestUpsertScanSecretCredType(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}

	payload := `{"username":"appuser","password":"s3cr3t","host":"127.0.0.1","port":"3306","db":"app"}`
	id, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeCred, "scan-ci3-database-app", "DEFAULT", payload, "database.php [default]")
	if err != nil || id == 0 {
		t.Fatalf("create cred: id=%d err=%v", id, err)
	}
	var typ, user string
	if err := d.SQL.QueryRow(`SELECT type, COALESCE(username,'') FROM secrets WHERE id = ?`, id).Scan(&typ, &user); err != nil {
		t.Fatal(err)
	}
	if typ != "cred" || user != "appuser" {
		t.Fatalf("row type=%q username=%q, want cred/appuser", typ, user)
	}
	if got, err := repo.Reveal(ctx, admin, id); err != nil || got != payload {
		t.Fatalf("reveal = %q, %v", got, err)
	}

	// Same payload again: no-op.
	if again, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeCred, "scan-ci3-database-app", "DEFAULT", payload, "database.php [default]"); err != nil || again != id {
		t.Fatalf("same payload: id=%d (want %d) err=%v", again, id, err)
	}
	// Rotated password: update in place.
	rotated := `{"username":"appuser","password":"n3w","host":"127.0.0.1","port":"3306","db":"app"}`
	if got, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeCred, "scan-ci3-database-app", "DEFAULT", rotated, "database.php [default]"); err != nil || got != id {
		t.Fatalf("rotate: id=%d (want %d) err=%v", got, id, err)
	}
	if got, _ := repo.Reveal(ctx, admin, id); got != rotated {
		t.Fatalf("reveal after rotate = %q", got)
	}
}

// The scan-admin- tier is admin-only: a captured DB-admin login or root key is
// never revealed or share-bundled by an editor, even one holding
// hosts.secrets.reveal. Admins always may.
func TestRevealScanAdminTierAdminOnly(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	mkUser := func(name, role string) vault.ActorContext {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`, name, "x", role).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return vault.ActorContext{UserID: id, Role: role}
	}
	editor, viewer := mkUser("eddie", "editor"), mkUser("vic", "viewer")

	// Admin-tier: a debian.cnf DB-admin login.
	adminID, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeCred, "scan-admin-mysql-debian-cnf", "DEBIAN_SYS_MAINT", `{"username":"debian-sys-maint","password":"adminpw"}`, "")
	if err != nil {
		t.Fatal(err)
	}
	// Normal tier: an app .env key.
	normalID, err := vault.UpsertScanSecret(ctx, d, hid, uid, models.SecretTypeEnvVar, "scan-dotenv-app", "DB_PASSWORD", "apppw", "")
	if err != nil {
		t.Fatal(err)
	}

	bundle := func(a vault.ActorContext, id int64) error {
		_, _, err := repo.CreateBundle(ctx, a, []vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: id}},
			vault.CreateBundleOpts{Title: "x", TTL: time.Hour})
		return err
	}

	// Editor holds hosts.secrets.reveal (seeded): the normal row yields, the
	// admin row does not — reveal and bundling both.
	if got, err := repo.Reveal(ctx, editor, normalID); err != nil || got != `{"value":"apppw"}` {
		t.Fatalf("editor reveal normal = %q, %v", got, err)
	}
	if err := bundle(editor, normalID); err != nil {
		t.Fatalf("editor bundle normal: %v", err)
	}
	if _, err := repo.Reveal(ctx, editor, adminID); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor reveal admin-tier: err = %v, want forbidden", err)
	}
	if err := bundle(editor, adminID); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor bundle admin-tier: err = %v, want forbidden", err)
	}
	if _, err := repo.Reveal(ctx, viewer, adminID); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("viewer reveal admin-tier: err = %v, want forbidden", err)
	}
	// Admin always may, both tiers.
	if got, err := repo.Reveal(ctx, admin, adminID); err != nil || got != `{"username":"debian-sys-maint","password":"adminpw"}` {
		t.Fatalf("admin reveal admin-tier = %q, %v", got, err)
	}
	if err := bundle(admin, adminID); err != nil {
		t.Fatalf("admin bundle admin-tier: %v", err)
	}
}

// A captured key (group scan-admin-…) is kept out of credential consolidation:
// it would otherwise be swept into a standalone shared credential, which has no
// extra reveal permission, silently dropping the admin gate. A real per-host
// key of the same fingerprint still consolidates.
func TestConsolidationExcludesScanGroups(t *testing.T) {
	d, h1, uid := newHostSecretFixture(t)
	ctx := context.Background()
	mkHost := func(n string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,'x','deploy') RETURNING id`, n, n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	h2, h3, h4 := mkHost("c2"), mkHost("c3"), mkHost("c4")
	priv, _ := testKeyPEM(t)
	payload := `{"username":"root","private_key_pem":` + jsonString(priv) + `}`

	// Two hosts carry the same captured root key in the admin tier.
	for _, h := range []int64{h1, h2} {
		if _, err := vault.UpsertScanSecret(ctx, d, h, uid, models.SecretTypeSSHKey, "scan-admin-root-ssh-keys", "ID_RSA", payload, ""); err != nil {
			t.Fatal(err)
		}
	}
	// Control: two other hosts share a real per-host login key.
	for _, h := range []int64{h3, h4} {
		if err := vault.HostSetSSHKey(ctx, d, h, uid, vault.HostSSHKey{Username: "deploy", PrivateKeyPEM: priv}); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := vault.PlanHostCredentialConsolidation(ctx, d.SQL)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 {
		t.Fatalf("plan = %d groups, want only the per-host key group (scan group excluded)", len(plan))
	}
	g := plan[0]
	if g.Username != "deploy" || len(g.HostIDs) != 2 {
		t.Fatalf("plan group = %+v, want the deploy key on h3/h4", g)
	}
	for _, h := range g.HostIDs {
		if h == h1 || h == h2 {
			t.Fatalf("captured scan key host %d swept into consolidation", h)
		}
	}
}
