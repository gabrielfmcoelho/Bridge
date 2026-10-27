package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// DependencyRepo owns SQL for dependency_inventory: the OS, runtime and
// package versions the scan found per host and container. Rows are not an
// asset of their own: a row is visible when its host or its (container)
// service is, the same rule findings follow through their links.
type DependencyRepo struct{ db *sql.DB }

// NewDependencyRepo constructs a DependencyRepo over db.
func NewDependencyRepo(db *sql.DB) *DependencyRepo { return &DependencyRepo{db: db} }

const dependencyCols = `d.id, d.host_id, h.nickname, h.oficial_slug, d.service_id, d.container, d.image, d.image_id,
	d.kind, d.name, d.version, d.version_source, d.lookup, d.ecosystem, d.lookup_name, d.lookup_version,
	d.eol_product, d.cve_count, d.max_severity, d.eol, d.eol_date, d.evaluated_at, d.first_seen_at, d.last_seen_at`

func scanDependency(sc interface{ Scan(...any) error }, d *models.Dependency) error {
	return sc.Scan(&d.ID, &d.HostID, &d.HostNickname, &d.HostSlug, &d.ServiceID, &d.Container, &d.Image, &d.ImageID,
		&d.Kind, &d.Name, &d.Version, &d.VersionSource, &d.Lookup, &d.Ecosystem, &d.LookupName, &d.LookupVersion,
		&d.EOLProduct, &d.CVECount, &d.MaxSeverity, &d.EOL, &d.EOLDate, &d.EvaluatedAt, &d.FirstSeenAt, &d.LastSeenAt)
}

// dependencyVisible is the row predicate: its live host or its service is
// visible to the caller.
func dependencyVisible(ctx context.Context) (string, []any) {
	hv, ha := VisibleExpr(ctx, AssetHost, "d.host_id")
	if hv == "TRUE" {
		return "TRUE", nil
	}
	sv, sa := VisibleExpr(ctx, AssetService, "d.service_id")
	return "(" + hv + " OR (d.service_id IS NOT NULL AND " + sv + "))", append(ha, sa...)
}

// HostScopes says which parts of a host a scan actually looked at, so
// ReplaceForHost only forgets what it re-read.
type HostScopes struct {
	// Probed lists the containers ("" = the host itself) whose probe
	// succeeded: their rows not in this scan are gone.
	Probed []string
	// Running is every container docker ps listed; with RunningKnown,
	// rows of containers not in it are removed (the container is gone).
	Running      []string
	RunningKnown bool
}

// ReplaceForHost upserts a scan's rows for hostID and removes what the scan
// proves is no longer there, in one transaction. Scopes that weren't probed
// (exec failed, over the cap) keep their previous rows.
func (r *DependencyRepo) ReplaceForHost(ctx context.Context, hostID int64, rows []models.Dependency, sc HostScopes) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	const perRow = 15
	const chunk = 500
	for start := 0; start < len(rows); start += chunk {
		end := min(start+chunk, len(rows))
		var b strings.Builder
		b.WriteString(`INSERT INTO dependency_inventory (host_id, service_id, container, image, image_id, kind, name, version,
			version_source, lookup, ecosystem, lookup_name, lookup_version, eol_product, last_seen_at) VALUES `)
		args := make([]any, 0, (end-start)*perRow)
		for i, d := range rows[start:end] {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
			args = append(args, hostID, d.ServiceID, d.Container, d.Image, d.ImageID, d.Kind, d.Name, d.Version,
				d.VersionSource, d.Lookup, d.Ecosystem, d.LookupName, d.LookupVersion, d.EOLProduct, now)
		}
		// A row repeated within one scan (same container/kind/name/version)
		// would make ON CONFLICT touch it twice: callers dedupe (dedupeDeps).
		b.WriteString(` ON CONFLICT (host_id, container, kind, name, version) DO UPDATE SET
			service_id = EXCLUDED.service_id, image = EXCLUDED.image, image_id = EXCLUDED.image_id,
			version_source = EXCLUDED.version_source, lookup = EXCLUDED.lookup, ecosystem = EXCLUDED.ecosystem,
			lookup_name = EXCLUDED.lookup_name, lookup_version = EXCLUDED.lookup_version,
			eol_product = EXCLUDED.eol_product, last_seen_at = EXCLUDED.last_seen_at`)
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return err
		}
	}
	if len(sc.Probed) > 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM dependency_inventory
			WHERE host_id = ? AND container = ANY(?) AND last_seen_at < ?`, hostID, sc.Probed, now); err != nil {
			return err
		}
	}
	if sc.RunningKnown {
		running := sc.Running
		if running == nil {
			running = []string{}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM dependency_inventory
			WHERE host_id = ? AND container <> '' AND NOT (container = ANY(?))`, hostID, running); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DedupeDependencies drops repeated (container, kind, name, version) rows,
// keeping the first.
func DedupeDependencies(rows []models.Dependency) []models.Dependency {
	seen := map[string]bool{}
	out := rows[:0:0]
	for _, d := range rows {
		k := d.Container + "\x1f" + d.Kind + "\x1f" + d.Name + "\x1f" + d.Version
		if !seen[k] {
			seen[k] = true
			out = append(out, d)
		}
	}
	return out
}

// List returns the visible rows matching f, ordered by host, container,
// kind and name, with the total for pagination. f.PerPage <= 0 = all rows.
// dependencyWhere is the FROM/WHERE of List and ListGroups: visible rows of
// live hosts, by kind (default os + runtime), host, service, flags and search.
func dependencyWhere(ctx context.Context, f models.DependencyFilter) (string, []any) {
	vis, args := dependencyVisible(ctx)
	where := ` FROM dependency_inventory d JOIN hosts h ON h.id = d.host_id AND h.deleted_at IS NULL WHERE ` + vis
	kinds := f.Kinds
	if len(kinds) == 0 {
		kinds = []string{models.DepKindOS, models.DepKindRuntime}
	}
	where += " AND d.kind = ANY(?)"
	args = append(args, kinds)
	if f.HostID != 0 {
		where += " AND d.host_id = ?"
		args = append(args, f.HostID)
	}
	if f.ServiceID != 0 {
		where += " AND d.service_id = ?"
		args = append(args, f.ServiceID)
	}
	if f.HasCVEs {
		where += " AND d.cve_count > 0"
	}
	if f.EOL {
		where += " AND d.eol"
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		where += ` AND (LOWER(d.name) LIKE ? OR LOWER(d.version) LIKE ? OR LOWER(d.container) LIKE ?
			OR LOWER(d.image) LIKE ? OR LOWER(h.nickname) LIKE ?)`
		args = append(args, like, like, like, like, like)
	}
	return where, args
}

func (r *DependencyRepo) List(ctx context.Context, f models.DependencyFilter) ([]models.Dependency, int, error) {
	where, args := dependencyWhere(ctx, f)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + dependencyCols + where + ` ORDER BY d.cve_count DESC, h.nickname, d.container, d.kind, d.name, d.version`
	if f.PerPage > 0 {
		page := max(f.Page, 1)
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.PerPage, (page-1)*f.PerPage)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Dependency{}
	for rows.Next() {
		var d models.Dependency
		if err := scanDependency(rows, &d); err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// ListGroups is List grouped by component version (kind, name, version):
// worst CVE count and severity first, then the most widespread.
func (r *DependencyRepo) ListGroups(ctx context.Context, f models.DependencyFilter) ([]models.DependencyGroup, int, error) {
	where, args := dependencyWhere(ctx, f)
	const group = ` GROUP BY d.kind, d.name, d.version`
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT 1`+where+group+`) g`, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT d.kind, d.name, d.version, COUNT(DISTINCT d.host_id), COUNT(*), MAX(d.cve_count),
		MAX(CASE d.max_severity WHEN 'critical' THEN 5 WHEN 'high' THEN 4 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 0 END) AS sev,
		BOOL_OR(d.eol), MAX(d.eol_date), COUNT(d.evaluated_at), MAX(d.lookup), MIN(d.version_source),
		to_json((ARRAY_AGG(json_build_object('id', d.id, 'host_id', d.host_id, 'host_nickname', h.nickname, 'host_slug', h.oficial_slug,
			'container', d.container, 'image', d.image, 'service_id', d.service_id, 'cve_count', d.cve_count)
			ORDER BY h.nickname, d.container))[1:50])::text` + where + group + `
		ORDER BY sev DESC, MAX(d.cve_count) DESC, COUNT(DISTINCT d.host_id) DESC, d.name, d.version`
	if f.PerPage > 0 {
		page := max(f.Page, 1)
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.PerPage, (page-1)*f.PerPage)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	severity := []string{"", "info", "low", "medium", "high", "critical"}
	out := []models.DependencyGroup{}
	for rows.Next() {
		var g models.DependencyGroup
		var sev int
		var instances string
		if err := rows.Scan(&g.Kind, &g.Name, &g.Version, &g.Hosts, &g.Installs, &g.CVECount, &sev,
			&g.EOL, &g.EOLDate, &g.Evaluated, &g.Lookup, &g.VersionSource, &instances); err != nil {
			return nil, 0, err
		}
		g.MaxSeverity = severity[sev]
		if err := json.Unmarshal([]byte(instances), &g.Instances); err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

// Get returns one visible row, or (nil, nil).
func (r *DependencyRepo) Get(ctx context.Context, id int64) (*models.Dependency, error) {
	vis, args := dependencyVisible(ctx)
	d := &models.Dependency{}
	err := scanDependency(r.db.QueryRowContext(ctx, `SELECT `+dependencyCols+`
		FROM dependency_inventory d JOIN hosts h ON h.id = d.host_id AND h.deleted_at IS NULL
		WHERE d.id = ? AND `+vis, append([]any{id}, args...)...), d)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// ForHosts returns every row of the given hosts (nil = all live hosts),
// unscoped: for the evaluation, which runs as system work.
func (r *DependencyRepo) ForHosts(ctx context.Context, hostIDs []int64) ([]models.Dependency, error) {
	q := `SELECT ` + dependencyCols + ` FROM dependency_inventory d JOIN hosts h ON h.id = d.host_id AND h.deleted_at IS NULL`
	var args []any
	if hostIDs != nil {
		q += ` WHERE d.host_id = ANY(?)`
		args = append(args, hostIDs)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY d.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Dependency
	for rows.Next() {
		var d models.Dependency
		if err := scanDependency(rows, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DependencyDerived is what an evaluation writes back onto a row.
type DependencyDerived struct {
	ID          int64
	CVECount    int
	MaxSeverity string
	EOL         bool
	EOLDate     string
	EvaluatedAt *time.Time
}

// SetDerived writes evaluation results onto rows, batched.
func (r *DependencyRepo) SetDerived(ctx context.Context, ds []DependencyDerived) error {
	const chunk = 500
	for start := 0; start < len(ds); start += chunk {
		part := ds[start:min(start+chunk, len(ds))]
		var b strings.Builder
		b.WriteString(`UPDATE dependency_inventory d SET cve_count = u.cve_count, max_severity = u.max_severity,
			eol = u.eol, eol_date = u.eol_date, evaluated_at = COALESCE(u.evaluated_at, d.evaluated_at) FROM (VALUES `)
		args := make([]any, 0, len(part)*6)
		for k, d := range part {
			if k > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?::bigint,?::integer,?,?::boolean,?,?::timestamptz)")
			args = append(args, d.ID, d.CVECount, d.MaxSeverity, d.EOL, d.EOLDate, d.EvaluatedAt)
		}
		b.WriteString(`) AS u(id, cve_count, max_severity, eol, eol_date, evaluated_at) WHERE d.id = u.id`)
		if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

// Advisory is one cached answer from OSV, NVD or endoflife.date.
type Advisory struct {
	Data      string
	Error     string
	FetchedAt time.Time
}

// AdvisoryRepo owns dependency_advisories, the lookup cache. Keys: osv
// "<ecosystem>|<package>|<version>" (ids) and "vuln|<id>" (details), nvd
// "<vendor:product>|<version>", eol "<product>".
type AdvisoryRepo struct{ db *sql.DB }

// NewAdvisoryRepo constructs an AdvisoryRepo over db.
func NewAdvisoryRepo(db *sql.DB) *AdvisoryRepo { return &AdvisoryRepo{db: db} }

// Get returns the cached entries among keys.
func (r *AdvisoryRepo) Get(ctx context.Context, source string, keys []string) (map[string]Advisory, error) {
	out := map[string]Advisory{}
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT lookup_key, data, error, fetched_at FROM dependency_advisories
		WHERE source = ? AND lookup_key = ANY(?)`, source, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var a Advisory
		if err := rows.Scan(&k, &a.Data, &a.Error, &a.FetchedAt); err != nil {
			return nil, err
		}
		out[k] = a
	}
	return out, rows.Err()
}

// Stale returns the keys with no entry, a failed one, or one older than
// maxAge.
func (r *AdvisoryRepo) Stale(ctx context.Context, source string, keys []string, maxAge time.Duration) ([]string, error) {
	have, err := r.Get(ctx, source, keys)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-maxAge)
	var out []string
	for _, k := range keys {
		a, ok := have[k]
		if !ok || a.Error != "" || a.FetchedAt.Before(cutoff) {
			out = append(out, k)
		}
	}
	return out, nil
}

// Put stores one answer. A failure records the error but keeps the last
// good data, so one bad lookup doesn't erase known CVEs.
func (r *AdvisoryRepo) Put(ctx context.Context, source, key, data, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO dependency_advisories (source, lookup_key, data, error, fetched_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (source, lookup_key) DO UPDATE SET error = EXCLUDED.error, fetched_at = EXCLUDED.fetched_at,
			data = CASE WHEN EXCLUDED.error = '' THEN EXCLUDED.data ELSE dependency_advisories.data END`,
		source, key, data, errMsg)
	return err
}
