package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Project struct {
	ID                          int64     `json:"id"`
	Name                        string    `json:"name"`
	Description                 string    `json:"description"`
	Situacao                    string    `json:"situacao"`
	SetorResponsavel            string    `json:"setor_responsavel"`
	Responsavel                 string    `json:"responsavel"`
	TemEmpresaExternaResponsavel bool     `json:"tem_empresa_externa_responsavel"`
	ContatoEmpresaResponsavel   string    `json:"contato_empresa_responsavel"`
	IsDirectlyManaged           bool      `json:"is_directly_managed"`
	IsResponsible               bool      `json:"is_responsible"`
	GitlabURL                   string    `json:"gitlab_url"`
	DocumentationURL            string    `json:"documentation_url"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

type ProjectResponsavel struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Nome      string `json:"nome"`
	Contato   string `json:"contato"`
}

func CreateProject(db *sql.DB, p *Project) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO projects (name, description, situacao, setor_responsavel, responsavel,
			tem_empresa_externa_responsavel, contato_empresa_responsavel,
			is_directly_managed, is_responsible, gitlab_url, documentation_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.Situacao, p.SetorResponsavel, p.Responsavel,
		p.TemEmpresaExternaResponsavel, p.ContatoEmpresaResponsavel,
		p.IsDirectlyManaged, p.IsResponsible, p.GitlabURL, p.DocumentationURL,
	)
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func GetProject(db *sql.DB, id int64) (*Project, error) {
	p := &Project{}
	err := db.QueryRow(
		`SELECT id, name, description, situacao, setor_responsavel, responsavel,
			tem_empresa_externa_responsavel, contato_empresa_responsavel,
			is_directly_managed, is_responsible, gitlab_url, documentation_url,
			created_at, updated_at
		FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Situacao, &p.SetorResponsavel, &p.Responsavel,
		&p.TemEmpresaExternaResponsavel, &p.ContatoEmpresaResponsavel,
		&p.IsDirectlyManaged, &p.IsResponsible, &p.GitlabURL, &p.DocumentationURL,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func ListProjects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(
		`SELECT id, name, description, situacao, setor_responsavel, responsavel,
			tem_empresa_externa_responsavel, contato_empresa_responsavel,
			is_directly_managed, is_responsible, gitlab_url, documentation_url,
			created_at, updated_at
		FROM projects ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Situacao, &p.SetorResponsavel, &p.Responsavel,
			&p.TemEmpresaExternaResponsavel, &p.ContatoEmpresaResponsavel,
			&p.IsDirectlyManaged, &p.IsResponsible, &p.GitlabURL, &p.DocumentationURL,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func UpdateProject(db *sql.DB, p *Project) error {
	_, err := db.Exec(
		`UPDATE projects SET name = ?, description = ?, situacao = ?, setor_responsavel = ?, responsavel = ?,
			tem_empresa_externa_responsavel = ?, contato_empresa_responsavel = ?,
			is_directly_managed = ?, is_responsible = ?, gitlab_url = ?, documentation_url = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		p.Name, p.Description, p.Situacao, p.SetorResponsavel, p.Responsavel,
		p.TemEmpresaExternaResponsavel, p.ContatoEmpresaResponsavel,
		p.IsDirectlyManaged, p.IsResponsible, p.GitlabURL, p.DocumentationURL,
		p.ID,
	)
	return err
}

func DeleteProject(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return err
}

func ProjectCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n)
	return n, err
}

// Project-Host links

// SetHostProjectLinks replaces all project links for a host.
func SetHostProjectLinks(db *sql.DB, hostID int64, projectIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM project_host_links WHERE host_id = ?`, hostID); err != nil {
		return err
	}
	for _, pid := range projectIDs {
		if _, err := tx.Exec(`INSERT INTO project_host_links (project_id, host_id) VALUES (?, ?)`, pid, hostID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListProjectsByHost returns projects directly linked to a host.
func ListProjectsByHost(db *sql.DB, hostID int64) ([]Project, error) {
	rows, err := db.Query(
		`SELECT p.id, p.name, p.description, p.situacao, p.setor_responsavel, p.responsavel,
			p.tem_empresa_externa_responsavel, p.contato_empresa_responsavel,
			p.is_directly_managed, p.is_responsible, p.gitlab_url, p.documentation_url,
			p.created_at, p.updated_at
		FROM projects p
		JOIN project_host_links l ON p.id = l.project_id
		WHERE l.host_id = ? ORDER BY p.name`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Situacao, &p.SetorResponsavel, &p.Responsavel,
			&p.TemEmpresaExternaResponsavel, &p.ContatoEmpresaResponsavel,
			&p.IsDirectlyManaged, &p.IsResponsible, &p.GitlabURL, &p.DocumentationURL,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProjectHostIDs returns host IDs directly linked to a project.
func GetProjectHostIDs(db *sql.DB, projectID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT host_id FROM project_host_links WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Project responsaveis

func SetProjectResponsaveis(db *sql.DB, projectID int64, responsaveis []ProjectResponsavel) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM project_responsaveis WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, r := range responsaveis {
		if _, err := tx.Exec(
			`INSERT INTO project_responsaveis (project_id, nome, contato) VALUES (?, ?, ?)`,
			projectID, r.Nome, r.Contato,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetProjectResponsaveis(db *sql.DB, projectID int64) ([]ProjectResponsavel, error) {
	rows, err := db.Query(
		`SELECT id, project_id, nome, contato FROM project_responsaveis WHERE project_id = ? ORDER BY nome`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProjectResponsavel
	for rows.Next() {
		var r ProjectResponsavel
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Nome, &r.Contato); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
