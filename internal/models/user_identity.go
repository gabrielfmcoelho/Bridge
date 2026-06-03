package models

import "time"

// UserExternalIdentity links a local user to an external auth provider account.
// Persistence lives in internal/store.UserIdentityRepo — this file is the pure
// data type only.
type UserExternalIdentity struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	ProviderName string    `json:"provider_name"`
	ExternalID   string    `json:"external_id"`
	ExternalData string    `json:"external_data"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
