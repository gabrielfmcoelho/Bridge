package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// LegacyMigrationStats reports how many legacy rows were migrated or skipped.
// Skipped includes rows that were already present in the unified secrets
// table from a prior run (idempotency).
type LegacyMigrationStats struct {
	ServiceCredsMigrated  int
	HostPasswordsMigrated int
	HostSSHKeysMigrated   int
	Skipped               int
}

// migrationAuditMetadata is the metadata blob attached to the `create` audit
// row for every secret materialised by the legacy migration. Lets operators
// distinguish migrated rows from rows created via the regular handlers.
var migrationAuditMetadata = []byte(`{"migration":"v0_legacy"}`)

// MigrateLegacySecrets copies legacy secret-bearing rows into the unified
// `secrets` table. Run once after applying migration v61.
//
// Sources covered:
//   - service_credentials.{credentials_ciphertext, credentials_nonce}
//     → secrets(type=cred,     scope=service, visibility=shared)
//   - hosts.{password_ciphertext, password_nonce}
//     → secrets(type=password, scope=host,    visibility=shared)
//   - hosts.{pub_key_*, priv_key_*}
//     → secrets(type=sshkey,   scope=host,    visibility=shared)
//
// Per Plans.md Task 1.5 the cred and password ciphertext are reused verbatim
// (the legacy on-disk shape is a free-form string blob; future readers will
// need to handle both legacy and spec §4.2 JSON shapes during a transition
// window). The sshkey pair is decrypted, combined into the spec §4.2 JSON
// shape `{"username","private_key_pem","public_key"}`, and re-encrypted —
// this is the only path that touches plaintext.
//
// Every materialised secret is owned by ownerUserID (typically the
// maintenance user resolved by username at the call site) and key_version=1.
// Every insert writes a matching `create` audit row with metadata
// `{"migration":"v0_legacy"}` so operators can audit-trail the bulk import.
//
// Idempotent: a second invocation skips rows that already have a matching
// secret per the spec's partial-unique key
// `(scope, parent_id, name)` WHERE visibility='shared' AND deleted_at IS NULL.
//
// The whole operation runs in a single transaction so a mid-flight failure
// leaves the legacy tables untouched and the secrets table unchanged.
func MigrateLegacySecrets(ctx context.Context, db *sql.DB, enc *database.Encryptor, ownerUserID int64) (LegacyMigrationStats, error) {
	var stats LegacyMigrationStats
	if ownerUserID <= 0 {
		return stats, fmt.Errorf("ownerUserID must be > 0 (resolved maintenance user)")
	}
	if enc == nil {
		return stats, fmt.Errorf("encryptor is nil")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := migrateServiceCredentials(tx, ownerUserID, &stats); err != nil {
		return stats, fmt.Errorf("service_credentials: %w", err)
	}
	if err := migrateHostPasswords(tx, ownerUserID, &stats); err != nil {
		return stats, fmt.Errorf("host passwords: %w", err)
	}
	if err := migrateHostSSHKeys(tx, enc, ownerUserID, &stats); err != nil {
		return stats, fmt.Errorf("host ssh keys: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

func migrateServiceCredentials(tx *sql.Tx, ownerUserID int64, stats *LegacyMigrationStats) error {
	rows, err := tx.Query(
		`SELECT service_id, role_name, credentials_ciphertext, credentials_nonce
		   FROM service_credentials
		  WHERE credentials_ciphertext IS NOT NULL
		    AND length(credentials_ciphertext) > 0`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			serviceID int64
			roleName  string
			ct, nonce []byte
		)
		if err := rows.Scan(&serviceID, &roleName, &ct, &nonce); err != nil {
			return err
		}
		inserted, err := insertSharedSecret(tx, "cred", "service", serviceID, roleName, ct, nonce, ownerUserID)
		if err != nil {
			return fmt.Errorf("service %d role %q: %w", serviceID, roleName, err)
		}
		if inserted {
			stats.ServiceCredsMigrated++
		} else {
			stats.Skipped++
		}
	}
	return rows.Err()
}

func migrateHostPasswords(tx *sql.Tx, ownerUserID int64, stats *LegacyMigrationStats) error {
	rows, err := tx.Query(
		`SELECT id, password_ciphertext, password_nonce
		   FROM hosts
		  WHERE password_ciphertext IS NOT NULL
		    AND length(password_ciphertext) > 0`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			hostID    int64
			ct, nonce []byte
		)
		if err := rows.Scan(&hostID, &ct, &nonce); err != nil {
			return err
		}
		inserted, err := insertSharedSecret(tx, "password", "host", hostID, "password", ct, nonce, ownerUserID)
		if err != nil {
			return fmt.Errorf("host %d password: %w", hostID, err)
		}
		if inserted {
			stats.HostPasswordsMigrated++
		} else {
			stats.Skipped++
		}
	}
	return rows.Err()
}

func migrateHostSSHKeys(tx *sql.Tx, enc *database.Encryptor, ownerUserID int64, stats *LegacyMigrationStats) error {
	rows, err := tx.Query(
		`SELECT id, ssh_user,
		        pub_key_ciphertext, pub_key_nonce,
		        priv_key_ciphertext, priv_key_nonce
		   FROM hosts
		  WHERE priv_key_ciphertext IS NOT NULL
		    AND length(priv_key_ciphertext) > 0
		    AND pub_key_ciphertext  IS NOT NULL
		    AND length(pub_key_ciphertext)  > 0`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pendingKey struct {
		hostID                  int64
		sshUser                 string
		pubCT, pubN, privCT, privN []byte
	}
	var pending []pendingKey
	for rows.Next() {
		var p pendingKey
		if err := rows.Scan(&p.hostID, &p.sshUser, &p.pubCT, &p.pubN, &p.privCT, &p.privN); err != nil {
			return err
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range pending {
		priv, err := enc.Decrypt(p.privCT, p.privN)
		if err != nil {
			return fmt.Errorf("host %d decrypt priv: %w", p.hostID, err)
		}
		pub, err := enc.Decrypt(p.pubCT, p.pubN)
		if err != nil {
			return fmt.Errorf("host %d decrypt pub: %w", p.hostID, err)
		}
		payload, err := json.Marshal(struct {
			Username      string `json:"username"`
			PrivateKeyPEM string `json:"private_key_pem"`
			PublicKey     string `json:"public_key"`
		}{
			Username:      p.sshUser,
			PrivateKeyPEM: priv,
			PublicKey:     pub,
		})
		if err != nil {
			return fmt.Errorf("host %d marshal sshkey: %w", p.hostID, err)
		}
		ct, nonce, err := enc.Encrypt(string(payload))
		if err != nil {
			return fmt.Errorf("host %d encrypt sshkey: %w", p.hostID, err)
		}
		inserted, err := insertSharedSecret(tx, "sshkey", "host", p.hostID, "ssh-key", ct, nonce, ownerUserID)
		if err != nil {
			return fmt.Errorf("host %d sshkey: %w", p.hostID, err)
		}
		if inserted {
			stats.HostSSHKeysMigrated++
		} else {
			stats.Skipped++
		}
	}
	return nil
}

// insertSharedSecret writes one migrated row + its audit entry. Returns
// (true, nil) on insert; (false, nil) when an existing shared secret with the
// same (scope, parent_id, name) already exists (idempotency on re-runs).
func insertSharedSecret(
	tx *sql.Tx,
	typ, scope string,
	parentID int64,
	name string,
	ct, nonce []byte,
	ownerUserID int64,
) (bool, error) {
	var existing int64
	err := tx.QueryRow(
		`SELECT id FROM secrets
		  WHERE scope = ? AND parent_id = ? AND name = ?
		    AND visibility = 'shared' AND deleted_at IS NULL`,
		scope, parentID, name,
	).Scan(&existing)
	switch {
	case err == nil:
		return false, nil
	case err == sql.ErrNoRows:
		// fall through to insert
	default:
		return false, fmt.Errorf("existence check: %w", err)
	}

	secretID, err := database.InsertReturningID(tx,
		`INSERT INTO secrets
		   (type, scope, visibility, parent_id, owner_user_id, name,
		    payload_ciphertext, payload_nonce, key_version, created_by)
		 VALUES (?, ?, 'shared', ?, ?, ?, ?, ?, 1, ?)`,
		typ, scope, parentID, ownerUserID, name, ct, nonce, ownerUserID,
	)
	if err != nil {
		return false, fmt.Errorf("insert: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO secret_audit_log (secret_id, action, actor_user_id, metadata)
		 VALUES (?, 'create', ?, ?)`,
		secretID, ownerUserID, migrationAuditMetadata,
	); err != nil {
		return false, fmt.Errorf("audit insert: %w", err)
	}
	return true, nil
}
