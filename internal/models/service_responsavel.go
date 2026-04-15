package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// ServiceResponsavel represents a junction between a service and a contact.
type ServiceResponsavel struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	Entity    string `json:"entity"`
}

// ServiceResponsavelInput is the input for creating/syncing service responsaveis.
type ServiceResponsavelInput struct {
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	Entity    string `json:"entity"`
}

// ListServiceResponsaveis returns all responsaveis for a service, joined with contact details.
func ListServiceResponsaveis(db *sql.DB, serviceID int64) ([]ServiceResponsavel, error) {
	rows, err := db.Query(`
		SELECT sr.id, sr.service_id, sr.contact_id, sr.is_main, sr.is_externo,
		       c.name, c.phone, c.role, c.entity
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
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.ContactID, &r.IsMain, &r.IsExterno,
			&r.Name, &r.Phone, &r.Role, &r.Entity); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetServiceMainResponsavelNamesBulk returns main responsavel names for all services in a single query.
func GetServiceMainResponsavelNamesBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT sr.service_id, c.name FROM service_responsaveis sr
		JOIN contacts c ON c.id = sr.contact_id
		WHERE sr.is_main AND NOT sr.is_externo`)
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

// SyncServiceResponsaveis replaces all responsaveis for a service with the given inputs.
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
		contactID := inp.ContactID

		if contactID == 0 && inp.Name != "" {
			err := tx.QueryRow(
				`SELECT id FROM contacts WHERE name = ? AND phone = ?`,
				inp.Name, inp.Phone,
			).Scan(&contactID)

			if err == sql.ErrNoRows {
				newID, insertErr := database.InsertReturningID(tx,
					`INSERT INTO contacts (name, phone, role, entity) VALUES (?, ?, ?, ?)`,
					inp.Name, inp.Phone, inp.Role, inp.Entity,
				)
				if insertErr != nil {
					return insertErr
				}
				contactID = newID
			} else if err != nil {
				return err
			} else {
				if inp.Role != "" || inp.Entity != "" {
					if _, err := tx.Exec(
						`UPDATE contacts SET role = ?, entity = ? WHERE id = ?`,
						inp.Role, inp.Entity, contactID,
					); err != nil {
						return err
					}
				}
			}
		}

		if contactID == 0 {
			continue
		}

		if _, err := tx.Exec(
			`INSERT INTO service_responsaveis (service_id, contact_id, is_main, is_externo) VALUES (?, ?, ?, ?)`,
			serviceID, contactID, inp.IsMain, inp.IsExterno,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
