package database

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"
)

// DB wraps a SQL database connection and an Encryptor for sensitive data.
type DB struct {
	SQL       *sql.DB
	Encryptor *Encryptor
	Dialect   DialectKind
	configDir string
}

// Open initialises the database and runs migrations. Backend selection is
// controlled by SSHCM_DB_DRIVER ("sqlite" default, or "postgres"). For the
// postgres driver, SSHCM_DB_DSN must be set to a pgx-compatible DSN.
func Open(configDir string) (*DB, error) {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("database: create dir: %w", err)
	}

	enc, err := NewEncryptor(filepath.Join(configDir, "secret.key"))
	if err != nil {
		return nil, fmt.Errorf("database: encryptor: %w", err)
	}

	cfg := resolveDriverConfig(configDir)
	active = cfg.kind

	var db *sql.DB
	switch cfg.kind {
	case DialectPostgres:
		if cfg.dsn == "" {
			return nil, fmt.Errorf("database: SSHCM_DB_DSN is required when SSHCM_DB_DRIVER=postgres")
		}
		registerPgxRebindDriver()
		db, err = sql.Open(pgxRebindDriverName, cfg.dsn)
		if err != nil {
			return nil, fmt.Errorf("database: open postgres: %w", err)
		}
		// Pool tuning. Postgres benefits from a bounded pool; sqlite is
		// effectively serial so we leave it at Go's defaults. Values are
		// env-overridable for deployments with unusual workloads.
		db.SetMaxOpenConns(envInt("SSHCM_DB_MAX_OPEN", 20))
		db.SetMaxIdleConns(envInt("SSHCM_DB_MAX_IDLE", 5))
		db.SetConnMaxLifetime(time.Duration(envInt("SSHCM_DB_MAX_LIFETIME_SEC", 1800)) * time.Second)
		db.SetConnMaxIdleTime(time.Duration(envInt("SSHCM_DB_MAX_IDLE_SEC", 300)) * time.Second)
	default:
		db, err = sql.Open(cfg.driver, cfg.dsn)
		if err != nil {
			return nil, fmt.Errorf("database: open sqlite: %w", err)
		}
		// SQLite-only PRAGMAs for WAL concurrency and FK enforcement.
		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: wal mode: %w", err)
		}
		if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: foreign keys: %w", err)
		}
	}

	d := &DB{SQL: db, Encryptor: enc, Dialect: cfg.kind, configDir: configDir}
	if err := d.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: migrations: %w", err)
	}

	if err := d.guardAgainstLostKey(); err != nil {
		db.Close()
		return nil, err
	}

	// Recompute SSH key fingerprints that used the old hex format.
	d.fixSSHKeyFingerprints()

	// Backfill encrypted key blobs on legacy host rows that still point at a
	// filesystem path. After this runs, a DB backup carries the full key data
	// and can be restored on any machine without broken key_path references.
	d.backfillHostKeyBlobs()

	return d, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.SQL.Close()
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// ConfigDir returns the configuration directory path.
func (d *DB) ConfigDir() string {
	return d.configDir
}

// DBPath returns the full path to the SQLite database file.
func (d *DB) DBPath() string {
	return filepath.Join(d.configDir, "sshcm.db")
}

// guardAgainstLostKey protects against silent data loss when the encryption
// key changes. It runs in two modes:
//
//  1. If SSHCM_RESET_ENCRYPTED_SETTINGS=true, it attempts to decrypt every
//     *_cipher value in app_settings with the current key and DELETEs the
//     rows that fail (along with their paired *_nonce row). This is the
//     escape hatch for "I lost the old key and want to start fresh without
//     hand-editing the database". Settings outside app_settings (host
//     passwords, SSH keys, tool credentials, OAuth tokens) are left alone —
//     wiping those carries much bigger blast radius and should be a
//     deliberate manual operation.
//
//  2. Otherwise, if the key was freshly generated this run AND app_settings
//     already contains encrypted rows, startup aborts. That combination
//     means the previous key was lost (e.g. a redeploy without a persistent
//     volume or SSHCM_SECRET_KEY) and existing ciphertext is unreadable —
//     continuing would silently break OAuth and integrations.
func (d *DB) guardAgainstLostKey() error {
	if envFlag("SSHCM_RESET_ENCRYPTED_SETTINGS") {
		return d.purgeUndecryptableSettings()
	}

	if d.Encryptor.Source() != KeySourceGenerated {
		return nil
	}

	var count int
	err := d.SQL.QueryRow(`
		SELECT COUNT(*) FROM app_settings
		WHERE key LIKE '%_cipher' AND value IS NOT NULL AND value <> ''
	`).Scan(&count)
	if err != nil {
		// If the check itself fails we'd rather start than refuse — the worst
		// case here is the user gets the original decrypt errors back.
		return nil
	}
	if count == 0 {
		return nil
	}

	return fmt.Errorf(
		"database: encryption key was just generated but %d encrypted setting(s) already exist in the database — "+
			"the previous key is gone and existing ciphertext cannot be decrypted. "+
			"Set SSHCM_SECRET_KEY to the previous base64 key (or mount the prior secret.key into %s) and restart. "+
			"To wipe the unreadable settings and start fresh, set SSHCM_RESET_ENCRYPTED_SETTINGS=true",
		count, d.configDir,
	)
}

// purgeUndecryptableSettings tries to decrypt every encrypted row in
// app_settings with the active key. Rows that fail (wrong key, corrupt data)
// are deleted together with their paired _nonce row. The action is logged so
// operators can audit what was wiped.
func (d *DB) purgeUndecryptableSettings() error {
	rows, err := d.SQL.Query(`
		SELECT key, value FROM app_settings
		WHERE key LIKE '%_cipher' AND value IS NOT NULL AND value <> ''
	`)
	if err != nil {
		return fmt.Errorf("purge encrypted settings: query: %w", err)
	}

	type entry struct{ key, value string }
	var ciphers []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.key, &e.value); err != nil {
			rows.Close()
			return fmt.Errorf("purge encrypted settings: scan: %w", err)
		}
		ciphers = append(ciphers, e)
	}
	rows.Close()

	var doomed []string
	for _, c := range ciphers {
		nonceKey := strings.TrimSuffix(c.key, "_cipher") + "_nonce"
		var nonceHex string
		if err := d.SQL.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, nonceKey).Scan(&nonceHex); err != nil {
			// Cipher without a nonce is already broken — purge it.
			doomed = append(doomed, c.key, nonceKey)
			continue
		}
		cipherBytes, errC := hex.DecodeString(c.value)
		nonceBytes, errN := hex.DecodeString(nonceHex)
		if errC != nil || errN != nil {
			doomed = append(doomed, c.key, nonceKey)
			continue
		}
		if _, err := d.Encryptor.Decrypt(cipherBytes, nonceBytes); err != nil {
			doomed = append(doomed, c.key, nonceKey)
		}
	}

	if len(doomed) == 0 {
		log.Printf("SSHCM_RESET_ENCRYPTED_SETTINGS=true: every encrypted setting decrypted cleanly, nothing to purge")
		return nil
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return fmt.Errorf("purge encrypted settings: begin: %w", err)
	}
	for _, k := range doomed {
		if _, err := tx.Exec(`DELETE FROM app_settings WHERE key = ?`, k); err != nil {
			tx.Rollback()
			return fmt.Errorf("purge encrypted settings: delete %q: %w", k, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("purge encrypted settings: commit: %w", err)
	}

	log.Printf("SSHCM_RESET_ENCRYPTED_SETTINGS=true: deleted %d undecryptable app_settings row(s): %v",
		len(doomed), doomed)
	return nil
}

func envFlag(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// fixSSHKeyFingerprints recomputes fingerprints for SSH keys that need it:
// keys with old truncated-hex fingerprints, or keys with only a private key
// (no public key stored). For the latter it also derives and stores the public key.
func (d *DB) fixSSHKeyFingerprints() {
	rows, err := d.SQL.Query(`SELECT id, fingerprint, pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce FROM ssh_keys`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var fp string
		var pubCT, pubNonce, privCT, privNonce []byte
		if err := rows.Scan(&id, &fp, &pubCT, &pubNonce, &privCT, &privNonce); err != nil {
			continue
		}
		// Skip if already in standard format
		if strings.HasPrefix(fp, "SHA256:") && len(fp) > 30 {
			continue
		}

		// Try from public key first
		if len(pubCT) > 0 {
			if pubKeyText, err := d.Encryptor.Decrypt(pubCT, pubNonce); err == nil {
				if pub, _, _, _, err := gossh.ParseAuthorizedKey([]byte(pubKeyText)); err == nil {
					d.SQL.Exec(`UPDATE ssh_keys SET fingerprint = ? WHERE id = ?`, gossh.FingerprintSHA256(pub), id)
					continue
				}
			}
		}

		// Derive from private key if no public key is available
		if len(privCT) > 0 {
			privKeyText, err := d.Encryptor.Decrypt(privCT, privNonce)
			if err != nil {
				continue
			}
			signer, err := gossh.ParsePrivateKey([]byte(privKeyText))
			if err != nil {
				continue
			}
			pub := signer.PublicKey()
			newFP := gossh.FingerprintSHA256(pub)
			// Store derived public key and fingerprint
			pubKeyStr := string(gossh.MarshalAuthorizedKey(pub))
			if ct, nonce, err := d.Encryptor.Encrypt(pubKeyStr); err == nil {
				d.SQL.Exec(`UPDATE ssh_keys SET fingerprint = ?, pub_key_ciphertext = ?, pub_key_nonce = ? WHERE id = ?`, newFP, ct, nonce, id)
			}
		}
	}
}

// backfillHostKeyBlobs one-shot normalizes legacy host rows where the key
// lives only as a filesystem path. For each such row it reads the private (and
// optional public) key file from disk, encrypts it via the DB encryptor, stores
// the blobs on the host row, and clears key_path. Rows whose file is missing
// are left alone and logged — the user can re-link an SSH key via the editor.
// Idempotent: rows already carrying a blob are skipped.
func (d *DB) backfillHostKeyBlobs() {
	rows, err := d.SQL.Query(`SELECT id, key_path, priv_key_ciphertext FROM hosts WHERE has_key = ? AND (priv_key_ciphertext IS NULL OR length(priv_key_ciphertext) = 0) AND key_path IS NOT NULL AND key_path <> ''`, true)
	if err != nil {
		return
	}
	type legacyRow struct {
		id   int64
		path string
	}
	var legacy []legacyRow
	for rows.Next() {
		var id int64
		var kp string
		var existing []byte
		if err := rows.Scan(&id, &kp, &existing); err != nil {
			continue
		}
		if len(existing) > 0 {
			continue
		}
		legacy = append(legacy, legacyRow{id: id, path: kp})
	}
	rows.Close()

	if len(legacy) == 0 {
		return
	}

	home, _ := os.UserHomeDir()
	expand := func(p string) string {
		if strings.HasPrefix(p, "~") && home != "" {
			return home + p[1:]
		}
		return p
	}

	for _, row := range legacy {
		privPath := expand(row.path)
		privBytes, rerr := os.ReadFile(privPath)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "[db] backfill: host id=%d key_path=%q missing on disk: %v — leaving row untouched; re-link an SSH key to restore access\n", row.id, privPath, rerr)
			continue
		}
		privCT, privNonce, eerr := d.Encryptor.Encrypt(string(privBytes))
		if eerr != nil {
			fmt.Fprintf(os.Stderr, "[db] backfill: host id=%d encrypt private key failed: %v\n", row.id, eerr)
			continue
		}

		var pubCT, pubNonce []byte
		if pubBytes, perr := os.ReadFile(privPath + ".pub"); perr == nil {
			if ct, nonce, eerr := d.Encryptor.Encrypt(string(pubBytes)); eerr == nil {
				pubCT, pubNonce = ct, nonce
			}
		}

		if _, uerr := d.SQL.Exec(
			`UPDATE hosts SET priv_key_ciphertext = ?, priv_key_nonce = ?, pub_key_ciphertext = ?, pub_key_nonce = ?, key_path = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			privCT, privNonce, pubCT, pubNonce, row.id,
		); uerr != nil {
			fmt.Fprintf(os.Stderr, "[db] backfill: host id=%d update failed: %v\n", row.id, uerr)
			continue
		}
		fmt.Fprintf(os.Stderr, "[db] backfill: host id=%d key migrated from %q into encrypted DB blob\n", row.id, privPath)
	}
}

func (d *DB) runMigrations() error {
	ctx := context.Background()

	migrations := migrationsSQLite
	if d.Dialect == DialectPostgres {
		migrations = migrationsPostgres
	}

	// Use a single connection so PRAGMA settings persist across transactions.
	conn, err := d.SQL.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	// Create the migrations tracking table. Postgres and SQLite share the
	// same DDL here; DATETIME is accepted by Postgres as an alias for
	// TIMESTAMP, and INTEGER PRIMARY KEY is valid in both dialects.
	trackingDDL := `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err = conn.ExecContext(ctx, trackingDDL); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Determine the current version.
	var current int
	err = conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), -1) FROM schema_migrations`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}

	// SQLite disables FK checks during schema changes (needed for the v38
	// table-rebuild migration). Postgres doesn't need this.
	if d.Dialect == DialectSQLite {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
			return fmt.Errorf("disable fk: %w", err)
		}
	}

	insertVersionSQL := `INSERT INTO schema_migrations (version) VALUES (?)`
	if d.Dialect == DialectPostgres {
		insertVersionSQL = `INSERT INTO schema_migrations (version) VALUES ($1)`
	}

	for i := current + 1; i < len(migrations); i++ {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", i, err)
		}

		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			if d.Dialect == DialectSQLite {
				conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`)
			}
			return fmt.Errorf("migration %d: %w", i, err)
		}

		if _, err := tx.Exec(insertVersionSQL, i); err != nil {
			tx.Rollback()
			if d.Dialect == DialectSQLite {
				conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`)
			}
			return fmt.Errorf("record migration %d: %w", i, err)
		}

		if err := tx.Commit(); err != nil {
			if d.Dialect == DialectSQLite {
				conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`)
			}
			return fmt.Errorf("commit migration %d: %w", i, err)
		}
	}

	// Re-enable FK checks (SQLite only).
	if d.Dialect == DialectSQLite {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
			return fmt.Errorf("enable fk: %w", err)
		}
	}

	return nil
}
