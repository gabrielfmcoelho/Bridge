package models

import (
	"database/sql"
	"fmt"
)

// ProjectResponsavelContact is the contact-based responsavel (same pattern as host_responsaveis).
// is_external/notes/role/entity are joined from the contacts table.
type ProjectResponsavelContact struct {
	ID         int64  `json:"id"`
	ProjectID  int64  `json:"project_id"`
	ContactID  int64  `json:"contact_id"`
	IsMain     bool   `json:"is_main"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	Entity     string `json:"entity"`
	Notes      string `json:"notes"`
	IsExternal bool   `json:"is_external"`
}

// ProjectResponsavelInput requires an existing contact id; inline creation is not supported.
type ProjectResponsavelInput struct {
	ContactID int64 `json:"contact_id"`
	IsMain    bool  `json:"is_main"`
}

// ListProjectResponsaveisContact returns all contact-based responsaveis for a project.
func ListProjectResponsaveisContact(db *sql.DB, projectID int64) ([]ProjectResponsavelContact, error) {
	rows, err := db.Query(`
		SELECT pr.id, pr.project_id, pr.contact_id, pr.is_main,
		       c.name, c.phone, c.role, c.entity, c.notes, c.is_external
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
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.ContactID, &r.IsMain,
			&r.Name, &r.Phone, &r.Role, &r.Entity, &r.Notes, &r.IsExternal); err != nil {
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
		WHERE pr.is_main AND NOT c.is_external AND pr.contact_id > 0`)
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

// SyncProjectResponsaveisContact replaces all responsaveis for a project. Each
// input must reference an existing contact.
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
		if inp.ContactID <= 0 {
			return fmt.Errorf("contact_id is required for each responsavel")
		}
		if _, err := tx.Exec(
			`INSERT INTO project_responsaveis (project_id, contact_id, is_main) VALUES (?, ?, ?)`,
			projectID, inp.ContactID, inp.IsMain,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
