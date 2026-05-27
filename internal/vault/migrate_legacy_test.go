package vault_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestMigrateLegacySecrets exercises the one-shot migration that copies
// service_credentials, hosts.password_* and hosts.{pub,priv}_key_* into the
// unified secrets table. Asserts the three invariants from Plans.md Task 1.5
// DoD: decrypted values match pre-migration plaintext, visibility=shared on
// all rows, owner_user_id resolves to the supplied maintenance user.
//
// Always runs (sqlite-only); the cross-dialect path is covered separately by
// TestCrossDialectRestore in internal/database.
func TestMigrateLegacySecrets(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	// Seed the maintenance user (production wires this up via the
	// seed-maintenance-user binary). The migration looks up by id, not
	// username, so the test resolves it once here.
	res, err := d.SQL.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
		"sead-manutencao", "x", "admin",
	)
	if err != nil {
		t.Fatalf("seed maintenance user: %v", err)
	}
	maintID, _ := res.LastInsertId()

	// Seed parent service and host so legacy rows have FK targets.
	svcRes, err := d.SQL.Exec(`INSERT INTO services (nickname) VALUES (?)`, "billing-api")
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	serviceID, _ := svcRes.LastInsertId()

	hostRes, err := d.SQL.Exec(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?)`,
		"db-01", "db-01", "10.0.0.1", "deploy",
	)
	if err != nil {
		t.Fatalf("seed host: %v", err)
	}
	hostID, _ := hostRes.LastInsertId()

	// Pre-encrypt the legacy payloads with the active encryptor; the
	// migration is expected to reuse the ciphertext for cred + password
	// and re-encrypt for sshkey.
	const (
		credPlain    = "postgres://billing:s3cr3t@db-01/billing"
		passwordPlain = "deploy-r00t-pw"
		privPlain    = "-----BEGIN OPENSSH PRIVATE KEY-----\nfakepriv\n-----END OPENSSH PRIVATE KEY-----"
		pubPlain     = "ssh-ed25519 AAAA... deploy@db-01"
	)

	credCT, credNonce, err := d.Encryptor.Encrypt(credPlain)
	if err != nil {
		t.Fatalf("enc cred: %v", err)
	}
	if _, err := d.SQL.Exec(
		`INSERT INTO service_credentials (service_id, role_name, credentials_ciphertext, credentials_nonce) VALUES (?,?,?,?)`,
		serviceID, "primary", credCT, credNonce,
	); err != nil {
		t.Fatalf("seed service_credential: %v", err)
	}

	pwCT, pwNonce, err := d.Encryptor.Encrypt(passwordPlain)
	if err != nil {
		t.Fatalf("enc host password: %v", err)
	}
	privCT, privNonce, err := d.Encryptor.Encrypt(privPlain)
	if err != nil {
		t.Fatalf("enc priv: %v", err)
	}
	pubCT, pubNonce, err := d.Encryptor.Encrypt(pubPlain)
	if err != nil {
		t.Fatalf("enc pub: %v", err)
	}
	if _, err := d.SQL.Exec(
		`UPDATE hosts SET
			has_password = 1, password_ciphertext = ?, password_nonce = ?,
			has_key = 1,
			pub_key_ciphertext = ?, pub_key_nonce = ?,
			priv_key_ciphertext = ?, priv_key_nonce = ?
		 WHERE id = ?`,
		pwCT, pwNonce, pubCT, pubNonce, privCT, privNonce, hostID,
	); err != nil {
		t.Fatalf("populate host secrets: %v", err)
	}

	// --- Run migration (function under test) ---
	stats, err := vault.MigrateLegacySecrets(ctx, d.SQL, d.Encryptor, maintID)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got, want := stats.ServiceCredsMigrated, 1; got != want {
		t.Errorf("service creds migrated: got %d, want %d", got, want)
	}
	if got, want := stats.HostPasswordsMigrated, 1; got != want {
		t.Errorf("host passwords migrated: got %d, want %d", got, want)
	}
	if got, want := stats.HostSSHKeysMigrated, 1; got != want {
		t.Errorf("host ssh keys migrated: got %d, want %d", got, want)
	}

	// --- Invariant 1: all migrated rows are visibility=shared, owner=maint ---
	rows, err := d.SQL.Query(
		`SELECT id, type, scope, visibility, parent_id, owner_user_id, name,
		        payload_ciphertext, payload_nonce, key_version
		   FROM secrets ORDER BY id`,
	)
	if err != nil {
		t.Fatalf("read secrets: %v", err)
	}
	defer rows.Close()

	type migratedRow struct {
		id, parentID, ownerID, keyVersion int64
		typ, scope, visibility, name      string
		ct, nonce                         []byte
	}
	var got []migratedRow
	for rows.Next() {
		var r migratedRow
		if err := rows.Scan(
			&r.id, &r.typ, &r.scope, &r.visibility, &r.parentID, &r.ownerID, &r.name,
			&r.ct, &r.nonce, &r.keyVersion,
		); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 3 {
		t.Fatalf("secrets row count: got %d, want 3", len(got))
	}
	for _, r := range got {
		if r.visibility != "shared" {
			t.Errorf("row %d: visibility=%q, want shared", r.id, r.visibility)
		}
		if r.ownerID != maintID {
			t.Errorf("row %d: owner_user_id=%d, want %d (maintenance)", r.id, r.ownerID, maintID)
		}
		if r.keyVersion != 1 {
			t.Errorf("row %d: key_version=%d, want 1", r.id, r.keyVersion)
		}
	}

	// --- Invariant 2: decrypted values match pre-migration plaintext ---
	for _, r := range got {
		plain, err := d.Encryptor.Decrypt(r.ct, r.nonce)
		if err != nil {
			t.Fatalf("decrypt row %d: %v", r.id, err)
		}
		switch r.typ {
		case "cred":
			if r.scope != "service" || r.parentID != serviceID {
				t.Errorf("cred row: scope=%q parent=%d; want service / %d", r.scope, r.parentID, serviceID)
			}
			if plain != credPlain {
				t.Errorf("cred plaintext mismatch: got %q want %q", plain, credPlain)
			}
		case "password":
			if r.scope != "host" || r.parentID != hostID {
				t.Errorf("password row: scope=%q parent=%d; want host / %d", r.scope, r.parentID, hostID)
			}
			if plain != passwordPlain {
				t.Errorf("password plaintext mismatch: got %q want %q", plain, passwordPlain)
			}
		case "sshkey":
			if r.scope != "host" || r.parentID != hostID {
				t.Errorf("sshkey row: scope=%q parent=%d; want host / %d", r.scope, r.parentID, hostID)
			}
			var payload struct {
				Username      string `json:"username"`
				PrivateKeyPEM string `json:"private_key_pem"`
				PublicKey     string `json:"public_key"`
			}
			if err := json.Unmarshal([]byte(plain), &payload); err != nil {
				t.Fatalf("sshkey payload is not JSON: %v (raw=%q)", err, plain)
			}
			if payload.Username != "deploy" {
				t.Errorf("sshkey username: got %q want %q", payload.Username, "deploy")
			}
			if payload.PrivateKeyPEM != privPlain {
				t.Errorf("sshkey private_key_pem mismatch:\n got %q\nwant %q", payload.PrivateKeyPEM, privPlain)
			}
			if payload.PublicKey != pubPlain {
				t.Errorf("sshkey public_key mismatch:\n got %q\nwant %q", payload.PublicKey, pubPlain)
			}
		default:
			t.Errorf("unexpected secret type %q on row %d", r.typ, r.id)
		}
	}

	// --- Invariant 3: idempotent (second run is a no-op) ---
	stats2, err := vault.MigrateLegacySecrets(ctx, d.SQL, d.Encryptor, maintID)
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if stats2.ServiceCredsMigrated != 0 || stats2.HostPasswordsMigrated != 0 || stats2.HostSSHKeysMigrated != 0 {
		t.Errorf("second run should be a no-op, got %+v", stats2)
	}
	if stats2.Skipped != 3 {
		t.Errorf("second run skipped: got %d, want 3", stats2.Skipped)
	}
	var total int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&total); err != nil {
		t.Fatalf("count secrets: %v", err)
	}
	if total != 3 {
		t.Errorf("after re-run, secrets count: got %d, want 3", total)
	}

	// --- Invariant 4: audit log records every migration insert ---
	var auditCount int
	if err := d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secret_audit_log WHERE action = 'create' AND actor_user_id = ?`,
		maintID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 3 {
		t.Errorf("audit rows for migration: got %d, want 3", auditCount)
	}
}

// TestMigrateLegacySecretsSkipsEmptyHostColumns guards against materialising
// a secret when the legacy host has no password and no key. Without this
// the migration would write empty-string ciphertext rows.
func TestMigrateLegacySecretsSkipsEmptyHostColumns(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	res, err := d.SQL.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
		"sead-manutencao", "x", "admin",
	)
	if err != nil {
		t.Fatalf("seed maint: %v", err)
	}
	maintID, _ := res.LastInsertId()

	// Host with NO secrets populated.
	if _, err := d.SQL.Exec(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?)`,
		"bare-host", "bare-host", "10.0.0.2", "",
	); err != nil {
		t.Fatalf("seed host: %v", err)
	}

	stats, err := vault.MigrateLegacySecrets(ctx, d.SQL, d.Encryptor, maintID)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.HostPasswordsMigrated != 0 || stats.HostSSHKeysMigrated != 0 {
		t.Errorf("bare host should produce zero rows, got %+v", stats)
	}
	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("secrets count: got %d, want 0", n)
	}
}
