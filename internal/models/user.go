package models

import "time"

// User kinds.
const (
	UserKindPerson  = "person"
	UserKindService = "service"
)

// User is a local or externally-provisioned account. Persistence lives in
// internal/store.UserRepo — this file is the pure data type only.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	AuthProvider string    `json:"auth_provider"`
	Email        string    `json:"email"`
	// Kind is "person" or "service": a service account owns API tokens for an
	// integration and can never sign in.
	Kind      string    `json:"kind"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
