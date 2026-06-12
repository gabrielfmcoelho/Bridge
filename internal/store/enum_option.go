package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// EnumOptionRepo owns all SQL for the enum_options table.
type EnumOptionRepo struct {
	db *sql.DB
}

// NewEnumOptionRepo constructs an EnumOptionRepo over the given DB handle.
func NewEnumOptionRepo(db *sql.DB) *EnumOptionRepo { return &EnumOptionRepo{db: db} }

// List returns the options for a single category, ordered by sort_order/value.
func (r *EnumOptionRepo) List(ctx context.Context, category string) ([]models.EnumOption, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT category, value, sort_order, color FROM enum_options WHERE category = ? ORDER BY sort_order, value`,
		category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.EnumOption
	for rows.Next() {
		var o models.EnumOption
		if err := rows.Scan(&o.Category, &o.Value, &o.SortOrder, &o.Color); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListAll returns every option grouped by category.
func (r *EnumOptionRepo) ListAll(ctx context.Context) (map[string][]models.EnumOption, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT category, value, sort_order, color FROM enum_options ORDER BY category, sort_order, value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string][]models.EnumOption)
	for rows.Next() {
		var o models.EnumOption
		if err := rows.Scan(&o.Category, &o.Value, &o.SortOrder, &o.Color); err != nil {
			return nil, err
		}
		m[o.Category] = append(m[o.Category], o)
	}
	return m, rows.Err()
}

// Create inserts an option, auto-assigning sort_order as max+1 within the
// category when unspecified. ON CONFLICT DO NOTHING is portable across SQLite
// (3.24+) and Postgres.
func (r *EnumOptionRepo) Create(ctx context.Context, o *models.EnumOption) error {
	if o.SortOrder == 0 {
		var maxOrder int
		if err := r.db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(sort_order), -1) FROM enum_options WHERE category = ?`, o.Category).Scan(&maxOrder); err != nil {
			return fmt.Errorf("enum_options: read max sort_order for %q: %w", o.Category, err)
		}
		o.SortOrder = maxOrder + 1
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO enum_options (category, value, sort_order, color) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		o.Category, o.Value, o.SortOrder, o.Color); err != nil {
		return fmt.Errorf("enum_options: insert %s=%q: %w", o.Category, o.Value, err)
	}
	return nil
}

// Update renames/recolors an option identified by (category, oldValue).
func (r *EnumOptionRepo) Update(ctx context.Context, category, oldValue, newValue, color string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE enum_options SET value = ?, color = ? WHERE category = ? AND value = ?`,
		newValue, color, category, oldValue)
	return err
}

// Delete removes an option identified by (category, value).
func (r *EnumOptionRepo) Delete(ctx context.Context, category, value string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM enum_options WHERE category = ? AND value = ?`, category, value)
	return err
}
