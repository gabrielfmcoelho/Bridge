package models

import "time"

// HostChamado represents a chamado (ticket/request) associated with a host.
// When linked to GLPI (ExternalSource="glpi"), the cached_* fields hold the
// last-known ticket title and status so lists render without re-querying GLPI
// on every page view. Persistence lives in internal/store.HostChamadoRepo —
// this file is the pure data types only.
type HostChamado struct {
	ID              int64      `json:"id"`
	HostID          int64      `json:"host_id"`
	ChamadoID       string     `json:"chamado_id"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	UserID          int64      `json:"user_id"`
	UserDisplayName string     `json:"user_display_name"`
	Date            string     `json:"date"`
	ExternalSource  string     `json:"external_source,omitempty"`
	ExternalURL     string     `json:"external_url,omitempty"`
	CachedTitle     string     `json:"cached_title,omitempty"`
	CachedStatus    string     `json:"cached_status,omitempty"`
	CachedAt        *time.Time `json:"cached_at,omitempty"`
}

// HostChamadoInput is the input for creating/syncing host chamados.
type HostChamadoInput struct {
	ChamadoID string `json:"chamado_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	UserID    int64  `json:"user_id"`
	Date      string `json:"date"`
}
