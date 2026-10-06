package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// APIKeyRepo owns SQL for api_keys. Keys are only reachable through their
// API: every read and write checks the parent api_catalog row is live and
// visible, so an invisible API's keys answer like a missing one.
type APIKeyRepo struct{ db *sql.DB }

// NewAPIKeyRepo constructs an APIKeyRepo over db.
func NewAPIKeyRepo(db *sql.DB) *APIKeyRepo { return &APIKeyRepo{db: db} }

const apiKeyCols = `k.id, k.api_id, k.label, k.source, k.external_label, k.secret_id, k.owner, k.owner_contact_id,
	COALESCE((SELECT c.name FROM contacts c WHERE c.id = k.owner_contact_id), ''),
	k.notes, k.scopes, k.rate_limit_per_minute, k.expires_at, k.revoked_at, k.grace_until, k.last_used_at,
	k.lifetime_uses, k.synced_at, k.created_by, k.created_at, k.updated_at`

func scanAPIKey(scanner interface{ Scan(...any) error }, k *models.APIKey) error {
	var scopes string
	if err := scanner.Scan(&k.ID, &k.APIID, &k.Label, &k.Source, &k.ExternalLabel, &k.SecretID, &k.Owner, &k.OwnerContactID,
		&k.OwnerContactName, &k.Notes, &scopes, &k.RateLimitPerMinute, &k.ExpiresAt, &k.RevokedAt, &k.GraceUntil,
		&k.LastUsedAt, &k.LifetimeUses, &k.SyncedAt, &k.CreatedBy, &k.CreatedAt, &k.UpdatedAt); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(scopes), &k.Scopes)
	if k.Scopes == nil {
		k.Scopes = []string{}
	}
	k.ComputeStatus(time.Now())
	return nil
}

// apiVisible is the predicate "the key's API is live and visible".
func apiVisible(ctx context.Context) (string, []any) {
	vis, args := VisibleExpr(ctx, AssetAPICatalog, "a.id")
	return `EXISTS (SELECT 1 FROM api_catalog a WHERE a.id = k.api_id AND a.deleted_at IS NULL AND ` + vis + `)`, args
}

// List returns an API's keys, newest first. Revoked keys (and passed grace
// periods) are included only when includeRevoked.
func (r *APIKeyRepo) List(ctx context.Context, apiID int64, includeRevoked bool) ([]models.APIKey, error) {
	vis, args := apiVisible(ctx)
	q := `SELECT ` + apiKeyCols + ` FROM api_keys k WHERE k.api_id = ? AND ` + vis
	args = append([]any{apiID}, args...)
	if !includeRevoked {
		q += ` AND k.revoked_at IS NULL AND (k.grace_until IS NULL OR k.grace_until > ?)`
		args = append(args, time.Now())
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY k.created_at DESC, k.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.APIKey{}
	for rows.Next() {
		var k models.APIKey
		if err := scanAPIKey(rows, &k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Get returns one key of an API, or (nil, nil) when missing or invisible.
func (r *APIKeyRepo) Get(ctx context.Context, apiID, id int64) (*models.APIKey, error) {
	vis, args := apiVisible(ctx)
	k := &models.APIKey{}
	err := scanAPIKey(r.db.QueryRowContext(ctx, `SELECT `+apiKeyCols+` FROM api_keys k WHERE k.id = ? AND k.api_id = ? AND `+vis,
		append([]any{id, apiID}, args...)...), k)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

// Create inserts a key row and reloads it (id, status, contact name). The
// caller has already checked the API is visible.
func (r *APIKeyRepo) Create(ctx context.Context, k *models.APIKey) error {
	scopes, _ := json.Marshal(nonNilStrings(k.Scopes))
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO api_keys (api_id, label, source, external_label, secret_id, owner, owner_contact_id, notes, scopes,
			rate_limit_per_minute, expires_at, revoked_at, grace_until, last_used_at, lifetime_uses, synced_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.APIID, k.Label, k.Source, k.ExternalLabel, k.SecretID, k.Owner, k.OwnerContactID, k.Notes, string(scopes),
		k.RateLimitPerMinute, k.ExpiresAt, k.RevokedAt, k.GraceUntil, k.LastUsedAt, k.LifetimeUses, k.SyncedAt, k.CreatedBy)
	if err != nil {
		return err
	}
	stored, err := r.Get(ctx, k.APIID, id)
	if err != nil {
		return err
	}
	if stored != nil {
		*k = *stored
	}
	return nil
}

// UpdateMeta changes the Bridge-side metadata of a key (owner, contact,
// notes; for manual keys also expiry). found is false when no visible key matched.
func (r *APIKeyRepo) UpdateMeta(ctx context.Context, k *models.APIKey) (found bool, err error) {
	vis, args := apiVisible(ctx)
	res, err := r.db.ExecContext(ctx,
		`UPDATE api_keys k SET owner = ?, owner_contact_id = ?, notes = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE k.id = ? AND k.api_id = ? AND `+vis,
		append([]any{k.Owner, k.OwnerContactID, k.Notes, k.ExpiresAt, k.ID, k.APIID}, args...)...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// MarkRevoked stamps revoked_at (keeping the first one).
func (r *APIKeyRepo) MarkRevoked(ctx context.Context, apiID, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP), updated_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND api_id = ?`, id, apiID)
	return err
}

// ApplyRemote overwrites the Keycloak-owned fields of a key row (sync, scope
// and rate-limit edits): label, client id, scopes, rate limit and whether it
// is revoked (a disabled client; the first revocation time is kept).
// Bridge-owned fields (owner, contact, notes, secret) are kept.
func (r *APIKeyRepo) ApplyRemote(ctx context.Context, id int64, k *models.APIKey) error {
	scopes, _ := json.Marshal(nonNilStrings(k.Scopes))
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET label = ?, external_label = ?, scopes = ?, rate_limit_per_minute = ?,
			revoked_at = CASE WHEN ? THEN COALESCE(revoked_at, CURRENT_TIMESTAMP) END,
			synced_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		k.Label, k.ExternalLabel, string(scopes), k.RateLimitPerMinute, k.RevokedAt != nil, id)
	return err
}

// SetSecret points a key row at its vault secret.
func (r *APIKeyRepo) SetSecret(ctx context.Context, id, secretID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET secret_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, secretID, id)
	return err
}

// BySecret returns the usable (active or in grace) key whose vault secret is
// secretID, or (nil, nil) when none. No visibility filter: it serves the
// public bundle redeem, where the share token is the capability; the API
// must still be live.
func (r *APIKeyRepo) BySecret(ctx context.Context, secretID int64) (*models.APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+apiKeyCols+` FROM api_keys k
		WHERE k.secret_id = ? AND k.revoked_at IS NULL
		  AND EXISTS (SELECT 1 FROM api_catalog a WHERE a.id = k.api_id AND a.deleted_at IS NULL)
		ORDER BY k.created_at DESC, k.id DESC`, secretID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		k := &models.APIKey{}
		if err := scanAPIKey(rows, k); err != nil {
			return nil, err
		}
		if k.Status == models.APIKeyStatusActive || k.Status == models.APIKeyStatusGrace {
			return k, nil
		}
	}
	return nil, rows.Err()
}

// ByExternalLabel maps an API's Keycloak client ids to their key rows' ids.
func (r *APIKeyRepo) ByExternalLabel(ctx context.Context, apiID int64) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT external_label, id FROM api_keys WHERE api_id = ? AND external_label IS NOT NULL`, apiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var label string
		var id int64
		if err := rows.Scan(&label, &id); err != nil {
			return nil, err
		}
		out[label] = id
	}
	return out, rows.Err()
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
