package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
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

// Create records a new scan snapshot for a host. The raw JSON `data` is stored
// verbatim, and the sortable resource metrics (cpu/ram/disk %, container count)
// are derived from it into typed columns (P2) so the host list can ORDER BY
// them. Unparseable/empty percentages are stored NULL.
func (r *HostScanRepo) Create(ctx context.Context, hostID int64, data string) error {
	cpu, ram, disk, containers := parseScanMetrics(data)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO host_scans (host_id, data, cpu_pct, ram_pct, disk_pct, containers_count)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		hostID, data, cpu, ram, disk, containers)
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
	vis, vargs := VisibleExpr(ctx, AssetHost, "host_id")
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT host_id) FROM host_scans WHERE `+vis, vargs...).Scan(&n)
	return n, err
}

// RecentWithHost returns the newest scans joined to their host identity, as a
// dashboard read-model (untyped rows, newest first).
func (r *HostScanRepo) RecentWithHost(ctx context.Context, limit int) ([]map[string]any, error) {
	vis, vargs := VisibleExpr(ctx, AssetHost, "h.id")
	rows, err := r.db.QueryContext(ctx, `
		SELECT hs.id, hs.host_id, h.nickname, h.oficial_slug, hs.scanned_at
		FROM host_scans hs
		JOIN hosts h ON h.id = hs.host_id
		WHERE `+vis+`
		ORDER BY hs.scanned_at DESC
		LIMIT ?`, append(vargs, limit)...)
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

// LatestListSummaryBulk returns, per scanned host, a JSON object with only the
// fields of its latest scan that the host list reads (resources, service and
// container names, idle verdict). The full scan JSON (disks, web servers,
// dependencies…) is ~30 KB a host and growing; this is ~1 KB. Each latest row
// is parsed once (MATERIALIZED), and a row that isn't valid JSON is skipped.
// No "?" jsonb operator here: the rebind driver turns every ? into a parameter.
func (r *HostScanRepo) LatestListSummaryBulk(ctx context.Context) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH latest AS MATERIALIZED (
			SELECT hs.host_id, hs.data::jsonb AS d
			FROM host_scans hs
			JOIN (SELECT host_id, MAX(scanned_at) AS max_at FROM host_scans GROUP BY host_id) l
			  ON hs.host_id = l.host_id AND hs.scanned_at = l.max_at
			WHERE pg_input_is_valid(hs.data, 'jsonb')
		)
		SELECT host_id, jsonb_strip_nulls(jsonb_build_object(
			'cpu', d->'cpu', 'cpu_usage', d->'cpu_usage',
			'ram', d->'ram', 'ram_percent', d->'ram_percent',
			'storage', d->'storage', 'disk_percent', d->'disk_percent',
			'services', d->'services', 'containers', d->'containers',
			'profile', CASE WHEN d->'profile' IS NOT NULL THEN jsonb_build_object(
				'idle', d->'profile'->'idle',
				'reasons', d->'profile'->'reasons',
				'counterfacts', d->'profile'->'counterfacts') END
		))::text
		FROM latest`)
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

// parseScanMetrics extracts the sortable resource numerics from a scan's JSON
// blob. cpu/ram/disk are display strings like "45%" → *float64 (nil when
// absent/unparseable, so they store as NULL — distinct from a real 0%).
// containers_count is the length of the containers array (0 when absent).
func parseScanMetrics(data string) (cpu, ram, disk *float64, containers int) {
	var s struct {
		CPUUsage    string   `json:"cpu_usage"`
		RAMPercent  string   `json:"ram_percent"`
		DiskPercent string   `json:"disk_percent"`
		Containers  []string `json:"containers"`
	}
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, nil, nil, 0
	}
	return pctPtr(s.CPUUsage), pctPtr(s.RAMPercent), pctPtr(s.DiskPercent), len(s.Containers)
}

// pctPtr parses a "45%"-style string to a *float64, returning nil when empty or
// non-numeric (→ SQL NULL).
func pctPtr(s string) *float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}
