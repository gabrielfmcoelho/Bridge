package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

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

// EnvVarTarget is one (scope, parent) destination for a bulk env_var write.
// A single bulk request may fan out to several targets — e.g. a project AND
// one of its services — all written inside one transaction.
type EnvVarTarget struct {
	Scope    models.SecretScope
	ParentID *int64
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

// BulkUpsertEnvVars writes a batch of env_var rows for a single (scope, parent)
// + group_label inside one transaction. Thin wrapper over the multi-target
// path so existing single-target callers/tests don't change.
func (r *SecretRepo) BulkUpsertEnvVars(
	ctx context.Context,
	actor ActorContext,
	scope models.SecretScope,
	parentID *int64,
	visibility models.SecretVisibility,
	groupLabel string,
	vars []EnvVarUpsert,
) (EnvVarBulkResult, error) {
	return r.BulkUpsertEnvVarsMulti(ctx, actor,
		[]EnvVarTarget{{Scope: scope, ParentID: parentID}},
		visibility, groupLabel, vars)
}

// BulkUpsertEnvVarsMulti writes the same var-set to one or more targets inside
// a single transaction. Each var either inserts (if no row matches the
// visibility-appropriate partial-unique key) or updates the existing row.
// Audit rows go to secret_audit_log per var per target.
//
// Validation order matters — we reject at the cheapest failing check so a
// 50-var request never touches the DB on the obvious errors:
//  1. ACL (so unauthorized callers fail before we scan their values)
//  2. group_label regex
//  3. per-var name regex + non-empty value + in-payload duplicate names
//  4. per-target scope/parent invariants + duplicate targets
//  5. project<->service guard (a service target must belong to a projeto
//     target when both are present — defense in depth; the UI also filters)
//
// After validation the tx opens; every var is upserted for every target. Any
// per-row failure rolls the WHOLE batch back — all targets, all vars.
func (r *SecretRepo) BulkUpsertEnvVarsMulti(
	ctx context.Context,
	actor ActorContext,
	targets []EnvVarTarget,
	visibility models.SecretVisibility,
	groupLabel string,
	vars []EnvVarUpsert,
) (EnvVarBulkResult, error) {
	var out EnvVarBulkResult

	// 1. ACL: the same rules as a single Create. We don't have a row yet so we
	// short-circuit decideAccess for the two interesting cases. Personal
	// env_vars are always owned by the caller (handlers reject a mismatched
	// owner_user_id for personal secrets, parity with Create).
	if visibility == models.SecretVisibilityShared {
		if actor.Role != "editor" && actor.Role != "admin" {
			return out, ErrSecretForbidden
		}
	}

	// 2. group_label.
	if err := models.ValidateEnvVarGroupLabel(groupLabel); err != nil {
		return out, err
	}

	// 3. vars.
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

	// 4. targets: scope/parent invariants (match the CHECK constraint on
	// secrets) + duplicate (scope, parent) detection.
	if len(targets) == 0 {
		return out, fmt.Errorf("no targets supplied")
	}
	targetSeen := make(map[string]struct{}, len(targets))
	for _, tg := range targets {
		if tg.Scope == models.SecretScopeAvulso && tg.ParentID != nil {
			return out, fmt.Errorf("scope %q (avulso) must not have parent_id", tg.Scope)
		}
		if tg.Scope != models.SecretScopeAvulso && tg.ParentID == nil {
			return out, fmt.Errorf("scope %q requires parent_id", tg.Scope)
		}
		key := string(tg.Scope) + ":"
		if tg.ParentID != nil {
			key += strconv.FormatInt(*tg.ParentID, 10)
		}
		if _, dup := targetSeen[key]; dup {
			return out, fmt.Errorf("duplicate target %q", key)
		}
		targetSeen[key] = struct{}{}
	}

	// 5. project<->service guard.
	if err := r.guardServiceInProject(ctx, targets); err != nil {
		return out, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	for _, tg := range targets {
		for _, v := range vars {
			created, err := r.upsertEnvVarRow(ctx, tx, actor, tg.Scope, tg.ParentID, visibility, groupLabel, v)
			if err != nil {
				return out, err
			}
			if created {
				out.Created++
			} else {
				out.Updated++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

// guardServiceInProject rejects a target set where a service is synced
// alongside a project it does not belong to. Only enforced when at least one
// projeto target is present — a service synced without any project stands
// alone (the supported "non-project env_var" case).
func (r *SecretRepo) guardServiceInProject(ctx context.Context, targets []EnvVarTarget) error {
	projectIDs := make(map[int64]struct{})
	for _, tg := range targets {
		if tg.Scope == models.SecretScopeProjeto && tg.ParentID != nil {
			projectIDs[*tg.ParentID] = struct{}{}
		}
	}
	if len(projectIDs) == 0 {
		return nil
	}
	for _, tg := range targets {
		if tg.Scope != models.SecretScopeService || tg.ParentID == nil {
			continue
		}
		var projectID sql.NullInt64
		err := r.db.QueryRowContext(ctx,
			`SELECT project_id FROM services WHERE id = ?`, *tg.ParentID).Scan(&projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("service %d not found", *tg.ParentID)
			}
			return err
		}
		if !projectID.Valid {
			return fmt.Errorf("service %d has no project; cannot sync alongside a project target", *tg.ParentID)
		}
		if _, ok := projectIDs[projectID.Int64]; !ok {
			return fmt.Errorf("service %d does not belong to the selected project", *tg.ParentID)
		}
	}
	return nil
}

// upsertEnvVarRow inserts or updates a single env_var row inside an open tx.
// Returns created=true on insert, false on update. Existence is looked up by
// the visibility-appropriate partial-unique key: shared uses (scope, parent,
// name, group_label); personal also includes owner_user_id so two users may
// each own a personal copy of "DB_URL" on the same parent.
func (r *SecretRepo) upsertEnvVarRow(
	ctx context.Context,
	tx *sql.Tx,
	actor ActorContext,
	scope models.SecretScope,
	parentID *int64,
	visibility models.SecretVisibility,
	groupLabel string,
	v EnvVarUpsert,
) (bool, error) {
	payload, err := json.Marshal(map[string]string{"value": v.Value})
	if err != nil {
		return false, err
	}
	ct, nonce, err := r.enc.Encrypt(string(payload))
	if err != nil {
		return false, err
	}

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
			return false, fmt.Errorf("update env_var %q: %w", v.Name, err)
		}
		meta, _ := json.Marshal(map[string][]string{"changed_fields": {"payload", "description"}})
		if err := writeAuditTx(tx, existingID, models.SecretAuditActionUpdate, actor, nil, meta); err != nil {
			return false, fmt.Errorf("audit update %q: %w", v.Name, err)
		}
		return false, nil

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
			return false, fmt.Errorf("insert env_var %q: %w", v.Name, err)
		}
		if err := writeAuditTx(tx, id, models.SecretAuditActionCreate, actor, nil, nil); err != nil {
			return false, fmt.Errorf("audit insert %q: %w", v.Name, err)
		}
		return true, nil

	default:
		return false, fmt.Errorf("lookup env_var %q: %w", v.Name, lookupErr)
	}
}
