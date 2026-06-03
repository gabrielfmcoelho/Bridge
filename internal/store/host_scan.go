package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostScanRepo owns SQL for host_scans (periodic host inventory snapshots).
// Referenced by several handlers (sshconfig writes; host + dashboard read);
// built inline until the Phase 2 container hoists it.
type HostScanRepo struct {
	db *sql.DB
}

// NewHostScanRepo constructs a HostScanRepo over the given DB handle.
func NewHostScanRepo(db *sql.DB) *HostScanRepo { return &HostScanRepo{db: db} }

// Create records a new scan snapshot (raw JSON data) for a host.
func (r *HostScanRepo) Create(ctx context.Context, hostID int64, data string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO host_scans (host_id, data) VALUES (?, ?)`, hostID, data)
	return err
}

// GetLatest returns the most recent scan for a host, or (nil, nil) if none.
func (r *HostScanRepo) GetLatest(ctx context.Context, hostID int64) (*models.HostScan, error) {
	s := &models.HostScan{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, host_id, data, scanned_at FROM host_scans WHERE host_id = ? ORDER BY scanned_at DESC LIMIT 1`,
		hostID,
	).Scan(&s.ID, &s.HostID, &s.Data, &s.ScannedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// Statuses returns the latest scan timestamp per host (host_id -> last scan).
func (r *HostScanRepo) Statuses(ctx context.Context) (map[int64]time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id, MAX(scanned_at) FROM host_scans GROUP BY host_id`)
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

// CountHostsWithScans returns how many distinct hosts have at least one scan.
func (r *HostScanRepo) CountHostsWithScans(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT host_id) FROM host_scans`).Scan(&n)
	return n, err
}

// RecentWithHost returns the newest scans joined to their host identity, as a
// dashboard read-model (untyped rows, newest first).
func (r *HostScanRepo) RecentWithHost(ctx context.Context, limit int) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hs.id, hs.host_id, h.nickname, h.oficial_slug, hs.scanned_at
		FROM host_scans hs
		JOIN hosts h ON h.id = hs.host_id
		ORDER BY hs.scanned_at DESC
		LIMIT ?`, limit)
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
			"id": id, "host_id": hostID, "nickname": nickname, "slug": slug, "scanned_at": scannedAt,
		})
	}
	return results, rows.Err()
}

// LatestDataBulk returns the latest scan JSON per host for all scanned hosts.
func (r *HostScanRepo) LatestDataBulk(ctx context.Context) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hs.host_id, hs.data
		FROM host_scans hs
		INNER JOIN (SELECT host_id, MAX(scanned_at) AS max_at FROM host_scans GROUP BY host_id) latest
		ON hs.host_id = latest.host_id AND hs.scanned_at = latest.max_at`)
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
