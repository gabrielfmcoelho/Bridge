package models

import "time"

// API key sources: registered by hand, or a Keycloak client issued/synced
// through the keycloak_apis integration.
const (
	APIKeySourceManual   = "manual"
	APIKeySourceKeycloak = "keycloak"
)

// API key states, computed on read from the timestamps (never stored).
const (
	APIKeyStatusActive  = "active"
	APIKeyStatusGrace   = "grace" // rotated away, still accepted until grace_until
	APIKeyStatusExpired = "expired"
	APIKeyStatusRevoked = "revoked"
)

// APIKey is one access key of a catalogued API, as Bridge tracks it. The
// plaintext never sits here: when Bridge knows it, it lives in the vault as
// an api_key secret (SecretID). Keycloak clients synced that Bridge didn't
// issue have no SecretID.
type APIKey struct {
	ID                 int64      `json:"id"`
	APIID              int64      `json:"api_id"`
	Label              string     `json:"label"`
	Source             string     `json:"source"`
	ExternalLabel      *string    `json:"external_label,omitempty"` // the Keycloak clientId
	SecretID           *int64     `json:"secret_id,omitempty"`
	Owner              string     `json:"owner"`
	OwnerContactID     *int64     `json:"owner_contact_id,omitempty"`
	OwnerContactName   string     `json:"owner_contact_name,omitempty"`
	Notes              string     `json:"notes"`
	Scopes             []string   `json:"scopes"`
	RateLimitPerMinute *int       `json:"rate_limit_per_minute,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at"`
	RevokedAt          *time.Time `json:"revoked_at"`
	GraceUntil         *time.Time `json:"grace_until"`
	LastUsedAt         *time.Time `json:"last_used_at"`
	LifetimeUses       int64      `json:"lifetime_uses"`
	SyncedAt           *time.Time `json:"synced_at"`
	CreatedBy          *int64     `json:"created_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Status             string     `json:"status"`
}

// ComputeStatus derives Status from the timestamps at time now: revoked wins,
// then a passed grace period or expiry, then an open grace period.
func (k *APIKey) ComputeStatus(now time.Time) {
	switch {
	case k.RevokedAt != nil:
		k.Status = APIKeyStatusRevoked
	case k.GraceUntil != nil && !now.Before(*k.GraceUntil):
		k.Status = APIKeyStatusRevoked
	case k.ExpiresAt != nil && !now.Before(*k.ExpiresAt):
		k.Status = APIKeyStatusExpired
	case k.GraceUntil != nil:
		k.Status = APIKeyStatusGrace
	default:
		k.Status = APIKeyStatusActive
	}
}
