package models

import "time"

// APIToken is a personal API token. It authenticates as its owner (same role,
// permissions and entidades); the plaintext is shown once at creation and
// only its hash is stored.
type APIToken struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Username   string     `json:"username"`
	OwnerKind  string     `json:"owner_kind"` // "person" or "service"
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"` // first characters of the token, for display
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
	// Scopes narrow what the token may call (internal/auth/scopes.go).
	Scopes []string `json:"scopes"`
	// RateLimitPerMinute is nil for the default.
	RateLimitPerMinute *int `json:"rate_limit_per_minute,omitempty"`
	// TodayRequests is filled on list responses.
	TodayRequests int64 `json:"today_requests"`
}
