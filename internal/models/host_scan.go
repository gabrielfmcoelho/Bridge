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
