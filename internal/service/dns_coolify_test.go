package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestDNSService_SyncFromCoolify(t *testing.T) {
	ctx := context.Background()
	svc, d := newDNSService(t)

	var hUUID, hIP, entID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, coolify_server_uuid) VALUES ('a','a','10.0.0.1','srv-a') RETURNING id`).Scan(&hUUID); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES ('b','b','10.0.0.2') RETURNING id`).Scan(&hIP); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`SELECT id FROM entidades ORDER BY id LIMIT 1`).Scan(&entID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`DELETE FROM asset_entidades WHERE asset_type = 'host' AND asset_id = ?`, hUUID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES ('host', ?, ?, 'creator')`, hUUID, entID); err != nil {
		t.Fatal(err)
	}
	// A pre-existing record (different case) with no links.
	pre := &service.DNSWrite{Record: models.DNSRecord{Domain: "Old.x.gov.br", Situacao: "active"}}
	if err := svc.Create(ctx, pre); err != nil {
		t.Fatal(err)
	}

	refs := []coolify.DomainRef{
		{Domain: "a.x.gov.br", HTTPS: true, ServerUUID: "srv-a", ServerIP: "9.9.9.9", Source: "app-a"},
		{Domain: "b.x.gov.br", ServerUUID: "unknown", ServerIP: "10.0.0.2", Source: "app-b"},
		{Domain: "old.x.gov.br", ServerUUID: "srv-a", Source: "app-old"},
		{Domain: "orphan.sslip.io", ServerUUID: "nope", ServerIP: "1.2.3.4", Source: "app-o"},
	}
	sum, err := svc.SyncFromCoolify(ctx, refs)
	if err != nil {
		t.Fatal(err)
	}
	want := service.CoolifySyncSummary{Found: 4, Created: 3, Existing: 1, LinksAdded: 3, NoHost: 1}
	if sum != want {
		t.Fatalf("first sync = %+v, want %+v", sum, want)
	}

	dns := store.NewDNSRepo(d.SQL)
	linked := func(domain string) []int64 {
		id, ok, err := dns.IDByDomain(ctx, domain)
		if err != nil || !ok {
			t.Fatalf("IDByDomain(%s) = %v, %v", domain, ok, err)
		}
		ids, _ := dns.HostIDs(ctx, id)
		return ids
	}
	if got := linked("a.x.gov.br"); len(got) != 1 || got[0] != hUUID {
		t.Fatalf("a links = %v, want [%d] (by UUID)", got, hUUID)
	}
	if got := linked("b.x.gov.br"); len(got) != 1 || got[0] != hIP {
		t.Fatalf("b links = %v, want [%d] (by IP)", got, hIP)
	}
	if got := linked("old.x.gov.br"); len(got) != 1 || got[0] != hUUID {
		t.Fatalf("existing record links = %v, want [%d]", got, hUUID)
	}
	if got := linked("orphan.sslip.io"); len(got) != 0 {
		t.Fatalf("orphan links = %v, want none", got)
	}

	aID, _, _ := dns.IDByDomain(ctx, "a.x.gov.br")
	detail, err := svc.Get(ctx, aID)
	if err != nil || detail == nil {
		t.Fatalf("get: %+v %v", detail, err)
	}
	if !detail.Record.HasHTTPS || detail.Record.Situacao != "active" || detail.Record.Observacoes != "Coolify: app-a" {
		t.Fatalf("created record = %+v", detail.Record)
	}
	if detail.Entidades.CreatorEntidadeID == nil || *detail.Entidades.CreatorEntidadeID != entID {
		t.Fatalf("grants = %+v, want creator %d copied from host", detail.Entidades, entID)
	}

	sum, err = svc.SyncFromCoolify(ctx, refs)
	if err != nil {
		t.Fatal(err)
	}
	want = service.CoolifySyncSummary{Found: 4, Existing: 4, NoHost: 1}
	if sum != want {
		t.Fatalf("second sync = %+v, want %+v", sum, want)
	}
}

func TestDNSService_ServiceAndProjectLinks(t *testing.T) {
	ctx := context.Background()
	svc, d := newDNSService(t)

	var projID, svcID int64
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p') RETURNING id`).Scan(&projID); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO services (nickname) VALUES ('s') RETURNING id`).Scan(&svcID); err != nil {
		t.Fatal(err)
	}

	// Create without links: detail returns empty arrays, not nil.
	w := &service.DNSWrite{Record: models.DNSRecord{Domain: "l.x.gov.br", Situacao: "active"}}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(ctx, w.Record.ID)
	if got.ServiceIDs == nil || got.ProjectIDs == nil || len(got.ServiceIDs)+len(got.ProjectIDs) != 0 {
		t.Fatalf("empty links = %v %v, want [] []", got.ServiceIDs, got.ProjectIDs)
	}

	sids, pids := []int64{svcID}, []int64{projID}
	if _, err := svc.Update(ctx, w.Record.ID, &service.DNSWrite{Record: w.Record, ServiceIDs: &sids, ProjectIDs: &pids}); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, w.Record.ID)
	if len(got.ServiceIDs) != 1 || got.ServiceIDs[0] != svcID || len(got.ProjectIDs) != 1 || got.ProjectIDs[0] != projID {
		t.Fatalf("links = %v %v", got.ServiceIDs, got.ProjectIDs)
	}

	// nil = unchanged.
	if _, err := svc.Update(ctx, w.Record.ID, &service.DNSWrite{Record: w.Record}); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, w.Record.ID)
	if len(got.ServiceIDs) != 1 || len(got.ProjectIDs) != 1 {
		t.Fatalf("nil update changed links: %v %v", got.ServiceIDs, got.ProjectIDs)
	}

	// Project detail = direct ∪ via services, deduped.
	var other int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('via.x.gov.br') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`UPDATE services SET project_id = ? WHERE id = ?`, projID, svcID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?)`, svcID, other); err != nil {
		t.Fatal(err)
	}
	pd, err := service.NewProjectService(d.SQL).Get(ctx, projID)
	if err != nil || pd == nil {
		t.Fatalf("project get: %v", err)
	}
	// w.Record is linked both directly and via the service — appears once.
	if len(pd.DNSIDs) != 2 || pd.DNSIDs[0] != w.Record.ID || pd.DNSIDs[1] != other {
		t.Fatalf("project dns_ids = %v, want [%d %d]", pd.DNSIDs, w.Record.ID, other)
	}
}
