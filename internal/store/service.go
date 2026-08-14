package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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
	source, discovery_kind, discovery_key,
	container_status, container_id, container_name, container_image, container_ports,
	discovered_at, last_seen_at, grafana_dashboard_uid, created_at, updated_at`

// serviceColsS is serviceCols qualified with the `s.` table alias, for joins.
const serviceColsS = `s.id, s.nickname, s.project_id, s.description, s.service_type, s.service_subtype,
	s.technology_stack, s.deploy_approach, s.orchestrator_tool, s.environment, s.port, s.version,
	s.orchestrator_managed, s.is_directly_managed, s.is_responsible, s.developed_by,
	s.is_external_dependency, s.external_provider, s.external_url, s.external_contact,
	s.repository_url, s.gitlab_url, s.documentation_url,
	s.source, s.discovery_kind, s.discovery_key,
	s.container_status, s.container_id, s.container_name, s.container_image, s.container_ports,
	s.discovered_at, s.last_seen_at, s.grafana_dashboard_uid, s.created_at, s.updated_at`

func scanService(scanner interface{ Scan(...any) error }, s *models.Service) error {
	return scanner.Scan(&s.ID, &s.Nickname, &s.ProjectID, &s.Description, &s.ServiceType, &s.ServiceSubtype,
		&s.TechnologyStack, &s.DeployApproach, &s.OrchestratorTool, &s.Environment, &s.Port, &s.Version,
		&s.OrchestratorManaged, &s.IsDirectlyManaged, &s.IsResponsible, &s.DevelopedBy,
		&s.IsExternalDependency, &s.ExternalProvider, &s.ExternalURL, &s.ExternalContact,
		&s.RepositoryURL, &s.GitlabURL, &s.DocumentationURL,
		&s.Source, &s.DiscoveryKind, &s.DiscoveryKey,
		&s.ContainerStatus, &s.ContainerID, &s.ContainerName, &s.ContainerImage, &s.ContainerPorts,
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
//
// discovery_kind/discovery_key are intentionally absent from both Create and
// Update: they are the scan's identity for a row and are written only by
// ReconcileDiscovered, so an API payload can't graft itself onto (or steal)
// another host's discovered service.
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
	err := scanService(r.db.QueryRowContext(ctx, `SELECT `+serviceCols+` FROM services WHERE id = ? AND deleted_at IS NULL`, id), s)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// List returns all services ordered by nickname.
func (r *ServiceRepo) List(ctx context.Context) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+serviceCols+` FROM services WHERE deleted_at IS NULL ORDER BY nickname`)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// serviceWhere builds the shared WHERE predicates (and bound args) for the
// filtered list/count queries. Mirrors ProjectRepo's filter assembly: dynamic
// clauses with `?` placeholders (the driver rebinds `?`→$N), ILIKE search via
// database.LikeOp(), a tag subquery against the unified tags table, and
// tri-state boolean filters inlined as literal true/false (no bound arg).
func serviceWhere(f models.ServiceFilter) ([]string, []any) {
	var args []any
	where := []string{"deleted_at IS NULL"}

	if f.Search != "" {
		op := database.LikeOp()
		where = append(where, "(nickname "+op+" ? OR description "+op+" ? OR technology_stack "+op+" ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s, s)
	}
	if f.Tag != "" {
		where = append(where, "id IN (SELECT entity_id FROM tags WHERE entity_type = 'service' AND tag = ?)")
		args = append(args, f.Tag)
	}
	if f.DevelopedBy != "" {
		where = append(where, "developed_by = ?")
		args = append(args, f.DevelopedBy)
	}
	switch f.IsExternalDependency {
	case "yes":
		where = append(where, "is_external_dependency = true")
	case "no":
		where = append(where, "is_external_dependency = false")
	}
	switch f.OrchestratorManaged {
	case "yes":
		where = append(where, "orchestrator_managed = true")
	case "no":
		where = append(where, "orchestrator_managed = false")
	}
	return where, args
}

// ListFiltered returns services matching the filter, with sort + pagination
// applied. Mirrors ProjectRepo.ListFiltered. When f.PerPage <= 0 the result is
// unbounded (no LIMIT) — the path the frontend's full-list query uses.
func (r *ServiceRepo) ListFiltered(ctx context.Context, f models.ServiceFilter) ([]models.Service, error) {
	query := `SELECT ` + serviceCols + ` FROM services`
	where, args := serviceWhere(f)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	allowedSorts := map[string]string{
		"nickname":         "nickname",
		"technology_stack": "technology_stack",
	}
	sortCol := "nickname"
	if col, ok := allowedSorts[f.SortBy]; ok {
		sortCol = col
	}
	sortDir := "ASC"
	if f.SortDir == "desc" {
		sortDir = "DESC"
	}
	query += " ORDER BY " + sortCol + " " + sortDir

	if f.PerPage > 0 {
		offset := 0
		if f.Page > 1 {
			offset = (f.Page - 1) * f.PerPage
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PerPage, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// CountFiltered returns the number of services matching the same predicates
// ListFiltered paginates over (no order/limit).
func (r *ServiceRepo) CountFiltered(ctx context.Context, f models.ServiceFilter) (int, error) {
	query := `SELECT COUNT(*) FROM services`
	where, args := serviceWhere(f)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// ListByProject returns services belonging to a project, ordered by nickname.
func (r *ServiceRepo) ListByProject(ctx context.Context, projectID int64) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+serviceCols+` FROM services WHERE project_id = ? AND deleted_at IS NULL ORDER BY nickname`, projectID)
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
		 WHERE l.host_id = ? AND s.deleted_at IS NULL ORDER BY s.nickname`, hostID)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}

// ListDiscoveredByHost returns every scan-owned service linked to a host —
// auto or fixed, container or host kind. Manual services (no discovery_key)
// are excluded so reconciliation can never touch a hand-made row.
func (r *ServiceRepo) ListDiscoveredByHost(ctx context.Context, hostID int64) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+serviceColsS+`
		 FROM services s JOIN service_host_links l ON s.id = l.service_id
		 WHERE l.host_id = ? AND s.deleted_at IS NULL AND s.source IN ('auto', 'fixed') AND s.discovery_key != ''
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
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM services WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CountsByHost returns host_id → number of linked services.
func (r *ServiceRepo) CountsByHost(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT l.host_id, COUNT(*) FROM service_host_links l JOIN services s ON s.id = l.service_id WHERE s.deleted_at IS NULL GROUP BY l.host_id`)
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
			SELECT l.host_id, l.project_id FROM project_host_links l
				JOIN projects p ON p.id = l.project_id
				WHERE p.deleted_at IS NULL
			UNION
			SELECT l.host_id, s.project_id FROM service_host_links l
				JOIN services s ON l.service_id = s.id
				WHERE s.project_id IS NOT NULL AND s.deleted_at IS NULL
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

// DiscoveredInventory is one scan's view of what runs on a host, as far as
// service reconciliation cares.
//
// ContainersKnown carries the distinction the container list alone can't make:
// an empty Containers slice means "this host runs no containers" only when
// docker actually answered. When docker was unreadable (no passwordless sudo,
// dead daemon, timeout) the container half of the sweep must be skipped
// entirely, or every container service on the host would be marked offline by
// a permissions blip.
type DiscoveredInventory struct {
	Containers      []sshtest.ContainerInfo
	ContainersKnown bool
	Services        []sshtest.DiscoveredService
}

// discoveredRow is a container or host service normalised to what the services
// table stores, so one reconcile loop handles both kinds.
type discoveredRow struct {
	kind           string // "container" | "host"
	key            string // per-host identity: container name | catalog service name
	nickname       string
	description    string
	serviceType    string
	serviceSubtype string
	version        string
	port           string
	containerID    string
	containerName  string
	containerImage string
	containerPorts string
	orchestrated   bool
}

// identity is the (kind, key) pair a row reconciles on.
func (d discoveredRow) identity() string { return d.kind + "\x00" + d.key }

func serviceIdentity(s models.Service) string { return s.DiscoveryKind + "\x00" + s.DiscoveryKey }

// containerRows normalises `docker ps` output.
func containerRows(containers []sshtest.ContainerInfo) []discoveredRow {
	rows := make([]discoveredRow, 0, len(containers))
	for _, c := range containers {
		if c.Name == "" {
			continue
		}
		inf := sshtest.InferFromImage(c.Image, c.Name)
		rows = append(rows, discoveredRow{
			kind: "container", key: c.Name,
			nickname:    inf.Nickname,
			description: "Auto-discovered from container " + c.Name,
			serviceType: inf.ServiceType, serviceSubtype: inf.ServiceSubtype,
			port:           extractFirstHostPort(c.Ports),
			containerID:    c.ID,
			containerName:  c.Name,
			containerImage: c.Image,
			containerPorts: c.Ports,
			orchestrated:   true,
		})
	}
	return rows
}

// hostServiceRows normalises the catalog hits that deserve a service row.
//
// A hit qualifies on HostRunning alone: the scan attributes every process by
// cgroup and every listener by publisher, so HostRunning means a live instance
// outside any container — never the container's own postgres seen through the
// shared PID namespace. A host running both gets a row here *and* a container
// row from the docker sweep, which is the point.
//
// Package-only hits ("installed, never started") are excluded by the same flag,
// so the services page lists what runs rather than what is merely present; when
// a live service stops, HostRunning goes false and the offline sweep picks it up.
func hostServiceRows(services []sshtest.DiscoveredService) []discoveredRow {
	rows := make([]discoveredRow, 0, len(services))
	for _, s := range services {
		if s.Name == "" || !s.HostRunning {
			continue
		}
		// HostPorts, not Ports: the latter also carries whatever a container
		// of the same software published, which is not this row's listener.
		port := ""
		if len(s.HostPorts) > 0 {
			port = strconv.Itoa(s.HostPorts[0])
		}
		desc := "Auto-discovered host service " + s.Name
		if s.Unit != "" {
			desc = "Auto-discovered from systemd unit " + s.Unit
		}
		rows = append(rows, discoveredRow{
			kind: "host", key: s.Name,
			nickname:    s.Label,
			description: desc,
			serviceType: sshtest.ServiceTypeForCatalog(s.Name), serviceSubtype: s.Label,
			version: s.Version,
			port:    port,
		})
	}
	return rows
}

// ReconcileDiscovered matches a scan against the existing auto/fixed services
// for a host: unknown workloads create auto services (linked to the host),
// known ones refresh their runtime metadata, and anything the scan could see
// but didn't find goes offline — all in one tx.
//
// Containers match on container ID first and name second, so renaming a
// container updates its row in place instead of orphaning it and creating a
// duplicate. Host services match on the catalog name.
//
// Only scan-owned fields are refreshed on an existing row. Nickname, port and
// version stay as first discovered, because a fixated service is expected to
// carry the operator's edits.
func (r *ServiceRepo) ReconcileDiscovered(ctx context.Context, hostID int64, inv DiscoveredInventory) error {
	existing, err := r.ListDiscoveredByHost(ctx, hostID)
	if err != nil {
		return err
	}
	byIdentity := make(map[string]*models.Service, len(existing))
	byContainerID := make(map[string]*models.Service)
	for i := range existing {
		byIdentity[serviceIdentity(existing[i])] = &existing[i]
		if existing[i].DiscoveryKind == "container" && existing[i].ContainerID != "" {
			byContainerID[existing[i].ContainerID] = &existing[i]
		}
	}

	rows := hostServiceRows(inv.Services)
	if inv.ContainersKnown {
		rows = append(rows, containerRows(inv.Containers)...)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	seen := make(map[string]bool, len(rows))

	for _, row := range rows {
		match := byIdentity[row.identity()]
		if match == nil && row.containerID != "" {
			// Renamed container: same engine ID, new name.
			match = byContainerID[row.containerID]
		}
		if match != nil {
			seen[serviceIdentity(*match)] = true
			seen[row.identity()] = true
			if _, err := tx.ExecContext(ctx,
				`UPDATE services SET discovery_key = ?, container_id = ?, container_name = ?,
					container_image = ?, container_ports = ?, container_status = 'online',
					last_seen_at = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?`,
				row.key, row.containerID, row.containerName, row.containerImage, row.containerPorts,
				now, match.ID,
			); err != nil {
				return err
			}
			continue
		}

		var id int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO services (nickname, description, service_type, service_subtype, version,
				source, discovery_kind, discovery_key,
				container_status, container_id, container_name, container_image, container_ports,
				port, orchestrator_managed, discovered_at, last_seen_at)
			VALUES (?, ?, ?, ?, ?, 'auto', ?, ?, 'online', ?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id`,
			row.nickname, row.description, row.serviceType, row.serviceSubtype, row.version,
			row.kind, row.key,
			row.containerID, row.containerName, row.containerImage, row.containerPorts,
			row.port, row.orchestrated, now, now,
		).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, id, hostID,
		); err != nil {
			return err
		}
		seen[row.identity()] = true
	}

	for _, svc := range existing {
		if seen[serviceIdentity(svc)] || svc.ContainerStatus == "offline" {
			continue
		}
		// Docker refused to answer — its containers are unknown, not gone.
		if svc.DiscoveryKind == "container" && !inv.ContainersKnown {
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

// ListTrash returns soft-deleted services (deleted_at set).
func (r *ServiceRepo) ListTrash(ctx context.Context) ([]models.Service, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+serviceCols+` FROM services WHERE deleted_at IS NOT NULL ORDER BY nickname`)
	if err != nil {
		return nil, err
	}
	return scanServices(rows)
}
