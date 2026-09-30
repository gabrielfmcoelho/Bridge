package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// APITokenRepo owns SQL for api_tokens (personal API tokens, stored hashed).
type APITokenRepo struct {
	db *sql.DB
}

// NewAPITokenRepo constructs an APITokenRepo over the given DB handle.
func NewAPITokenRepo(db *sql.DB) *APITokenRepo { return &APITokenRepo{db: db} }

const apiTokenCols = `t.id, t.user_id, u.username, u.kind, t.name, t.prefix, t.expires_at, t.last_used_at, t.revoked_at, t.created_at,
	t.scopes, t.rate_limit_per_minute,
	COALESCE((SELECT requests FROM api_token_usage g WHERE g.token_id = t.id AND g.day = CURRENT_DATE), 0)`

func scanAPIToken(scanner interface{ Scan(...any) error }, t *models.APIToken) error {
	var scopes string
	if err := scanner.Scan(&t.ID, &t.UserID, &t.Username, &t.OwnerKind, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt,
		&scopes, &t.RateLimitPerMinute, &t.TodayRequests); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(scopes), &t.Scopes)
	if t.Scopes == nil {
		t.Scopes = []string{}
	}
	return nil
}

// DB exposes the handle for handlers that need a sibling repo.
func (r *APITokenRepo) DB() *sql.DB { return r.db }

// Create stores a token by its hash and reloads t as stored (id, owner's
// username, created_at).
func (r *APITokenRepo) Create(ctx context.Context, t *models.APIToken, hash []byte) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO api_tokens (user_id, name, token_hash, prefix, expires_at, scopes, rate_limit_per_minute) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.UserID, t.Name, hash, t.Prefix, t.ExpiresAt, scopesJSON(t.Scopes), t.RateLimitPerMinute,
	)
	if err != nil {
		return err
	}
	stored, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	*t = *stored
	return nil
}

// List returns one user's tokens, or every user's when userID is 0, newest
// first. Revoked tokens are included (the UI shows them as revoked).
func (r *APITokenRepo) List(ctx context.Context, userID int64) ([]models.APIToken, error) {
	q := `SELECT ` + apiTokenCols + ` FROM api_tokens t JOIN users u ON u.id = t.user_id`
	var args []any
	if userID != 0 {
		q += ` WHERE t.user_id = ?`
		args = append(args, userID)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY t.created_at DESC, t.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.APIToken{}
	for rows.Next() {
		var t models.APIToken
		if err := scanAPIToken(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one token, or (nil, nil).
func (r *APITokenRepo) Get(ctx context.Context, id int64) (*models.APIToken, error) {
	t := &models.APIToken{}
	err := scanAPIToken(r.db.QueryRowContext(ctx, `SELECT `+apiTokenCols+` FROM api_tokens t JOIN users u ON u.id = t.user_id WHERE t.id = ?`, id), t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Revoke marks a token revoked; revoking twice keeps the first timestamp.
func (r *APITokenRepo) Revoke(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND revoked_at IS NULL`, id)
	return err
}

// AuthenticatedToken is what a presented token resolves to.
type AuthenticatedToken struct {
	ID                 int64
	UserID             int64
	Scopes             []string
	RateLimitPerMinute *int
}

// Authenticate resolves a token hash to its token when it is neither revoked
// nor expired, and stamps last_used_at (at most once a minute, so a busy
// script doesn't write on every request). ok is false for any unusable token.
func (r *APITokenRepo) Authenticate(ctx context.Context, hash []byte) (tok AuthenticatedToken, ok bool, err error) {
	var scopes string
	err = r.db.QueryRowContext(ctx,
		`SELECT id, user_id, scopes, rate_limit_per_minute FROM api_tokens
		  WHERE token_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`,
		hash, time.Now(),
	).Scan(&tok.ID, &tok.UserID, &scopes, &tok.RateLimitPerMinute)
	if err == sql.ErrNoRows {
		return tok, false, nil
	}
	if err != nil {
		return tok, false, err
	}
	_ = json.Unmarshal([]byte(scopes), &tok.Scopes)
	_, err = r.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND (last_used_at IS NULL OR last_used_at < ?)`,
		tok.ID, time.Now().Add(-time.Minute),
	)
	return tok, true, err
}

// AddUsage adds request counts per token to today's usage rows.
func (r *APITokenRepo) AddUsage(ctx context.Context, counts map[int64]int64) error {
	for id, n := range counts {
		if _, err := r.db.ExecContext(ctx,
			`INSERT INTO api_token_usage (token_id, day, requests) VALUES (?, CURRENT_DATE, ?)
			 ON CONFLICT (token_id, day) DO UPDATE SET requests = api_token_usage.requests + EXCLUDED.requests`,
			id, n); err != nil {
			return err
		}
	}
	return nil
}

// Usage returns a token's requests per day ("YYYY-MM-DD" → count) for the
// last days, and its lifetime total.
func (r *APITokenRepo) Usage(ctx context.Context, tokenID int64, days int) (map[string]int64, int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT to_char(day, 'YYYY-MM-DD'), requests FROM api_token_usage
		  WHERE token_id = ? AND day > CURRENT_DATE - ?::int ORDER BY day`, tokenID, days)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var day string
		var n int64
		if err := rows.Scan(&day, &n); err != nil {
			return nil, 0, err
		}
		out[day] = n
	}
	var total int64
	err = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(requests), 0) FROM api_token_usage WHERE token_id = ?`, tokenID).Scan(&total)
	return out, total, err
}

func scopesJSON(s []string) string {
	if s == nil {
		s = []string{}
	}
	b, _ := json.Marshal(s)
	return string(b)
}
