package store

import (
	"context"
	"database/sql"
)

// WebRoute is one proxy_pass / fastcgi_pass of a scanned web server (v134):
// site name × location path × target, with the Bridge host and service the
// target reaches when known (0 = unknown). Scan-owned, never edited by hand.
type WebRoute struct {
	ProxyHostID     int64  `json:"proxy_host_id"`
	ProxyServiceID  int64  `json:"proxy_service_id,omitempty"`
	ServerName      string `json:"server_name"` // "" = default / IP-only site
	DNSID           int64  `json:"dns_id,omitempty"`
	Path            string `json:"path"` // "" = whole site (no location block)
	Target          string `json:"target"`
	TargetAddr      string `json:"target_addr"`
	TargetPort      int    `json:"target_port"`
	TargetHostID    int64  `json:"target_host_id,omitempty"`
	TargetServiceID int64  `json:"target_service_id,omitempty"`
}

// Route statuses, worst last: what the UI tells the operator to do next.
const (
	RouteLinked      = "linked"       // host and service known
	RouteNoService   = "no_service"   // host known, port not owned by a service in its latest scan
	RouteNotScanned  = "not_scanned"  // host known, never scanned
	RouteUnknownHost = "unknown_host" // no single live host has that address
)

// WebRouteView is a WebRoute with names for display and its status. Ids and
// names the caller can't see are blanked (a hidden target reads unknown_host).
type WebRouteView struct {
	WebRoute
	ProxyHostSlug     string `json:"proxy_host_slug"`
	ProxyHostName     string `json:"proxy_host_name"`
	ProxyServiceName  string `json:"proxy_service_name,omitempty"`
	Domain            string `json:"domain,omitempty"`
	TargetHostSlug    string `json:"target_host_slug,omitempty"`
	TargetHostName    string `json:"target_host_name,omitempty"`
	TargetServiceName string `json:"target_service_name,omitempty"`
	Status            string `json:"status"`
}

// WebRouteRepo owns SQL for web_routes.
type WebRouteRepo struct{ db *sql.DB }

func NewWebRouteRepo(db *sql.DB) *WebRouteRepo { return &WebRouteRepo{db: db} }

// ReplaceForHost makes the proxy host's routes rs (one tx), except those of
// the proxy services in keep (a server whose config couldn't be read this
// scan): their previous rows stay.
func (r *WebRouteRepo) ReplaceForHost(ctx context.Context, proxyHostID int64, keep []int64, rs []WebRoute) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if keep == nil {
		keep = []int64{}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_routes WHERE proxy_host_id = ?
		AND (proxy_service_id IS NULL OR NOT (proxy_service_id = ANY(?)))`, proxyHostID, keep); err != nil {
		return err
	}
	for _, w := range rs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO web_routes (proxy_host_id, proxy_service_id, server_name, dns_id, path, target,
				target_addr, target_port, target_host_id, target_service_id)
			VALUES (?, NULLIF(?, 0), ?, NULLIF(?, 0), ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0))
			ON CONFLICT DO NOTHING`,
			proxyHostID, w.ProxyServiceID, w.ServerName, w.DNSID, w.Path, w.Target,
			w.TargetAddr, w.TargetPort, w.TargetHostID, w.TargetServiceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListInbound returns other hosts' routes that reach hostID: already resolved
// to it, or aimed at one of its addresses. Unscoped (scan pass). DNSID is 0
// when the record is in the trash: nothing links to it.
func (r *WebRouteRepo) ListInbound(ctx context.Context, hostID int64, addrs []string) ([]WebRoute, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.proxy_host_id, COALESCE(w.proxy_service_id, 0), w.server_name, COALESCE(d.id, 0), w.path, w.target,
			w.target_addr, w.target_port, COALESCE(w.target_host_id, 0), COALESCE(w.target_service_id, 0)
		FROM web_routes w
		LEFT JOIN dns_records d ON d.id = w.dns_id AND d.deleted_at IS NULL
		WHERE w.proxy_host_id <> ? AND (w.target_host_id = ? OR w.target_addr = ANY(?))`, hostID, hostID, addrs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebRoute
	for rows.Next() {
		var w WebRoute
		if err := rows.Scan(&w.ProxyHostID, &w.ProxyServiceID, &w.ServerName, &w.DNSID, &w.Path, &w.Target,
			&w.TargetAddr, &w.TargetPort, &w.TargetHostID, &w.TargetServiceID); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// SetTarget records what a route reaches (0 = unknown). Reports a change.
func (r *WebRouteRepo) SetTarget(ctx context.Context, w WebRoute, hostID, serviceID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE web_routes SET target_host_id = NULLIF(?, 0), target_service_id = NULLIF(?, 0)
		WHERE proxy_host_id = ? AND server_name = ? AND path = ? AND target = ?
		  AND (target_host_id IS DISTINCT FROM NULLIF(?, 0) OR target_service_id IS DISTINCT FROM NULLIF(?, 0))`,
		hostID, serviceID, w.ProxyHostID, w.ServerName, w.Path, w.Target, hostID, serviceID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Outbound lists the routes a host's web servers forward.
func (r *WebRouteRepo) Outbound(ctx context.Context, hostID int64) ([]WebRouteView, error) {
	return r.views(ctx, `r.proxy_host_id = ?`, hostID)
}

// Inbound lists other hosts' routes that reach this host.
func (r *WebRouteRepo) Inbound(ctx context.Context, hostID int64) ([]WebRouteView, error) {
	return r.views(ctx, `r.target_host_id = ? AND r.proxy_host_id <> ?`, hostID, hostID)
}

// ByDNS lists the routes serving one DNS record.
func (r *WebRouteRepo) ByDNS(ctx context.Context, dnsID int64) ([]WebRouteView, error) {
	return r.views(ctx, `r.dns_id = ?`, dnsID)
}

// views reads routes whose proxy host the caller can see (the route is that
// host's config), then blanks the other ends it can't see.
func (r *WebRouteRepo) views(ctx context.Context, where string, args ...any) ([]WebRouteView, error) {
	vis, vargs := VisibleExpr(ctx, AssetHost, "r.proxy_host_id")
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.proxy_host_id, ph.oficial_slug, ph.nickname,
			COALESCE(ps.id, 0), COALESCE(ps.nickname, ''),
			r.server_name, COALESCE(d.id, 0), COALESCE(d.domain, ''),
			r.path, r.target, r.target_addr, r.target_port,
			COALESCE(th.id, 0), COALESCE(th.oficial_slug, ''), COALESCE(th.nickname, ''),
			COALESCE(ts.id, 0), COALESCE(ts.nickname, ''),
			EXISTS (SELECT 1 FROM host_scans s WHERE s.host_id = th.id)
		FROM web_routes r
		JOIN hosts ph ON ph.id = r.proxy_host_id AND ph.deleted_at IS NULL
		LEFT JOIN services ps ON ps.id = r.proxy_service_id AND ps.deleted_at IS NULL
		LEFT JOIN dns_records d ON d.id = r.dns_id AND d.deleted_at IS NULL
		LEFT JOIN hosts th ON th.id = r.target_host_id AND th.deleted_at IS NULL
		LEFT JOIN services ts ON ts.id = r.target_service_id AND ts.deleted_at IS NULL
		WHERE `+where+` AND `+vis+`
		ORDER BY r.server_name, r.path, r.target`, append(args, vargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebRouteView{}
	for rows.Next() {
		var v WebRouteView
		var scanned bool
		if err := rows.Scan(&v.ProxyHostID, &v.ProxyHostSlug, &v.ProxyHostName,
			&v.ProxyServiceID, &v.ProxyServiceName,
			&v.ServerName, &v.DNSID, &v.Domain,
			&v.Path, &v.Target, &v.TargetAddr, &v.TargetPort,
			&v.TargetHostID, &v.TargetHostSlug, &v.TargetHostName,
			&v.TargetServiceID, &v.TargetServiceName, &scanned); err != nil {
			return nil, err
		}
		v.Status = routeStatus(v.TargetHostID, v.TargetServiceID, scanned)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sets, err := VisibleSets(ctx, r.db, AssetHost, AssetService, AssetDNS)
	if err != nil {
		return nil, err
	}
	hosts, svcs, dns := sets[0], sets[1], sets[2]
	for i := range out {
		v := &out[i]
		if !visible(hosts, v.TargetHostID) {
			v.TargetHostID, v.TargetHostSlug, v.TargetHostName = 0, "", ""
			v.TargetServiceID, v.TargetServiceName, v.Status = 0, "", RouteUnknownHost
		}
		if !visible(svcs, v.TargetServiceID) {
			v.TargetServiceID, v.TargetServiceName = 0, ""
			if v.Status == RouteLinked {
				v.Status = RouteNoService
			}
		}
		if !visible(svcs, v.ProxyServiceID) {
			v.ProxyServiceID, v.ProxyServiceName = 0, ""
		}
		if !visible(dns, v.DNSID) {
			v.DNSID, v.Domain = 0, ""
		}
	}
	return out, nil
}

// visible: id 0 has nothing to hide; a nil set sees everything.
func visible(set map[int64]bool, id int64) bool {
	return id == 0 || set == nil || set[id]
}

func routeStatus(hostID, serviceID int64, scanned bool) string {
	switch {
	case hostID == 0:
		return RouteUnknownHost
	case serviceID != 0:
		return RouteLinked
	case !scanned:
		return RouteNotScanned
	default:
		return RouteNoService
	}
}
