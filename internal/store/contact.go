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

const contactCols = `contacts.id, contacts.name, contacts.phone, contacts.email, contacts.role, contacts.entity, contacts.notes, contacts.is_external, contacts.deleted_at`

// contactLive keeps trashed contacts out of every live read.
const contactLive = "contacts.deleted_at IS NULL"

func scanContact(scanner interface{ Scan(...any) error }, c *models.Contact) error {
	return scanner.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Role, &c.Entity, &c.Notes, &c.IsExternal, &c.DeletedAt)
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

// ErrContactExists is returned by Create when (name, phone) is taken; the
// existing contact is reported so the caller can open or restore it — the
// new values never overwrite it.
type ErrContactExists struct {
	ID      int64
	Trashed bool
}

func (e ErrContactExists) Error() string { return "contact already exists" }

// Create inserts a contact and sets c.ID. A (name, phone) already in use,
// live or in the trash, returns ErrContactExists.
func (r *ContactRepo) Create(ctx context.Context, c *models.Contact) error {
	var existing int64
	var trashed bool
	err := r.db.QueryRowContext(ctx,
		`SELECT id, deleted_at IS NOT NULL FROM contacts WHERE name = ? AND phone = ?`, c.Name, c.Phone).Scan(&existing, &trashed)
	if err == nil {
		return ErrContactExists{ID: existing, Trashed: trashed}
	}
	if err != sql.ErrNoRows {
		return err
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO contacts (name, phone, email, role, entity, notes, is_external) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Phone, c.Email, c.Role, c.Entity, c.Notes, c.IsExternal,
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
		`UPDATE contacts SET name = ?, phone = ?, email = ?, role = ?, entity = ?, notes = ?, is_external = ? WHERE id = ? AND `+contactLive+` AND `+vis,
		append([]any{c.Name, c.Phone, c.Email, c.Role, c.Entity, c.Notes, c.IsExternal, c.ID}, vargs...)...,
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

// ContactUse is one asset a contact is responsável for.
type ContactUse struct {
	Type   string `json:"type"` // host | dns | service | project
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug,omitempty"` // hosts link by slug
	IsMain bool   `json:"is_main"`
}

// usageSources: per asset type, its table, display column and asset type for
// the entidade scope. Only live, visible assets count.
var usageSources = []struct {
	typ, table, name, slug string
	asset                  AssetType
}{
	{"host", "hosts", "nickname", "oficial_slug", AssetHost},
	{"dns", "dns_records", "domain", "", AssetDNS},
	{"service", "services", "nickname", "", AssetService},
	{"project", "projects", "name", "", AssetProject},
	{"api_catalog", "api_catalog", "name", "", AssetAPICatalog},
}

// Usage lists the assets contact id is responsável for that the caller sees.
func (r *ContactRepo) Usage(ctx context.Context, id int64) ([]ContactUse, error) {
	out := []ContactUse{}
	for _, src := range usageSources {
		vis, vargs := VisibleExpr(ctx, src.asset, "a.id")
		slug := "''"
		if src.slug != "" {
			slug = "a." + src.slug
		}
		rows, err := r.db.QueryContext(ctx,
			`SELECT a.id, a.`+src.name+`, `+slug+`, rs.is_main FROM responsaveis rs
			   JOIN `+src.table+` a ON a.id = rs.entity_id AND a.deleted_at IS NULL
			  WHERE rs.entity_type = ? AND rs.contact_id = ? AND `+vis+`
			  ORDER BY rs.is_main DESC, a.`+src.name,
			append([]any{src.typ, id}, vargs...)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			u := ContactUse{Type: src.typ}
			if err := rows.Scan(&u.ID, &u.Name, &u.Slug, &u.IsMain); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, u)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UsageCounts returns contact id → asset type → how many visible, live assets
// the contact is responsável for (the list's "Responsável por" column).
func (r *ContactRepo) UsageCounts(ctx context.Context) (map[int64]map[string]int, error) {
	out := map[int64]map[string]int{}
	for _, src := range usageSources {
		vis, vargs := VisibleExpr(ctx, src.asset, "a.id")
		rows, err := r.db.QueryContext(ctx,
			`SELECT rs.contact_id, COUNT(*) FROM responsaveis rs
			   JOIN `+src.table+` a ON a.id = rs.entity_id AND a.deleted_at IS NULL
			  WHERE rs.entity_type = ? AND `+vis+`
			  GROUP BY rs.contact_id`,
			append([]any{src.typ}, vargs...)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var cid int64
			var n int
			if err := rows.Scan(&cid, &n); err != nil {
				rows.Close()
				return nil, err
			}
			if out[cid] == nil {
				out[cid] = map[string]int{}
			}
			out[cid][src.typ] = n
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
