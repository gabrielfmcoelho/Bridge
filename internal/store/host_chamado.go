package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostChamadoRepo owns SQL for host_chamados (tickets linked to a host, with
// cached external-source fields). Referenced by host_chamado/host/glpi handlers;
// built inline until the Phase 2 container hoists it.
type HostChamadoRepo struct {
	db *sql.DB
}

// NewHostChamadoRepo constructs a HostChamadoRepo over the given DB handle.
func NewHostChamadoRepo(db *sql.DB) *HostChamadoRepo { return &HostChamadoRepo{db: db} }

const chamadoCols = `hc.id, hc.host_id, hc.chamado_id, hc.title, hc.status, hc.user_id, COALESCE(u.display_name, '') AS user_display_name, hc.date,
	hc.external_source, hc.external_url, hc.cached_title, hc.cached_status, hc.cached_at`

func scanChamado(scanner interface{ Scan(...any) error }, c *models.HostChamado) error {
	return scanner.Scan(&c.ID, &c.HostID, &c.ChamadoID, &c.Title, &c.Status, &c.UserID, &c.UserDisplayName, &c.Date,
		&c.ExternalSource, &c.ExternalURL, &c.CachedTitle, &c.CachedStatus, &c.CachedAt,
	)
}

// ListByHost returns all chamados for a host, joined with the user display name.
func (r *HostChamadoRepo) ListByHost(ctx context.Context, hostID int64) ([]models.HostChamado, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+chamadoCols+`
		FROM host_chamados hc
		LEFT JOIN users u ON u.id = hc.user_id
		WHERE hc.host_id = ?
		ORDER BY hc.date DESC, hc.id DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.HostChamado
	for rows.Next() {
		var c models.HostChamado
		if err := scanChamado(rows, &c); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// Get returns a single chamado by id, or (nil, nil) if absent.
func (r *HostChamadoRepo) Get(ctx context.Context, id int64) (*models.HostChamado, error) {
	c := &models.HostChamado{}
	err := scanChamado(r.db.QueryRowContext(ctx, `
		SELECT `+chamadoCols+`
		FROM host_chamados hc
		LEFT JOIN users u ON u.id = hc.user_id
		WHERE hc.id = ?`, id), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// Create inserts a single chamado (defaulting status) and returns its id.
func (r *HostChamadoRepo) Create(ctx context.Context, hostID int64, inp *models.HostChamadoInput) (int64, error) {
	if inp.Status == "" {
		inp.Status = "in_execution"
	}
	return database.InsertReturningID(r.db,
		`INSERT INTO host_chamados (host_id, chamado_id, title, status, user_id, date) VALUES (?, ?, ?, ?, ?, ?)`,
		hostID, inp.ChamadoID, inp.Title, inp.Status, inp.UserID, inp.Date,
	)
}

// Update writes a single chamado's mutable fields by id.
func (r *HostChamadoRepo) Update(ctx context.Context, id int64, inp *models.HostChamadoInput) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE host_chamados SET chamado_id = ?, title = ?, status = ?, user_id = ?, date = ? WHERE id = ?`,
		inp.ChamadoID, inp.Title, inp.Status, inp.UserID, inp.Date, id,
	)
	return err
}

// Delete removes a single chamado by id.
func (r *HostChamadoRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM host_chamados WHERE id = ?`, id)
	return err
}

// UpdateCache refreshes the cached_* columns after a live external lookup.
func (r *HostChamadoRepo) UpdateCache(ctx context.Context, id int64, externalSource, externalURL, title, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE host_chamados SET external_source = ?, external_url = ?, cached_title = ?, cached_status = ?, cached_at = CURRENT_TIMESTAMP WHERE id = ?`,
		externalSource, externalURL, title, status, id,
	)
	return err
}

// CreateExternal inserts a chamado linked to an external system (e.g. a freshly
// created GLPI ticket), pre-populating the cache columns. Returns the id.
func (r *HostChamadoRepo) CreateExternal(ctx context.Context, hostID, userID int64, chamadoID, title, status, date, externalSource, externalURL string) (int64, error) {
	if status == "" {
		status = "in_execution"
	}
	return database.InsertReturningID(r.db,
		`INSERT INTO host_chamados
			(host_id, chamado_id, title, status, user_id, date, external_source, external_url, cached_title, cached_status, cached_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		hostID, chamadoID, title, status, userID, date, externalSource, externalURL, title, status,
	)
}

// Sync replaces all chamados for a host with the given inputs (one tx).
func (r *HostChamadoRepo) Sync(ctx context.Context, hostID int64, inputs []models.HostChamadoInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM host_chamados WHERE host_id = ?`, hostID); err != nil {
		return err
	}
	for _, inp := range inputs {
		status := inp.Status
		if status == "" {
			status = "in_execution"
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO host_chamados (host_id, chamado_id, title, status, user_id, date) VALUES (?, ?, ?, ?, ?, ?)`,
			hostID, inp.ChamadoID, inp.Title, status, inp.UserID, inp.Date,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CountsBulk returns host_id → number of chamados, for the host list view.
func (r *HostChamadoRepo) CountsBulk(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id, COUNT(*) FROM host_chamados GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	return scanCountMap(rows)
}
