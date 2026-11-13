package service_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A DNS record shared by two sibling entidades links hosts, services and
// projects of both. Each side sees only its own ids (and counts) in the
// record's detail and list row, and saving the record from one side keeps the
// other side's links, which its picker never showed.
func TestDNSService_LinkedIDsRespectScope(t *testing.T) {
	bg := context.Background()
	svc, d := newDNSService(t)
	grants := store.NewAssetEntidadeRepo(d.SQL)
	ent := func(slug string) int64 {
		t.Helper()
		var id int64
		if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = ?`, slug).Scan(&id); err != nil {
			t.Fatalf("entidade %s: %v", slug, err)
		}
		return id
	}
	sga, sgp := ent("sga"), ent("sgp")
	q := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := d.SQL.QueryRow(sql, args...).Scan(&id); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return id
	}
	grant := func(a store.AssetType, id int64, g models.AssetGrants) {
		t.Helper()
		if err := grants.Replace(bg, d.SQL, a, id, g); err != nil {
			t.Fatalf("grant: %v", err)
		}
	}
	only := func(e int64) models.AssetGrants { return models.AssetGrants{CreatorEntidadeID: &e} }

	rec := q(`INSERT INTO dns_records (domain) VALUES ('shared.pi.gov.br') RETURNING id`)
	grant(store.AssetDNS, rec, models.AssetGrants{CreatorEntidadeID: &sga, ResponsibleEntidadeIDs: []int64{sgp}})
	hostA := q(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('a', 'a') RETURNING id`)
	hostB := q(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('b', 'b') RETURNING id`)
	svcA := q(`INSERT INTO services (nickname) VALUES ('sa') RETURNING id`)
	svcB := q(`INSERT INTO services (nickname) VALUES ('sb') RETURNING id`)
	projA := q(`INSERT INTO projects (name) VALUES ('pa') RETURNING id`)
	projB := q(`INSERT INTO projects (name) VALUES ('pb') RETURNING id`)
	grant(store.AssetHost, hostA, only(sga))
	grant(store.AssetHost, hostB, only(sgp))
	grant(store.AssetService, svcA, only(sga))
	grant(store.AssetService, svcB, only(sgp))
	grant(store.AssetProject, projA, only(sga))
	grant(store.AssetProject, projB, only(sgp))
	dns := store.NewDNSRepo(d.SQL)
	for _, err := range []error{
		dns.SetHostLinks(bg, rec, []int64{hostA, hostB}),
		dns.SetServiceLinks(bg, rec, []int64{svcA, svcB}),
		dns.SetProjectLinks(bg, rec, []int64{projA, projB}),
	} {
		if err != nil {
			t.Fatalf("seed links: %v", err)
		}
	}

	asSGA := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})
	det, err := svc.Get(asSGA, rec)
	if err != nil || det == nil {
		t.Fatalf("Get = %v, %v", det, err)
	}
	if !reflect.DeepEqual(det.HostIDs, []int64{hostA}) || !reflect.DeepEqual(det.ServiceIDs, []int64{svcA}) || !reflect.DeepEqual(det.ProjectIDs, []int64{projA}) {
		t.Fatalf("SGA detail hosts=%v services=%v projects=%v; want only SGA's %d/%d/%d", det.HostIDs, det.ServiceIDs, det.ProjectIDs, hostA, svcA, projA)
	}
	list, err := svc.List(asSGA, models.DNSFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if row := list[0]; !reflect.DeepEqual(row.HostIDs, []int64{hostA}) || row.ServicesCount != 1 || row.ProjectsCount != 1 {
		t.Fatalf("SGA list row hosts=%v services=%d projects=%d; want [%d]/1/1", row.HostIDs, row.ServicesCount, row.ProjectsCount, hostA)
	}

	// SGA saves the record clearing every link it sees: SGP's links stay.
	none := []int64{}
	if _, err := svc.Update(asSGA, rec, &service.DNSWrite{Record: models.DNSRecord{Domain: "shared.pi.gov.br"}, HostIDs: &none, ServiceIDs: &none}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	full, _ := svc.Get(bg, rec)
	if !reflect.DeepEqual(full.HostIDs, []int64{hostB}) || !reflect.DeepEqual(full.ServiceIDs, []int64{svcB}) {
		t.Fatalf("after SGA save, unscoped hosts=%v services=%v; want SGP's [%d]/[%d] kept", full.HostIDs, full.ServiceIDs, hostB, svcB)
	}
}
