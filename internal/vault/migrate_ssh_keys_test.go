package vault_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	gossh "golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// testKeyPEM returns a fresh OpenSSH private key and its fingerprint.
func testKeyPEM(t *testing.T) (string, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := gossh.NewSignerFromKey(priv)
	return string(pem.EncodeToMemory(block)), gossh.FingerprintSHA256(signer.PublicKey())
}

func seedSSHKey(t *testing.T, d *database.DB, name, user, priv, pass string) int64 {
	t.Helper()
	enc := func(s string) ([]byte, []byte) {
		if s == "" {
			return nil, nil
		}
		ct, n, err := d.Encryptor.Encrypt(s)
		if err != nil {
			t.Fatal(err)
		}
		return ct, n
	}
	pct, pn := enc(priv)
	wct, wn := enc(pass)
	typ := "key"
	if priv == "" {
		typ = "password"
	}
	var id int64
	if err := d.SQL.QueryRow(
		`INSERT INTO ssh_keys (name, credential_type, username, description, priv_key_ciphertext, priv_key_nonce, password_ciphertext, password_nonce)
		 VALUES (?, ?, ?, 'desc', ?, ?, ?, ?) RETURNING id`, name, typ, user, pct, pn, wct, wn).Scan(&id); err != nil {
		t.Fatalf("seed ssh_key: %v", err)
	}
	return id
}

func TestMigrateSSHKeysToVault(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	priv, fp := testKeyPEM(t)

	keyID := seedSSHKey(t, d, "deploy-key", "deploy", priv, "")
	bothID := seedSSHKey(t, d, "both", "root", priv, "pw-both")
	passID := seedSSHKey(t, d, "taken", "ops", "", "pw-new")
	// "taken" already exists in the vault with another value → suffixed.
	if _, err := d.SQL.Exec(`INSERT INTO secrets (type, scope, visibility, owner_user_id, name, payload_ciphertext, payload_nonce, key_version, created_by)
		VALUES ('password','avulso','shared',?, 'taken', ?, ?, 1, ?)`, uid, []byte("x"), []byte("y"), uid); err != nil {
		t.Fatal(err)
	}
	var ent int64
	d.SQL.QueryRow(`INSERT INTO entidades (name, slug) VALUES ('E','e') RETURNING id`).Scan(&ent)
	d.SQL.Exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES ('ssh_key', ?, ?, 'responsible')`, keyID, ent)
	// The host's login user referenced the key through ssh_key_id.
	d.SQL.Exec(`INSERT INTO host_remote_users (host_id, username, ssh_key_id) VALUES (?, 'deploy', ?)`, hid, keyID)

	st, err := vault.MigrateSSHKeysToVault(ctx, d.SQL, d.Encryptor, uid)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// deploy-key (sshkey), both (sshkey + "both (senha)"), taken-2 (password).
	if st.Created != 4 || st.Renamed != 1 || st.Links != 1 {
		t.Fatalf("stats = %+v", st)
	}

	var name, user, sshFP string
	var secretID int64
	if err := d.SQL.QueryRow(`SELECT id, name, username, ssh_fingerprint FROM secrets WHERE legacy_ssh_key_id = ? AND type = 'sshkey'`, keyID).
		Scan(&secretID, &name, &user, &sshFP); err != nil {
		t.Fatalf("migrated key: %v", err)
	}
	if name != "deploy-key" || user != "deploy" || sshFP != fp {
		t.Fatalf("migrated key = %q %q %q; want deploy-key deploy %s", name, user, sshFP, fp)
	}
	var grants int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM asset_entidades WHERE asset_type = 'secret' AND asset_id = ?`, secretID).Scan(&grants)
	if grants != 1 {
		t.Fatalf("grants copied = %d, want 1", grants)
	}
	var n int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE legacy_ssh_key_id = ?`, bothID).Scan(&n)
	if n != 2 {
		t.Fatalf("key+password row → %d secrets, want 2", n)
	}
	d.SQL.QueryRow(`SELECT name FROM secrets WHERE legacy_ssh_key_id = ?`, passID).Scan(&name)
	if name != "taken-2" {
		t.Fatalf("collision name = %q, want taken-2", name)
	}

	// The host now resolves the key through the link — it has no copy.
	key, ok, err := vault.HostGetSSHKey(ctx, d, hid)
	if err != nil || !ok || key.PrivateKeyPEM != priv || key.PublicKey == "" {
		t.Fatalf("linked key = %+v ok=%v err=%v", key, ok, err)
	}

	// Idempotent.
	if st, err := vault.MigrateSSHKeysToVault(ctx, d.SQL, d.Encryptor, uid); err != nil || st.Created+st.Reused != 0 {
		t.Fatalf("second run = %+v, %v; want no-op", st, err)
	}
}

// A per-host key write wins over the shared link, as for passwords.
func TestHostSetSSHKeyBreaksLink(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	shared, _ := testKeyPEM(t)
	own, _ := testKeyPEM(t)
	keyID := seedSSHKey(t, d, "shared", "deploy", shared, "")
	d.SQL.Exec(`INSERT INTO host_remote_users (host_id, username, ssh_key_id) VALUES (?, 'deploy', ?)`, hid, keyID)
	if _, err := vault.MigrateSSHKeysToVault(ctx, d.SQL, d.Encryptor, uid); err != nil {
		t.Fatal(err)
	}
	if err := vault.HostSetSSHKey(ctx, d, hid, uid, vault.HostSSHKey{Username: "deploy", PrivateKeyPEM: own}); err != nil {
		t.Fatal(err)
	}
	key, ok, _ := vault.HostGetSSHKey(ctx, d, hid)
	if !ok || key.PrivateKeyPEM != own {
		t.Fatalf("after own key: got shared=%v", key.PrivateKeyPEM == shared)
	}
	var linked *int64
	d.SQL.QueryRow(`SELECT key_secret_id FROM host_remote_users WHERE host_id = ?`, hid).Scan(&linked)
	if linked != nil {
		t.Fatalf("link kept after per-host write")
	}
}

// A vault-form password ({"value":…}) linked to a host logs in with the value,
// and groups with the same password stored raw.
func TestLinkedPasswordUnwrapsAndFingerprints(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	actor := vault.ActorContext{UserID: uid, Role: "admin"}
	id := createSecret(t, repo, actor, "password", "avulso", "shared", nil, "wrapped", `{"value":"s3cret"}`)
	d.SQL.Exec(`INSERT INTO host_remote_users (host_id, username, secret_id) VALUES (?, 'deploy', ?)`, hid, id)
	if pw, ok, err := vault.HostGetPassword(ctx, d, hid); err != nil || !ok || pw != "s3cret" {
		t.Fatalf("linked password = %q ok=%v err=%v; want s3cret", pw, ok, err)
	}
	if err := vault.HostSetPassword(ctx, d, hid, uid, "s3cret"); err != nil {
		t.Fatal(err)
	}
	var same, diff int
	d.SQL.QueryRow(`SELECT COUNT(DISTINCT value_fingerprint) FROM secrets WHERE type = 'password' AND deleted_at IS NULL`).Scan(&same)
	if same != 1 {
		t.Fatalf("raw and wrapped s3cret → %d fingerprints, want 1", same)
	}
	createSecret(t, repo, actor, "password", "avulso", "shared", nil, "other", `{"value":"different"}`)
	d.SQL.QueryRow(`SELECT COUNT(DISTINCT value_fingerprint) FROM secrets WHERE type = 'password' AND deleted_at IS NULL`).Scan(&diff)
	if diff != 2 {
		t.Fatalf("different value shares a fingerprint")
	}
}

func createSecret(t *testing.T, repo *vault.SecretRepo, actor vault.ActorContext, typ, scope, vis string, parent *int64, name, payload string) int64 {
	t.Helper()
	id, err := repo.Create(context.Background(), actor, &models.Secret{
		Type: models.SecretType(typ), Scope: models.SecretScope(scope), Visibility: models.SecretVisibility(vis),
		ParentID: parent, OwnerUserID: actor.UserID, Name: name, KeyVersion: 1, CreatedBy: actor.UserID,
	}, payload)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return id
}
