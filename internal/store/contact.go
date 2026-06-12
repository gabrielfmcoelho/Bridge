package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ContactRepo owns all SQL for the contacts table. It is the reference
// implementation of the entity-repository pattern: the model (models.Contact)
// is a pure data type, every query lives here, and dialect handling goes
// through the database package helpers.
type ContactRepo struct {
	db *sql.DB
}

// NewContactRepo constructs a ContactRepo over the given DB handle.
func NewContactRepo(db *sql.DB) *ContactRepo { return &ContactRepo{db: db} }

// List returns all contacts ordered by name.
func (r *ContactRepo) List(ctx context.Context) ([]models.Contact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, phone, role, entity, notes, is_external FROM contacts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []models.Contact
	for rows.Next() {
		var c models.Contact
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Role, &c.Entity, &c.Notes, &c.IsExternal); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// Create upserts a contact on (name, phone). On conflict it returns the
// existing row's id so callers can reliably continue. The dummy
// `SET name = EXCLUDED.name` is portable between SQLite and Postgres and
// guarantees RETURNING id yields a row even on conflict.
func (r *ContactRepo) Create(ctx context.Context, c *models.Contact) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO contacts (name, phone, role, entity, notes, is_external) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name, phone) DO UPDATE SET name = EXCLUDED.name`,
		c.Name, c.Phone, c.Role, c.Entity, c.Notes, c.IsExternal,
	)
	if err != nil {
		return err
	}
	c.ID = id
	return nil
}

// Update writes the mutable fields of an existing contact by id.
func (r *ContactRepo) Update(ctx context.Context, c *models.Contact) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE contacts SET name = ?, phone = ?, role = ?, entity = ?, notes = ?, is_external = ? WHERE id = ?`,
		c.Name, c.Phone, c.Role, c.Entity, c.Notes, c.IsExternal, c.ID,
	)
	return err
}

// Delete removes a contact by id.
func (r *ContactRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM contacts WHERE id = ?`, id)
	return err
}
