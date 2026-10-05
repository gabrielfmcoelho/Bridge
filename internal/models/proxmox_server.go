package models

import "time"

// ProxmoxServer is one Proxmox VE cluster the host sync reads. TokenID is the
// full "user@realm!tokenname"; the secret is stored encrypted and never
// returned, only has_token.
type ProxmoxServer struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	BaseURL     string    `json:"base_url"`
	TokenID     string    `json:"token_id"`
	TokenCipher []byte    `json:"-"`
	TokenNonce  []byte    `json:"-"`
	HasToken    bool      `json:"has_token"`
	SkipVerify  bool      `json:"skip_verify"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProxmoxServerInput is the create / partial-update body: an omitted field
// keeps its value; an empty or masked token_secret keeps the stored one.
type ProxmoxServerInput struct {
	Name        *string `json:"name"`
	BaseURL     *string `json:"base_url"`
	TokenID     *string `json:"token_id"`
	TokenSecret *string `json:"token_secret"`
	SkipVerify  *bool   `json:"skip_verify"`
	Enabled     *bool   `json:"enabled"`
}
