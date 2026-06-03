package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostAlertRepo owns SQL for host_alerts and the issue_alert_links read paths.
// Referenced by alert/host/grafana-webhook/issue handlers; built inline until
// the Phase 2 container hoists it.
type HostAlertRepo struct {
	db *sql.DB
}

// NewHostAlertRepo constructs a HostAlertRepo over the given DB handle.
func NewHostAlertRepo(db *sql.DB) *HostAlertRepo { return &HostAlertRepo{db: db} }

const hostAlertCols = `id, host_id, type, level, message, description, source, status, external_id, external_source, created_at, updated_at`

func scanHostAlert(scanner interface{ Scan(...any) error }, a *models.HostAlert) error {
	return scanner.Scan(&a.ID, &a.HostID, &a.Type, &a.Level, &a.Message, &a.Description, &a.Source, &a.Status, &a.ExternalID, &a.ExternalSource, &a.CreatedAt, &a.UpdatedAt)
}

// Create inserts a manual alert (defaulting source) and sets a.ID.
func (r *HostAlertRepo) Create(ctx context.Context, a *models.HostAlert) error {
	if a.Source == "" {
		a.Source = "manual"
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO host_alerts (host_id, type, level, message, description, source) VALUES (?, ?, ?, ?, ?, ?)`,
		a.HostID, a.Type, a.Level, a.Message, a.Description, a.Source,
	)
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

// Get returns an alert by id, or (nil, nil) if absent.
func (r *HostAlertRepo) Get(ctx context.Context, id int64) (*models.HostAlert, error) {
	a := &models.HostAlert{}
	err := scanHostAlert(r.db.QueryRowContext(ctx, `SELECT `+hostAlertCols+` FROM host_alerts WHERE id = ?`, id), a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// ListByHost returns a host's alerts, newest first.
func (r *HostAlertRepo) ListByHost(ctx context.Context, hostID int64) ([]models.HostAlert, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+hostAlertCols+` FROM host_alerts WHERE host_id = ? ORDER BY created_at DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []models.HostAlert
	for rows.Next() {
		var a models.HostAlert
		if err := scanHostAlert(rows, &a); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// ListBulk returns host_id -> alerts for all hosts (newest first per host).
func (r *HostAlertRepo) ListBulk(ctx context.Context) (map[int64][]models.HostAlert, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+hostAlertCols+` FROM host_alerts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64][]models.HostAlert)
	for rows.Next() {
		var a models.HostAlert
		if err := scanHostAlert(rows, &a); err != nil {
			return nil, err
		}
		m[a.HostID] = append(m[a.HostID], a)
	}
	return m, rows.Err()
}

// Update writes the editable fields of a manual alert by id.
func (r *HostAlertRepo) Update(ctx context.Context, a *models.HostAlert) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE host_alerts SET type = ?, level = ?, message = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		a.Type, a.Level, a.Message, a.Description, a.ID,
	)
	return err
}

// Delete removes an alert by id.
func (r *HostAlertRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM host_alerts WHERE id = ?`, id)
	return err
}

// GetExternal returns the alert keyed by (external_source, external_id), or
// (nil, nil) if none.
func (r *HostAlertRepo) GetExternal(ctx context.Context, source, externalID string) (*models.HostAlert, error) {
	a := &models.HostAlert{}
	err := scanHostAlert(r.db.QueryRowContext(ctx,
		`SELECT `+hostAlertCols+` FROM host_alerts WHERE external_source = ? AND external_id = ? LIMIT 1`,
		source, externalID), a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// UpsertExternal creates or updates an externally-sourced alert keyed by
// (external_source, external_id) — used by webhook ingestion to avoid
// duplicates. Returns the resulting alert with ID set.
func (r *HostAlertRepo) UpsertExternal(ctx context.Context, a *models.HostAlert) (*models.HostAlert, error) {
	if a.ExternalSource == "" || a.ExternalID == "" {
		return nil, sql.ErrNoRows
	}
	existing, err := r.GetExternal(ctx, a.ExternalSource, a.ExternalID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		_, err := r.db.ExecContext(ctx,
			`UPDATE host_alerts SET host_id = ?, type = ?, level = ?, message = ?, description = ?,
				source = ?, status = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`,
			a.HostID, a.Type, a.Level, a.Message, a.Description, a.Source, a.Status, existing.ID,
		)
		if err != nil {
			return nil, err
		}
		a.ID = existing.ID
		return a, nil
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO host_alerts (host_id, type, level, message, description, source, status, external_id, external_source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.HostID, a.Type, a.Level, a.Message, a.Description, a.Source, a.Status, a.ExternalID, a.ExternalSource,
	)
	if err != nil {
		return nil, err
	}
	a.ID = id
	return a, nil
}

// Resolve marks an alert resolved.
func (r *HostAlertRepo) Resolve(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE host_alerts SET status = 'resolved', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// ResolveByIssue resolves all alerts linked to a given issue.
func (r *HostAlertRepo) ResolveByIssue(ctx context.Context, issueID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE host_alerts SET status = 'resolved', updated_at = CURRENT_TIMESTAMP
		 WHERE id IN (SELECT alert_id FROM issue_alert_links WHERE issue_id = ?)`, issueID)
	return err
}

// LinkedIssueIDsByHost returns alert_id -> first linked issue_id for a host's alerts.
func (r *HostAlertRepo) LinkedIssueIDsByHost(ctx context.Context, hostID int64) (map[int64]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.alert_id, l.issue_id FROM issue_alert_links l
		 JOIN host_alerts a ON a.id = l.alert_id
		 WHERE a.host_id = ?`, hostID)
	if err != nil {
		return nil, err
	}
	return scanAlertIssueMap(rows)
}

// LinkedIssueIDsBulk returns alert_id -> first linked issue_id for ALL alerts.
func (r *HostAlertRepo) LinkedIssueIDsBulk(ctx context.Context) (map[int64]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT alert_id, issue_id FROM issue_alert_links`)
	if err != nil {
		return nil, err
	}
	return scanAlertIssueMap(rows)
}

func scanAlertIssueMap(rows *sql.Rows) (map[int64]int64, error) {
	defer rows.Close()
	m := make(map[int64]int64)
	for rows.Next() {
		var alertID, issueID int64
		if err := rows.Scan(&alertID, &issueID); err != nil {
			return nil, err
		}
		if _, exists := m[alertID]; !exists {
			m[alertID] = issueID
		}
	}
	return m, rows.Err()
}
