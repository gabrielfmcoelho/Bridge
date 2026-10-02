package service

import (
	"context"
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// CoolifyInventorySummary is what one Coolify inventory sync did.
type CoolifyInventorySummary struct {
	Resources        int      `json:"resources"`
	Matched          int      `json:"matched"`           // resources with at least one scanned container
	Created          int      `json:"created"`           // placeholders for resources no scan has seen
	Updated          int      `json:"updated"`           // services whose Coolify fields changed
	MergedDuplicates int      `json:"merged_duplicates"` // redeploy copies folded into their live row
	ProjectsSet      int      `json:"projects_set"`      // services given a project by the mapping
	HostsUUIDFilled  int      `json:"hosts_uuid_filled"` // hosts that gained their Coolify server uuid (by IP)
	APIsLinked       int      `json:"apis_linked"`       // APIs linked to services through their URL's DNS
	Unmatched        []string `json:"unmatched"`         // resources whose server has no Bridge host
}

// CoolifyInventoryService stamps Coolify's view of each resource onto the
// Bridge services running it.
type CoolifyInventoryService struct {
	db       *sql.DB
	services *store.ServiceRepo
	hosts    *store.HostRepo
	projects *store.ProjectRepo
	dns      *store.DNSRepo
	apis     *store.APICatalogRepo
}

// NewCoolifyInventoryService wires the repos.
func NewCoolifyInventoryService(db *sql.DB) *CoolifyInventoryService {
	return &CoolifyInventoryService{db: db, services: store.NewServiceRepo(db), hosts: store.NewHostRepo(db),
		projects: store.NewProjectRepo(db), dns: store.NewDNSRepo(db), apis: store.NewAPICatalogRepo(db)}
}

// Sync, unscoped (the caller must be admin):
//  1. folds redeploy copies into their live row;
//  2. per resource, stamps project/environment/stack/repo on the containers
//     carrying its uuid, or creates offline placeholders on its host when no
//     scan has seen it (a stack: one per member) — a placeholder is folded into
//     the real row once a scan finds the container;
//  3. fills hosts' Coolify server uuid found by IP;
//  4. gives unassigned services the Bridge project mapped to their Coolify
//     project/environment (exact environment first, then "every environment").
//
// DNS→service links are the DNS sync's job (SyncFromCoolify), run after this
// so it finds the stamped services; LinkAPIs runs after that, so it finds
// the DNS→service links.
func (s *CoolifyInventoryService) Sync(ctx context.Context, resources []coolify.Resource) (CoolifyInventorySummary, error) {
	sum := CoolifyInventorySummary{Resources: len(resources), Unmatched: []string{}}
	merged, err := s.services.MergeContainerDuplicates(ctx)
	if err != nil {
		return sum, err
	}
	sum.MergedDuplicates = merged

	containers, err := s.services.ContainersForCoolify(ctx)
	if err != nil {
		return sum, err
	}
	byUUID := map[string][]store.CoolifyContainer{}
	for _, c := range containers {
		if c.ResourceUUID != "" {
			byUUID[c.ResourceUUID] = append(byUUID[c.ResourceUUID], c)
		}
	}
	hostByUUID, hostByIP, err := s.hosts.CoolifyIndex(ctx)
	if err != nil {
		return sum, err
	}
	mapping, err := s.projects.CoolifyLinks(ctx)
	if err != nil {
		return sum, err
	}

	for _, r := range resources {
		f := store.CoolifyFields{ResourceUUID: r.UUID, ResourceType: r.Type, Project: r.Project, Environment: r.Environment,
			Stack: r.Name, GitRepository: r.GitRepository, GitBranch: r.GitBranch}

		hostID, found := hostByUUID[r.Server.UUID]
		if !found || r.Server.UUID == "" {
			hostID, found = hostByIP[r.Server.IP]
			found = found && r.Server.IP != ""
			if found && r.Server.UUID != "" {
				filled, err := s.hosts.SetCoolifyServerUUIDIfEmpty(ctx, hostID, r.Server.UUID)
				if err != nil {
					return sum, err
				}
				if filled {
					sum.HostsUUIDFilled++
					hostByUUID[r.Server.UUID] = hostID
				}
			}
		}

		var ids []int64
		matches := byUUID[r.UUID]
		var real, placeholders []store.CoolifyContainer
		for _, c := range matches {
			if c.Placeholder() {
				placeholders = append(placeholders, c)
			} else {
				real = append(real, c)
			}
		}
		switch {
		case len(real) > 0:
			sum.Matched++
			for _, c := range real {
				ids = append(ids, c.ID)
			}
			if len(placeholders) > 0 {
				var dups []int64
				for _, p := range placeholders {
					dups = append(dups, p.ID)
				}
				if err := s.services.MergeInto(ctx, real[0].ID, dups, ""); err != nil {
					return sum, err
				}
			}
		case len(placeholders) > 0:
			for _, p := range placeholders {
				ids = append(ids, p.ID)
			}
		case !found:
			sum.Unmatched = append(sum.Unmatched, r.Name)
			continue
		default:
			keys := r.Members
			if len(keys) == 0 {
				keys = []string{r.UUID}
			}
			for _, key := range keys {
				nickname := r.Name
				if len(r.Members) > 0 {
					nickname = r.Name + " · " + strings.TrimSuffix(key, "-"+r.UUID)
				}
				id, err := s.services.CreateCoolifyPlaceholder(ctx, hostID, key, nickname, f)
				if err != nil {
					return sum, err
				}
				sum.Created++
				ids = append(ids, id)
			}
		}

		projectID := mappedProject(mapping, r.Project, r.Environment)
		for _, id := range ids {
			changed, err := s.services.SetCoolifyFields(ctx, id, f)
			if err != nil {
				return sum, err
			}
			if changed {
				sum.Updated++
			}
			if projectID != 0 {
				set, err := s.services.SetProjectIfEmpty(ctx, id, projectID)
				if err != nil {
					return sum, err
				}
				if set {
					sum.ProjectsSet++
				}
			}
		}
	}

	return sum, nil
}

// mappedProject is the Bridge project for a Coolify project/environment: an
// exact environment mapping wins over a project-wide one ("" environment).
// 0 = unmapped.
func mappedProject(links []models.ProjectCoolifyLink, project, env string) int64 {
	var wide int64
	for _, l := range links {
		if l.CoolifyProject != project || project == "" {
			continue
		}
		if l.CoolifyEnvironment == env {
			return l.ProjectID
		}
		if l.CoolifyEnvironment == "" && wide == 0 {
			wide = l.ProjectID
		}
	}
	return wide
}

// LinkAPIs links each API that has no services to the services behind its
// addresses (URL host → DNS record → linked services) and, when it has no
// project, to those services' projects. The extra urls win over base_url:
// base_url is usually the gateway, whose DNS leads to the gateway container
// shared by every API, while the extras name the origin. base_url counts only
// when no extra resolves. Run it after the DNS sync.
func (s *CoolifyInventoryService) LinkAPIs(ctx context.Context) (int, error) {
	unlinked, err := s.apis.UnlinkedAPIs(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for apiID, addrs := range unlinked {
		svcSet, err := s.servicesBehind(ctx, addrs.Extras)
		if err != nil {
			return n, err
		}
		if len(svcSet) == 0 {
			if svcSet, err = s.servicesBehind(ctx, []string{addrs.Base}); err != nil {
				return n, err
			}
		}
		if len(svcSet) == 0 {
			continue
		}
		svcIDs := make([]int64, 0, len(svcSet))
		for id := range svcSet {
			svcIDs = append(svcIDs, id)
		}
		sort.Slice(svcIDs, func(i, j int) bool { return svcIDs[i] < svcIDs[j] })
		projOf, err := s.services.ServiceProjects(ctx, svcIDs)
		if err != nil {
			return n, err
		}
		projSet := map[int64]bool{}
		var projIDs []int64
		for _, id := range svcIDs {
			if p, ok := projOf[id]; ok && !projSet[p] {
				projSet[p] = true
				projIDs = append(projIDs, p)
			}
		}
		if err := s.apis.AutoLink(ctx, apiID, svcIDs, projIDs); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func urlHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// servicesBehind is the set of services linked to the DNS records of urls' hosts.
func (s *CoolifyInventoryService) servicesBehind(ctx context.Context, urls []string) (map[int64]bool, error) {
	svcSet := map[int64]bool{}
	for _, raw := range urls {
		host := urlHost(raw)
		if host == "" {
			continue
		}
		dnsID, found, trashed, err := s.dns.IDByDomain(ctx, host)
		if err != nil {
			return nil, err
		}
		if !found || trashed {
			continue
		}
		ids, err := s.dns.ServiceIDs(ctx, dnsID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			svcSet[id] = true
		}
	}
	return svcSet, nil
}
