package models

import (
	"database/sql"
	"fmt"
)

// HostResponsavel represents a junction between a host and a contact (responsavel).
// is_external/notes/role/entity are joined from the contacts table — they are
// intrinsic to the contact, not to the host link.
type HostResponsavel struct {
	ID         int64  `json:"id"`
	HostID     int64  `json:"host_id"`
	ContactID  int64  `json:"contact_id"`
	IsMain     bool   `json:"is_main"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	Entity     string `json:"entity"`
	Notes      string `json:"notes"`
	IsExternal bool   `json:"is_external"`
}

// HostResponsavelInput is the input for syncing host responsaveis.
// Hosts can only link to existing contacts — contact creation is a separate
// operation against /api/contacts. ContactID is required.
type HostResponsavelInput struct {
	ContactID int64 `json:"contact_id"`
	IsMain    bool  `json:"is_main"`
}

// ListHostResponsaveis returns all responsaveis for a host, joined with contact details.
func ListHostResponsaveis(db *sql.DB, hostID int64) ([]HostResponsavel, error) {
	rows, err := db.Query(`
		SELECT hr.id, hr.host_id, hr.contact_id, hr.is_main,
		       c.name, c.phone, c.role, c.entity, c.notes, c.is_external
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
		if err := rows.Scan(&r.ID, &r.HostID, &r.ContactID, &r.IsMain,
			&r.Name, &r.Phone, &r.Role, &r.Entity, &r.Notes, &r.IsExternal); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetMainResponsavelName returns the name of the main internal responsavel for a host.
func GetMainResponsavelName(db *sql.DB, hostID int64) string {
	var name string
	_ = db.QueryRow(`
		SELECT c.name FROM host_responsaveis hr
		JOIN contacts c ON c.id = hr.contact_id
		WHERE hr.host_id = ? AND hr.is_main AND NOT c.is_external
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
		WHERE hr.is_main AND NOT c.is_external`)
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
// Each input must reference an existing contact (ContactID > 0). Inline contact
// creation is not supported — manage contacts via /api/contacts.
func SyncHostResponsaveis(db *sql.DB, hostID int64, inputs []HostResponsavelInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM host_responsaveis WHERE host_id = ?`, hostID); err != nil {
		return err
	}

	for _, inp := range inputs {
		if inp.ContactID <= 0 {
			return fmt.Errorf("contact_id is required for each responsavel")
		}
		if _, err := tx.Exec(
			`INSERT INTO host_responsaveis (host_id, contact_id, is_main) VALUES (?, ?, ?)`,
			hostID, inp.ContactID, inp.IsMain,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
