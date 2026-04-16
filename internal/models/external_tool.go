package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type ExternalTool struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	URL            string    `json:"url"`
	Icon           string    `json:"icon"`
	EmbedEnabled   bool      `json:"embed_enabled"`
	SortOrder      int       `json:"sort_order"`
	ServiceID      *int64    `json:"service_id"`
	DNSID          *int64    `json:"dns_id"`
	Source         string    `json:"source"`
	HasCredentials bool      `json:"has_credentials"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func CreateExternalTool(db *sql.DB, t *ExternalTool) error {
	if t.Source == "" {
		t.Source = "manual"
	}
	id, err := database.InsertReturningID(db,
		`INSERT INTO external_tools (name, description, url, icon, embed_enabled, sort_order, service_id, dns_id, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.Description, t.URL, t.Icon, t.EmbedEnabled, t.SortOrder, t.ServiceID, t.DNSID, t.Source,
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
		`SELECT t.id, t.name, t.description, t.url, t.icon, t.embed_enabled, t.sort_order,
			t.service_id, t.dns_id, t.source,
			(CASE WHEN t.service_id IS NOT NULL AND EXISTS (SELECT 1 FROM service_credentials sc WHERE sc.service_id = t.service_id) THEN 1 ELSE 0 END),
			t.created_at, t.updated_at
		FROM external_tools t WHERE t.id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.URL, &t.Icon, &t.EmbedEnabled, &t.SortOrder,
		&t.ServiceID, &t.DNSID, &t.Source, &t.HasCredentials, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func ListExternalTools(db *sql.DB) ([]ExternalTool, error) {
	rows, err := db.Query(
		`SELECT t.id, t.name, t.description, t.url, t.icon, t.embed_enabled, t.sort_order,
			t.service_id, t.dns_id, t.source,
			(CASE WHEN t.service_id IS NOT NULL AND EXISTS (SELECT 1 FROM service_credentials sc WHERE sc.service_id = t.service_id) THEN 1 ELSE 0 END),
			t.created_at, t.updated_at
		FROM external_tools t ORDER BY t.sort_order, t.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []ExternalTool
	for rows.Next() {
		var t ExternalTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.URL, &t.Icon, &t.EmbedEnabled, &t.SortOrder,
			&t.ServiceID, &t.DNSID, &t.Source, &t.HasCredentials, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func UpdateExternalTool(db *sql.DB, t *ExternalTool) error {
	_, err := db.Exec(
		`UPDATE external_tools SET name = ?, description = ?, url = ?, icon = ?, embed_enabled = ?, sort_order = ?,
			service_id = ?, dns_id = ?, source = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		t.Name, t.Description, t.URL, t.Icon, t.EmbedEnabled, t.SortOrder,
		t.ServiceID, t.DNSID, t.Source, t.ID,
	)
	return err
}

func DeleteExternalTool(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM external_tools WHERE id = ?`, id)
	return err
}

// GetToolByServiceAndDNS looks up an existing synced tool for a service+DNS pair.
func GetToolByServiceAndDNS(db *sql.DB, serviceID, dnsID int64) (*ExternalTool, error) {
	t := &ExternalTool{}
	err := db.QueryRow(
		`SELECT t.id, t.name, t.description, t.url, t.icon, t.embed_enabled, t.sort_order,
			t.service_id, t.dns_id, t.source, 0, t.created_at, t.updated_at
		FROM external_tools t WHERE t.service_id = ? AND t.dns_id = ?`, serviceID, dnsID,
	).Scan(&t.ID, &t.Name, &t.Description, &t.URL, &t.Icon, &t.EmbedEnabled, &t.SortOrder,
		&t.ServiceID, &t.DNSID, &t.Source, &t.HasCredentials, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}
