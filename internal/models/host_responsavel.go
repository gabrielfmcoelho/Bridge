package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// HostResponsavel represents a junction between a host and a contact (responsavel).
type HostResponsavel struct {
	ID        int64  `json:"id"`
	HostID    int64  `json:"host_id"`
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	// Joined from contacts:
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
	Entity string `json:"entity"`
}

// HostResponsavelInput is the input for creating/syncing host responsaveis.
type HostResponsavelInput struct {
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	// For creating new contacts inline:
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
	Entity string `json:"entity"`
}

// ListHostResponsaveis returns all responsaveis for a host, joined with contact details.
func ListHostResponsaveis(db *sql.DB, hostID int64) ([]HostResponsavel, error) {
	rows, err := db.Query(`
		SELECT hr.id, hr.host_id, hr.contact_id, hr.is_main, hr.is_externo,
		       c.name, c.phone, c.role, c.entity
		FROM host_responsaveis hr
		JOIN contacts c ON c.id = hr.contact_id
		WHERE hr.host_id = ?
		ORDER BY hr.is_main DESC, c.name ASC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HostResponsavel
	for rows.Next() {
		var r HostResponsavel
		if err := rows.Scan(&r.ID, &r.HostID, &r.ContactID, &r.IsMain, &r.IsExterno,
			&r.Name, &r.Phone, &r.Role, &r.Entity); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetMainResponsavelName returns the name of the main (is_main=1) internal responsavel for a host.
func GetMainResponsavelName(db *sql.DB, hostID int64) string {
	var name string
	_ = db.QueryRow(`
		SELECT c.name FROM host_responsaveis hr
		JOIN contacts c ON c.id = hr.contact_id
		WHERE hr.host_id = ? AND hr.is_main AND NOT hr.is_externo
		LIMIT 1`, hostID).Scan(&name)
	return name
}

// GetChamadosCount returns the number of chamados for a host.
func GetChamadosCount(db *sql.DB, hostID int64) int {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM host_chamados WHERE host_id = ?`, hostID).Scan(&count)
	return count
}

// GetMainResponsavelNamesBulk returns main responsavel names for all hosts in a single query.
func GetMainResponsavelNamesBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT hr.host_id, c.name FROM host_responsaveis hr
		JOIN contacts c ON c.id = hr.contact_id
		WHERE hr.is_main AND NOT hr.is_externo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]string)
	for rows.Next() {
		var hostID int64
		var name string
		if err := rows.Scan(&hostID, &name); err != nil {
			return nil, err
		}
		m[hostID] = name
	}
	return m, rows.Err()
}

// GetChamadosCountsBulk returns chamados counts for all hosts in a single query.
func GetChamadosCountsBulk(db *sql.DB) (map[int64]int, error) {
	rows, err := db.Query(`SELECT host_id, COUNT(*) FROM host_chamados GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]int)
	for rows.Next() {
		var hostID int64
		var count int
		if err := rows.Scan(&hostID, &count); err != nil {
			return nil, err
		}
		m[hostID] = count
	}
	return m, rows.Err()
}

// SyncHostResponsaveis replaces all responsaveis for a host with the given inputs.
// If an input has ContactID=0 and a Name, a new contact is created.
func SyncHostResponsaveis(db *sql.DB, hostID int64, inputs []HostResponsavelInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing
	if _, err := tx.Exec(`DELETE FROM host_responsaveis WHERE host_id = ?`, hostID); err != nil {
		return err
	}

	for _, inp := range inputs {
		contactID := inp.ContactID

		if contactID == 0 && inp.Name != "" {
			// Try to find existing contact by name+phone
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
				// Update existing contact's role/entity if provided
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
			`INSERT INTO host_responsaveis (host_id, contact_id, is_main, is_externo) VALUES (?, ?, ?, ?)`,
			hostID, contactID, inp.IsMain, inp.IsExterno,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
