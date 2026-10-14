package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// SentinelRepo owns sentinel_host_snapshots / sentinel_service_snapshots and
// the host lookups the Sentinel sync needs.
type SentinelRepo struct {
	db *sql.DB
}

// NewSentinelRepo constructs a SentinelRepo over the given DB handle.
func NewSentinelRepo(db *sql.DB) *SentinelRepo { return &SentinelRepo{db: db} }

const sentinelHostCols = `host_id, state, detail, metrics_enabled, traffic_enabled, push_ok, last_push_at,
	cpu_avg_24h, cpu_p95_24h, mem_p95_24h, disk_pct, load1, synced_at`

func scanSentinelHost(s interface{ Scan(...any) error }, h *models.SentinelHostSnapshot) error {
	return s.Scan(&h.HostID, &h.State, &h.Detail, &h.MetricsEnabled, &h.TrafficEnabled, &h.PushOK, &h.LastPushAt,
		&h.CPUAvg24h, &h.CPUP95_24h, &h.MemP95_24h, &h.DiskPct, &h.Load1, &h.SyncedAt)
}

// sentinelServiceSelect reads snapshots with their service's nickname.
const sentinelServiceSelect = `SELECT ss.service_id, ss.host_id, ss.req_24h, ss.s4xx_24h, ss.s5xx_24h, ss.p50_ms, ss.p95_ms, ss.p99_ms,
	ss.visitors_24h, ss.container_state, ss.container_health, ss.restart_count, ss.prev_restart_count, ss.cpu_pct, ss.mem_pct,
	ss.traefik_bypass, ss.synced_at, sv.nickname
	FROM sentinel_service_snapshots ss JOIN services sv ON sv.id = ss.service_id`

func scanSentinelService(s interface{ Scan(...any) error }, v *models.SentinelServiceSnapshot) error {
	return s.Scan(&v.ServiceID, &v.HostID, &v.Req24h, &v.S4xx24h, &v.S5xx24h, &v.P50Ms, &v.P95Ms, &v.P99Ms, &v.Visitors24h,
		&v.ContainerState, &v.ContainerHealth, &v.RestartCount, &v.PrevRestartCount, &v.CPUPct, &v.MemPct,
		&v.TraefikBypass, &v.SyncedAt, &v.ServiceName)
}

// UpsertHost replaces a host's snapshot.
func (r *SentinelRepo) UpsertHost(ctx context.Context, h *models.SentinelHostSnapshot) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sentinel_host_snapshots (host_id, state, detail, metrics_enabled, traffic_enabled, push_ok, last_push_at,
			cpu_avg_24h, cpu_p95_24h, mem_p95_24h, disk_pct, load1, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT (host_id) DO UPDATE SET state = excluded.state, detail = excluded.detail,
			metrics_enabled = excluded.metrics_enabled, traffic_enabled = excluded.traffic_enabled,
			push_ok = excluded.push_ok, last_push_at = excluded.last_push_at,
			cpu_avg_24h = excluded.cpu_avg_24h, cpu_p95_24h = excluded.cpu_p95_24h, mem_p95_24h = excluded.mem_p95_24h,
			disk_pct = excluded.disk_pct, load1 = excluded.load1, synced_at = CURRENT_TIMESTAMP`,
		h.HostID, h.State, h.Detail, h.MetricsEnabled, h.TrafficEnabled, h.PushOK, h.LastPushAt,
		h.CPUAvg24h, h.CPUP95_24h, h.MemP95_24h, h.DiskPct, h.Load1)
	return err
}

// UpsertService replaces a service's snapshot; the restart count it replaces
// becomes prev_restart_count, so the alert can tell a restart happened
// between two syncs.
func (r *SentinelRepo) UpsertService(ctx context.Context, v *models.SentinelServiceSnapshot) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sentinel_service_snapshots (service_id, host_id, req_24h, s4xx_24h, s5xx_24h, p50_ms, p95_ms, p99_ms,
			visitors_24h, container_state, container_health, restart_count, cpu_pct, mem_pct, traefik_bypass, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT (service_id) DO UPDATE SET host_id = excluded.host_id,
			req_24h = excluded.req_24h, s4xx_24h = excluded.s4xx_24h, s5xx_24h = excluded.s5xx_24h,
			p50_ms = excluded.p50_ms, p95_ms = excluded.p95_ms, p99_ms = excluded.p99_ms, visitors_24h = excluded.visitors_24h,
			container_state = excluded.container_state, container_health = excluded.container_health,
			prev_restart_count = sentinel_service_snapshots.restart_count, restart_count = excluded.restart_count,
			cpu_pct = excluded.cpu_pct, mem_pct = excluded.mem_pct, traefik_bypass = excluded.traefik_bypass,
			synced_at = CURRENT_TIMESTAMP`,
		v.ServiceID, v.HostID, v.Req24h, v.S4xx24h, v.S5xx24h, v.P50Ms, v.P95Ms, v.P99Ms,
		v.Visitors24h, v.ContainerState, v.ContainerHealth, v.RestartCount, v.CPUPct, v.MemPct, v.TraefikBypass)
	return err
}

// PruneServices drops a host's service snapshots not in keep (services no
// longer matched), so their alerts go away with them.
func (r *SentinelRepo) PruneServices(ctx context.Context, hostID int64, keep []int64) error {
	if keep == nil {
		keep = []int64{}
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM sentinel_service_snapshots WHERE host_id = ? AND NOT (service_id = ANY(?::bigint[]))`, hostID, keep)
	return err
}

// GetHost returns a host's snapshot, or (nil, nil) before its first sync.
func (r *SentinelRepo) GetHost(ctx context.Context, hostID int64) (*models.SentinelHostSnapshot, error) {
	h := &models.SentinelHostSnapshot{}
	err := scanSentinelHost(r.db.QueryRowContext(ctx,
		`SELECT `+sentinelHostCols+` FROM sentinel_host_snapshots WHERE host_id = ?`, hostID), h)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

// HostsBulk returns every host snapshot keyed by host id.
func (r *SentinelRepo) HostsBulk(ctx context.Context) (map[int64]models.SentinelHostSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sentinelHostCols+` FROM sentinel_host_snapshots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]models.SentinelHostSnapshot{}
	for rows.Next() {
		var h models.SentinelHostSnapshot
		if err := scanSentinelHost(rows, &h); err != nil {
			return nil, err
		}
		out[h.HostID] = h
	}
	return out, rows.Err()
}

// GetService returns a service's snapshot, or (nil, nil) if none.
func (r *SentinelRepo) GetService(ctx context.Context, serviceID int64) (*models.SentinelServiceSnapshot, error) {
	v := &models.SentinelServiceSnapshot{}
	err := scanSentinelService(r.db.QueryRowContext(ctx, sentinelServiceSelect+` WHERE ss.service_id = ?`, serviceID), v)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

// ServicesBulk returns service snapshots keyed by service id; hostID 0 = all.
func (r *SentinelRepo) ServicesBulk(ctx context.Context, hostID int64) (map[int64]models.SentinelServiceSnapshot, error) {
	q, args := sentinelServiceSelect+` WHERE sv.deleted_at IS NULL`, []any{}
	if hostID != 0 {
		q, args = q+` AND ss.host_id = ?`, append(args, hostID)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]models.SentinelServiceSnapshot{}
	for rows.Next() {
		var v models.SentinelServiceSnapshot
		if err := scanSentinelService(rows, &v); err != nil {
			return nil, err
		}
		out[v.ServiceID] = v
	}
	return out, rows.Err()
}

// CoolifyHosts returns every live host linked to a Coolify server — what the
// Sentinel sync visits. Unscoped: the sync is an admin job.
func (r *SentinelRepo) CoolifyHosts(ctx context.Context) ([]models.Host, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+hostColumns()+` FROM hosts
		 WHERE coolify_server_uuid IS NOT NULL AND coolify_server_uuid != '' AND deleted_at IS NULL
		 ORDER BY nickname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Host
	for rows.Next() {
		var h models.Host
		if err := rows.Scan(hostScanDest(&h)...); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// CoolifyHostForService returns the Coolify-linked host a service runs on, or
// (nil, nil) when none of its hosts is linked. Callers have already checked
// the service's visibility.
func (r *SentinelRepo) CoolifyHostForService(ctx context.Context, serviceID int64) (*models.Host, error) {
	h := &models.Host{}
	err := r.db.QueryRowContext(ctx,
		`SELECT `+hostColumns()+` FROM hosts JOIN service_host_links l ON l.host_id = hosts.id
		 WHERE l.service_id = ? AND hosts.coolify_server_uuid IS NOT NULL AND hosts.coolify_server_uuid != ''
		   AND hosts.deleted_at IS NULL
		 ORDER BY hosts.id LIMIT 1`, serviceID).Scan(hostScanDest(h)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}
