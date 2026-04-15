package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// DNSResponsavel represents a junction between a DNS record and a contact.
type DNSResponsavel struct {
	ID        int64  `json:"id"`
	DNSID     int64  `json:"dns_id"`
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	Entity    string `json:"entity"`
}

// DNSResponsavelInput is the input for creating/syncing DNS responsaveis.
type DNSResponsavelInput struct {
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	Entity    string `json:"entity"`
}

// ListDNSResponsaveis returns all responsaveis for a DNS record, joined with contact details.
func ListDNSResponsaveis(db *sql.DB, dnsID int64) ([]DNSResponsavel, error) {
	rows, err := db.Query(`
		SELECT dr.id, dr.dns_id, dr.contact_id, dr.is_main, dr.is_externo,
		       c.name, c.phone, c.role, c.entity
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
		if err := rows.Scan(&r.ID, &r.DNSID, &r.ContactID, &r.IsMain, &r.IsExterno,
			&r.Name, &r.Phone, &r.Role, &r.Entity); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetDNSMainResponsavelNamesBulk returns main responsavel names for all DNS records in a single query.
func GetDNSMainResponsavelNamesBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT dr.dns_id, c.name FROM dns_responsaveis dr
		JOIN contacts c ON c.id = dr.contact_id
		WHERE dr.is_main AND NOT dr.is_externo`)
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

// SyncDNSResponsaveis replaces all responsaveis for a DNS record with the given inputs.
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
			`INSERT INTO dns_responsaveis (dns_id, contact_id, is_main, is_externo) VALUES (?, ?, ?, ?)`,
			dnsID, contactID, inp.IsMain, inp.IsExterno,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
