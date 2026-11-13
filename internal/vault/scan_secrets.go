package vault

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// scan_secrets.go — secrets a host scan finds on the VM (a Redis password
// passed to JBoss as -DcacheServerPassword=…). Each is a shared env_var of the
// host, group "scan-<origin>", payload {"value": …}. Seeing the host is not
// enough to read one: revealPermitted also asks for PermHostSecretsReveal.

const (
	// ScanSecretGroupPrefix starts the group_label of every scan-found secret.
	ScanSecretGroupPrefix = "scan-"
	// PermHostSecretsReveal is the permission that reveals scan-found secrets.
	PermHostSecretsReveal = "hosts.secrets.reveal"
)

// UpsertScanSecret stores value as the host's (groupLabel, name) scan secret
// and returns its id. The same value and description write nothing; a change
// updates the row in place, audited. actorUserID <= 0 falls back to the system
// actor. Name and group are checked before anything is written; no error
// carries the value.
func UpsertScanSecret(ctx context.Context, db *database.DB, hostID, actorUserID int64, groupLabel, name, value, description string) (int64, error) {
	if err := models.ValidateEnvVarName(name); err != nil {
		return 0, err
	}
	if err := models.ValidateEnvVarGroupLabel(groupLabel); err != nil {
		return 0, err
	}
	// Without the prefix the row would fall back to the any-viewer reveal rule.
	if !strings.HasPrefix(groupLabel, ScanSecretGroupPrefix) {
		return 0, fmt.Errorf("scan secret group %q must start with %q", groupLabel, ScanSecretGroupPrefix)
	}
	if value == "" {
		return 0, fmt.Errorf("scan secret %s: empty value", name)
	}
	if actorUserID <= 0 {
		if actorUserID = resolveSystemActor(ctx, db); actorUserID == 0 {
			return 0, fmt.Errorf("scan secret write: no users exist to attribute the secret to")
		}
	}
	payload, err := json.Marshal(map[string]string{"value": value})
	if err != nil {
		return 0, err
	}
	plain := string(payload)
	derived, err := deriveCols(db.Encryptor, models.SecretTypeEnvVar, plain)
	if err != nil {
		return 0, err
	}
	desc := sql.NullString{String: description, Valid: description != ""}
	actor := ActorContext{UserID: actorUserID, Role: "admin"}

	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var (
		id      int64
		fp      []byte
		oldDesc sql.NullString
	)
	switch err := tx.QueryRowContext(ctx,
		`SELECT id, value_fingerprint, description FROM secrets
		  WHERE type = 'env_var' AND scope = 'host' AND parent_id = ? AND name = ? AND group_label = ?
		    AND visibility = 'shared' AND deleted_at IS NULL`,
		hostID, name, groupLabel,
	).Scan(&id, &fp, &oldDesc); {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return 0, fmt.Errorf("scan secret lookup: %w", err)
	}

	var changed []string
	if id != 0 {
		if !bytes.Equal(fp, derived.valueFP) {
			changed = append(changed, "payload")
		}
		if oldDesc.String != description {
			changed = append(changed, "description")
		}
		if len(changed) == 0 {
			return id, nil
		}
	}
	ct, nonce, err := db.Encryptor.Encrypt(plain)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		if id, err = database.InsertReturningID(tx,
			`INSERT INTO secrets
			   (type, scope, visibility, parent_id, owner_user_id, name, group_label, description,
			    payload_ciphertext, payload_nonce, key_version, created_by)
			 VALUES ('env_var', 'host', 'shared', ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
			hostID, actorUserID, name, groupLabel, desc, ct, nonce, actorUserID,
		); err != nil {
			return 0, fmt.Errorf("insert scan secret %s: %w", name, err)
		}
		if err := writeAuditTx(tx, id, models.SecretAuditActionCreate, actor, nil, nil); err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE secrets SET payload_ciphertext = ?, payload_nonce = ?, description = ?,
			                    updated_at = CURRENT_TIMESTAMP
			  WHERE id = ?`,
			ct, nonce, desc, id,
		); err != nil {
			return 0, fmt.Errorf("update scan secret %s: %w", name, err)
		}
		meta, _ := json.Marshal(map[string][]string{"changed_fields": changed})
		if err := writeAuditTx(tx, id, models.SecretAuditActionUpdate, actor, nil, meta); err != nil {
			return 0, err
		}
	}
	if err := stampDerived(ctx, tx, db.Encryptor, id, models.SecretTypeEnvVar, plain); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}
