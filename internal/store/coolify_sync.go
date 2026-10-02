package store

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// CoolifyFields is what the Coolify sync stores on a service it matched.
type CoolifyFields struct {
	ResourceUUID  string
	ResourceType  string
	Project       string
	Environment   string
	Stack         string
	GitRepository string
	GitBranch     string
}

// CoolifyContainer is a live container-kind service as the Coolify sync sees
// it: its first host, scan identity and Coolify resource.
type CoolifyContainer struct {
	ID           int64
	HostID       int64
	Key          string // normalized discovery key (deploy timestamp dropped)
	ResourceUUID string // stored, or parsed from the container name
	Status       string
	Source       string
	ProjectID    *int64
	LastSeenAt   *time.Time
}

// Placeholder reports a row the Coolify sync created that no scan has seen yet.
func (c CoolifyContainer) Placeholder() bool { return c.Source == "coolify" && c.LastSeenAt == nil }

// ContainersForCoolify lists every live container-kind service, unscoped (the
// sync runs as admin over the whole inventory).
func (r *ServiceRepo) ContainersForCoolify(ctx context.Context) ([]CoolifyContainer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, COALESCE(MIN(l.host_id), 0), s.container_name, s.discovery_key, s.container_status,
			s.source, s.project_id, s.last_seen_at, s.coolify_resource_uuid
		FROM services s LEFT JOIN service_host_links l ON l.service_id = s.id
		WHERE s.deleted_at IS NULL AND s.discovery_kind = 'container'
		GROUP BY s.id ORDER BY s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoolifyContainer
	for rows.Next() {
		var c CoolifyContainer
		var name, key string
		if err := rows.Scan(&c.ID, &c.HostID, &name, &key, &c.Status, &c.Source, &c.ProjectID, &c.LastSeenAt, &c.ResourceUUID); err != nil {
			return nil, err
		}
		if key == "" {
			key = name
		}
		var uuid string
		c.Key, uuid = coolify.ContainerKey(key)
		if c.ResourceUUID == "" {
			c.ResourceUUID = uuid
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCoolifyFields stores f on a service; changed is false when nothing differed.
func (r *ServiceRepo) SetCoolifyFields(ctx context.Context, id int64, f CoolifyFields) (changed bool, err error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE services SET coolify_resource_uuid = ?, coolify_resource_type = ?, coolify_project = ?,
			coolify_environment = ?, coolify_stack = ?, git_repository = ?, git_branch = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND (coolify_resource_uuid, coolify_resource_type, coolify_project, coolify_environment,
			coolify_stack, git_repository, git_branch) IS DISTINCT FROM (?, ?, ?, ?, ?, ?, ?)`,
		f.ResourceUUID, f.ResourceType, f.Project, f.Environment, f.Stack, f.GitRepository, f.GitBranch, id,
		f.ResourceUUID, f.ResourceType, f.Project, f.Environment, f.Stack, f.GitRepository, f.GitBranch)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// CreateCoolifyPlaceholder records a Coolify resource no scan has seen: an
// offline container-kind service keyed like the scan will key its container,
// so the next scan of hostID adopts it instead of creating a twin. Linked to
// the host, visible to whoever sees the host.
func (r *ServiceRepo) CreateCoolifyPlaceholder(ctx context.Context, hostID int64, key, nickname string, f CoolifyFields) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO services (nickname, description, source, discovery_kind, discovery_key, container_status,
			container_name, orchestrator_managed, orchestrator_tool, discovered_at,
			coolify_resource_uuid, coolify_resource_type, coolify_project, coolify_environment, coolify_stack,
			git_repository, git_branch)
		VALUES (?, ?, 'coolify', 'container', ?, 'offline', ?, TRUE, 'coolify', CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		nickname, "Recurso do Coolify ainda não visto por um scan", key, key,
		f.ResourceUUID, f.ResourceType, f.Project, f.Environment, f.Stack, f.GitRepository, f.GitBranch,
	).Scan(&id); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, id, hostID); err != nil {
		return 0, err
	}
	if err := NewAssetEntidadeRepo(r.db).CopyFrom(ctx, tx, AssetHost, hostID, AssetService, id); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// SetProjectIfEmpty gives a service a project only when it has none: a
// project chosen by hand always wins over the Coolify mapping.
func (r *ServiceRepo) SetProjectIfEmpty(ctx context.Context, id, projectID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE services SET project_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND project_id IS NULL`, projectID, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ServiceIDsForCoolify finds the live services of a Coolify resource; with a
// member key (a stack's `<name>-<uuid>`), only that member's container.
func (r *ServiceRepo) ServiceIDsForCoolify(ctx context.Context, resourceUUID, member string) ([]int64, error) {
	q := `SELECT id FROM services WHERE deleted_at IS NULL AND coolify_resource_uuid = ?`
	args := []any{resourceUUID}
	if member != "" {
		q += ` AND discovery_key = ?`
		args = append(args, member)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// MergeContainerDuplicates folds the copies Coolify redeploys left behind —
// container rows on the same host whose names differ only by the deploy
// timestamp — into one survivor (the online one, else the most recently seen)
// and moves their links to it; the copies go to the trash. Survivors get the
// normalized key, so later scans match them. Idempotent. Returns how many
// copies were merged.
func (r *ServiceRepo) MergeContainerDuplicates(ctx context.Context) (int, error) {
	all, err := r.ContainersForCoolify(ctx)
	if err != nil {
		return 0, err
	}
	type gk struct {
		host int64
		key  string
	}
	groups := map[gk][]CoolifyContainer{}
	for _, c := range all {
		if c.HostID == 0 {
			continue
		}
		groups[gk{c.HostID, c.Key}] = append(groups[gk{c.HostID, c.Key}], c)
	}
	merged := 0
	for k, g := range groups {
		sort.SliceStable(g, func(i, j int) bool { return survivorFirst(g[i], g[j]) })
		var dups []int64
		for _, c := range g[1:] {
			dups = append(dups, c.ID)
		}
		if err := r.MergeInto(ctx, g[0].ID, dups, k.key); err != nil {
			return merged, err
		}
		merged += len(dups)
	}
	return merged, nil
}

// survivorFirst orders a duplicate group: online before offline, then scanned
// rows before Coolify placeholders, then most recently seen, then newest id.
func survivorFirst(a, b CoolifyContainer) bool {
	if (a.Status == "online") != (b.Status == "online") {
		return a.Status == "online"
	}
	if a.Placeholder() != b.Placeholder() {
		return !a.Placeholder()
	}
	switch {
	case a.LastSeenAt != nil && b.LastSeenAt == nil:
		return true
	case a.LastSeenAt == nil && b.LastSeenAt != nil:
		return false
	case a.LastSeenAt != nil && !a.LastSeenAt.Equal(*b.LastSeenAt):
		return a.LastSeenAt.After(*b.LastSeenAt)
	}
	return a.ID > b.ID
}

// MergeInto moves every link of dups to survivor (hosts, DNS, dependencies,
// APIs, tools, responsáveis, entidade grants, a project the survivor lacks),
// trashes dups and, when key is set, gives the survivor that discovery key.
// One transaction.
func (r *ServiceRepo) MergeInto(ctx context.Context, survivor int64, dups []int64, key string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, d := range dups {
		for _, q := range []string{
			`INSERT INTO service_host_links (service_id, host_id) SELECT ?, host_id FROM service_host_links WHERE service_id = ? ON CONFLICT DO NOTHING`,
			`INSERT INTO service_dns_links (service_id, dns_id) SELECT ?, dns_id FROM service_dns_links WHERE service_id = ? ON CONFLICT DO NOTHING`,
			`INSERT INTO api_service_links (service_id, api_id) SELECT ?, api_id FROM api_service_links WHERE service_id = ? ON CONFLICT DO NOTHING`,
			`INSERT INTO responsaveis (entity_type, entity_id, contact_id, is_main)
				SELECT 'service', ?, contact_id, is_main FROM responsaveis WHERE entity_type = 'service' AND entity_id = ? ON CONFLICT DO NOTHING`,
		} {
			if _, err := tx.ExecContext(ctx, q, survivor, d); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO service_dependencies (service_id, depends_on_id)
				SELECT ?, depends_on_id FROM service_dependencies WHERE service_id = ? AND depends_on_id <> ? ON CONFLICT DO NOTHING`,
			survivor, d, survivor); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO service_dependencies (service_id, depends_on_id)
				SELECT service_id, ? FROM service_dependencies WHERE depends_on_id = ? AND service_id <> ? ON CONFLICT DO NOTHING`,
			survivor, d, survivor); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE external_tools SET service_id = ? WHERE service_id = ?`, survivor, d); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE services SET project_id = (SELECT project_id FROM services WHERE id = ?) WHERE id = ? AND project_id IS NULL`,
			d, survivor); err != nil {
			return err
		}
		if err := NewAssetEntidadeRepo(r.db).CopyFrom(ctx, tx, AssetService, d, AssetService, survivor); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE services SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, d); err != nil {
			return err
		}
	}
	if key != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE services SET discovery_key = ? WHERE id = ? AND discovery_key <> ?`, key, survivor, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetCoolifyServerUUIDIfEmpty records a host's Coolify server uuid (found by
// IP) unless it already has one.
func (r *HostRepo) SetCoolifyServerUUIDIfEmpty(ctx context.Context, hostID int64, uuid string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE hosts SET coolify_server_uuid = ? WHERE id = ? AND COALESCE(coolify_server_uuid, '') = ''`, uuid, hostID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// AddServiceLink links a DNS record to a service (no-op when linked).
func (r *DNSRepo) AddServiceLink(ctx context.Context, dnsID, serviceID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, serviceID, dnsID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// CoolifyLinks returns every Bridge project ↔ Coolify project/environment mapping.
func (r *ProjectRepo) CoolifyLinks(ctx context.Context) ([]models.ProjectCoolifyLink, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT project_id, coolify_project, coolify_environment FROM project_coolify_links ORDER BY project_id, coolify_project, coolify_environment`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ProjectCoolifyLink
	for rows.Next() {
		var l models.ProjectCoolifyLink
		if err := rows.Scan(&l.ProjectID, &l.CoolifyProject, &l.CoolifyEnvironment); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CoolifyLinksFor returns one project's mappings (never nil).
func (r *ProjectRepo) CoolifyLinksFor(ctx context.Context, projectID int64) ([]models.ProjectCoolifyLink, error) {
	all, err := r.CoolifyLinks(ctx)
	if err != nil {
		return nil, err
	}
	out := []models.ProjectCoolifyLink{}
	for _, l := range all {
		if l.ProjectID == projectID {
			out = append(out, l)
		}
	}
	return out, nil
}

// SetCoolifyLinks replaces a project's Coolify mappings (one tx).
func (r *ProjectRepo) SetCoolifyLinks(ctx context.Context, projectID int64, links []models.ProjectCoolifyLink) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_coolify_links WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_coolify_links (project_id, coolify_project, coolify_environment) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			projectID, l.CoolifyProject, l.CoolifyEnvironment); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// APIAddresses is what an API declares: its main base_url (often the gateway)
// and its extra labelled urls (often the origin).
type APIAddresses struct {
	Base   string
	Extras []string
}

// UnlinkedAPIs lists live APIs with no service links, with every address they
// declare — the API auto-link candidates.
func (r *APICatalogRepo) UnlinkedAPIs(ctx context.Context) (map[int64]APIAddresses, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.base_url, TRUE FROM api_catalog a
		WHERE a.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM api_service_links l WHERE l.api_id = a.id)
		UNION ALL
		SELECT u.api_id, u.url, FALSE FROM api_catalog_urls u JOIN api_catalog a ON a.id = u.api_id
		WHERE a.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM api_service_links l WHERE l.api_id = a.id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]APIAddresses{}
	for rows.Next() {
		var id int64
		var u sql.NullString
		var base bool
		if err := rows.Scan(&id, &u, &base); err != nil {
			return nil, err
		}
		a := out[id]
		if base {
			a.Base = u.String
		} else {
			a.Extras = append(a.Extras, u.String)
		}
		out[id] = a
	}
	return out, rows.Err()
}

// AutoLink links an API to services and, when it has no project yet, to
// projects. Used only for APIs UnlinkedAPIs returned (manual links win).
func (r *APICatalogRepo) AutoLink(ctx context.Context, apiID int64, serviceIDs, projectIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, s := range serviceIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_service_links (api_id, service_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, apiID, s); err != nil {
			return err
		}
	}
	var hasProject bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM api_project_links WHERE api_id = ?)`, apiID).Scan(&hasProject); err != nil {
		return err
	}
	if !hasProject {
		for _, p := range projectIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO api_project_links (api_id, project_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, apiID, p); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// ServiceProjects returns the project of each given service that has one.
func (r *ServiceRepo) ServiceProjects(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range ids {
		var p sql.NullInt64
		err := r.db.QueryRowContext(ctx, `SELECT project_id FROM services WHERE id = ? AND deleted_at IS NULL`, id).Scan(&p)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if p.Valid {
			out[id] = p.Int64
		}
	}
	return out, nil
}
