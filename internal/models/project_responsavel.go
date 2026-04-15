package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// ProjectResponsavelContact is the contact-based responsavel (same pattern as host_responsaveis).
type ProjectResponsavelContact struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	Entity    string `json:"entity"`
}

// ProjectResponsavelInput is the input for creating/syncing project responsaveis (contact-based).
type ProjectResponsavelInput struct {
	ContactID int64  `json:"contact_id"`
	IsMain    bool   `json:"is_main"`
	IsExterno bool   `json:"is_externo"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	Entity    string `json:"entity"`
}

// ListProjectResponsaveisContact returns all contact-based responsaveis for a project.
func ListProjectResponsaveisContact(db *sql.DB, projectID int64) ([]ProjectResponsavelContact, error) {
	rows, err := db.Query(`
		SELECT pr.id, pr.project_id, pr.contact_id, pr.is_main, pr.is_externo,
		       c.name, c.phone, c.role, c.entity
		FROM project_responsaveis pr
		JOIN contacts c ON c.id = pr.contact_id
		WHERE pr.project_id = ? AND pr.contact_id > 0
		ORDER BY pr.is_main DESC, c.name ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProjectResponsavelContact
	for rows.Next() {
		var r ProjectResponsavelContact
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.ContactID, &r.IsMain, &r.IsExterno,
			&r.Name, &r.Phone, &r.Role, &r.Entity); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetProjectMainResponsavelNamesBulk returns main responsavel names for all projects.
func GetProjectMainResponsavelNamesBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`
		SELECT pr.project_id, c.name FROM project_responsaveis pr
		JOIN contacts c ON c.id = pr.contact_id
		WHERE pr.is_main AND NOT pr.is_externo AND pr.contact_id > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]string)
	for rows.Next() {
		var projectID int64
		var name string
		if err := rows.Scan(&projectID, &name); err != nil {
			return nil, err
		}
		m[projectID] = name
	}
	return m, rows.Err()
}

// SyncProjectResponsaveisContact replaces all responsaveis for a project (contact-based pattern).
func SyncProjectResponsaveisContact(db *sql.DB, projectID int64, inputs []ProjectResponsavelInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM project_responsaveis WHERE project_id = ?`, projectID); err != nil {
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
			`INSERT INTO project_responsaveis (project_id, contact_id, is_main, is_externo, nome, contato) VALUES (?, ?, ?, ?, ?, ?)`,
			projectID, contactID, inp.IsMain, inp.IsExterno, inp.Name, inp.Phone,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
