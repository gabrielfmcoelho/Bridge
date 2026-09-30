package store

import (
	"context"
	"database/sql"
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

const apiTokenCols = `t.id, t.user_id, u.username, t.name, t.prefix, t.expires_at, t.last_used_at, t.revoked_at, t.created_at`

func scanAPIToken(scanner interface{ Scan(...any) error }, t *models.APIToken) error {
	return scanner.Scan(&t.ID, &t.UserID, &t.Username, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt)
}

// Create stores a token by its hash and reloads t as stored (id, owner's
// username, created_at).
func (r *APITokenRepo) Create(ctx context.Context, t *models.APIToken, hash []byte) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO api_tokens (user_id, name, token_hash, prefix, expires_at) VALUES (?, ?, ?, ?, ?)`,
		t.UserID, t.Name, hash, t.Prefix, t.ExpiresAt,
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

// Authenticate resolves a token hash to (token id, user id) when the token is
// neither revoked nor expired, and stamps last_used_at (at most once a minute,
// so a busy script doesn't write on every request). ok is false for any
// unusable token.
func (r *APITokenRepo) Authenticate(ctx context.Context, hash []byte) (tokenID, userID int64, ok bool, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT id, user_id FROM api_tokens
		  WHERE token_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`,
		hash, time.Now(),
	).Scan(&tokenID, &userID)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND (last_used_at IS NULL OR last_used_at < ?)`,
		tokenID, time.Now().Add(-time.Minute),
	)
	return tokenID, userID, true, err
}
