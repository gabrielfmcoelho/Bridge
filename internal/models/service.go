package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Service struct {
	ID                     int64     `json:"id"`
	Nickname               string    `json:"nickname"`
	ProjectID              *int64    `json:"project_id"`
	Description            string    `json:"description"`
	ServiceType            string    `json:"service_type"`
	ServiceSubtype         string    `json:"service_subtype"`
	TechnologyStack        string    `json:"technology_stack"`
	DeployApproach         string    `json:"deploy_approach"`
	OrchestratorTool       string    `json:"orchestrator_tool"`
	Environment            string    `json:"environment"`
	Port                   string    `json:"port"`
	Version                string    `json:"version"`
	OrchestratorManaged    bool      `json:"orchestrator_managed"`
	IsDirectlyManaged      bool      `json:"is_directly_managed"`
	IsResponsible          bool      `json:"is_responsible"`
	DevelopedBy            string    `json:"developed_by"`
	IsExternalDependency   bool      `json:"is_external_dependency"`
	ExternalProvider       string    `json:"external_provider"`
	ExternalURL            string    `json:"external_url"`
	ExternalContact        string    `json:"external_contact"`
	RepositoryURL          string    `json:"repository_url"`
	GitlabURL              string    `json:"gitlab_url"`
	DocumentationURL       string    `json:"documentation_url"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func CreateService(db *sql.DB, s *Service) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO services (nickname, project_id, description, service_type, service_subtype,
			technology_stack, deploy_approach, orchestrator_tool, environment, port, version,
			orchestrator_managed, is_directly_managed, is_responsible, developed_by,
			is_external_dependency, external_provider, external_url, external_contact,
			repository_url, gitlab_url, documentation_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Nickname, s.ProjectID, s.Description, s.ServiceType, s.ServiceSubtype,
		s.TechnologyStack, s.DeployApproach, s.OrchestratorTool, s.Environment, s.Port, s.Version,
		s.OrchestratorManaged, s.IsDirectlyManaged, s.IsResponsible, s.DevelopedBy,
		s.IsExternalDependency, s.ExternalProvider, s.ExternalURL, s.ExternalContact,
		s.RepositoryURL, s.GitlabURL, s.DocumentationURL,
	)
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

const serviceCols = `id, nickname, project_id, description, service_type, service_subtype,
	technology_stack, deploy_approach, orchestrator_tool, environment, port, version,
	orchestrator_managed, is_directly_managed, is_responsible, developed_by,
	is_external_dependency, external_provider, external_url, external_contact,
	repository_url, gitlab_url, documentation_url, created_at, updated_at`

func scanService(scanner interface{ Scan(...any) error }, s *Service) error {
	return scanner.Scan(&s.ID, &s.Nickname, &s.ProjectID, &s.Description, &s.ServiceType, &s.ServiceSubtype,
		&s.TechnologyStack, &s.DeployApproach, &s.OrchestratorTool, &s.Environment, &s.Port, &s.Version,
		&s.OrchestratorManaged, &s.IsDirectlyManaged, &s.IsResponsible, &s.DevelopedBy,
		&s.IsExternalDependency, &s.ExternalProvider, &s.ExternalURL, &s.ExternalContact,
		&s.RepositoryURL, &s.GitlabURL, &s.DocumentationURL, &s.CreatedAt, &s.UpdatedAt,
	)
}

func GetService(db *sql.DB, id int64) (*Service, error) {
	s := &Service{}
	err := scanService(db.QueryRow(`SELECT `+serviceCols+` FROM services WHERE id = ?`, id), s)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func ListServices(db *sql.DB) ([]Service, error) {
	rows, err := db.Query(`SELECT ` + serviceCols + ` FROM services ORDER BY nickname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := scanService(rows, &s); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func ListServicesByProject(db *sql.DB, projectID int64) ([]Service, error) {
	rows, err := db.Query(`SELECT `+serviceCols+` FROM services WHERE project_id = ? ORDER BY nickname`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := scanService(rows, &s); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func ListServicesByHost(db *sql.DB, hostID int64) ([]Service, error) {
	// Build prefixed column list from serviceCols
	rows, err := db.Query(
		`SELECT s.id, s.nickname, s.project_id, s.description, s.service_type, s.service_subtype,
			s.technology_stack, s.deploy_approach, s.orchestrator_tool, s.environment, s.port, s.version,
			s.orchestrator_managed, s.is_directly_managed, s.is_responsible, s.developed_by,
			s.is_external_dependency, s.external_provider, s.external_url, s.external_contact,
			s.repository_url, s.gitlab_url, s.documentation_url, s.created_at, s.updated_at
		FROM services s
		JOIN service_host_links l ON s.id = l.service_id
		WHERE l.host_id = ? ORDER BY s.nickname`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := scanService(rows, &s); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func UpdateService(db *sql.DB, s *Service) error {
	_, err := db.Exec(
		`UPDATE services SET nickname = ?, project_id = ?, description = ?, service_type = ?, service_subtype = ?,
			technology_stack = ?, deploy_approach = ?, orchestrator_tool = ?, environment = ?, port = ?, version = ?,
			orchestrator_managed = ?, is_directly_managed = ?, is_responsible = ?, developed_by = ?,
			is_external_dependency = ?, external_provider = ?, external_url = ?, external_contact = ?,
			repository_url = ?, gitlab_url = ?, documentation_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		s.Nickname, s.ProjectID, s.Description, s.ServiceType, s.ServiceSubtype,
		s.TechnologyStack, s.DeployApproach, s.OrchestratorTool, s.Environment, s.Port, s.Version,
		s.OrchestratorManaged, s.IsDirectlyManaged, s.IsResponsible, s.DevelopedBy,
		s.IsExternalDependency, s.ExternalProvider, s.ExternalURL, s.ExternalContact,
		s.RepositoryURL, s.GitlabURL, s.DocumentationURL, s.ID,
	)
	return err
}

func DeleteService(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM services WHERE id = ?`, id)
	return err
}

func ServiceCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM services`).Scan(&n)
	return n, err
}

// GetServiceCountsByHost returns a map of host_id → number of linked services.
func GetServiceCountsByHost(db *sql.DB) (map[int64]int, error) {
	rows, err := db.Query(`SELECT host_id, COUNT(*) FROM service_host_links GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]int)
	for rows.Next() {
		var hostID int64
		var cnt int
		if err := rows.Scan(&hostID, &cnt); err != nil {
			return nil, err
		}
		m[hostID] = cnt
	}
	return m, rows.Err()
}

// GetProjectCountsByHost returns a map of host_id → number of distinct projects
// linked either directly via project_host_links or indirectly via services.
func GetProjectCountsByHost(db *sql.DB) (map[int64]int, error) {
	rows, err := db.Query(`
		SELECT host_id, COUNT(DISTINCT project_id) FROM (
			SELECT host_id, project_id FROM project_host_links
			UNION
			SELECT l.host_id, s.project_id FROM service_host_links l
				JOIN services s ON l.service_id = s.id
				WHERE s.project_id IS NOT NULL
		) GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]int)
	for rows.Next() {
		var hostID int64
		var cnt int
		if err := rows.Scan(&hostID, &cnt); err != nil {
			return nil, err
		}
		m[hostID] = cnt
	}
	return m, rows.Err()
}

// Service-Host links

func SetServiceHostLinks(db *sql.DB, serviceID int64, hostIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM service_host_links WHERE service_id = ?`, serviceID); err != nil {
		return err
	}
	for _, hid := range hostIDs {
		if _, err := tx.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, serviceID, hid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetServiceHostIDs(db *sql.DB, serviceID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT host_id FROM service_host_links WHERE service_id = ?`, serviceID)
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

// SetHostServiceLinks replaces all service links for a host.
func SetHostServiceLinks(db *sql.DB, hostID int64, serviceIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM service_host_links WHERE host_id = ?`, hostID); err != nil {
		return err
	}
	for _, sid := range serviceIDs {
		if _, err := tx.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, sid, hostID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Service-DNS links

func SetServiceDNSLinks(db *sql.DB, serviceID int64, dnsIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM service_dns_links WHERE service_id = ?`, serviceID); err != nil {
		return err
	}
	for _, did := range dnsIDs {
		if _, err := tx.Exec(`INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?)`, serviceID, did); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetServiceDNSIDs(db *sql.DB, serviceID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT dns_id FROM service_dns_links WHERE service_id = ?`, serviceID)
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

// Service dependencies

func SetServiceDependencies(db *sql.DB, serviceID int64, dependsOnIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM service_dependencies WHERE service_id = ?`, serviceID); err != nil {
		return err
	}
	for _, depID := range dependsOnIDs {
		if _, err := tx.Exec(`INSERT INTO service_dependencies (service_id, depends_on_id) VALUES (?, ?)`, serviceID, depID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetServiceDependencyIDs(db *sql.DB, serviceID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT depends_on_id FROM service_dependencies WHERE service_id = ?`, serviceID)
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

func GetServiceDependentIDs(db *sql.DB, serviceID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT service_id FROM service_dependencies WHERE depends_on_id = ?`, serviceID)
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
