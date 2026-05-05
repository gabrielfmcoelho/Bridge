package models

import (
	"database/sql"
	"fmt"
)

// DNSResponsavel represents a junction between a DNS record and a contact.
// is_external/notes/role/entity are joined from the contacts table.
type DNSResponsavel struct {
	ID         int64  `json:"id"`
	DNSID      int64  `json:"dns_id"`
	ContactID  int64  `json:"contact_id"`
	IsMain     bool   `json:"is_main"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	Entity     string `json:"entity"`
	Notes      string `json:"notes"`
	IsExternal bool   `json:"is_external"`
}

// DNSResponsavelInput requires an existing contact id; inline creation is not supported.
type DNSResponsavelInput struct {
	ContactID int64 `json:"contact_id"`
	IsMain    bool  `json:"is_main"`
}

// ListDNSResponsaveis returns all responsaveis for a DNS record, joined with contact details.
func ListDNSResponsaveis(db *sql.DB, dnsID int64) ([]DNSResponsavel, error) {
	rows, err := db.Query(`
		SELECT dr.id, dr.dns_id, dr.contact_id, dr.is_main,
		       c.name, c.phone, c.role, c.entity, c.notes, c.is_external
		FROM dns_responsaveis dr
		JOIN contacts c ON c.id = dr.contact_id
		WHERE dr.dns_id = ?
		ORDER BY dr.is_main DESC, c.name ASC`, dnsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DNSResponsavel
	for rows.Next() {
		var r DNSResponsavel
		if err := rows.Scan(&r.ID, &r.DNSID, &r.ContactID, &r.IsMain,
			&r.Name, &r.Phone, &r.Role, &r.Entity, &r.Notes, &r.IsExternal); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetDNSMainResponsavelNamesBulk returns main responsavel names for all DNS records.
func GetDNSMainResponsavelNamesBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT dr.dns_id, c.name FROM dns_responsaveis dr
		JOIN contacts c ON c.id = dr.contact_id
		WHERE dr.is_main AND NOT c.is_external`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]string)
	for rows.Next() {
		var dnsID int64
		var name string
		if err := rows.Scan(&dnsID, &name); err != nil {
			return nil, err
		}
		m[dnsID] = name
	}
	return m, rows.Err()
}

// SyncDNSResponsaveis replaces all responsaveis for a DNS record. Each input
// must reference an existing contact.
func SyncDNSResponsaveis(db *sql.DB, dnsID int64, inputs []DNSResponsavelInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM dns_responsaveis WHERE dns_id = ?`, dnsID); err != nil {
		return err
	}

	for _, inp := range inputs {
		if inp.ContactID <= 0 {
			return fmt.Errorf("contact_id is required for each responsavel")
		}
		if _, err := tx.Exec(
			`INSERT INTO dns_responsaveis (dns_id, contact_id, is_main) VALUES (?, ?, ?)`,
			dnsID, inp.ContactID, inp.IsMain,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
