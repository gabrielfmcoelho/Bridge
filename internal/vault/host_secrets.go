package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// host_secrets.go — helpers that bridge the legacy host-credentials access
// pattern (read host.PasswordCiphertext + decrypt with master key) over to
// the unified secrets table. This is the last consumer surface that touched
// the legacy `hosts.password_*` / `hosts.{pub,priv}_key_*` columns; once
// these helpers are wired everywhere, those columns can drop (Plans.md 1.9
// host-column slice).
//
// All host secrets are stored with visibility='shared' (an SSH password
// for a server is operational, not personal). The canonical names are
// hard-coded to match what migrate_legacy.go used: 'password' for the SSH
// password row, 'ssh-key' for the keypair JSON row. Callers that need
// custom names (multiple key pairs per host, etc.) can drop down to the
// generic CRUD on SecretRepo.

const (
	hostPasswordName = "password"
	hostSSHKeyName   = "ssh-key"
)

// HostSSHKey is the JSON payload shape for host SSH keys (matches what
// migrate_legacy.go produces). Decoded by HostGetSSHKey, encoded by
// HostSetSSHKey.
type HostSSHKey struct {
	Username      string `json:"username"`
	PrivateKeyPEM string `json:"private_key_pem"`
	PublicKey     string `json:"public_key"`
}

// HostGetPassword returns the host's shared SSH password, or ("", false, nil)
// when no password is stored. The bool is the "exists" indicator and
// matches the role that hosts.has_password used to play; callers that
// previously gated on host.HasPassword should gate on the bool here.
//
// Skips ACL because this is a server-side internal lookup performed by
// SSH automation (not a user-facing reveal). Use SecretRepo.Reveal for
// audit-logged reveals via the HTTP surface.
func HostGetPassword(ctx context.Context, db *database.DB, hostID int64) (string, bool, error) {
	return decryptHostSecret(ctx, db, hostID, models.SecretTypePassword, hostPasswordName, func(plain string) (string, error) {
		// Password payload format is the raw plaintext (legacy migrated
		// rows from migrate_legacy don't JSON-wrap). New rows written by
		// HostSetPassword follow the same shape so this stays a one-liner.
		return plain, nil
	})
}

// HostGetSSHKey returns the host's stored SSH key triple. ok=false when
// no key is stored. The JSON envelope matches spec §4.2 sshkey shape and
// what migrate_legacy.go emits.
func HostGetSSHKey(ctx context.Context, db *database.DB, hostID int64) (key HostSSHKey, ok bool, err error) {
	raw, ok, err := decryptHostSecret(ctx, db, hostID, models.SecretTypeSSHKey, hostSSHKeyName, func(plain string) (string, error) {
		return plain, nil
	})
	if err != nil || !ok {
		return HostSSHKey{}, ok, err
	}
	if err := json.Unmarshal([]byte(raw), &key); err != nil {
		return HostSSHKey{}, false, fmt.Errorf("host %d ssh-key payload: %w", hostID, err)
	}
	return key, true, nil
}

// HostSetPassword upserts the host's shared password as a secrets row.
// Empty plaintext clears the secret (soft-deletes the row) so callers can
// "remove the password" without dropping the host. actorUserID is the
// user performing the write — used for the audit row.
func HostSetPassword(ctx context.Context, db *database.DB, hostID, actorUserID int64, plaintext string) error {
	return upsertHostSecret(ctx, db, hostID, actorUserID,
		models.SecretTypePassword, hostPasswordName, plaintext)
}

// HostSetSSHKey upserts the JSON {username, private_key_pem, public_key}
// envelope under name='ssh-key'. Empty private+public clears the secret.
func HostSetSSHKey(ctx context.Context, db *database.DB, hostID, actorUserID int64, key HostSSHKey) error {
	if key.PrivateKeyPEM == "" && key.PublicKey == "" {
		// Clear path — soft-delete the existing row if any.
		return upsertHostSecret(ctx, db, hostID, actorUserID,
			models.SecretTypeSSHKey, hostSSHKeyName, "")
	}
	payload, err := json.Marshal(key)
	if err != nil {
		return fmt.Errorf("host ssh-key marshal: %w", err)
	}
	return upsertHostSecret(ctx, db, hostID, actorUserID,
		models.SecretTypeSSHKey, hostSSHKeyName, string(payload))
}

// decryptHostSecret looks up a live shared secret of (type, name) for the
// host, decrypts the payload, and runs `transform` on it. Returns
// ("", false, nil) when no live secret exists.
func decryptHostSecret(
	ctx context.Context,
	db *database.DB,
	hostID int64,
	typ models.SecretType,
	name string,
	transform func(string) (string, error),
) (string, bool, error) {
	var ct, nonce []byte
	err := db.SQL.QueryRowContext(ctx,
		`SELECT payload_ciphertext, payload_nonce FROM secrets
		  WHERE type = ? AND scope = 'host' AND parent_id = ? AND name = ?
		    AND visibility = 'shared' AND deleted_at IS NULL
		  LIMIT 1`,
		string(typ), hostID, name,
	).Scan(&ct, &nonce)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("host %d %s lookup: %w", hostID, typ, err)
	}
	plain, err := db.Encryptor.Decrypt(ct, nonce)
	if err != nil {
		return "", false, fmt.Errorf("host %d %s decrypt: %w", hostID, typ, err)
	}
	out, err := transform(plain)
	if err != nil {
		return "", false, err
	}
	return out, true, nil
}

// resolveSystemActor falls back to a deterministic user id when the caller
// didn't supply one. Tries username='sead-manutencao' (the maintenance
// user seeded by seed-maintenance-user) first, then any admin user, then
// any user at all. Required because secrets.owner_user_id is NOT NULL +
// FK — every host-secret write must point at a real user.
func resolveSystemActor(ctx context.Context, db *database.DB) int64 {
	var id int64
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT id FROM users WHERE username = 'sead-manutencao' LIMIT 1`).Scan(&id); err == nil && id > 0 {
		return id
	}
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`).Scan(&id); err == nil && id > 0 {
		return id
	}
	_ = db.SQL.QueryRowContext(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&id)
	return id
}

// upsertHostSecret writes/updates/clears the (type, name) secret for the
// host inside a single transaction. Empty plaintext soft-deletes any
// existing row; non-empty inserts or updates.
func upsertHostSecret(
	ctx context.Context,
	db *database.DB,
	hostID, actorUserID int64,
	typ models.SecretType,
	name string,
	plaintext string,
) error {
	if actorUserID <= 0 {
		actorUserID = resolveSystemActor(ctx, db)
		if actorUserID == 0 {
			return fmt.Errorf("host secret write: no users exist to attribute the secret to")
		}
	}
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingID int64
	switch err := tx.QueryRowContext(ctx,
		`SELECT id FROM secrets
		  WHERE type = ? AND scope = 'host' AND parent_id = ? AND name = ?
		    AND visibility = 'shared' AND deleted_at IS NULL`,
		string(typ), hostID, name,
	).Scan(&existingID); {
	case errors.Is(err, sql.ErrNoRows):
		existingID = 0
	case err != nil:
		return fmt.Errorf("host secret lookup: %w", err)
	}

	if plaintext == "" {
		if existingID == 0 {
			return tx.Commit() // nothing to clear; no-op
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE secrets SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			  WHERE id = ?`, existingID,
		); err != nil {
			return fmt.Errorf("clear host secret: %w", err)
		}
		if err := writeAuditTx(tx, existingID, models.SecretAuditActionDelete,
			ActorContext{UserID: actorUserID, Role: "admin"}, nil, nil); err != nil {
			return err
		}
		return tx.Commit()
	}

	ct, nonce, err := db.Encryptor.Encrypt(plaintext)
	if err != nil {
		return err
	}

	if existingID == 0 {
		// INSERT path.
		id, err := database.InsertReturningID(tx,
			`INSERT INTO secrets
			   (type, scope, visibility, parent_id, owner_user_id, name,
			    payload_ciphertext, payload_nonce, key_version, created_by)
			 VALUES (?, 'host', 'shared', ?, ?, ?, ?, ?, 1, ?)`,
			string(typ), hostID, actorUserID, name, ct, nonce, actorUserID,
		)
		if err != nil {
			return fmt.Errorf("insert host secret: %w", err)
		}
		if err := writeAuditTx(tx, id, models.SecretAuditActionCreate,
			ActorContext{UserID: actorUserID, Role: "admin"}, nil, nil); err != nil {
			return err
		}
		return tx.Commit()
	}

	// UPDATE path.
	if _, err := tx.ExecContext(ctx,
		`UPDATE secrets SET payload_ciphertext = ?, payload_nonce = ?,
		                    updated_at = CURRENT_TIMESTAMP, deleted_at = NULL
		  WHERE id = ?`,
		ct, nonce, existingID,
	); err != nil {
		return fmt.Errorf("update host secret: %w", err)
	}
	meta, _ := json.Marshal(map[string][]string{"changed_fields": {"payload"}})
	if err := writeAuditTx(tx, existingID, models.SecretAuditActionUpdate,
		ActorContext{UserID: actorUserID, Role: "admin"}, nil, meta); err != nil {
		return err
	}
	return tx.Commit()
}
