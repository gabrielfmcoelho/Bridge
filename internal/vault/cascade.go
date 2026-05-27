package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// CascadeSoftDelete soft-deletes every secret matching (scope, parentID)
// inside the given transaction and writes one `delete` audit row per
// affected secret. Returns the number of secrets touched.
//
// Both visibilities (shared + personal) are cascaded — the parent's deletion
// is the source of truth for the entire surface attached to it, regardless
// of who owns each child. avulso secrets are never cascaded: by definition
// they have no parent (CHECK constraint on the secrets table) so they
// cannot match. Already-soft-deleted rows are skipped so a double-cascade
// from a poorly ordered handler doesn't write duplicate audit rows.
//
// Per spec §4.1 D3 the cascade is part of the parent's delete transaction:
// callers pass in their open *sql.Tx so the cascade composes atomically.
//
// Currently only `service`, `host`, and `tool` scopes have a parent capable
// of being cascaded — `avulso` short-circuits to a no-op.
func CascadeSoftDelete(ctx context.Context, tx *sql.Tx, scope models.SecretScope, parentID int64, actor ActorContext) (int64, error) {
	if scope == models.SecretScopeAvulso {
		return 0, nil
	}
	if !scope.Valid() {
		return 0, fmt.Errorf("invalid scope %q", scope)
	}

	// Collect the IDs we're about to flip — needed to write per-secret
	// audit rows after the UPDATE. Could do this in a single CTE on
	// Postgres, but a two-step (select then update) is portable across
	// the dialects this codebase supports.
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM secrets
		  WHERE scope = ? AND parent_id = ? AND deleted_at IS NULL`,
		string(scope), parentID,
	)
	if err != nil {
		return 0, fmt.Errorf("collect children: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE secrets SET deleted_at = CURRENT_TIMESTAMP
		  WHERE scope = ? AND parent_id = ? AND deleted_at IS NULL`,
		string(scope), parentID,
	); err != nil {
		return 0, fmt.Errorf("soft-delete: %w", err)
	}

	meta := cascadeMeta(scope, parentID)
	for _, id := range ids {
		if err := writeAuditTx(tx, id, models.SecretAuditActionDelete, actor, nil, meta); err != nil {
			return 0, fmt.Errorf("audit %d: %w", id, err)
		}
	}
	return int64(len(ids)), nil
}

// CascadeRestore is the symmetric inverse: clears deleted_at on every
// secret matching (scope, parentID) that is currently soft-deleted and
// writes a `restore` audit row per affected row. Returns the number of
// restored secrets.
//
// Like CascadeSoftDelete this is meant to be called inside the parent's
// restore transaction. Currently no parent table has a soft-delete
// mechanism, so this function is intentionally dormant — added now for
// completeness so the cascade surface is symmetric and ready to wire when
// parent soft-delete arrives in a later spec.
func CascadeRestore(ctx context.Context, tx *sql.Tx, scope models.SecretScope, parentID int64, actor ActorContext) (int64, error) {
	if scope == models.SecretScopeAvulso {
		return 0, nil
	}
	if !scope.Valid() {
		return 0, fmt.Errorf("invalid scope %q", scope)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM secrets
		  WHERE scope = ? AND parent_id = ? AND deleted_at IS NOT NULL`,
		string(scope), parentID,
	)
	if err != nil {
		return 0, fmt.Errorf("collect children: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE secrets SET deleted_at = NULL
		  WHERE scope = ? AND parent_id = ? AND deleted_at IS NOT NULL`,
		string(scope), parentID,
	); err != nil {
		return 0, fmt.Errorf("restore: %w", err)
	}

	meta := cascadeMeta(scope, parentID)
	for _, id := range ids {
		if err := writeAuditTx(tx, id, models.SecretAuditActionRestore, actor, nil, meta); err != nil {
			return 0, fmt.Errorf("audit %d: %w", id, err)
		}
	}
	return int64(len(ids)), nil
}

// cascadeMeta returns the audit metadata blob attached to every cascade-
// triggered audit row, so operators can distinguish cascades from
// direct deletes/restores when reading the audit log.
func cascadeMeta(scope models.SecretScope, parentID int64) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"cascade_from": fmt.Sprintf("%s:%d", scope, parentID),
	})
	return b
}
