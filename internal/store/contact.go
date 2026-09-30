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

// DB exposes the underlying handle so the contact handler (which holds only
// this repo) can drive the entidade-grants repo for create/update.
func (r *ContactRepo) DB() *sql.DB { return r.db }

const contactCols = `contacts.id, contacts.name, contacts.phone, contacts.role, contacts.entity, contacts.notes, contacts.is_external, contacts.deleted_at`

// contactLive keeps trashed contacts out of every live read.
const contactLive = "contacts.deleted_at IS NULL"

func scanContact(scanner interface{ Scan(...any) error }, c *models.Contact) error {
	return scanner.Scan(&c.ID, &c.Name, &c.Phone, &c.Role, &c.Entity, &c.Notes, &c.IsExternal, &c.DeletedAt)
}

// List returns all visible contacts ordered by name.
func (r *ContactRepo) List(ctx context.Context) ([]models.Contact, error) {
	vis, vargs := VisibleExpr(ctx, AssetContact, "contacts.id")
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+contactCols+` FROM contacts WHERE `+contactLive+` AND `+vis+` ORDER BY contacts.name`, vargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []models.Contact
	for rows.Next() {
		var c models.Contact
		if err := scanContact(rows, &c); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// Get returns a visible contact by id, or (nil, nil) if absent/invisible.
func (r *ContactRepo) Get(ctx context.Context, id int64) (*models.Contact, error) {
	vis, vargs := VisibleExpr(ctx, AssetContact, "contacts.id")
	c := &models.Contact{}
	err := scanContact(r.db.QueryRowContext(ctx,
		`SELECT `+contactCols+` FROM contacts WHERE contacts.id = ? AND `+contactLive+` AND `+vis, append([]any{id}, vargs...)...), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// Create upserts a contact on (name, phone). On conflict it returns the
// existing row's id so callers can reliably continue — and takes a trashed
// one out of the trash: adding the same person again means they're back. The dummy
// `SET name = EXCLUDED.name` is portable between SQLite and Postgres and
// guarantees RETURNING id yields a row even on conflict.
func (r *ContactRepo) Create(ctx context.Context, c *models.Contact) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO contacts (name, phone, role, entity, notes, is_external) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name, phone) DO UPDATE SET name = EXCLUDED.name, deleted_at = NULL`,
		c.Name, c.Phone, c.Role, c.Entity, c.Notes, c.IsExternal,
	)
	if err != nil {
		return err
	}
	c.ID = id
	return nil
}

// Update writes the mutable fields of an existing visible contact by id.
func (r *ContactRepo) Update(ctx context.Context, c *models.Contact) error {
	vis, vargs := VisibleExpr(ctx, AssetContact, "contacts.id")
	_, err := r.db.ExecContext(ctx,
		`UPDATE contacts SET name = ?, phone = ?, role = ?, entity = ?, notes = ?, is_external = ? WHERE id = ? AND `+contactLive+` AND `+vis,
		append([]any{c.Name, c.Phone, c.Role, c.Entity, c.Notes, c.IsExternal, c.ID}, vargs...)...,
	)
	return err
}

// Delete moves a visible contact to the trash; the entities it's responsável
// for keep the link, hidden until a restore.
func (r *ContactRepo) Delete(ctx context.Context, id int64) error {
	vis, vargs := VisibleExpr(ctx, AssetContact, "contacts.id")
	_, err := r.db.ExecContext(ctx,
		`UPDATE contacts SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND `+contactLive+` AND `+vis, append([]any{id}, vargs...)...)
	return err
}

// ListTrash returns the visible contacts in the trash, latest first.
func (r *ContactRepo) ListTrash(ctx context.Context) ([]models.Contact, error) {
	vis, vargs := VisibleExpr(ctx, AssetContact, "contacts.id")
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+contactCols+` FROM contacts WHERE contacts.deleted_at IS NOT NULL AND `+vis+` ORDER BY contacts.deleted_at DESC`, vargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Contact{}
	for rows.Next() {
		var c models.Contact
		if err := scanContact(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Restore takes a visible contact out of the trash; false when there was none.
func (r *ContactRepo) Restore(ctx context.Context, id int64) (bool, error) {
	vis, vargs := VisibleExpr(ctx, AssetContact, "contacts.id")
	res, err := r.db.ExecContext(ctx,
		`UPDATE contacts SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL AND `+vis, append([]any{id}, vargs...)...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
