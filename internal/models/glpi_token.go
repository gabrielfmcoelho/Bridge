package models

import "time"

// GlpiToken is a named GLPI account profile — each one ties sshcm to a single
// GLPI user's personal API token. One app supports N of these (e.g. per team).
// Persistence lives in internal/store.GlpiTokenRepo — this file is the pure
// data types only.
type GlpiToken struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	UserTokenCipher []byte    `json:"-"`
	UserTokenNonce  []byte    `json:"-"`
	HasToken        bool      `json:"has_token"`
	DefaultEntityID int       `json:"default_entity_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// GlpiTokenInput is the admin-supplied subset — user_token is plaintext on the
// way in and gets encrypted before persistence.
type GlpiTokenInput struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	UserToken       string `json:"user_token"`
	DefaultEntityID int    `json:"default_entity_id"`
}
