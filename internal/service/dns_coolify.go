package service

import (
	"context"
	"log"

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
	// ServiceLinksAdded counts DNS→service links: the domain's Coolify
	// resource (or stack member) matched a service the Coolify sync stamped.
	ServiceLinksAdded int `json:"service_links_added"`
}

// SyncFromCoolify upserts one DNS record per Coolify domain and links it to
// the Bridge host running it (matched by Coolify server UUID, then by IP =
// hosts.hostname) and to the service running it (the one the Coolify sync
// stamped with the domain's resource, or stack member). It only adds: existing records keep their fields and links,
// gaining the host link if missing. New records inherit the host's entidade
// grants; with no host they have none (admin-only until triaged). Runs
// unscoped — the caller must be admin.
func (s *DNSService) SyncFromCoolify(ctx context.Context, refs []coolify.DomainRef) (CoolifySyncSummary, error) {
	sum := CoolifySyncSummary{Found: len(refs)}
	byUUID, byHostname, err := store.NewHostRepo(s.db).CoolifyIndex(ctx)
	if err != nil {
		return sum, err
	}
	// New records start in whichever situação carries the "active" role.
	activeSituacao, err := store.NewEnumOptionRepo(s.db).SituacaoValue(ctx, store.SituacaoActive)
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

		dnsID, exists, trashed, err := s.dns.IDByDomain(ctx, ref.Domain)
		if err != nil {
			return sum, err
		}
		if trashed {
			// Someone deleted it: don't recreate it or link hosts to it.
			log.Printf("[coolify] dns sync: %s is in the trash, skipped", ref.Domain)
			sum.Existing++
			continue
		}
		if exists {
			sum.Existing++
		} else {
			rec := models.DNSRecord{Domain: ref.Domain, HasHTTPS: ref.HTTPS, Situacao: activeSituacao, Observacoes: "Coolify: " + ref.Source}
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
		if ref.ResourceUUID == "" {
			continue
		}
		svcIDs, err := store.NewServiceRepo(s.db).ServiceIDsForCoolify(ctx, ref.ResourceUUID, ref.Member)
		if err != nil {
			return sum, err
		}
		for _, sid := range svcIDs {
			added, err := s.dns.AddServiceLink(ctx, dnsID, sid)
			if err != nil {
				return sum, err
			}
			if added {
				sum.ServiceLinksAdded++
			}
		}
	}
	return sum, nil
}
