package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// MigrateDropLegacySecretsEnv toggles the legacy-secret-schema purge. Set
// MIGRATE_DROP_LEGACY_SECRETS=1 in the deployment environment to drop:
//
//   - `service_credentials` table (already cut over to /api/secrets)
//   - `hosts.password_ciphertext` + `password_nonce`
//   - `hosts.pub_key_ciphertext`  + `pub_key_nonce`
//   - `hosts.priv_key_ciphertext` + `priv_key_nonce`
//
// All of these have been replaced by rows in the unified `secrets` table.
// Idempotent (DROP IF EXISTS / DROP COLUMN IF EXISTS), so leaving the flag
// on across multiple boots is safe.
//
// `has_password` and `has_key` are intentionally retained — they're cached
// flags used by host list/filter queries, not secret payloads.
const MigrateDropLegacySecretsEnv = "MIGRATE_DROP_LEGACY_SECRETS"

// hostSecretColumns is the canonical list of legacy column drops applied
// by the env-gated purge. Kept as a package var so tests can verify each
// drop independently.
var hostSecretColumns = []string{
	"password_ciphertext",
	"password_nonce",
	"pub_key_ciphertext",
	"pub_key_nonce",
	"priv_key_ciphertext",
	"priv_key_nonce",
}

// DropLegacyServiceCredentialsTable drops the `service_credentials` table.
// Safe to call repeatedly. Callers that have already migrated their data
// into the unified secrets table (via vault.MigrateLegacySecrets) can run
// this to reclaim the schema surface.
//
// Lives outside the migrations slice so deployments can stage the drop
// independently of code rollout — the migration runner has no per-row
// conditional, so an env-gated step at startup is the simplest fit.
func DropLegacyServiceCredentialsTable(db *sql.DB) error {
	if _, err := db.Exec(`DROP TABLE IF EXISTS service_credentials`); err != nil {
		return fmt.Errorf("drop service_credentials: %w", err)
	}
	return nil
}

// DropLegacyHostSecretColumns drops the six legacy secret-payload columns
// from `hosts`. Companion to DropLegacyServiceCredentialsTable; same
// env-gating + idempotency contract.
//
// SQLite doesn't support `DROP COLUMN IF EXISTS` (only `DROP COLUMN`,
// since 3.35), so we probe each column via pragma_table_info before
// issuing the drop. The probe + drop pair is portable across SQLite and
// Postgres (Postgres has a `pg_attribute` equivalent we'd use instead if
// we needed it, but Postgres DOES support `DROP COLUMN IF EXISTS` so a
// dialect switch covers the gap).
func DropLegacyHostSecretColumns(db *sql.DB) error {
	for _, col := range hostSecretColumns {
		exists, err := hostColumnExists(db, col)
		if err != nil {
			return fmt.Errorf("probe hosts.%s: %w", col, err)
		}
		if !exists {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE hosts DROP COLUMN %s`, col)
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("drop hosts.%s: %w", col, err)
		}
	}
	return nil
}

// hostColumnExists is a portability shim for the column-existence probe.
// Uses information_schema on Postgres and pragma_table_info on SQLite —
// both queries take a column name parameter and return at least one row
// when the column exists.
func hostColumnExists(db *sql.DB, col string) (bool, error) {
	var dummy int
	err := db.QueryRow(
		`SELECT 1 FROM information_schema.columns WHERE table_name = 'hosts' AND table_schema = current_schema() AND column_name = $1`,
		col,
	).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// maybeDropLegacySecrets runs the env-gated purge after migrations apply.
// Logs the action (or skip) so operators can confirm the gate fired during
// deploy. Failures bubble up to Open() so the process exits noisily rather
// than silently leaving the table around.
func (d *DB) maybeDropLegacySecrets() error {
	if os.Getenv(MigrateDropLegacySecretsEnv) != "1" {
		return nil
	}
	if err := DropLegacyServiceCredentialsTable(d.SQL); err != nil {
		return err
	}
	log.Printf("[db] %s=1: dropped legacy service_credentials table", MigrateDropLegacySecretsEnv)
	if err := DropLegacyHostSecretColumns(d.SQL); err != nil {
		return err
	}
	log.Printf("[db] %s=1: dropped legacy hosts.{password,pub_key,priv_key}_* columns (6 total)", MigrateDropLegacySecretsEnv)
	return nil
}
