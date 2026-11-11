package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// OSUpdateRepo owns os_update_runs / os_update_results. Runs are admin-only
// (the handlers gate them), so nothing here is entidade-scoped.
type OSUpdateRepo struct{ db *sql.DB }

func NewOSUpdateRepo(db *sql.DB) *OSUpdateRepo { return &OSUpdateRepo{db: db} }

// CreateRun inserts the run and one pending result per host.
func (r *OSUpdateRepo) CreateRun(ctx context.Context, run *models.OSUpdateRun, hostIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `INSERT INTO os_update_runs (kind, reboot, concurrency, target, requested_by)
		VALUES (?, ?, ?, ?, ?) RETURNING id, status, created_at`,
		run.Kind, run.Reboot, run.Concurrency, run.Target, run.RequestedBy).Scan(&run.ID, &run.Status, &run.CreatedAt); err != nil {
		return err
	}
	for _, id := range hostIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO os_update_results (run_id, host_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, run.ID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const osRunCols = `r.id, r.kind, r.reboot, r.concurrency, r.target, r.status, r.requested_by, COALESCE(u.display_name, ''),
	r.created_at, r.finished_at,
	(SELECT COALESCE(json_object_agg(s.status, s.n), '{}')::text FROM
		(SELECT status, COUNT(*) AS n FROM os_update_results WHERE run_id = r.id GROUP BY status) s)
	FROM os_update_runs r LEFT JOIN users u ON u.id = r.requested_by`

func scanOSRun(sc interface{ Scan(...any) error }, run *models.OSUpdateRun) error {
	var counts string
	if err := sc.Scan(&run.ID, &run.Kind, &run.Reboot, &run.Concurrency, &run.Target, &run.Status, &run.RequestedBy,
		&run.RequestedName, &run.CreatedAt, &run.FinishedAt, &counts); err != nil {
		return err
	}
	return json.Unmarshal([]byte(counts), &run.Counts)
}

// ListRuns returns the newest runs first, with their per-status counts.
func (r *OSUpdateRepo) ListRuns(ctx context.Context, limit int) ([]models.OSUpdateRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+osRunCols+` ORDER BY r.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OSUpdateRun{}
	for rows.Next() {
		var run models.OSUpdateRun
		if err := scanOSRun(rows, &run); err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// GetRun returns the run with every host's result, or (nil, nil).
func (r *OSUpdateRepo) GetRun(ctx context.Context, id int64) (*models.OSUpdateRun, error) {
	run := &models.OSUpdateRun{}
	if err := scanOSRun(r.db.QueryRowContext(ctx, `SELECT `+osRunCols+` WHERE r.id = ?`, id), run); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+osResultCols+` WHERE x.run_id = ? ORDER BY h.nickname`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var res models.OSUpdateResult
		if err := scanOSResult(rows, &res); err != nil {
			return nil, err
		}
		run.Results = append(run.Results, res)
	}
	return run, rows.Err()
}

const osResultCols = `x.run_id, x.host_id, h.nickname, h.oficial_slug, x.status, x.reason, x.pkg_manager, x.os_before, x.os_after,
	x.upgradable, x.security_upgradable, x.upgraded, x.reboot_required, x.rebooted, x.output, x.started_at, x.finished_at
	FROM os_update_results x JOIN hosts h ON h.id = x.host_id`

func scanOSResult(sc interface{ Scan(...any) error }, x *models.OSUpdateResult) error {
	return sc.Scan(&x.RunID, &x.HostID, &x.HostNickname, &x.HostSlug, &x.Status, &x.Reason, &x.PkgManager, &x.OSBefore, &x.OSAfter,
		&x.Upgradable, &x.SecurityUpgradable, &x.Upgraded, &x.RebootRequired, &x.Rebooted, &x.Output, &x.StartedAt, &x.FinishedAt)
}

// PendingHosts returns the run's hosts still pending, in id order.
func (r *OSUpdateRepo) PendingHosts(ctx context.Context, runID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id FROM os_update_results WHERE run_id = ? AND status = 'pending' ORDER BY host_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// StartResult marks a host running. False when it was no longer pending
// (the run was cancelled meanwhile).
func (r *OSUpdateRepo) StartResult(ctx context.Context, runID, hostID int64, osBefore string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE os_update_results SET status = 'running', os_before = ?, started_at = now()
		WHERE run_id = ? AND host_id = ? AND status = 'pending'`, osBefore, runID, hostID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// FinishResult writes a host's outcome.
func (r *OSUpdateRepo) FinishResult(ctx context.Context, x *models.OSUpdateResult) error {
	_, err := r.db.ExecContext(ctx, `UPDATE os_update_results SET status = ?, reason = ?, pkg_manager = ?, os_after = ?,
		upgradable = ?, security_upgradable = ?, upgraded = ?, reboot_required = ?, rebooted = ?, output = ?, finished_at = now()
		WHERE run_id = ? AND host_id = ?`,
		x.Status, x.Reason, x.PkgManager, x.OSAfter, x.Upgradable, x.SecurityUpgradable, x.Upgraded, x.RebootRequired, x.Rebooted, x.Output,
		x.RunID, x.HostID)
	return err
}

// SkipPending marks every still-pending host of the run skipped.
func (r *OSUpdateRepo) SkipPending(ctx context.Context, runID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE os_update_results SET status = 'skipped', reason = ?, finished_at = now()
		WHERE run_id = ? AND status = 'pending'`, reason, runID)
	return err
}

// FinishRun closes a running run with status (done / cancelled). False when
// it wasn't running.
func (r *OSUpdateRepo) FinishRun(ctx context.Context, runID int64, status string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE os_update_runs SET status = ?, finished_at = now() WHERE id = ? AND status = 'running'`, status, runID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// MarkInterrupted closes the runs a restart cut short: their running hosts
// failed (the update may still be going on the host), pending ones skipped.
func (r *OSUpdateRepo) MarkInterrupted(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		WITH cut AS (UPDATE os_update_runs SET status = 'interrupted', finished_at = now() WHERE status = 'running' RETURNING id)
		UPDATE os_update_results x SET
			status = CASE x.status WHEN 'running' THEN 'failed' ELSE 'skipped' END,
			reason = CASE x.status WHEN 'running' THEN 'interrupted by a Bridge restart: the update may still be running on the host'
				ELSE 'interrupted by a Bridge restart' END,
			finished_at = now()
		FROM cut WHERE x.run_id = cut.id AND x.status IN ('pending','running')`)
	return err
}

// LatestByHost returns each host's newest finished result that counted
// upgrades (any kind), for the "pending updates" column.
func (r *OSUpdateRepo) LatestByHost(ctx context.Context) ([]models.OSUpdateResult, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+osResultCols+`
		JOIN (SELECT DISTINCT ON (host_id) run_id, host_id FROM os_update_results
			WHERE status = 'success' AND upgradable IS NOT NULL ORDER BY host_id, finished_at DESC) l
		  ON l.run_id = x.run_id AND l.host_id = x.host_id
		WHERE h.deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OSUpdateResult{}
	for rows.Next() {
		var x models.OSUpdateResult
		if err := scanOSResult(rows, &x); err != nil {
			return nil, err
		}
		x.Output = "" // the column only needs the counts
		out = append(out, x)
	}
	return out, rows.Err()
}
