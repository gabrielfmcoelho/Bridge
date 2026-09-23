package service

import (
	"context"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// CoolifySyncSummary is what one Coolify → DNS sync did.
type CoolifySyncSummary struct {
	Found      int `json:"found"`
	Created    int `json:"created"`
	Existing   int `json:"existing"`
	LinksAdded int `json:"links_added"`
	NoHost     int `json:"no_host"`
}

// SyncFromCoolify upserts one DNS record per Coolify domain and links it to
// the Bridge host running it (matched by Coolify server UUID, then by IP =
// hosts.hostname). It only adds: existing records keep their fields and links,
// gaining the host link if missing. New records inherit the host's entidade
// grants; with no host they have none (admin-only until triaged). Runs
// unscoped — the caller must be admin.
func (s *DNSService) SyncFromCoolify(ctx context.Context, refs []coolify.DomainRef) (CoolifySyncSummary, error) {
	sum := CoolifySyncSummary{Found: len(refs)}
	byUUID, byHostname, err := store.NewHostRepo(s.db).CoolifyIndex(ctx)
	if err != nil {
		return sum, err
	}
	for _, ref := range refs {
		hostID, ok := byUUID[ref.ServerUUID]
		if !ok || ref.ServerUUID == "" {
			hostID, ok = byHostname[ref.ServerIP]
			ok = ok && ref.ServerIP != ""
		}
		if !ok {
			sum.NoHost++
		}

		dnsID, exists, err := s.dns.IDByDomain(ctx, ref.Domain)
		if err != nil {
			return sum, err
		}
		if exists {
			sum.Existing++
		} else {
			rec := models.DNSRecord{Domain: ref.Domain, HasHTTPS: ref.HTTPS, Situacao: "active", Observacoes: "Coolify: " + ref.Source}
			if err := s.dns.Create(ctx, &rec); err != nil {
				return sum, err
			}
			dnsID = rec.ID
			sum.Created++
			if ok {
				if err := s.grants.CopyFrom(ctx, s.db, store.AssetHost, hostID, store.AssetDNS, dnsID); err != nil {
					return sum, err
				}
			}
		}
		if ok {
			added, err := s.dns.AddHostLink(ctx, dnsID, hostID)
			if err != nil {
				return sum, err
			}
			if added {
				sum.LinksAdded++
			}
		}
	}
	return sum, nil
}
