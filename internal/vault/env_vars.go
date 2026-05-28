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

// EnvVarUpsert is a single env_var inside a bulk-upsert payload. Description
// is intentionally optional and stored in the row's `description` column,
// not the encrypted blob — operators need to read it without unsealing.
type EnvVarUpsert struct {
	Name        string
	Value       string
	Description *string
}

// EnvVarBulkResult reports what BulkUpsertEnvVars did. Useful in handler
// responses so the frontend can render "5 created, 2 updated" toast.
type EnvVarBulkResult struct {
	Created int
	Updated int
}

// ErrEnvVarDuplicateInPayload signals that the same env var name appeared
// twice within a single bulk request. Per Task 2.2 this rejects the whole
// batch BEFORE any writes — sending the same key twice is almost certainly
// a UI bug, and silently picking last-wins would mask it.
var ErrEnvVarDuplicateInPayload = errors.New("env_var name appears multiple times in payload")

// BulkUpsertEnvVars writes a batch of env_var rows for the same parent +
// group_label inside one transaction. Each var either inserts (if no row
// matches the visibility-appropriate partial-unique key) or updates the
// existing row's payload + description. Audit rows go to secret_audit_log
// with the natural action ('create' or 'update') so the history view shows
// each var's lineage individually.
//
// Validation order matters — we reject the whole payload at the cheapest
// failing check so a 50-var request with one bad name doesn't ever touch
// the DB:
//  1. ACL (so unauthorized callers fail before we scan their values)
//  2. group_label regex
//  3. per-var name regex + non-empty value
//  4. in-payload duplicate names
//
// After validation, the tx opens. Any per-row DB failure rolls the whole
// batch back — spec 2.2 explicit DoD.
func (r *SecretRepo) BulkUpsertEnvVars(
	ctx context.Context,
	actor ActorContext,
	scope models.SecretScope,
	parentID *int64,
	visibility models.SecretVisibility,
	groupLabel string,
	vars []EnvVarUpsert,
) (EnvVarBulkResult, error) {
	var out EnvVarBulkResult

	// ACL: the same rules as a single Create. We don't have a row yet so we
	// short-circuit decideAccess for the two interesting cases.
	if visibility == models.SecretVisibilityShared {
		if actor.Role != "editor" && actor.Role != "admin" {
			return out, ErrSecretForbidden
		}
	}
	// Personal env_vars are always owned by the caller — handlers that
	// accept an owner_user_id different from the caller for personal
	// secrets reject at the handler layer (parity with Create).

	if err := models.ValidateEnvVarGroupLabel(groupLabel); err != nil {
		return out, err
	}
	if len(vars) == 0 {
		return out, fmt.Errorf("no env vars supplied")
	}
	seen := make(map[string]struct{}, len(vars))
	for _, v := range vars {
		if err := models.ValidateEnvVarName(v.Name); err != nil {
			return out, err
		}
		if v.Value == "" {
			return out, fmt.Errorf("env_var %q: value required", v.Name)
		}
		if _, dup := seen[v.Name]; dup {
			return out, fmt.Errorf("%w: %q", ErrEnvVarDuplicateInPayload, v.Name)
		}
		seen[v.Name] = struct{}{}
	}

	// Scope/parent_id invariants must match the CHECK constraint on secrets.
	if scope == models.SecretScopeAvulso && parentID != nil {
		return out, fmt.Errorf("scope %q (avulso) must not have parent_id", scope)
	}
	if scope != models.SecretScopeAvulso && parentID == nil {
		return out, fmt.Errorf("scope %q requires parent_id", scope)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	for _, v := range vars {
		payload, err := json.Marshal(map[string]string{"value": v.Value})
		if err != nil {
			return out, err
		}
		ct, nonce, err := r.enc.Encrypt(string(payload))
		if err != nil {
			return out, err
		}

		// Existence lookup by the visibility-appropriate partial-unique key.
		// Shared: (scope, parent_id, name, group_label). Personal: also
		// includes owner_user_id so two users may each own a personal copy
		// of "DB_URL" on the same service.
		var existingID int64
		var lookupErr error
		if visibility == models.SecretVisibilityShared {
			lookupErr = tx.QueryRowContext(ctx,
				`SELECT id FROM secrets
				  WHERE type = 'env_var' AND scope = ? AND parent_id = ?
				    AND name = ? AND group_label = ?
				    AND visibility = 'shared' AND deleted_at IS NULL`,
				string(scope), parentID, v.Name, groupLabel,
			).Scan(&existingID)
		} else {
			lookupErr = tx.QueryRowContext(ctx,
				`SELECT id FROM secrets
				  WHERE type = 'env_var' AND scope = ? AND parent_id = ?
				    AND owner_user_id = ? AND name = ? AND group_label = ?
				    AND visibility = 'personal' AND deleted_at IS NULL`,
				string(scope), parentID, actor.UserID, v.Name, groupLabel,
			).Scan(&existingID)
		}

		switch {
		case lookupErr == nil:
			// UPDATE path: payload + description + updated_at.
			if _, err := tx.ExecContext(ctx,
				`UPDATE secrets
				    SET payload_ciphertext = ?, payload_nonce = ?,
				        description = ?, updated_at = CURRENT_TIMESTAMP
				  WHERE id = ?`,
				ct, nonce, v.Description, existingID,
			); err != nil {
				return out, fmt.Errorf("update env_var %q: %w", v.Name, err)
			}
			meta, _ := json.Marshal(map[string][]string{"changed_fields": {"payload", "description"}})
			if err := writeAuditTx(tx, existingID, models.SecretAuditActionUpdate, actor, nil, meta); err != nil {
				return out, fmt.Errorf("audit update %q: %w", v.Name, err)
			}
			out.Updated++

		case errors.Is(lookupErr, sql.ErrNoRows):
			// INSERT path.
			owner := actor.UserID
			id, err := database.InsertReturningID(tx,
				`INSERT INTO secrets
				    (type, scope, visibility, parent_id, owner_user_id, name, group_label,
				     description, payload_ciphertext, payload_nonce, key_version, created_by)
				 VALUES ('env_var', ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
				string(scope), string(visibility), parentID, owner,
				v.Name, groupLabel, v.Description, ct, nonce, owner,
			)
			if err != nil {
				return out, fmt.Errorf("insert env_var %q: %w", v.Name, err)
			}
			if err := writeAuditTx(tx, id, models.SecretAuditActionCreate, actor, nil, nil); err != nil {
				return out, fmt.Errorf("audit insert %q: %w", v.Name, err)
			}
			out.Created++

		default:
			return out, fmt.Errorf("lookup env_var %q: %w", v.Name, lookupErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}
