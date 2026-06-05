package service

import (
	"context"
	"database/sql"
	"sort"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// ProjectService owns project read enrichment and write orchestration (project
// + tags + responsáveis + the host/dns aggregation derived from the project's
// services, plus cascade-aware delete). It composes the project repo, the tag
// repo, and the responsável/service model helpers (the latter migrate to store
// in a later phase).
type ProjectService struct {
	db       *sql.DB
	projects *store.ProjectRepo
	services *store.ServiceRepo
	tags     *store.TagRepo
}

// NewProjectService constructs a ProjectService over the given DB handle.
func NewProjectService(db *sql.DB) *ProjectService {
	return &ProjectService{
		db:       db,
		projects: store.NewProjectRepo(db),
		services: store.NewServiceRepo(db),
		tags:     store.NewTagRepo(db),
	}
}

// ProjectListItem is an enriched project for the list view.
type ProjectListItem struct {
	models.Project
	Tags                []string `json:"tags"`
	MainResponsavelName string   `json:"main_responsavel_name"`
}

// ProjectDetail is the full single-project view (project + relations). HostIDs
// and DNSIDs are aggregated (deduplicated, sorted) across the project's services.
type ProjectDetail struct {
	Project      *models.Project                    `json:"project"`
	Tags         []string                           `json:"tags"`
	Responsaveis []models.ProjectResponsavelContact `json:"responsaveis"`
	Services     []models.Service                   `json:"services"`
	HostIDs      []int64                            `json:"host_ids"`
	DNSIDs       []int64                            `json:"dns_ids"`
}

// ProjectWrite carries the create/update payload (project + relations). For
// update, nil relation slices mean "leave unchanged"; for create, a nil or
// empty slice means "none".
type ProjectWrite struct {
	Project      models.Project
	Tags         *[]string
	Responsaveis *[]models.ProjectResponsavelInput
}

// List returns all projects enriched with tags and the main responsável name
// (bulk-loaded to avoid N+1 on the shared maps).
func (s *ProjectService) List(ctx context.Context) ([]ProjectListItem, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, err
	}
	tagMap, err := s.tags.GetAll(ctx, "project")
	if err != nil {
		return nil, err
	}
	mainNames, err := models.GetProjectMainResponsavelNamesBulk(s.db)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectListItem, len(projects))
	for i, p := range projects {
		out[i] = ProjectListItem{
			Project:             p,
			Tags:                tagMap[p.ID],
			MainResponsavelName: mainNames[p.ID],
		}
	}
	return out, nil
}

// Get returns the enriched single project, or (nil, nil) if not found.
func (s *ProjectService) Get(ctx context.Context, id int64) (*ProjectDetail, error) {
	p, err := s.projects.Get(ctx, id)
	if err != nil || p == nil {
		return nil, err
	}
	tags, err := s.tags.Get(ctx, "project", id)
	if err != nil {
		return nil, err
	}
	resp, err := models.ListProjectResponsaveisContact(s.db, id)
	if err != nil {
		return nil, err
	}
	services, err := s.services.ListByProject(ctx, id)
	if err != nil {
		return nil, err
	}
	hostIDs, dnsIDs, err := s.aggregateServiceLinks(ctx, services)
	if err != nil {
		return nil, err
	}
	return &ProjectDetail{
		Project:      p,
		Tags:         tags,
		Responsaveis: resp,
		Services:     services,
		HostIDs:      hostIDs,
		DNSIDs:       dnsIDs,
	}, nil
}

// aggregateServiceLinks collects the deduplicated, sorted host and dns ids
// reachable through a project's services. Unlike the old handler (which
// swallowed each lookup error with `_`), it propagates failures.
func (s *ProjectService) aggregateServiceLinks(ctx context.Context, services []models.Service) (hostIDs, dnsIDs []int64, err error) {
	hostSet := map[int64]struct{}{}
	dnsSet := map[int64]struct{}{}
	for _, svc := range services {
		hids, err := s.services.HostIDs(ctx, svc.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, hid := range hids {
			hostSet[hid] = struct{}{}
		}
		dids, err := s.services.DNSIDs(ctx, svc.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, did := range dids {
			dnsSet[did] = struct{}{}
		}
	}
	return sortedKeys(hostSet), sortedKeys(dnsSet), nil
}

func sortedKeys(set map[int64]struct{}) []int64 {
	if len(set) == 0 {
		return nil
	}
	out := make([]int64, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Create inserts the project and applies its relations (only the non-empty ones).
// On success w.Project.ID is set.
func (s *ProjectService) Create(ctx context.Context, w *ProjectWrite) error {
	if err := s.projects.Create(ctx, &w.Project); err != nil {
		return err
	}
	if w.Tags != nil && len(*w.Tags) > 0 {
		if err := s.tags.Set(ctx, "project", w.Project.ID, *w.Tags); err != nil {
			return err
		}
	}
	if w.Responsaveis != nil && len(*w.Responsaveis) > 0 {
		if err := models.SyncProjectResponsaveisContact(s.db, w.Project.ID, *w.Responsaveis); err != nil {
			return err
		}
	}
	return nil
}

// Update writes the project by id and applies any relations whose slice is
// non-nil (nil = leave unchanged). Returns false if the project doesn't exist.
func (s *ProjectService) Update(ctx context.Context, id int64, w *ProjectWrite) (bool, error) {
	existing, err := s.projects.Get(ctx, id)
	if err != nil || existing == nil {
		return false, err
	}
	w.Project.ID = id
	if err := s.projects.Update(ctx, &w.Project); err != nil {
		return true, err
	}
	if w.Tags != nil {
		if err := s.tags.Set(ctx, "project", id, *w.Tags); err != nil {
			return true, err
		}
	}
	if w.Responsaveis != nil {
		if err := models.SyncProjectResponsaveisContact(s.db, id, *w.Responsaveis); err != nil {
			return true, err
		}
	}
	return true, nil
}

// Delete removes the project's tags then cascade-deletes the project (and any
// vault secrets scoped to it) via the store cascade registry, atomically.
func (s *ProjectService) Delete(ctx context.Context, actor models.ActorContext, id int64) error {
	if err := s.tags.Delete(ctx, "project", id); err != nil {
		return err
	}
	return store.DeleteParent(ctx, s.db, actor, models.SecretScopeProjeto, id)
}
