package models

import (
	"database/sql"
	"time"
)

type HostScan struct {
	ID        int64     `json:"id"`
	HostID    int64     `json:"host_id"`
	Data      string    `json:"data"`
	ScannedAt time.Time `json:"scanned_at"`
}

func CreateHostScan(db *sql.DB, hostID int64, data string) error {
	_, err := db.Exec(
		`INSERT INTO host_scans (host_id, data) VALUES (?, ?)`,
		hostID, data,
	)
	return err
}

func GetLatestHostScan(db *sql.DB, hostID int64) (*HostScan, error) {
	s := &HostScan{}
	err := db.QueryRow(
		`SELECT id, host_id, data, scanned_at FROM host_scans WHERE host_id = ? ORDER BY scanned_at DESC LIMIT 1`,
		hostID,
	).Scan(&s.ID, &s.HostID, &s.Data, &s.ScannedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func GetHostScanStatuses(db *sql.DB) (map[int64]time.Time, error) {
	rows, err := db.Query(`SELECT host_id, MAX(scanned_at) FROM host_scans GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]time.Time)
	for rows.Next() {
		var id int64
		var ts string
		if err := rows.Scan(&id, &ts); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02 15:04:05", ts)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, ts)
		}
		m[id] = t
	}
	return m, rows.Err()
}

func HostsWithScanCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(DISTINCT host_id) FROM host_scans`).Scan(&n)
	return n, err
}

func RecentScansWithHost(db *sql.DB, limit int) ([]map[string]any, error) {
	rows, err := db.Query(`
		SELECT hs.id, hs.host_id, h.nickname, h.oficial_slug, hs.scanned_at
		FROM host_scans hs
		JOIN hosts h ON h.id = hs.host_id
		ORDER BY hs.scanned_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, hostID int64
		var nickname, slug string
		var scannedAt time.Time
		if err := rows.Scan(&id, &hostID, &nickname, &slug, &scannedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"id":         id,
			"host_id":    hostID,
			"nickname":   nickname,
			"slug":       slug,
			"scanned_at": scannedAt,
		})
	}
	return results, rows.Err()
}

// GetLatestScanDataBulk returns the latest scan JSON data per host for all hosts that have scans.
func GetLatestScanDataBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT hs.host_id, hs.data
		FROM host_scans hs
		INNER JOIN (SELECT host_id, MAX(scanned_at) AS max_at FROM host_scans GROUP BY host_id) latest
		ON hs.host_id = latest.host_id AND hs.scanned_at = latest.max_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]string)
	for rows.Next() {
		var id int64
		var data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		m[id] = data
	}
	return m, rows.Err()
}

func ListHostScans(db *sql.DB, hostID int64) ([]HostScan, error) {
	rows, err := db.Query(
		`SELECT id, host_id, data, scanned_at FROM host_scans WHERE host_id = ? ORDER BY scanned_at DESC LIMIT 20`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []HostScan
	for rows.Next() {
		var s HostScan
		if err := rows.Scan(&s.ID, &s.HostID, &s.Data, &s.ScannedAt); err != nil {
			return nil, err
		}
		scans = append(scans, s)
	}
	return scans, rows.Err()
}
