// Package service is the domain/business-logic layer. A service composes
// repositories (internal/store) and the remaining model helpers into the
// enrichment and cross-entity orchestration that handlers used to do inline.
// Dependency direction: api -> service -> {store, models, database}.
//
// Services are the home for the "list X and attach its tags + responsáveis +
// links" composition that bloated the handlers — the handlers shrink to
// parse → call service → render, and the composition becomes unit-testable
// without HTTP.
package service

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// DNSService owns DNS read enrichment and write orchestration (record + tags +
// host links + responsáveis). It composes the DNS repo, the tag repo, and the
// responsável model helpers (the latter migrate to store in Phase 3).
type DNSService struct {
	db   *sql.DB
	dns  *store.DNSRepo
	tags *store.TagRepo
}

// NewDNSService constructs a DNSService over the given DB handle. (Until the DI
// container is fully populated, it builds the repos it needs.)
func NewDNSService(db *sql.DB) *DNSService {
	return &DNSService{db: db, dns: store.NewDNSRepo(db), tags: store.NewTagRepo(db)}
}

// DNSListItem is an enriched DNS record for the list view.
type DNSListItem struct {
	models.DNSRecord
	Tags                []string `json:"tags"`
	HostIDs             []int64  `json:"host_ids"`
	MainResponsavelName string   `json:"main_responsavel_name"`
}

// DNSDetail is the full single-record view (record + relations).
type DNSDetail struct {
	Record       *models.DNSRecord       `json:"dns_record"`
	Tags         []string                `json:"tags"`
	HostIDs      []int64                 `json:"host_ids"`
	Responsaveis []models.DNSResponsavel `json:"responsaveis"`
}

// DNSWrite carries the create/update payload (record + relations). For update,
// nil relation slices mean "leave unchanged"; for create, len==0 means "none".
type DNSWrite struct {
	Record       models.DNSRecord
	Tags         *[]string
	HostIDs      *[]int64
	Responsaveis *[]models.DNSResponsavelInput
}

// List returns all DNS records enriched with tags, host links, and the main
// responsável name (bulk-loaded to avoid N+1 on the shared maps).
func (s *DNSService) List(ctx context.Context) ([]DNSListItem, error) {
	records, err := s.dns.List(ctx)
	if err != nil {
		return nil, err
	}
	tagMap, err := s.tags.GetAll(ctx, "dns")
	if err != nil {
		return nil, err
	}
	mainNames, err := models.GetDNSMainResponsavelNamesBulk(s.db)
	if err != nil {
		return nil, err
	}
	out := make([]DNSListItem, len(records))
	for i, rec := range records {
		hostIDs, err := s.dns.HostIDs(ctx, rec.ID)
		if err != nil {
			return nil, err
		}
		out[i] = DNSListItem{
			DNSRecord:           rec,
			Tags:                tagMap[rec.ID],
			HostIDs:             hostIDs,
			MainResponsavelName: mainNames[rec.ID],
		}
	}
	return out, nil
}

// Get returns the enriched single record, or (nil, nil) if not found.
func (s *DNSService) Get(ctx context.Context, id int64) (*DNSDetail, error) {
	rec, err := s.dns.Get(ctx, id)
	if err != nil || rec == nil {
		return nil, err
	}
	tags, err := s.tags.Get(ctx, "dns", id)
	if err != nil {
		return nil, err
	}
	hostIDs, err := s.dns.HostIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := models.ListDNSResponsaveis(s.db, id)
	if err != nil {
		return nil, err
	}
	return &DNSDetail{Record: rec, Tags: tags, HostIDs: hostIDs, Responsaveis: resp}, nil
}

// Create inserts the record and applies its relations (only the non-empty ones).
// On success w.Record.ID is set.
func (s *DNSService) Create(ctx context.Context, w *DNSWrite) error {
	if err := s.dns.Create(ctx, &w.Record); err != nil {
		return err
	}
	if w.Tags != nil && len(*w.Tags) > 0 {
		if err := s.tags.Set(ctx, "dns", w.Record.ID, *w.Tags); err != nil {
			return err
		}
	}
	if w.HostIDs != nil && len(*w.HostIDs) > 0 {
		if err := s.dns.SetHostLinks(ctx, w.Record.ID, *w.HostIDs); err != nil {
			return err
		}
	}
	if w.Responsaveis != nil && len(*w.Responsaveis) > 0 {
		if err := models.SyncDNSResponsaveis(s.db, w.Record.ID, *w.Responsaveis); err != nil {
			return err
		}
	}
	return nil
}

// Update writes the record by id and applies any relations whose slice is
// non-nil (nil = leave unchanged). Returns false if the record doesn't exist.
func (s *DNSService) Update(ctx context.Context, id int64, w *DNSWrite) (bool, error) {
	existing, err := s.dns.Get(ctx, id)
	if err != nil || existing == nil {
		return false, err
	}
	w.Record.ID = id
	if err := s.dns.Update(ctx, &w.Record); err != nil {
		return true, err
	}
	if w.Tags != nil {
		if err := s.tags.Set(ctx, "dns", id, *w.Tags); err != nil {
			return true, err
		}
	}
	if w.HostIDs != nil {
		if err := s.dns.SetHostLinks(ctx, id, *w.HostIDs); err != nil {
			return true, err
		}
	}
	if w.Responsaveis != nil {
		if err := models.SyncDNSResponsaveis(s.db, id, *w.Responsaveis); err != nil {
			return true, err
		}
	}
	return true, nil
}

// Delete removes the record and its tags.
func (s *DNSService) Delete(ctx context.Context, id int64) error {
	if err := s.tags.Delete(ctx, "dns", id); err != nil {
		return err
	}
	return s.dns.Delete(ctx, id)
}
