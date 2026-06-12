package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

const oauthStateDuration = 10 * time.Minute

// OAuthStateRepo owns SQL for oauth_states (CSRF state for OAuth redirect flows).
type OAuthStateRepo struct {
	db *sql.DB
}

// NewOAuthStateRepo constructs an OAuthStateRepo over the given DB handle.
func NewOAuthStateRepo(db *sql.DB) *OAuthStateRepo { return &OAuthStateRepo{db: db} }

// Create generates a random state token, stores it, and returns it.
func (r *OAuthStateRepo) Create(ctx context.Context, provider string) (*models.OAuthState, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	s := &models.OAuthState{
		State:     hex.EncodeToString(b),
		Provider:  provider,
		ExpiresAt: time.Now().Add(oauthStateDuration),
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO oauth_states (state, provider, expires_at) VALUES (?, ?, ?)`,
		s.State, s.Provider, s.ExpiresAt); err != nil {
		return nil, err
	}
	return s, nil
}

// Validate checks a state token exists, consumes it (single use), and returns
// it only if it had not expired. Returns (nil, nil) for unknown or expired
// tokens.
func (r *OAuthStateRepo) Validate(ctx context.Context, state string) (*models.OAuthState, error) {
	s := &models.OAuthState{}
	err := r.db.QueryRowContext(ctx,
		`SELECT state, provider, created_at, expires_at FROM oauth_states WHERE state = ?`, state,
	).Scan(&s.State, &s.Provider, &s.CreatedAt, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Consume the state (single use), then enforce expiry.
	r.db.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = ?`, state)
	if time.Now().After(s.ExpiresAt) {
		return nil, nil
	}
	return s, nil
}
