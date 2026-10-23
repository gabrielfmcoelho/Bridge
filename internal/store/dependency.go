package store

import (
	"context"
	"database/sql"
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
func (r *DependencyRepo) List(ctx context.Context, f models.DependencyFilter) ([]models.Dependency, int, error) {
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
