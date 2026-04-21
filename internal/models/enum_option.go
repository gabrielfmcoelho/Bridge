package models

import (
	"database/sql"
	"fmt"
)

type EnumOption struct {
	Category  string `json:"category"`
	Value     string `json:"value"`
	SortOrder int    `json:"sort_order"`
	Color     string `json:"color"`
}

func ListEnumOptions(db *sql.DB, category string) ([]EnumOption, error) {
	rows, err := db.Query(
		`SELECT category, value, sort_order, color FROM enum_options WHERE category = ? ORDER BY sort_order, value`,
		category,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []EnumOption
	for rows.Next() {
		var o EnumOption
		if err := rows.Scan(&o.Category, &o.Value, &o.SortOrder, &o.Color); err != nil {
			return nil, err
		}
		options = append(options, o)
	}
	return options, rows.Err()
}

func ListAllEnumOptions(db *sql.DB) (map[string][]EnumOption, error) {
	rows, err := db.Query(`SELECT category, value, sort_order, color FROM enum_options ORDER BY category, sort_order, value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string][]EnumOption)
	for rows.Next() {
		var o EnumOption
		if err := rows.Scan(&o.Category, &o.Value, &o.SortOrder, &o.Color); err != nil {
			return nil, err
		}
		m[o.Category] = append(m[o.Category], o)
	}
	return m, rows.Err()
}

func CreateEnumOption(db *sql.DB, o *EnumOption) error {
	// Auto-assign sort_order as max+1 if not specified.
	if o.SortOrder == 0 {
		var maxOrder int
		if err := db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM enum_options WHERE category = ?`, o.Category).Scan(&maxOrder); err != nil {
			return fmt.Errorf("enum_options: read max sort_order for %q: %w", o.Category, err)
		}
		o.SortOrder = maxOrder + 1
	}
	// `ON CONFLICT DO NOTHING` without a target is supported by both SQLite
	// (3.24+) and Postgres. The older `INSERT OR IGNORE` syntax is SQLite-only
	// and caused silent 500s in production when adding enum options on Postgres.
	if _, err := db.Exec(
		`INSERT INTO enum_options (category, value, sort_order, color) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		o.Category, o.Value, o.SortOrder, o.Color,
	); err != nil {
		return fmt.Errorf("enum_options: insert %s=%q: %w", o.Category, o.Value, err)
	}
	return nil
}

func UpdateEnumOption(db *sql.DB, category, oldValue, newValue, color string) error {
	_, err := db.Exec(
		`UPDATE enum_options SET value = ?, color = ? WHERE category = ? AND value = ?`,
		newValue, color, category, oldValue,
	)
	return err
}

func DeleteEnumOption(db *sql.DB, category, value string) error {
	_, err := db.Exec(`DELETE FROM enum_options WHERE category = ? AND value = ?`, category, value)
	return err
}

func ListEnumCategories(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT category FROM enum_options ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}
