package models

import "time"

// HostScan is a periodic inventory snapshot of a host (raw JSON in Data).
// Persistence lives in internal/store.HostScanRepo — this file is the pure data
// type only.
type HostScan struct {
	ID        int64     `json:"id"`
	HostID    int64     `json:"host_id"`
	Data      string    `json:"data"`
	ScannedAt time.Time `json:"scanned_at"`
}

// HostCredentialSource is one config-file login (Phase 1 credential_sources)
// from a live host's latest scan — the field names and provenance only, never
// a value. HostScanRepo.LatestCredentialSources reads them so a database or
// service page can show "login found on host X (scan)".
type HostCredentialSource struct {
	HostID       int64
	HostNickname string
	HostSlug     string
	Detector     string
	Path         string
	Locator      string
	Kind         string // cred | sshkey | env_var
	Engine       string // fields.engine
	DBHost       string // fields.host
	DBName       string // fields.db
	User         string // fields.user
}
