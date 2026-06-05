package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// ServiceRepo owns SQL for the services table and its junctions
// (service_host_links, service_dns_links, service_dependencies). Used by the
// service-layer ServiceService/ProjectService and inline by
// host/graph/dashboard/grafana/tool/sshconfig handlers.
type ServiceRepo struct {
	db *sql.DB
}

// NewServiceRepo constructs a ServiceRepo over the given DB handle.
func NewServiceRepo(db *sql.DB) *ServiceRepo { return &ServiceRepo{db: db} }

const serviceCols = `id, nickname, project_id, description, service_type, service_subtype,
	technology_stack, deploy_approach, orchestrator_tool, environment, port, version,
	orchestrator_managed, is_directly_managed, is_responsible, developed_by,
	is_external_dependency, external_provider, external_url, external_contact,
	repository_url, gitlab_url, documentation_url,
	source, container_status, container_id, container_name, container_image, container_ports,
	discovered_at, last_seen_at, grafana_dashboard_uid, created_at, updated_at`

// serviceColsS is serviceCols qualified with the `s.` table alias, for joins.
const serviceColsS = `s.id, s.nickname, s.project_id, s.description, s.service_type, s.service_subtype,
	s.technology_stack, s.deploy_approach, s.orchestrator_tool, s.environment, s.port, s.version,
	s.orchestrator_managed, s.is_directly_managed, s.is_responsible, s.developed_by,
	s.is_external_dependency, s.external_provider, s.external_url, s.external_contact,
	s.repository_url, s.gitlab_url, s.documentation_url,
	s.source, s.container_status, s.container_id, s.container_name, s.container_image, s.container_ports,
	s.discovered_at, s.last_seen_at, s.grafana_dashboard_uid, s.created_at, s.updated_at`

func scanService(scanner interface{ Scan(...any) error }, s *models.Service) error {
	return scanner.Scan(&s.ID, &s.Nickname, &s.ProjectID, &s.Description, &s.ServiceType, &s.ServiceSubtype,
		&s.TechnologyStack, &s.DeployApproach, &s.OrchestratorTool, &s.Environment, &s.Port, &s.Version,
		&s.OrchestratorManaged, &s.IsDirectlyManaged, &s.IsResponsible, &s.DevelopedBy,
		&s.IsExternalDependency, &s.ExternalProvider, &s.ExternalURL, &s.ExternalContact,
		&s.RepositoryURL, &s.GitlabURL, &s.DocumentationURL,
		&s.Source, &s.ContainerStatus, &s.ContainerID, &s.ContainerName, &s.ContainerImage, &s.ContainerPorts,
		&s.DiscoveredAt, &s.LastSeenAt, &s.GrafanaDashboardUID, &s.CreatedAt, &s.UpdatedAt,
	)
}

func scanServices(rows *sql.Rows) ([]models.Service, error) {
	defer rows.Close()
	var services []models.Service
	for rows.Next() {
		var s models.Service
		if err := scanService(rows, &s); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

// Create inserts a service (defaulting Source to "manual") and sets s.ID.
func (r *ServiceRepo) Create(ctx context.Context, s *models.Service) error {
	if s.Source == "" {
		s.Source = "manual"
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO services (nickname, project_id, description, service_type, service_subtype,
			technology_stack, deploy_approach, orchestrator_tool, environment, port, version,
			orchestrator_managed, is_directly_managed, is_responsible, developed_by,
			is_external_dependency, external_provider, external_url, external_contact,
			repository_url, gitlab_url, documentation_url,
			source, container_status, container_id, container_name, container_image, container_ports,
			discovered_at, last_seen_at, grafana_dashboard_uid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Nickname, s.ProjectID, s.Description, s.ServiceType, s.ServiceSubtype,
		s.TechnologyStack, s.DeployApproach, s.OrchestratorTool, s.Environment, s.Port, s.Version,
		s.OrchestratorManaged, s.IsDirectlyManaged, s.IsResponsible, s.DevelopedBy,
		s.IsExternalDependency, s.ExternalProvider, s.ExternalURL, s.ExternalContact,
		s.RepositoryURL, s.GitlabURL, s.DocumentationURL,
		s.Source, s.ContainerStatus, s.ContainerID, s.ContainerName, s.ContainerImage, s.ContainerPorts,
		s.DiscoveredAt, s.LastSeenAt, s.GrafanaDashboardUID,
	)
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

// Get returns a service by id, or (nil, nil) if absent.
func (r *ServiceRepo) Get(ctx context.Context, id int64) (*models.Service, error) {
	s := &models.Service{}
	err := scanService(r.db.QueryRowContext(ctx, `SELECT `+serviceCols+` FROM services WHERE id = ?`, id), s)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// List returns all services ordered by nickname.
func (r *ServiceRepo) List(ctx context.Context) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+serviceCols+` FROM services ORDER BY nickname`)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// ListByProject returns services belonging to a project, ordered by nickname.
func (r *ServiceRepo) ListByProject(ctx context.Context, projectID int64) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+serviceCols+` FROM services WHERE project_id = ? ORDER BY nickname`, projectID)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// ListByHost returns services linked to a host, ordered by nickname.
func (r *ServiceRepo) ListByHost(ctx context.Context, hostID int64) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+serviceColsS+`
		 FROM services s JOIN service_host_links l ON s.id = l.service_id
		 WHERE l.host_id = ? ORDER BY s.nickname`, hostID)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// ListContainerServicesByHost returns all auto/fixed services linked to a host
// that have a container_name set.
func (r *ServiceRepo) ListContainerServicesByHost(ctx context.Context, hostID int64) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+serviceColsS+`
		 FROM services s JOIN service_host_links l ON s.id = l.service_id
		 WHERE l.host_id = ? AND s.source IN ('auto', 'fixed') AND s.container_name != ''
		 ORDER BY s.nickname`, hostID)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// Update writes the mutable fields of a service by id.
func (r *ServiceRepo) Update(ctx context.Context, s *models.Service) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE services SET nickname = ?, project_id = ?, description = ?, service_type = ?, service_subtype = ?,
			technology_stack = ?, deploy_approach = ?, orchestrator_tool = ?, environment = ?, port = ?, version = ?,
			orchestrator_managed = ?, is_directly_managed = ?, is_responsible = ?, developed_by = ?,
			is_external_dependency = ?, external_provider = ?, external_url = ?, external_contact = ?,
			repository_url = ?, gitlab_url = ?, documentation_url = ?,
			source = ?, container_status = ?, container_id = ?, container_name = ?, container_image = ?, container_ports = ?,
			discovered_at = ?, last_seen_at = ?, grafana_dashboard_uid = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		s.Nickname, s.ProjectID, s.Description, s.ServiceType, s.ServiceSubtype,
		s.TechnologyStack, s.DeployApproach, s.OrchestratorTool, s.Environment, s.Port, s.Version,
		s.OrchestratorManaged, s.IsDirectlyManaged, s.IsResponsible, s.DevelopedBy,
		s.IsExternalDependency, s.ExternalProvider, s.ExternalURL, s.ExternalContact,
		s.RepositoryURL, s.GitlabURL, s.DocumentationURL,
		s.Source, s.ContainerStatus, s.ContainerID, s.ContainerName, s.ContainerImage, s.ContainerPorts,
		s.DiscoveredAt, s.LastSeenAt, s.GrafanaDashboardUID, s.ID,
	)
	return err
}

// Fixate converts an auto-discovered service to a fixed service.
func (r *ServiceRepo) Fixate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE services SET source = 'fixed', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND source = 'auto'`, id)
	return err
}

// UpdateContainerBinding rebinds a fixed/manual service to a different container.
func (r *ServiceRepo) UpdateContainerBinding(ctx context.Context, id int64, containerName, containerID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE services SET container_name = ?, container_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		containerName, containerID, id,
	)
	return err
}

// Delete removes a service row by id. (Vault cascade is handled separately via
// the cascade registry in the delete path.)
func (r *ServiceRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM services WHERE id = ?`, id)
	return err
}

// Count returns the total number of services.
func (r *ServiceRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM services`).Scan(&n)
	return n, err
}

// CountsByHost returns host_id → number of linked services.
func (r *ServiceRepo) CountsByHost(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id, COUNT(*) FROM service_host_links GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	return scanCountMap(rows)
}

// ProjectCountsByHost returns host_id → number of distinct projects linked
// either directly via project_host_links or indirectly via services.
func (r *ServiceRepo) ProjectCountsByHost(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx, `
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
	return scanCountMap(rows)
}

// SetHostLinks replaces all host links for a service (one tx).
func (r *ServiceRepo) SetHostLinks(ctx context.Context, serviceID int64, hostIDs []int64) error {
	return r.replaceLinks(ctx, `service_host_links`, `service_id`, `host_id`, serviceID, hostIDs)
}

// HostIDs returns host ids linked to a service.
func (r *ServiceRepo) HostIDs(ctx context.Context, serviceID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id FROM service_host_links WHERE service_id = ?`, serviceID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// SetServicesForHost replaces all service links for a host (one tx).
func (r *ServiceRepo) SetServicesForHost(ctx context.Context, hostID int64, serviceIDs []int64) error {
	return r.replaceLinks(ctx, `service_host_links`, `host_id`, `service_id`, hostID, serviceIDs)
}

// SetDNSLinks replaces all dns links for a service (one tx).
func (r *ServiceRepo) SetDNSLinks(ctx context.Context, serviceID int64, dnsIDs []int64) error {
	return r.replaceLinks(ctx, `service_dns_links`, `service_id`, `dns_id`, serviceID, dnsIDs)
}

// DNSIDs returns dns ids linked to a service.
func (r *ServiceRepo) DNSIDs(ctx context.Context, serviceID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT dns_id FROM service_dns_links WHERE service_id = ?`, serviceID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// SetDependencies replaces all dependency edges for a service (one tx).
func (r *ServiceRepo) SetDependencies(ctx context.Context, serviceID int64, dependsOnIDs []int64) error {
	return r.replaceLinks(ctx, `service_dependencies`, `service_id`, `depends_on_id`, serviceID, dependsOnIDs)
}

// DependencyIDs returns the ids a service depends on.
func (r *ServiceRepo) DependencyIDs(ctx context.Context, serviceID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT depends_on_id FROM service_dependencies WHERE service_id = ?`, serviceID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// DependentIDs returns the ids that depend on a service.
func (r *ServiceRepo) DependentIDs(ctx context.Context, serviceID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT service_id FROM service_dependencies WHERE depends_on_id = ?`, serviceID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// replaceLinks deletes all rows in a two-column junction matching keyCol=keyID,
// then inserts (keyID, v) for each v in vals — atomically.
func (r *ServiceRepo) replaceLinks(ctx context.Context, table, keyCol, valCol string, keyID int64, vals []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+keyCol+` = ?`, keyID); err != nil {
		return err
	}
	for _, v := range vals {
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+keyCol+`, `+valCol+`) VALUES (?, ?)`, keyID, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReconcileContainers matches discovered containers against existing auto/fixed
// services for a host: new containers create auto services (linked to the host),
// matching containers refresh status/metadata, and unseen services go offline —
// all in one tx.
func (r *ServiceRepo) ReconcileContainers(ctx context.Context, hostID int64, containers []sshtest.ContainerInfo) error {
	existing, err := r.ListContainerServicesByHost(ctx, hostID)
	if err != nil {
		return err
	}
	byName := make(map[string]*models.Service, len(existing))
	for i := range existing {
		byName[existing[i].ContainerName] = &existing[i]
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	seen := make(map[string]bool)

	for _, c := range containers {
		if c.Name == "" {
			continue
		}
		if svc, ok := byName[c.Name]; ok {
			seen[c.Name] = true
			if _, err := tx.ExecContext(ctx,
				`UPDATE services SET container_id = ?, container_image = ?, container_ports = ?,
					container_status = 'online', last_seen_at = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?`,
				c.ID, c.Image, c.Ports, now, svc.ID,
			); err != nil {
				return err
			}
			continue
		}

		inf := sshtest.InferFromImage(c.Image, c.Name)
		port := extractFirstHostPort(c.Ports)
		var id int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO services (nickname, description, service_type, service_subtype,
				source, container_status, container_id, container_name, container_image, container_ports,
				port, orchestrator_managed, discovered_at, last_seen_at)
			VALUES (?, ?, ?, ?, 'auto', 'online', ?, ?, ?, ?, ?, 1, ?, ?)
			RETURNING id`,
			inf.Nickname, "Auto-discovered from container "+c.Name,
			inf.ServiceType, inf.ServiceSubtype,
			c.ID, c.Name, c.Image, c.Ports,
			port, now, now,
		).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, id, hostID,
		); err != nil {
			return err
		}
		seen[c.Name] = true
	}

	for _, svc := range existing {
		if seen[svc.ContainerName] || svc.ContainerStatus == "offline" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE services SET container_status = 'offline', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			svc.ID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// scanCountMap reads (id, count) rows into a map.
func scanCountMap(rows *sql.Rows) (map[int64]int, error) {
	defer rows.Close()
	m := make(map[int64]int)
	for rows.Next() {
		var id int64
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, err
		}
		m[id] = cnt
	}
	return m, rows.Err()
}

// extractFirstHostPort pulls the first host-mapped port from a Docker ports
// string like "0.0.0.0:8080->80/tcp, 5432/tcp".
func extractFirstHostPort(ports string) string {
	for _, part := range strings.Split(ports, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "->"); idx > 0 {
			hostPart := part[:idx]
			if colonIdx := strings.LastIndex(hostPart, ":"); colonIdx >= 0 {
				return hostPart[colonIdx+1:]
			}
			return hostPart
		}
	}
	return ""
}
