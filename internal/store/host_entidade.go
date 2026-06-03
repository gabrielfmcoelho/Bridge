package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostEntidadeRepo owns SQL for host_entidades (the host ↔ entidade junction;
// one row is the main entidade shown on the host card).
type HostEntidadeRepo struct {
	db *sql.DB
}

// NewHostEntidadeRepo constructs a HostEntidadeRepo over the given DB handle.
func NewHostEntidadeRepo(db *sql.DB) *HostEntidadeRepo { return &HostEntidadeRepo{db: db} }

// List returns all entidades linked to a host, main first.
func (r *HostEntidadeRepo) List(ctx context.Context, hostID int64) ([]models.HostEntidade, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, entidade, is_main
		FROM host_entidades
		WHERE host_id = ?
		ORDER BY is_main DESC, entidade ASC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.HostEntidade
	for rows.Next() {
		var e models.HostEntidade
		if err := rows.Scan(&e.ID, &e.HostID, &e.Entidade, &e.IsMain); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// MainBulk returns host_id → main entidade for every host in one query (avoids
// N+1 when rendering the host list).
func (r *HostEntidadeRepo) MainBulk(ctx context.Context) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id, entidade FROM host_entidades WHERE is_main`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]string)
	for rows.Next() {
		var hostID int64
		var entidade string
		if err := rows.Scan(&hostID, &entidade); err != nil {
			return nil, err
		}
		m[hostID] = entidade
	}
	return m, rows.Err()
}

// Sync replaces all entidades for a host with the given inputs, enforcing
// exactly one main (the first flagged, else the alphabetically-first row).
// Empty entidade strings are rejected.
func (r *HostEntidadeRepo) Sync(ctx context.Context, hostID int64, inputs []models.HostEntidadeInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM host_entidades WHERE host_id = ?`, hostID); err != nil {
		return err
	}

	mainSeen := false
	for _, inp := range inputs {
		if inp.Entidade == "" {
			return fmt.Errorf("entidade is required")
		}
		isMain := inp.IsMain && !mainSeen
		if isMain {
			mainSeen = true
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO host_entidades (host_id, entidade, is_main) VALUES (?, ?, ?)`,
			hostID, inp.Entidade, isMain,
		); err != nil {
			return err
		}
	}

	if !mainSeen && len(inputs) > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE host_entidades SET is_main = TRUE
			WHERE id = (SELECT id FROM host_entidades WHERE host_id = ? ORDER BY entidade ASC LIMIT 1)`,
			hostID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
