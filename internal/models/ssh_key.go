package models

import "time"

// SSHKey is a managed SSH credential (key pair or password). The encrypted
// material is never serialized to JSON. Persistence lives in
// internal/store.SSHKeyRepo — this file is the pure data type only.
type SSHKey struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	CredentialType     string    `json:"credential_type"` // "key" or "password"
	Username           string    `json:"username"`
	Description        string    `json:"description"`
	PubKeyCiphertext   []byte    `json:"-"`
	PubKeyNonce        []byte    `json:"-"`
	PrivKeyCiphertext  []byte    `json:"-"`
	PrivKeyNonce       []byte    `json:"-"`
	PasswordCiphertext []byte    `json:"-"`
	PasswordNonce      []byte    `json:"-"`
	Fingerprint        string    `json:"fingerprint"`
	CreatedAt          time.Time `json:"created_at"`
}
