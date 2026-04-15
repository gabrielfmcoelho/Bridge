package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

const oauthStateDuration = 10 * time.Minute

// OAuthState tracks CSRF state for OAuth redirect flows.
type OAuthState struct {
	State     string    `json:"state"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateOAuthState generates a random state token and stores it in the database.
func CreateOAuthState(db *sql.DB, provider string) (*OAuthState, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}

	s := &OAuthState{
		State:     hex.EncodeToString(b),
		Provider:  provider,
		ExpiresAt: time.Now().Add(oauthStateDuration),
	}

	_, err := db.Exec(
		`INSERT INTO oauth_states (state, provider, expires_at) VALUES (?, ?, ?)`,
		s.State, s.Provider, s.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// ValidateOAuthState checks a state token exists, is not expired, and consumes it (single use).
func ValidateOAuthState(db *sql.DB, state string) (*OAuthState, error) {
	s := &OAuthState{}
	err := db.QueryRow(
		`SELECT state, provider, created_at, expires_at FROM oauth_states WHERE state = ?`,
		state,
	).Scan(&s.State, &s.Provider, &s.CreatedAt, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Consume the state (single use).
	db.Exec(`DELETE FROM oauth_states WHERE state = ?`, state)

	if time.Now().After(s.ExpiresAt) {
		return nil, nil
	}
	return s, nil
}

// CleanExpiredOAuthStates removes expired state tokens.
func CleanExpiredOAuthStates(db *sql.DB) {
	db.Exec(`DELETE FROM oauth_states WHERE expires_at < ?`, time.Now())
}
