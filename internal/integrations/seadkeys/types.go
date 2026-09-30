package seadkeys

import (
	"strings"
	"time"
)

// KeySummary mirrors the service's KeySummary (never carries the plaintext).
// Timestamps stay ISO strings on the wire; Time parses them leniently, so one
// odd value (e.g. a naive timestamp from an old record) can't fail a sync.
type KeySummary struct {
	Label              string   `json:"label"`
	Owner              string   `json:"owner"`
	CreatedAt          *string  `json:"created_at"`
	ExpiresAt          *string  `json:"expires_at"`
	RevokedAt          *string  `json:"revoked_at"`
	GraceUntil         *string  `json:"grace_until"`
	Scopes             []string `json:"scopes"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute"`
	Notes              string   `json:"notes"`
	LifetimeUses       int64    `json:"lifetime_uses"`
	LastUsedAt         *string  `json:"last_used_at"`
	Status             string   `json:"status"` // ACTIVE | GRACE | EXPIRED | REVOKED
}

// Time parses an ISO-8601 timestamp from the service (Python isoformat, with
// or without offset; naive ones are UTC). nil for nil, empty or unparsable.
func Time(s *string) *time.Time {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999"} {
		if t, err := time.Parse(layout, strings.TrimSpace(*s)); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

// Rotated reports whether this entry is the leftover of a rotation
// ("<label>__rotated__<hash8>"), which the service keeps forever.
func (k KeySummary) Rotated() bool { return strings.Contains(k.Label, "__rotated__") }

// CreateRequest is the body of POST /admin/keys. ExpiresDays nil or 0 means
// the key never expires; RateLimitPerMinute 0 means unlimited.
type CreateRequest struct {
	Label              string   `json:"label"`
	Owner              string   `json:"owner"`
	ExpiresDays        *int     `json:"expires_days,omitempty"`
	Scopes             []string `json:"scopes,omitempty"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute,omitempty"`
	Notes              string   `json:"notes"`
}

// CreateResponse is a KeySummary plus the one-time plaintext (create, rotate).
type CreateResponse struct {
	KeySummary
	Plaintext string `json:"plaintext"`
}

// Usage is GET /admin/keys/{label}/usage: daily is "YYYYMMDD" → requests.
type Usage struct {
	Label      string           `json:"label"`
	Lifetime   int64            `json:"lifetime"`
	LastUsedAt *string          `json:"last_used_at"`
	Daily      map[string]int64 `json:"daily"`
}
