package models

import "time"

// HostAlert is an alert raised against a host — manual, auto-computed, or
// ingested from an external source (Grafana). Persistence lives in
// internal/store.HostAlertRepo — this file is the pure data type only.
type HostAlert struct {
	ID             int64     `json:"id"`
	HostID         int64     `json:"host_id"`
	Type           string    `json:"type"`
	Level          string    `json:"level"`
	Message        string    `json:"message"`
	Description    string    `json:"description"`
	Source         string    `json:"source"`
	Status         string    `json:"status"`
	ExternalID     string    `json:"external_id,omitempty"`
	ExternalSource string    `json:"external_source,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
