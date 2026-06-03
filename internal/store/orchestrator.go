package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// OrchestratorRepo owns all SQL for the orchestrators table (one per host).
// It is referenced by several handlers (orchestrator, dashboard, host); until
// the Phase 2 DI container hoists construction, callers build it inline with
// NewOrchestratorRepo — cheap, since it only wraps a *sql.DB.
type OrchestratorRepo struct {
	db *sql.DB
}

// NewOrchestratorRepo constructs an OrchestratorRepo over the given DB handle.
func NewOrchestratorRepo(db *sql.DB) *OrchestratorRepo { return &OrchestratorRepo{db: db} }

const orchestratorCols = `id, host_id, type, version, observacoes, created_at, updated_at`

// List returns all orchestrators ordered by type.
func (r *OrchestratorRepo) List(ctx context.Context) ([]models.Orchestrator, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+orchestratorCols+` FROM orchestrators ORDER BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Orchestrator
	for rows.Next() {
		var o models.Orchestrator
		if err := rows.Scan(&o.ID, &o.HostID, &o.Type, &o.Version, &o.Observacoes, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Get returns the orchestrator with the given id, or (nil, nil) if absent.
func (r *OrchestratorRepo) Get(ctx context.Context, id int64) (*models.Orchestrator, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT `+orchestratorCols+` FROM orchestrators WHERE id = ?`, id))
}

// GetByHost returns the orchestrator attached to hostID, or (nil, nil).
func (r *OrchestratorRepo) GetByHost(ctx context.Context, hostID int64) (*models.Orchestrator, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT `+orchestratorCols+` FROM orchestrators WHERE host_id = ?`, hostID))
}

func (r *OrchestratorRepo) scanOne(row *sql.Row) (*models.Orchestrator, error) {
	o := &models.Orchestrator{}
	err := row.Scan(&o.ID, &o.HostID, &o.Type, &o.Version, &o.Observacoes, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

// Create inserts a new orchestrator and sets o.ID.
func (r *OrchestratorRepo) Create(ctx context.Context, o *models.Orchestrator) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO orchestrators (host_id, type, version, observacoes) VALUES (?, ?, ?, ?)`,
		o.HostID, o.Type, o.Version, o.Observacoes,
	)
	if err != nil {
		return err
	}
	o.ID = id
	return nil
}

// Update writes the mutable fields of an existing orchestrator by id.
func (r *OrchestratorRepo) Update(ctx context.Context, o *models.Orchestrator) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE orchestrators SET type = ?, version = ?, observacoes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		o.Type, o.Version, o.Observacoes, o.ID,
	)
	return err
}

// Delete removes an orchestrator by id.
func (r *OrchestratorRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM orchestrators WHERE id = ?`, id)
	return err
}

// Count returns the total number of orchestrators (dashboard metric).
func (r *OrchestratorRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orchestrators`).Scan(&n)
	return n, err
}
