package database

import (
	"context"
	"database/sql"
	"fmt"
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

	// Recompute SSH key fingerprints that used the old hex format.
	d.fixSSHKeyFingerprints()

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
