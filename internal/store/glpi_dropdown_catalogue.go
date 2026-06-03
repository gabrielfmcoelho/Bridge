package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// GlpiDropdownCatalogueRepo owns SQL for glpi_dropdown_catalogues (admin-curated
// option lists per GLPI itemtype, used by the Formcreator picker).
type GlpiDropdownCatalogueRepo struct {
	db *sql.DB
}

// NewGlpiDropdownCatalogueRepo constructs the repo over the given DB handle.
func NewGlpiDropdownCatalogueRepo(db *sql.DB) *GlpiDropdownCatalogueRepo {
	return &GlpiDropdownCatalogueRepo{db: db}
}

const glpiDropdownCatalogueCols = `id, itemtype, options, option_count, updated_at, updated_by`

func scanGlpiDropdownCatalogue(scanner interface{ Scan(...any) error }, c *models.GlpiDropdownCatalogue) error {
	var opts []byte
	if err := scanner.Scan(&c.ID, &c.Itemtype, &opts, &c.OptionCount, &c.UpdatedAt, &c.UpdatedBy); err != nil {
		return err
	}
	if len(opts) == 0 {
		c.Options = json.RawMessage("[]")
	} else {
		c.Options = json.RawMessage(opts)
	}
	return nil
}

// List returns every itemtype's summary (no options payload).
func (r *GlpiDropdownCatalogueRepo) List(ctx context.Context) ([]models.GlpiDropdownCatalogueSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT itemtype, option_count, updated_at, updated_by FROM glpi_dropdown_catalogues ORDER BY itemtype`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.GlpiDropdownCatalogueSummary
	for rows.Next() {
		var s models.GlpiDropdownCatalogueSummary
		if err := rows.Scan(&s.Itemtype, &s.OptionCount, &s.UpdatedAt, &s.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get returns a single itemtype's full catalogue, or (nil, nil) when absent.
func (r *GlpiDropdownCatalogueRepo) Get(ctx context.Context, itemtype string) (*models.GlpiDropdownCatalogue, error) {
	c := &models.GlpiDropdownCatalogue{}
	err := scanGlpiDropdownCatalogue(r.db.QueryRowContext(ctx,
		`SELECT `+glpiDropdownCatalogueCols+` FROM glpi_dropdown_catalogues WHERE itemtype = ?`, itemtype), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Upsert writes the row for an itemtype, inserting if absent. Two-step
// (UPDATE then INSERT) to stay portable across SQLite/Postgres without
// resetting auto-increment ids.
func (r *GlpiDropdownCatalogueRepo) Upsert(ctx context.Context, itemtype string, options []byte, count int, userID *int64) error {
	if len(options) == 0 {
		options = []byte("[]")
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE glpi_dropdown_catalogues
		    SET options = ?, option_count = ?, updated_at = CURRENT_TIMESTAMP, updated_by = ?
		  WHERE itemtype = ?`,
		string(options), count, userID, itemtype,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	_, err = database.InsertReturningID(r.db,
		`INSERT INTO glpi_dropdown_catalogues (itemtype, options, option_count, updated_by)
		 VALUES (?, ?, ?, ?)`,
		itemtype, string(options), count, userID,
	)
	return err
}

// Delete drops the row for an itemtype (idempotent — no error when absent).
func (r *GlpiDropdownCatalogueRepo) Delete(ctx context.Context, itemtype string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM glpi_dropdown_catalogues WHERE itemtype = ?`, itemtype)
	return err
}
