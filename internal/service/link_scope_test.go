package service_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// scopeFixture seeds assets granted to one or both of two sibling entidades,
// SGA and SGP, for the link-visibility tests of each side.
type scopeFixture struct {
	t        *testing.T
	db       *sql.DB
	sga, sgp int64
}

func newScopeFixture(t *testing.T, db *sql.DB) *scopeFixture {
	f := &scopeFixture{t: t, db: db}
	f.sga = f.id(`SELECT id FROM entidades WHERE slug = 'sga'`)
	f.sgp = f.id(`SELECT id FROM entidades WHERE slug = 'sgp'`)
	return f
}

func (f *scopeFixture) id(q string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.db.QueryRow(q, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
	return id
}

// grant makes the first entidade the asset's creator and the rest responsible.
func (f *scopeFixture) grant(a store.AssetType, id, creator int64, responsible ...int64) {
	f.t.Helper()
	g := models.AssetGrants{CreatorEntidadeID: &creator, ResponsibleEntidadeIDs: responsible}
	if err := store.NewAssetEntidadeRepo(f.db).Replace(context.Background(), f.db, a, id, g); err != nil {
		f.t.Fatalf("grant: %v", err)
	}
}

func (f *scopeFixture) host(slug string, ent int64) int64 {
	id := f.id(`INSERT INTO hosts (nickname, oficial_slug) VALUES (?, ?) RETURNING id`, slug, slug)
	f.grant(store.AssetHost, id, ent)
	return id
}

func (f *scopeFixture) dns(domain string, ent int64) int64 {
	id := f.id(`INSERT INTO dns_records (domain) VALUES (?) RETURNING id`, domain)
	f.grant(store.AssetDNS, id, ent)
	return id
}

func (f *scopeFixture) service(nick string, project *int64, ent int64) int64 {
	id := f.id(`INSERT INTO services (nickname, project_id) VALUES (?, ?) RETURNING id`, nick, project)
	f.grant(store.AssetService, id, ent)
	return id
}

func (f *scopeFixture) project(name string, ent int64) int64 {
	id := f.id(`INSERT INTO projects (name) VALUES (?) RETURNING id`, name)
	f.grant(store.AssetProject, id, ent)
	return id
}

func (f *scopeFixture) as(ent int64) context.Context {
	return store.WithScope(context.Background(), store.Scope{EntidadeIDs: []int64{ent}})
}

func (f *scopeFixture) must(errs ...error) {
	f.t.Helper()
	for _, err := range errs {
		if err != nil {
			f.t.Fatalf("seed: %v", err)
		}
	}
}

// A service shared by SGA and SGP links hosts, DNS records and dependencies
// of both. SGA sees only its own ids in the detail and list row, and saving
// the service as SGA keeps SGP's links, which its pickers never showed.
func TestServiceService_LinkedIDsRespectScope(t *testing.T) {
	bg := context.Background()
	svc, d := newServiceService(t)
	f := newScopeFixture(t, d.SQL)
	shared := f.id(`INSERT INTO services (nickname) VALUES ('shared') RETURNING id`)
	f.grant(store.AssetService, shared, f.sga, f.sgp)
	hostA, hostB := f.host("a", f.sga), f.host("b", f.sgp)
	dnsA, dnsB := f.dns("a.pi.gov.br", f.sga), f.dns("b.pi.gov.br", f.sgp)
	depA, depB := f.service("dep-a", nil, f.sga), f.service("dep-b", nil, f.sgp)
	userA, userB := f.service("user-a", nil, f.sga), f.service("user-b", nil, f.sgp)
	repo := store.NewServiceRepo(d.SQL)
	f.must(
		repo.SetHostLinks(bg, shared, []int64{hostA, hostB}),
		repo.SetDNSLinks(bg, shared, []int64{dnsA, dnsB}),
		repo.SetDependencies(bg, shared, []int64{depA, depB}),
		repo.SetDependencies(bg, userA, []int64{shared}),
		repo.SetDependencies(bg, userB, []int64{shared}),
	)

	asSGA := f.as(f.sga)
	det, err := svc.Get(asSGA, shared)
	if err != nil || det == nil {
		t.Fatalf("Get = %v, %v", det, err)
	}
	if got, want := [][]int64{det.HostIDs, det.DNSIDs, det.DependsOnIDs, det.DependentIDs}, [][]int64{{hostA}, {dnsA}, {depA}, {userA}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA detail hosts/dns/depends/dependents = %v; want only SGA's %v", got, want)
	}
	list, err := svc.List(asSGA, models.ServiceFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var row *service.ServiceListItem
	for i := range list {
		if list[i].ID == shared {
			row = &list[i]
		}
	}
	if row == nil {
		t.Fatalf("List as SGA has no service %d: %+v", shared, list)
	}
	if got, want := [][]int64{row.HostIDs, row.DNSIDs, row.DependsOnIDs}, [][]int64{{hostA}, {dnsA}, {depA}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA list row hosts/dns/depends = %v; want only SGA's %v", got, want)
	}

	none := []int64{}
	if _, err := svc.Update(asSGA, shared, &service.ServiceWrite{Service: models.Service{Nickname: "shared"}, HostIDs: &none, DNSIDs: &none, DependsOnIDs: &none}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	full, _ := svc.Get(bg, shared)
	if got, want := [][]int64{full.HostIDs, full.DNSIDs, full.DependsOnIDs}, [][]int64{{hostB}, {dnsB}, {depB}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after SGA save, unscoped hosts/dns/depends = %v; want SGP's %v kept", got, want)
	}
}

// A project shared by SGA and SGP has direct hosts and DNS records of both
// and a service of each; SGA's service also links SGP's host and record. SGA
// counts and lists only what it can see, and its save keeps SGP's links.
func TestProjectService_LinkedIDsRespectScope(t *testing.T) {
	bg := context.Background()
	svc, d := newProjectService(t)
	f := newScopeFixture(t, d.SQL)
	proj := f.id(`INSERT INTO projects (name) VALUES ('shared') RETURNING id`)
	f.grant(store.AssetProject, proj, f.sga, f.sgp)
	hostA, hostB, hostC := f.host("a", f.sga), f.host("b", f.sgp), f.host("c", f.sga)
	dnsA, dnsB, dnsC := f.dns("a.pi.gov.br", f.sga), f.dns("b.pi.gov.br", f.sgp), f.dns("c.pi.gov.br", f.sga)
	svcA, svcB := f.service("sa", &proj, f.sga), f.service("sb", &proj, f.sgp)
	projects, services := store.NewProjectRepo(d.SQL), store.NewServiceRepo(d.SQL)
	f.must(
		projects.SetDirectHosts(bg, proj, []int64{hostA, hostB}),
		projects.SetDirectDNS(bg, proj, []int64{dnsA, dnsB}),
		services.SetHostLinks(bg, svcA, []int64{hostB, hostC}),
		services.SetDNSLinks(bg, svcA, []int64{dnsB, dnsC}),
	)

	asSGA := f.as(f.sga)
	det, err := svc.Get(asSGA, proj)
	if err != nil || det == nil {
		t.Fatalf("Get = %v, %v", det, err)
	}
	if got, want := [][]int64{det.HostIDs, det.DNSIDs, det.DirectHostIDs, det.DirectDNSIDs}, [][]int64{{hostA, hostC}, {dnsA, dnsC}, {hostA}, {dnsA}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA detail hosts/dns/direct hosts/direct dns = %v; want only SGA's %v", got, want)
	}
	list, err := svc.List(asSGA, models.ProjectFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if row := list[0]; row.ServicesCount != 1 || row.HostsCount != 2 || row.DNSCount != 2 {
		t.Fatalf("SGA list row services=%d hosts=%d dns=%d; want 1/2/2", row.ServicesCount, row.HostsCount, row.DNSCount)
	}

	none := []int64{}
	if _, err := svc.Update(asSGA, proj, &service.ProjectWrite{Project: models.Project{Name: "shared"}, ServiceIDs: &none, DirectHostIDs: &none, DirectDNSIDs: &none}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	full, _ := svc.Get(bg, proj)
	if len(full.Services) != 1 || full.Services[0].ID != svcB || !reflect.DeepEqual(full.DirectHostIDs, []int64{hostB}) || !reflect.DeepEqual(full.DirectDNSIDs, []int64{dnsB}) {
		t.Fatalf("after SGA save, unscoped services=%v direct hosts=%v direct dns=%v; want SGP's %d/[%d]/[%d] kept", full.Services, full.DirectHostIDs, full.DirectDNSIDs, svcB, hostB, dnsB)
	}
}

// A host shared by SGA and SGP links DNS records, services and projects of
// both; SGA's service belongs to SGP's project. SGA's list row counts only
// what it can see, and saving the host as SGA keeps SGP's links.
func TestHostService_LinkedIDsRespectScope(t *testing.T) {
	bg := context.Background()
	svc, d := newHostService(t)
	f := newScopeFixture(t, d.SQL)
	h := &models.Host{Nickname: "shared", OficialSlug: "shared", Hostname: "10.0.0.9", User: "root", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(bg, h); err != nil {
		t.Fatalf("create host: %v", err)
	}
	f.grant(store.AssetHost, h.ID, f.sga, f.sgp)
	dnsA, dnsB := f.dns("a.pi.gov.br", f.sga), f.dns("b.pi.gov.br", f.sgp)
	projA, projB := f.project("pa", f.sga), f.project("pb", f.sgp)
	svcA, svcB := f.service("sa", &projB, f.sga), f.service("sb", nil, f.sgp)
	f.must(
		store.NewDNSRepo(d.SQL).SetLinksForHost(bg, h.ID, []int64{dnsA, dnsB}),
		store.NewServiceRepo(d.SQL).SetServicesForHost(bg, h.ID, []int64{svcA, svcB}),
		store.NewProjectRepo(d.SQL).SetProjectsForHost(bg, h.ID, []int64{projA, projB}),
	)

	asSGA := f.as(f.sga)
	list, err := svc.List(asSGA, models.HostFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if row := list[0]; row.DNSCount != 1 || row.ServicesCount != 1 || row.ProjectsCount != 1 {
		t.Fatalf("SGA list row dns=%d services=%d projects=%d; want 1/1/1", row.DNSCount, row.ServicesCount, row.ProjectsCount)
	}

	none := []int64{}
	if _, err := svc.Update(asSGA, 0, &service.HostWrite{Host: *h, DNSIDs: &none, ServiceIDs: &none, ProjectIDs: &none}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	full, err := svc.Get(bg, h.OficialSlug)
	if err != nil || full == nil {
		t.Fatalf("Get = %v, %v", full, err)
	}
	if len(full.DNSRecords) != 1 || full.DNSRecords[0].ID != dnsB || len(full.Services) != 1 || full.Services[0].ID != svcB || len(full.Projects) != 1 || full.Projects[0].ID != projB {
		t.Fatalf("after SGA save, unscoped dns=%v services=%v projects=%v; want SGP's %d/%d/%d kept", full.DNSRecords, full.Services, full.Projects, dnsB, svcB, projB)
	}
}

// A catalogued API shared by SGA and SGP is served by, consumed by and part
// of assets of both. SGA's detail and list row show only its own ids, and an
// SGA save keeps SGP's links, which its pickers never showed.
func TestAPICatalogRepo_LinkedIDsRespectScope(t *testing.T) {
	bg := context.Background()
	_, d := newServiceService(t)
	f := newScopeFixture(t, d.SQL)
	user := f.id(`INSERT INTO users (username, password_hash, role) VALUES ('scope-api', 'x', 'admin') RETURNING id`)
	api := f.id(`INSERT INTO api_catalog (scope, name, source_type, spec_json, owner_user_id, created_by)
		VALUES ('avulso', 'shared', 'url', '{}', ?, ?) RETURNING id`, user, user)
	f.grant(store.AssetAPICatalog, api, f.sga, f.sgp)
	svcA, svcB := f.service("serve-a", nil, f.sga), f.service("serve-b", nil, f.sgp)
	conA, conB := f.service("call-a", nil, f.sga), f.service("call-b", nil, f.sgp)
	projA, projB := f.project("pa", f.sga), f.project("pb", f.sgp)
	repo := store.NewAPICatalogRepo(d.SQL)
	f.must(repo.SetLinks(bg, api, []int64{svcA, svcB}, []int64{projA, projB}, []int64{conA, conB}))

	asSGA := f.as(f.sga)
	det, err := repo.Get(asSGA, api)
	if err != nil || det == nil {
		t.Fatalf("Get = %v, %v", det, err)
	}
	want := [][]int64{{svcA}, {conA}, {projA}}
	if got := [][]int64{det.ServiceIDs, det.ConsumerServiceIDs, det.ProjectIDs}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA detail services/consumers/projects = %v; want only SGA's %v", got, want)
	}
	list, err := repo.List(asSGA, models.APICatalogFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if got := [][]int64{list[0].ServiceIDs, list[0].ConsumerServiceIDs, list[0].ProjectIDs}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA list row services/consumers/projects = %v; want only SGA's %v", got, want)
	}

	f.must(repo.SetLinks(asSGA, api, []int64{}, []int64{}, []int64{}))
	full, _ := repo.Get(bg, api)
	if got, want := [][]int64{full.ServiceIDs, full.ConsumerServiceIDs, full.ProjectIDs}, [][]int64{{svcB}, {conB}, {projB}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after SGA save, unscoped services/consumers/projects = %v; want SGP's %v kept", got, want)
	}
}

// A catalogued database shared by SGA and SGP is consumed by services and
// part of projects of both: same contract as the API above.
func TestDatabaseRepo_LinkedIDsRespectScope(t *testing.T) {
	bg := context.Background()
	_, d := newServiceService(t)
	f := newScopeFixture(t, d.SQL)
	db := f.id(`INSERT INTO databases (name, engine) VALUES ('shared', 'postgres') RETURNING id`)
	f.grant(store.AssetDatabase, db, f.sga, f.sgp)
	conA, conB := f.service("call-a", nil, f.sga), f.service("call-b", nil, f.sgp)
	projA, projB := f.project("pa", f.sga), f.project("pb", f.sgp)
	repo := store.NewDatabaseRepo(d.SQL)
	f.must(repo.SetLinks(bg, db, []int64{conA, conB}, []int64{projA, projB}))

	asSGA := f.as(f.sga)
	det, err := repo.Get(asSGA, db)
	if err != nil || det == nil {
		t.Fatalf("Get = %v, %v", det, err)
	}
	want := [][]int64{{conA}, {projA}}
	if got := [][]int64{det.ConsumerServiceIDs, det.ProjectIDs}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA detail consumers/projects = %v; want only SGA's %v", got, want)
	}
	list, err := repo.List(asSGA, models.DatabaseFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if got := [][]int64{list[0].ConsumerServiceIDs, list[0].ProjectIDs}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SGA list row consumers/projects = %v; want only SGA's %v", got, want)
	}

	f.must(repo.SetLinks(asSGA, db, []int64{}, []int64{}))
	full, _ := repo.Get(bg, db)
	if got, want := [][]int64{full.ConsumerServiceIDs, full.ProjectIDs}, [][]int64{{conB}, {projB}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after SGA save, unscoped consumers/projects = %v; want SGP's %v kept", got, want)
	}
}
