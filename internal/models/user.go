package models

import "time"

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
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
