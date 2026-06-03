package models

import "time"

// ExternalTool is an embeddable third-party tool/dashboard link, optionally
// bound to a service + DNS record. Persistence lives in
// internal/store.ExternalToolRepo — this file is the pure data type only.
type ExternalTool struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	URL            string    `json:"url"`
	Icon           string    `json:"icon"`
	EmbedEnabled   bool      `json:"embed_enabled"`
	SortOrder      int       `json:"sort_order"`
	ServiceID      *int64    `json:"service_id"`
	DNSID          *int64    `json:"dns_id"`
	Source         string    `json:"source"`
	HasCredentials bool      `json:"has_credentials"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
