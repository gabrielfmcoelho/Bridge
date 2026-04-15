package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type ExternalTool struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	URL          string    `json:"url"`
	Icon         string    `json:"icon"`
	EmbedEnabled bool      `json:"embed_enabled"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func CreateExternalTool(db *sql.DB, t *ExternalTool) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO external_tools (name, description, url, icon, embed_enabled, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)`,
		t.Name, t.Description, t.URL, t.Icon, t.EmbedEnabled, t.SortOrder,
	)
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

func GetExternalTool(db *sql.DB, id int64) (*ExternalTool, error) {
	t := &ExternalTool{}
	err := db.QueryRow(
		`SELECT id, name, description, url, icon, embed_enabled, sort_order, created_at, updated_at
		FROM external_tools WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.URL, &t.Icon, &t.EmbedEnabled, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func ListExternalTools(db *sql.DB) ([]ExternalTool, error) {
	rows, err := db.Query(
		`SELECT id, name, description, url, icon, embed_enabled, sort_order, created_at, updated_at
		FROM external_tools ORDER BY sort_order, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []ExternalTool
	for rows.Next() {
		var t ExternalTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.URL, &t.Icon, &t.EmbedEnabled, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func UpdateExternalTool(db *sql.DB, t *ExternalTool) error {
	_, err := db.Exec(
		`UPDATE external_tools SET name = ?, description = ?, url = ?, icon = ?, embed_enabled = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		t.Name, t.Description, t.URL, t.Icon, t.EmbedEnabled, t.SortOrder, t.ID,
	)
	return err
}

func DeleteExternalTool(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM external_tools WHERE id = ?`, id)
	return err
}
