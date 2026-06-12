package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Sentinel errors for service lifecycle rules. Handlers map these to 4xx; the
// service layer itself stays HTTP-agnostic.
var (
	// ErrServiceNotAuto is returned by Fixate for a non-auto service.
	ErrServiceNotAuto = errors.New("only auto-discovered services can be fixated")
	// ErrServiceCannotRebindAuto is returned by UpdateContainer for an auto service.
	ErrServiceCannotRebindAuto = errors.New("cannot rebind auto services; fixate first")
)

// ServiceService owns service read enrichment and write orchestration (service
// + tags + host/dns/dependency links + responsáveis, plus cascade-aware delete
// and the auto/fixed lifecycle rules). It composes the service repo, the tag
// repo, and the responsável model helpers (the latter migrate to store later).
type ServiceService struct {
	db           *sql.DB
	services     *store.ServiceRepo
	tags         *store.TagRepo
	responsaveis *store.ResponsavelRepo
}

// NewServiceService constructs a ServiceService over the given DB handle.
func NewServiceService(db *sql.DB) *ServiceService {
	return &ServiceService{
		db:           db,
		services:     store.NewServiceRepo(db),
		tags:         store.NewTagRepo(db),
		responsaveis: store.NewResponsavelRepo(db),
	}
}

// ServiceListItem is an enriched service for the list view.
type ServiceListItem struct {
	models.Service
	Tags                []string `json:"tags"`
	HostIDs             []int64  `json:"host_ids"`
	DNSIDs              []int64  `json:"dns_ids"`
	DependsOnIDs        []int64  `json:"depends_on_ids"`
	MainResponsavelName string   `json:"main_responsavel_name"`
}

// ServiceDetail is the full single-service view (service + relations).
type ServiceDetail struct {
	Service      *models.Service      `json:"service"`
	Tags         []string             `json:"tags"`
	HostIDs      []int64              `json:"host_ids"`
	DNSIDs       []int64              `json:"dns_ids"`
	DependsOnIDs []int64              `json:"depends_on_ids"`
	DependentIDs []int64              `json:"dependent_ids"`
	Responsaveis []models.Responsavel `json:"responsaveis"`
}

// ServiceWrite carries the create/update payload (service + relations). For
// update, nil relation slices mean "leave unchanged"; for create, a nil or
// empty slice means "none".
type ServiceWrite struct {
	Service      models.Service
	Tags         *[]string
	HostIDs      *[]int64
	DNSIDs       *[]int64
	DependsOnIDs *[]int64
	Responsaveis *[]models.ResponsavelInput
}

// List returns the services matching the filter (server-side filter/sort/
// pagination), enriched with tags, link ids, and the main responsável name. The
// tag and main-name maps are bulk-by-id, so enriching just the returned page is
// fine.
func (s *ServiceService) List(ctx context.Context, f models.ServiceFilter) ([]ServiceListItem, error) {
	services, err := s.services.ListFiltered(ctx, f)
	if err != nil {
		return nil, err
	}
	tagMap, err := s.tags.GetAll(ctx, "service")
	if err != nil {
		return nil, err
	}
	mainNames, err := s.responsaveis.MainNamesBulk(ctx, "service")
	if err != nil {
		return nil, err
	}
	out := make([]ServiceListItem, len(services))
	for i, svc := range services {
		hostIDs, err := s.services.HostIDs(ctx, svc.ID)
		if err != nil {
			return nil, err
		}
		dnsIDs, err := s.services.DNSIDs(ctx, svc.ID)
		if err != nil {
			return nil, err
		}
		depIDs, err := s.services.DependencyIDs(ctx, svc.ID)
		if err != nil {
			return nil, err
		}
		out[i] = ServiceListItem{
			Service:             svc,
			Tags:                tagMap[svc.ID],
			HostIDs:             hostIDs,
			DNSIDs:              dnsIDs,
			DependsOnIDs:        depIDs,
			MainResponsavelName: mainNames[svc.ID],
		}
	}
	return out, nil
}

// Count returns the number of services matching the filter (the predicates the
// list view paginates over).
func (s *ServiceService) Count(ctx context.Context, f models.ServiceFilter) (int, error) {
	return s.services.CountFiltered(ctx, f)
}

// Get returns the enriched single service, or (nil, nil) if not found.
func (s *ServiceService) Get(ctx context.Context, id int64) (*ServiceDetail, error) {
	svc, err := s.services.Get(ctx, id)
	if err != nil || svc == nil {
		return nil, err
	}
	tags, err := s.tags.Get(ctx, "service", id)
	if err != nil {
		return nil, err
	}
	hostIDs, err := s.services.HostIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	dnsIDs, err := s.services.DNSIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	dependsOn, err := s.services.DependencyIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	dependents, err := s.services.DependentIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := s.responsaveis.List(ctx, "service", id)
	if err != nil {
		return nil, err
	}
	return &ServiceDetail{
		Service:      svc,
		Tags:         tags,
		HostIDs:      hostIDs,
		DNSIDs:       dnsIDs,
		DependsOnIDs: dependsOn,
		DependentIDs: dependents,
		Responsaveis: resp,
	}, nil
}

// Create inserts the service and applies its relations (only the non-empty ones).
// On success w.Service.ID is set.
func (s *ServiceService) Create(ctx context.Context, w *ServiceWrite) error {
	if err := s.services.Create(ctx, &w.Service); err != nil {
		return err
	}
	id := w.Service.ID
	if w.Tags != nil && len(*w.Tags) > 0 {
		if err := s.tags.Set(ctx, "service", id, *w.Tags); err != nil {
			return err
		}
	}
	if w.HostIDs != nil && len(*w.HostIDs) > 0 {
		if err := s.services.SetHostLinks(ctx, id, *w.HostIDs); err != nil {
			return err
		}
	}
	if w.DNSIDs != nil && len(*w.DNSIDs) > 0 {
		if err := s.services.SetDNSLinks(ctx, id, *w.DNSIDs); err != nil {
			return err
		}
	}
	if w.DependsOnIDs != nil && len(*w.DependsOnIDs) > 0 {
		if err := s.services.SetDependencies(ctx, id, *w.DependsOnIDs); err != nil {
			return err
		}
	}
	if w.Responsaveis != nil && len(*w.Responsaveis) > 0 {
		if err := s.responsaveis.Sync(ctx, "service", id, *w.Responsaveis); err != nil {
			return err
		}
	}
	return nil
}

// Update writes the service by id and applies any relations whose slice is
// non-nil (nil = leave unchanged). Returns false if the service doesn't exist.
func (s *ServiceService) Update(ctx context.Context, id int64, w *ServiceWrite) (bool, error) {
	existing, err := s.services.Get(ctx, id)
	if err != nil || existing == nil {
		return false, err
	}
	w.Service.ID = id
	if err := s.services.Update(ctx, &w.Service); err != nil {
		return true, err
	}
	if w.Tags != nil {
		if err := s.tags.Set(ctx, "service", id, *w.Tags); err != nil {
			return true, err
		}
	}
	if w.HostIDs != nil {
		if err := s.services.SetHostLinks(ctx, id, *w.HostIDs); err != nil {
			return true, err
		}
	}
	if w.DNSIDs != nil {
		if err := s.services.SetDNSLinks(ctx, id, *w.DNSIDs); err != nil {
			return true, err
		}
	}
	if w.DependsOnIDs != nil {
		if err := s.services.SetDependencies(ctx, id, *w.DependsOnIDs); err != nil {
			return true, err
		}
	}
	if w.Responsaveis != nil {
		if err := s.responsaveis.Sync(ctx, "service", id, *w.Responsaveis); err != nil {
			return true, err
		}
	}
	return true, nil
}

// Delete removes the service's tags then cascade-deletes the service (and any
// vault secrets scoped to it) via the store cascade registry, atomically.
func (s *ServiceService) Delete(ctx context.Context, actor models.ActorContext, id int64) error {
	if err := s.tags.Delete(ctx, "service", id); err != nil {
		return err
	}
	return store.DeleteParent(ctx, s.db, actor, models.SecretScopeService, id)
}

// Fixate converts an auto-discovered service to a fixed one. Returns (nil, nil)
// if the service does not exist, ErrServiceNotAuto if it is not auto, otherwise
// the updated service.
func (s *ServiceService) Fixate(ctx context.Context, id int64) (*models.Service, error) {
	svc, err := s.services.Get(ctx, id)
	if err != nil || svc == nil {
		return nil, err
	}
	if svc.Source != "auto" {
		return nil, ErrServiceNotAuto
	}
	if err := s.services.Fixate(ctx, id); err != nil {
		return nil, err
	}
	svc.Source = "fixed"
	return svc, nil
}

// UpdateContainer rebinds a fixed/manual service to a different container.
// Returns (nil, nil) if the service does not exist, ErrServiceCannotRebindAuto
// if it is still auto, otherwise the updated service.
func (s *ServiceService) UpdateContainer(ctx context.Context, id int64, containerName, containerID string) (*models.Service, error) {
	svc, err := s.services.Get(ctx, id)
	if err != nil || svc == nil {
		return nil, err
	}
	if svc.Source == "auto" {
		return nil, ErrServiceCannotRebindAuto
	}
	if err := s.services.UpdateContainerBinding(ctx, id, containerName, containerID); err != nil {
		return nil, err
	}
	svc.ContainerName = containerName
	svc.ContainerID = containerID
	return svc, nil
}

// ListTrash returns soft-deleted services (the trash view).
func (s *ServiceService) ListTrash(ctx context.Context) ([]models.Service, error) {
	return s.services.ListTrash(ctx)
}

// Restore un-deletes a service (and cascade-restores its secrets) by id.
func (s *ServiceService) Restore(ctx context.Context, actor models.ActorContext, id int64) error {
	return store.RestoreParent(ctx, s.db, actor, models.SecretScopeService, id)
}
