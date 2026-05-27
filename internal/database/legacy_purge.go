package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// MigrateDropLegacySecretsEnv toggles the legacy-secret-schema purge. Set
// MIGRATE_DROP_LEGACY_SECRETS=1 in the deployment environment to drop the
// `service_credentials` table at startup. Idempotent (DROP IF EXISTS), so
// leaving the flag on across multiple boots is safe.
//
// The host password and SSH key columns on the `hosts` table are *not*
// purged by this hook — they're still actively consumed by host_handlers,
// coolify_handlers, sshconfig_handlers, sshkey_handlers, and import_handlers.
// A separate task that migrates those callers off the legacy columns must
// land before that part of the schema can drop.
const MigrateDropLegacySecretsEnv = "MIGRATE_DROP_LEGACY_SECRETS"

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
	return nil
}
