package models

import "time"

// OAuthState tracks CSRF state for OAuth redirect flows. Persistence lives in
// internal/store.OAuthStateRepo — this file is the pure data type only.
type OAuthState struct {
	State     string    `json:"state"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
