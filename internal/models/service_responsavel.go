package models

import (
	"database/sql"
	"fmt"
)

// ServiceResponsavel represents a junction between a service and a contact.
// is_external/notes/role/entity are joined from the contacts table.
type ServiceResponsavel struct {
	ID         int64  `json:"id"`
	ServiceID  int64  `json:"service_id"`
	ContactID  int64  `json:"contact_id"`
	IsMain     bool   `json:"is_main"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	Entity     string `json:"entity"`
	Notes      string `json:"notes"`
	IsExternal bool   `json:"is_external"`
}

// ServiceResponsavelInput requires an existing contact id; inline creation is not supported.
type ServiceResponsavelInput struct {
	ContactID int64 `json:"contact_id"`
	IsMain    bool  `json:"is_main"`
}

// ListServiceResponsaveis returns all responsaveis for a service, joined with contact details.
func ListServiceResponsaveis(db *sql.DB, serviceID int64) ([]ServiceResponsavel, error) {
	rows, err := db.Query(`
		SELECT sr.id, sr.service_id, sr.contact_id, sr.is_main,
		       c.name, c.phone, c.role, c.entity, c.notes, c.is_external
		FROM service_responsaveis sr
		JOIN contacts c ON c.id = sr.contact_id
		WHERE sr.service_id = ?
		ORDER BY sr.is_main DESC, c.name ASC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ServiceResponsavel
	for rows.Next() {
		var r ServiceResponsavel
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.ContactID, &r.IsMain,
			&r.Name, &r.Phone, &r.Role, &r.Entity, &r.Notes, &r.IsExternal); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetServiceMainResponsavelNamesBulk returns main responsavel names for all services.
func GetServiceMainResponsavelNamesBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT sr.service_id, c.name FROM service_responsaveis sr
		JOIN contacts c ON c.id = sr.contact_id
		WHERE sr.is_main AND NOT c.is_external`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]string)
	for rows.Next() {
		var serviceID int64
		var name string
		if err := rows.Scan(&serviceID, &name); err != nil {
			return nil, err
		}
		m[serviceID] = name
	}
	return m, rows.Err()
}

// SyncServiceResponsaveis replaces all responsaveis for a service. Each input
// must reference an existing contact.
func SyncServiceResponsaveis(db *sql.DB, serviceID int64, inputs []ServiceResponsavelInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM service_responsaveis WHERE service_id = ?`, serviceID); err != nil {
		return err
	}

	for _, inp := range inputs {
		if inp.ContactID <= 0 {
			return fmt.Errorf("contact_id is required for each responsavel")
		}
		if _, err := tx.Exec(
			`INSERT INTO service_responsaveis (service_id, contact_id, is_main) VALUES (?, ?, ?)`,
			serviceID, inp.ContactID, inp.IsMain,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
