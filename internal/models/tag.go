package models

import (
	"database/sql"
	"sort"
)

// SetTags replaces all tags for a given entity.
func SetTags(db *sql.DB, entityType string, entityID int64, tags []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM tags WHERE entity_type = ? AND entity_id = ?`, entityType, entityID); err != nil {
		return err
	}
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO tags (entity_type, entity_id, tag) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			entityType, entityID, tag,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetTags returns sorted tags for a single entity.
func GetTags(db *sql.DB, entityType string, entityID int64) ([]string, error) {
	rows, err := db.Query(
		`SELECT tag FROM tags WHERE entity_type = ? AND entity_id = ? ORDER BY tag`,
		entityType, entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// GetAllTags returns a map of entityID -> sorted tags for a given entity type.
func GetAllTags(db *sql.DB, entityType string) (map[int64][]string, error) {
	rows, err := db.Query(
		`SELECT entity_id, tag FROM tags WHERE entity_type = ? ORDER BY entity_id, tag`,
		entityType,
	)
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

// GetDistinctTags returns all unique tags for a given entity type, sorted.
func GetDistinctTags(db *sql.DB, entityType string) ([]string, error) {
	rows, err := db.Query(
		`SELECT DISTINCT tag FROM tags WHERE entity_type = ? ORDER BY tag`,
		entityType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// GetAllDistinctTags returns all unique tags across all entity types, sorted.
func GetAllDistinctTags(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT tag FROM tags ORDER BY tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// DeleteTags removes all tags for a given entity.
func DeleteTags(db *sql.DB, entityType string, entityID int64) error {
	_, err := db.Exec(`DELETE FROM tags WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	return err
}

func AddTag(db *sql.DB, entityType string, entityID int64, tag string) error {
	_, err := db.Exec(
		`INSERT INTO tags (entity_type, entity_id, tag) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		entityType, entityID, tag,
	)
	return err
}

func RemoveTag(db *sql.DB, entityType string, entityID int64, tag string) error {
	_, err := db.Exec(
		`DELETE FROM tags WHERE entity_type = ? AND entity_id = ? AND tag = ?`,
		entityType, entityID, tag,
	)
	return err
}
