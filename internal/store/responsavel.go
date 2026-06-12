package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ResponsavelRepo owns SQL for the polymorphic `responsaveis` table (the R3
// unification of the old host/dns/service/project_responsaveis junctions).
// entity_type is one of host/dns/service/project — the same vocabulary the
// `tags` table uses.
type ResponsavelRepo struct {
	db *sql.DB
}

// NewResponsavelRepo constructs a ResponsavelRepo over the given DB handle.
func NewResponsavelRepo(db *sql.DB) *ResponsavelRepo { return &ResponsavelRepo{db: db} }

// List returns an entity's responsáveis joined with contact details, ordered
// main-first then by contact name.
func (r *ResponsavelRepo) List(ctx context.Context, entityType string, entityID int64) ([]models.Responsavel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rs.id, rs.contact_id, rs.is_main,
		       c.name, c.phone, c.role, c.entity, c.notes, c.is_external
		FROM responsaveis rs
		JOIN contacts c ON c.id = rs.contact_id
		WHERE rs.entity_type = ? AND rs.entity_id = ?
		ORDER BY rs.is_main DESC, c.name ASC`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Responsavel
	for rows.Next() {
		var x models.Responsavel
		if err := rows.Scan(&x.ID, &x.ContactID, &x.IsMain,
			&x.Name, &x.Phone, &x.Role, &x.Entity, &x.Notes, &x.IsExternal); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// MainNamesBulk returns entity_id → main *internal* responsável name for every
// entity of the given type (is_main AND the contact is not external) — the
// bulk lookup the list views use to label rows.
func (r *ResponsavelRepo) MainNamesBulk(ctx context.Context, entityType string) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rs.entity_id, c.name
		FROM responsaveis rs
		JOIN contacts c ON c.id = rs.contact_id
		WHERE rs.entity_type = ? AND rs.is_main AND NOT c.is_external`, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]string)
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		m[id] = name
	}
	return m, rows.Err()
}

// Sync replaces all responsáveis for an entity with the given inputs (one tx).
// Each input must reference an existing contact (ContactID > 0).
func (r *ResponsavelRepo) Sync(ctx context.Context, entityType string, entityID int64, inputs []models.ResponsavelInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM responsaveis WHERE entity_type = ? AND entity_id = ?`, entityType, entityID); err != nil {
		return err
	}
	for _, inp := range inputs {
		if inp.ContactID <= 0 {
			return fmt.Errorf("contact_id is required for each responsavel")
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO responsaveis (entity_type, entity_id, contact_id, is_main) VALUES (?, ?, ?, ?)`,
			entityType, entityID, inp.ContactID, inp.IsMain,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
