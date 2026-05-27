package database_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
	"github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// TestCrossDialectRestoreWithLegacySecrets satisfies the DoD wording for
// Plans.md Task 1.5: "Extended TestCrossDialectRestore migrates a fixture
// with 1 service cred + 1 host password + 1 host sshkey and asserts:
// decrypted values match pre-migration plaintext; all migrated rows have
// visibility='shared'; owner_user_id resolves to the maintenance user."
//
// Lives in package database_test (external) because the cross-dialect path
// needs to call into internal/vault, which itself imports internal/database
// — placing this test in package database would create an import cycle.
//
// Always skips unless SSHCM_TEST_PG_DSN is set, mirroring the sibling
// TestCrossDialectRestore. The sqlite-only happy path is covered by
// vault.TestMigrateLegacySecrets which always runs.
func TestCrossDialectRestoreWithLegacySecrets(t *testing.T) {
	dsn := os.Getenv("SSHCM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SSHCM_TEST_PG_DSN not set")
	}

	const (
		credPlain     = "mysql://billing@db-01/billing?password=s3cret"
		passwordPlain = "deploy-r00t-pw-xdialect"
		privPlain     = "-----BEGIN OPENSSH PRIVATE KEY-----\nxdialect-priv\n-----END OPENSSH PRIVATE KEY-----"
		pubPlain      = "ssh-ed25519 AAAA...xd deploy@db-01"
	)

	// --- Source: sqlite seeded with users + service + host + legacy rows ---
	srcDir := t.TempDir()
	src, err := database.Open(srcDir)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer src.Close()

	if _, err := src.SQL.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
		"sead-manutencao", "x", "admin",
	); err != nil {
		t.Fatalf("seed maint user: %v", err)
	}
	// maint id is re-resolved by username on the pg side after restore;
	// the sqlite-side id is irrelevant.

	svcRes, err := src.SQL.Exec(`INSERT INTO services (nickname) VALUES (?)`, "billing-api")
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	serviceIDSrc, _ := svcRes.LastInsertId()

	hostRes, err := src.SQL.Exec(
		`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,?,?)`,
		"db-01", "db-01", "10.0.0.1", "deploy",
	)
	if err != nil {
		t.Fatalf("seed host: %v", err)
	}
	hostIDSrc, _ := hostRes.LastInsertId()

	credCT, credNonce, err := src.Encryptor.Encrypt(credPlain)
	if err != nil {
		t.Fatalf("enc cred: %v", err)
	}
	pwCT, pwNonce, err := src.Encryptor.Encrypt(passwordPlain)
	if err != nil {
		t.Fatalf("enc pw: %v", err)
	}
	privCT, privNonce, err := src.Encryptor.Encrypt(privPlain)
	if err != nil {
		t.Fatalf("enc priv: %v", err)
	}
	pubCT, pubNonce, err := src.Encryptor.Encrypt(pubPlain)
	if err != nil {
		t.Fatalf("enc pub: %v", err)
	}
	if _, err := src.SQL.Exec(
		`INSERT INTO service_credentials (service_id, role_name, credentials_ciphertext, credentials_nonce) VALUES (?,?,?,?)`,
		serviceIDSrc, "primary", credCT, credNonce,
	); err != nil {
		t.Fatalf("seed service_credential: %v", err)
	}
	if _, err := src.SQL.Exec(
		`UPDATE hosts SET
			has_password = 1, password_ciphertext = ?, password_nonce = ?,
			has_key = 1,
			pub_key_ciphertext = ?, pub_key_nonce = ?,
			priv_key_ciphertext = ?, priv_key_nonce = ?
		 WHERE id = ?`,
		pwCT, pwNonce, pubCT, pubNonce, privCT, privNonce, hostIDSrc,
	); err != nil {
		t.Fatalf("populate host secrets: %v", err)
	}

	// --- Backup -> portable ---
	var buf bytes.Buffer
	if err := src.WritePortableBackup(&buf); err != nil {
		t.Fatalf("backup: %v", err)
	}
	backup, err := database.ReadPortableBackup(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// --- Target: pg, restore, then run the migration ---
	sql.Register("pgx-test-vault", &stdlib.Driver{})
	rawPG, err := sql.Open("pgx-test-vault", dsn)
	if err != nil {
		t.Fatalf("open pg: %v", err)
	}
	defer rawPG.Close()

	os.Setenv("SSHCM_DB_DRIVER", "postgres")
	os.Setenv("SSHCM_DB_DSN", dsn)
	defer os.Unsetenv("SSHCM_DB_DRIVER")
	defer os.Unsetenv("SSHCM_DB_DSN")

	pgDir := filepath.Join(t.TempDir(), "pgcfg")
	tgt, err := database.Open(pgDir)
	if err != nil {
		t.Fatalf("open pg DB: %v", err)
	}
	defer tgt.Close()

	if err := tgt.RestorePortable(backup); err != nil {
		t.Fatalf("restore into pg: %v", err)
	}

	// Resolve the maintenance user id on the restored pg side (id may differ
	// from the sqlite source after restore re-assigns sequences).
	var maintIDTgt int64
	if err := tgt.SQL.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE username = $1`, "sead-manutencao").Scan(&maintIDTgt); err != nil {
		t.Fatalf("resolve maint user on pg: %v", err)
	}

	stats, err := vault.MigrateLegacySecrets(context.Background(), tgt.SQL, tgt.Encryptor, maintIDTgt)
	if err != nil {
		t.Fatalf("migrate on pg: %v", err)
	}
	if stats.ServiceCredsMigrated != 1 || stats.HostPasswordsMigrated != 1 || stats.HostSSHKeysMigrated != 1 {
		t.Fatalf("pg migration counts: got %+v", stats)
	}

	// --- Invariant: 3 rows, all shared, owner=maint, decrypted plaintexts intact ---
	rows, err := tgt.SQL.QueryContext(context.Background(),
		`SELECT type, scope, visibility, owner_user_id,
		        payload_ciphertext, payload_nonce
		   FROM secrets ORDER BY id`,
	)
	if err != nil {
		t.Fatalf("read secrets on pg: %v", err)
	}
	defer rows.Close()

	var seen int
	for rows.Next() {
		var (
			typ, scope, visibility string
			ownerID                int64
			ct, nonce              []byte
		)
		if err := rows.Scan(&typ, &scope, &visibility, &ownerID, &ct, &nonce); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		if visibility != "shared" {
			t.Errorf("type=%s: visibility=%q, want shared", typ, visibility)
		}
		if ownerID != maintIDTgt {
			t.Errorf("type=%s: owner=%d, want %d (maint)", typ, ownerID, maintIDTgt)
		}
		plain, err := tgt.Encryptor.Decrypt(ct, nonce)
		if err != nil {
			t.Fatalf("decrypt type=%s: %v", typ, err)
		}
		switch typ {
		case "cred":
			if plain != credPlain {
				t.Errorf("cred plaintext: got %q want %q", plain, credPlain)
			}
		case "password":
			if plain != passwordPlain {
				t.Errorf("password plaintext: got %q want %q", plain, passwordPlain)
			}
		case "sshkey":
			var p struct {
				Username      string `json:"username"`
				PrivateKeyPEM string `json:"private_key_pem"`
				PublicKey     string `json:"public_key"`
			}
			if err := json.Unmarshal([]byte(plain), &p); err != nil {
				t.Fatalf("sshkey json: %v", err)
			}
			if p.Username != "deploy" || p.PrivateKeyPEM != privPlain || p.PublicKey != pubPlain {
				t.Errorf("sshkey payload mismatch: %+v", p)
			}
		default:
			t.Errorf("unexpected type %q", typ)
		}
	}
	if seen != 3 {
		t.Errorf("migrated rows on pg: got %d, want 3", seen)
	}
}
