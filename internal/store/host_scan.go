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
// container names, idle verdict, OS: the pretty name, arch and pending reboot
// from the scan, distro id/version/EOL from the host's own dependency_inventory
// os row). The full scan JSON (disks, web servers,
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
				'counterfacts', d->'profile'->'counterfacts') END,
			'os_info', NULLIF(jsonb_strip_nulls(jsonb_build_object(
				'pretty', d->'os', 'arch', d->'arch', 'reboot_required', d->'reboot_required',
				'id', o.name, 'version', o.version, 'eol', o.eol, 'eol_date', NULLIF(o.eol_date, ''))), '{}'::jsonb)
		))::text
		FROM latest
		LEFT JOIN LATERAL (SELECT di.name, di.version, di.eol, di.eol_date FROM dependency_inventory di
			WHERE di.host_id = latest.host_id AND di.kind = 'os' AND di.container = '' LIMIT 1) o ON true`)
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

// LatestGitRemotes returns the git remotes of every live host's latest scan
// (git_remotes in the JSON), host-level first, then by container and path.
// Same latest-row rules as LatestListSummaryBulk; a latest scan that isn't
// valid JSON or has no git_remotes array contributes nothing.
func (r *HostScanRepo) LatestGitRemotes(ctx context.Context) ([]models.HostGitRemote, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH latest AS MATERIALIZED (
			SELECT hs.host_id, hs.data::jsonb AS d
			FROM host_scans hs
			JOIN (SELECT host_id, MAX(scanned_at) AS max_at FROM host_scans GROUP BY host_id) l
			  ON hs.host_id = l.host_id AND hs.scanned_at = l.max_at
			WHERE pg_input_is_valid(hs.data, 'jsonb')
		)
		SELECT h.id, h.nickname, h.oficial_slug, h.hostname,
			COALESCE(g->>'container', ''), COALESCE(g->>'container_image', ''),
			COALESCE(g->>'path', ''), COALESCE(g->>'name', ''), COALESCE(g->>'url', ''),
			COALESCE(g->>'commit', ''), COALESCE(g->>'branch', ''), COALESCE(g->>'pulled_at', ''), COALESCE(g->>'pull_kind', '')
		FROM latest
		JOIN hosts h ON h.id = latest.host_id AND h.deleted_at IS NULL
		CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(d->'git_remotes') = 'array'
			THEN d->'git_remotes' ELSE '[]'::jsonb END) g
		ORDER BY h.nickname, h.id, 5, 7, 8`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.HostGitRemote
	for rows.Next() {
		var g models.HostGitRemote
		var pulledAt string // text: a value that isn't a number reads as unknown, not as a failed query
		if err := rows.Scan(&g.HostID, &g.HostName, &g.HostSlug, &g.HostAddress, &g.Container, &g.ContainerImage, &g.Path, &g.Name, &g.URL,
			&g.Commit, &g.Branch, &pulledAt, &g.PullKind); err != nil {
			return nil, err
		}
		g.PulledAt, _ = strconv.ParseInt(pulledAt, 10, 64)
		out = append(out, g)
	}
	return out, rows.Err()
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
