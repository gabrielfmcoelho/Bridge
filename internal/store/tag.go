package store

import (
	"context"
	"database/sql"
	"sort"
)

// TagRepo owns SQL for the polymorphic tags table (entity_type, entity_id, tag).
// It is referenced by nearly every entity's handlers for tag enrichment; built
// inline until the Phase 2 container hoists it.
type TagRepo struct {
	db *sql.DB
}

// NewTagRepo constructs a TagRepo over the given DB handle.
func NewTagRepo(db *sql.DB) *TagRepo { return &TagRepo{db: db} }

// Set replaces all tags for an entity (one tx). Empty tags are skipped.
func (r *TagRepo) Set(ctx context.Context, entityType string, entityID int64, tags []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE entity_type = ? AND entity_id = ?`, entityType, entityID); err != nil {
		return err
	}
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tags (entity_type, entity_id, tag) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			entityType, entityID, tag); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get returns sorted tags for a single entity.
func (r *TagRepo) Get(ctx context.Context, entityType string, entityID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT tag FROM tags WHERE entity_type = ? AND entity_id = ? ORDER BY tag`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	return scanStrings(rows)
}

// GetAll returns entity_id -> sorted tags for a given entity type.
func (r *TagRepo) GetAll(ctx context.Context, entityType string) (map[int64][]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT entity_id, tag FROM tags WHERE entity_type = ? ORDER BY entity_id, tag`, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64][]string)
	for rows.Next() {
		var id int64
		var tag string
		if err := rows.Scan(&id, &tag); err != nil {
			return nil, err
		}
		m[id] = append(m[id], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for k := range m {
		sort.Strings(m[k])
	}
	return m, nil
}

// Distinct returns all unique tags for a given entity type, sorted.
func (r *TagRepo) Distinct(ctx context.Context, entityType string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT tag FROM tags WHERE entity_type = ? ORDER BY tag`, entityType)
	if err != nil {
		return nil, err
	}
	return scanStrings(rows)
}

// AllDistinct returns all unique tags across all entity types, sorted.
func (r *TagRepo) AllDistinct(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT tag FROM tags ORDER BY tag`)
	if err != nil {
		return nil, err
	}
	return scanStrings(rows)
}

// Delete removes all tags for an entity.
func (r *TagRepo) Delete(ctx context.Context, entityType string, entityID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	return err
}

// Add inserts a single tag (idempotent).
func (r *TagRepo) Add(ctx context.Context, entityType string, entityID int64, tag string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tags (entity_type, entity_id, tag) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		entityType, entityID, tag)
	return err
}

// Remove deletes a single tag.
func (r *TagRepo) Remove(ctx context.Context, entityType string, entityID int64, tag string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM tags WHERE entity_type = ? AND entity_id = ? AND tag = ?`, entityType, entityID, tag)
	return err
}

func scanStrings(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
