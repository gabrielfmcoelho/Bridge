package models

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// GlpiDropdownCatalogue is an admin-curated list of options for a single GLPI
// itemtype (ITILCategory, Entity, Location, Supplier, User, Group, …). Used
// by the Formcreator picker when the REST profile doesn't have rights to read
// the itemtype directly. The catalogue is the display source only — submissions
// still post integer ids to Formcreator, and those ids must match GLPI.
type GlpiDropdownCatalogue struct {
	ID          int64           `json:"id"`
	Itemtype    string          `json:"itemtype"`
	Options     json.RawMessage `json:"options"`
	OptionCount int             `json:"option_count"`
	UpdatedAt   time.Time       `json:"updated_at"`
	UpdatedBy   *int64          `json:"updated_by,omitempty"`
}

// GlpiDropdownCatalogueSummary is the list-view shape (no options body).
type GlpiDropdownCatalogueSummary struct {
	Itemtype    string    `json:"itemtype"`
	OptionCount int       `json:"option_count"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   *int64    `json:"updated_by,omitempty"`
}

const glpiDropdownCatalogueCols = `id, itemtype, options, option_count, updated_at, updated_by`

func scanGlpiDropdownCatalogue(scanner interface{ Scan(...any) error }, c *GlpiDropdownCatalogue) error {
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

// ListGlpiDropdownCatalogues returns every itemtype's summary — no options
// payload, since the list page only shows counts + timestamps.
func ListGlpiDropdownCatalogues(db *sql.DB) ([]GlpiDropdownCatalogueSummary, error) {
	rows, err := db.Query(`SELECT itemtype, option_count, updated_at, updated_by FROM glpi_dropdown_catalogues ORDER BY itemtype`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GlpiDropdownCatalogueSummary
	for rows.Next() {
		var s GlpiDropdownCatalogueSummary
		if err := rows.Scan(&s.Itemtype, &s.OptionCount, &s.UpdatedAt, &s.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetGlpiDropdownCatalogue returns a single itemtype's full catalogue, or nil
// when no row exists. Callers that need to differentiate "absent" from "empty
// options" check `c == nil` vs `len(c.Options) == 0`.
func GetGlpiDropdownCatalogue(db *sql.DB, itemtype string) (*GlpiDropdownCatalogue, error) {
	c := &GlpiDropdownCatalogue{}
	err := scanGlpiDropdownCatalogue(db.QueryRow(
		`SELECT `+glpiDropdownCatalogueCols+` FROM glpi_dropdown_catalogues WHERE itemtype = ?`,
		itemtype,
	), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpsertGlpiDropdownCatalogue writes the row for an itemtype, inserting if
// absent. The caller supplies pre-validated JSON bytes + the matching option
// count so the DB stays consistent with the payload.
func UpsertGlpiDropdownCatalogue(db *sql.DB, itemtype string, options []byte, count int, userID *int64) error {
	if len(options) == 0 {
		options = []byte("[]")
	}
	// Two-step upsert so we stay portable across SQLite's INSERT OR REPLACE
	// (which would reset auto-increment ids) and Postgres' ON CONFLICT. Just
	// try UPDATE first, fall through to INSERT when no rows matched.
	res, err := db.Exec(
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
	_, err = database.InsertReturningID(db,
		`INSERT INTO glpi_dropdown_catalogues (itemtype, options, option_count, updated_by)
		 VALUES (?, ?, ?, ?)`,
		itemtype, string(options), count, userID,
	)
	return err
}

// DeleteGlpiDropdownCatalogue drops the row for an itemtype. Returns no error
// when nothing was deleted — idempotent from the admin's POV.
func DeleteGlpiDropdownCatalogue(db *sql.DB, itemtype string) error {
	_, err := db.Exec(`DELETE FROM glpi_dropdown_catalogues WHERE itemtype = ?`, itemtype)
	return err
}
