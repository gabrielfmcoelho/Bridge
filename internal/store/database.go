package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// DatabaseRepo owns SQL for the Atlas databases catalogue (databases,
// database_consumer_links, database_project_links). Every read is scoped by
// VisibleExpr on the database itself; the instance summary is shown only
// when the caller can also see that service (and its host).
type DatabaseRepo struct{ db *sql.DB }

// NewDatabaseRepo constructs a DatabaseRepo over db.
func NewDatabaseRepo(db *sql.DB) *DatabaseRepo { return &DatabaseRepo{db: db} }

const databaseCols = `d.id, d.name, d.db_name, d.description, d.use_cases, d.engine, d.instance_service_id,
	d.external_provider, d.host, d.port, d.connection_mode, d.tunnel_host_id, d.tunnel_target, d.tls_mode,
	d.credential_secret_id IS NOT NULL, d.sensitivity, d.environment, d.backup_policy, d.backup_retention_days,
	d.last_backup_at, d.created_by, d.created_at, d.updated_at, d.deleted_at`

func scanDatabase(sc interface{ Scan(...any) error }, d *models.Database) error {
	return sc.Scan(&d.ID, &d.Name, &d.DBName, &d.Description, &d.UseCases, &d.Engine, &d.InstanceServiceID,
		&d.ExternalProvider, &d.Host, &d.Port, &d.ConnectionMode, &d.TunnelHostID, &d.TunnelTarget, &d.TLSMode,
		&d.HasCredential, &d.Sensitivity, &d.Environment, &d.BackupPolicy, &d.BackupRetentionDays,
		&d.LastBackupAt, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)
}

// Create inserts a database row (grants and links are set by the caller).
func (r *DatabaseRepo) Create(ctx context.Context, d *models.Database) error {
	d.ApplyDefaults()
	if err := d.Validate(); err != nil {
		return err
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO databases (name, db_name, description, use_cases, engine, instance_service_id, external_provider,
			host, port, connection_mode, tunnel_host_id, tunnel_target, tls_mode, sensitivity, environment,
			backup_policy, backup_retention_days, last_backup_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Name, d.DBName, d.Description, d.UseCases, d.Engine, d.InstanceServiceID, d.ExternalProvider,
		d.Host, d.Port, d.ConnectionMode, d.TunnelHostID, d.TunnelTarget, d.TLSMode, d.Sensitivity, d.Environment,
		d.BackupPolicy, d.BackupRetentionDays, d.LastBackupAt, d.CreatedBy)
	if err != nil {
		return err
	}
	d.ID = id
	return nil
}

// Update writes every editable field of d (the handler merges a partial
// request onto the current row first). Returns sql.ErrNoRows when no live,
// visible row matched.
func (r *DatabaseRepo) Update(ctx context.Context, d *models.Database) error {
	if err := d.Validate(); err != nil {
		return err
	}
	vis, vargs := VisibleExpr(ctx, AssetDatabase, "databases.id")
	res, err := r.db.ExecContext(ctx,
		`UPDATE databases SET name = ?, db_name = ?, description = ?, use_cases = ?, engine = ?, instance_service_id = ?,
			external_provider = ?, host = ?, port = ?, connection_mode = ?, tunnel_host_id = ?, tunnel_target = ?,
			tls_mode = ?, sensitivity = ?, environment = ?, backup_policy = ?, backup_retention_days = ?,
			last_backup_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL AND `+vis,
		append([]any{d.Name, d.DBName, d.Description, d.UseCases, d.Engine, d.InstanceServiceID,
			d.ExternalProvider, d.Host, d.Port, d.ConnectionMode, d.TunnelHostID, d.TunnelTarget,
			d.TLSMode, d.Sensitivity, d.Environment, d.BackupPolicy, d.BackupRetentionDays,
			d.LastBackupAt, d.ID}, vargs...)...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Get loads a live, visible database with its links and instance summary, or
// (nil, nil).
func (r *DatabaseRepo) Get(ctx context.Context, id int64) (*models.Database, error) {
	vis, vargs := VisibleExpr(ctx, AssetDatabase, "d.id")
	d := &models.Database{}
	err := scanDatabase(r.db.QueryRowContext(ctx,
		`SELECT `+databaseCols+` FROM databases d WHERE d.id = ? AND d.deleted_at IS NULL AND `+vis,
		append([]any{id}, vargs...)...), d)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := r.enrich(ctx, []models.Database{*d})
	if err != nil {
		return nil, err
	}
	return &out[0], nil
}

// List returns live, visible databases filtered per f, ordered by name.
func (r *DatabaseRepo) List(ctx context.Context, f models.DatabaseFilter) ([]models.Database, error) {
	vis, args := VisibleExpr(ctx, AssetDatabase, "d.id")
	q := `SELECT ` + databaseCols + ` FROM databases d WHERE d.deleted_at IS NULL AND ` + vis
	if f.Query != "" {
		like := "%" + strings.ToLower(f.Query) + "%"
		q += ` AND (LOWER(d.name) LIKE ? OR LOWER(d.db_name) LIKE ? OR LOWER(d.description) LIKE ?
			OR LOWER(d.use_cases) LIKE ? OR LOWER(d.external_provider) LIKE ?)`
		args = append(args, like, like, like, like, like)
	}
	for _, eq := range []struct{ col, v string }{
		{"d.engine", f.Engine}, {"d.environment", f.Environment}, {"d.sensitivity", f.Sensitivity},
	} {
		if eq.v != "" {
			q += " AND " + eq.col + " = ?"
			args = append(args, eq.v)
		}
	}
	if f.InstanceServiceID != 0 {
		q += " AND d.instance_service_id = ?"
		args = append(args, f.InstanceServiceID)
	}
	if f.ConsumerServiceID != 0 {
		q += " AND d.id IN (SELECT database_id FROM database_consumer_links WHERE service_id = ?)"
		args = append(args, f.ConsumerServiceID)
	}
	if f.ProjectID != 0 {
		q += ` AND (d.id IN (SELECT database_id FROM database_project_links WHERE project_id = ?)
			OR d.instance_service_id IN (SELECT id FROM services WHERE project_id = ? AND deleted_at IS NULL)
			OR d.id IN (SELECT l.database_id FROM database_consumer_links l JOIN services s ON s.id = l.service_id
				WHERE s.project_id = ? AND s.deleted_at IS NULL))`
		args = append(args, f.ProjectID, f.ProjectID, f.ProjectID)
	}
	q += " ORDER BY d.name, d.id"
	return r.query(ctx, q, args...)
}

// ListTrash returns soft-deleted, visible databases, newest first.
func (r *DatabaseRepo) ListTrash(ctx context.Context) ([]models.Database, error) {
	vis, args := VisibleExpr(ctx, AssetDatabase, "d.id")
	return r.query(ctx, `SELECT `+databaseCols+` FROM databases d WHERE d.deleted_at IS NOT NULL AND `+vis+
		` ORDER BY d.deleted_at DESC`, args...)
}

func (r *DatabaseRepo) query(ctx context.Context, q string, args ...any) ([]models.Database, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := []models.Database{}
	for rows.Next() {
		var d models.Database
		if err := scanDatabase(rows, &d); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.enrich(ctx, out)
}

// enrich fills links and the instance summary on a page of rows, one query
// per kind.
func (r *DatabaseRepo) enrich(ctx context.Context, dbs []models.Database) ([]models.Database, error) {
	if len(dbs) == 0 {
		return dbs, nil
	}
	ids := make([]any, len(dbs))
	var svcIDs []int64
	for i := range dbs {
		ids[i] = dbs[i].ID
		dbs[i].ConsumerServiceIDs, dbs[i].ProjectIDs = []int64{}, []int64{}
		if dbs[i].InstanceServiceID != nil {
			svcIDs = append(svcIDs, *dbs[i].InstanceServiceID)
		}
	}
	ph := placeholders(len(ids))
	byID := map[int64]*models.Database{}
	for i := range dbs {
		byID[dbs[i].ID] = &dbs[i]
	}
	for _, q := range []struct {
		sql string
		set func(d *models.Database, other int64)
	}{
		{`SELECT l.database_id, l.service_id FROM database_consumer_links l JOIN services s ON s.id = l.service_id
			WHERE s.deleted_at IS NULL AND l.database_id IN (` + ph + `) ORDER BY l.service_id`,
			func(d *models.Database, o int64) { d.ConsumerServiceIDs = append(d.ConsumerServiceIDs, o) }},
		{`SELECT l.database_id, l.project_id FROM database_project_links l JOIN projects p ON p.id = l.project_id
			WHERE p.deleted_at IS NULL AND l.database_id IN (` + ph + `) ORDER BY l.project_id`,
			func(d *models.Database, o int64) { d.ProjectIDs = append(d.ProjectIDs, o) }},
	} {
		rows, err := r.db.QueryContext(ctx, q.sql, ids...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var dbID, other int64
			if err := rows.Scan(&dbID, &other); err != nil {
				rows.Close()
				return nil, err
			}
			q.set(byID[dbID], other)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	refs, err := r.instanceRefs(ctx, svcIDs)
	if err != nil {
		return nil, err
	}
	for i := range dbs {
		if dbs[i].InstanceServiceID != nil {
			if ref, ok := refs[*dbs[i].InstanceServiceID]; ok {
				dbs[i].Instance = &ref
			}
		}
	}
	return dbs, nil
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

// instanceSelect reads a services row as an instance summary: its first
// visible host (by id) through service_host_links. The host predicate's args
// come first (it sits in the SELECT list), then the service predicate's.
func instanceSelect(ctx context.Context) (selectSQL string, args []any) {
	hvis, hargs := VisibleExpr(ctx, AssetHost, "h.id")
	svis, sargs := VisibleExpr(ctx, AssetService, "s.id")
	return `SELECT s.id, s.nickname, s.service_subtype, s.container_name, s.container_image, s.container_status, s.port,
			hh.id, COALESCE(hh.nickname, ''), COALESCE(hh.oficial_slug, ''),
			s.service_kind, s.coolify_resource_type
		FROM services s
		LEFT JOIN LATERAL (SELECT h.id, h.nickname, h.oficial_slug FROM service_host_links l
			JOIN hosts h ON h.id = l.host_id
			WHERE l.service_id = s.id AND h.deleted_at IS NULL AND ` + hvis + ` ORDER BY h.id LIMIT 1) hh ON TRUE
		WHERE s.deleted_at IS NULL AND ` + svis, append(hargs, sargs...)
}

func scanInstance(rows *sql.Rows) (models.DatabaseInstance, error) {
	var in models.DatabaseInstance
	err := rows.Scan(&in.ID, &in.Nickname, &in.Subtype, &in.ContainerName, &in.ContainerImage, &in.ContainerStatus,
		&in.Port, &in.HostID, &in.HostNickname, &in.HostSlug, &in.Kind, &in.CoolifyResourceType)
	in.Engine = models.InferDatabaseEngine(in.Subtype, in.ContainerImage)
	return in, err
}

// instanceRefs loads the visible services among ids as instance summaries.
func (r *DatabaseRepo) instanceRefs(ctx context.Context, ids []int64) (map[int64]models.DatabaseInstanceRef, error) {
	out := map[int64]models.DatabaseInstanceRef{}
	if len(ids) == 0 {
		return out, nil
	}
	sel, args := instanceSelect(ctx)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, sel+` AND s.id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		in, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		out[in.ID] = in.DatabaseInstanceRef
	}
	return out, rows.Err()
}

// Instances lists the visible engines found by the scan or Coolify — services
// of kind database/cache, or Coolify database resources — with how many
// visible catalogued databases point at each.
func (r *DatabaseRepo) Instances(ctx context.Context) ([]models.DatabaseInstance, error) {
	sel, args := instanceSelect(ctx)
	rows, err := r.db.QueryContext(ctx,
		sel+` AND (s.service_kind IN ('database', 'cache') OR s.coolify_resource_type = 'database')
		ORDER BY s.nickname, s.id`, args...)
	if err != nil {
		return nil, err
	}
	out := []models.DatabaseInstance{}
	for rows.Next() {
		in, err := scanInstance(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, in)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	vis, vargs := VisibleExpr(ctx, AssetDatabase, "d.id")
	crows, err := r.db.QueryContext(ctx, `SELECT d.instance_service_id, COUNT(*) FROM databases d
		WHERE d.deleted_at IS NULL AND d.instance_service_id IS NOT NULL AND `+vis+` GROUP BY d.instance_service_id`, vargs...)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	counts := map[int64]int{}
	for crows.Next() {
		var id int64
		var n int
		if err := crows.Scan(&id, &n); err != nil {
			return nil, err
		}
		counts[id] = n
	}
	for i := range out {
		out[i].RegisteredCount = counts[out[i].ID]
	}
	return out, crows.Err()
}

// SetLinks replaces a database's consumer services and projects. A nil slice
// leaves that side untouched; an empty one clears it.
func (r *DatabaseRepo) SetLinks(ctx context.Context, id int64, consumerIDs, projectIDs []int64) error {
	if consumerIDs != nil {
		if err := replaceLinks(ctx, r.db, `database_consumer_links`, `database_id`, `service_id`, id, dedupeIDs(consumerIDs)); err != nil {
			return err
		}
	}
	if projectIDs != nil {
		if err := replaceLinks(ctx, r.db, `database_project_links`, `database_id`, `project_id`, id, dedupeIDs(projectIDs)); err != nil {
			return err
		}
	}
	return nil
}

// Restore clears deleted_at. found is false when no trashed, visible row
// matched.
func (r *DatabaseRepo) Restore(ctx context.Context, id int64) (found bool, err error) {
	vis, vargs := VisibleExpr(ctx, AssetDatabase, "databases.id")
	res, err := r.db.ExecContext(ctx, `UPDATE databases SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NOT NULL AND `+vis, append([]any{id}, vargs...)...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// IsUniqueViolation reports whether err is Postgres' unique_violation (23505).
func IsUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
