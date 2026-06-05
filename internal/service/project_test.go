package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

func newProjectService(t *testing.T) (*service.ProjectService, *database.DB) {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return service.NewProjectService(d.SQL), d
}

func TestProjectService_CreateEnrichesListAndGet(t *testing.T) {
	ctx := context.Background()
	svc, d := newProjectService(t)

	// Seed a contact to use as the project's main responsável.
	cr, _ := d.SQL.Exec(`INSERT INTO contacts (name, phone, role) VALUES ('Ada','555-1','lead')`)
	contactID, _ := cr.LastInsertId()

	tags := []string{"infra", "core"}
	resp := []models.ResponsavelInput{{ContactID: contactID, IsMain: true}}
	w := &service.ProjectWrite{
		Project:      models.Project{Name: "Atlas", Description: "catalog", Situacao: "active"},
		Tags:         &tags,
		Responsaveis: &resp,
	}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	if w.Project.ID == 0 {
		t.Fatal("create did not set ID")
	}
	projID := w.Project.ID

	// List is enriched with sorted tags + main responsável name.
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	got := list[0]
	if got.Name != "Atlas" {
		t.Fatalf("name = %q", got.Name)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "core" || got.Tags[1] != "infra" {
		t.Fatalf("tags = %v, want sorted [core infra]", got.Tags)
	}
	if got.MainResponsavelName != "Ada" {
		t.Fatalf("main responsavel = %q, want Ada", got.MainResponsavelName)
	}

	// Seed a service under the project, linked to a host and a dns record, so
	// Get aggregates host_ids/dns_ids from the project's services.
	sr, _ := d.SQL.Exec(`INSERT INTO services (nickname, project_id) VALUES ('svc', ?)`, projID)
	svcID, _ := sr.LastInsertId()
	hr, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1')`)
	hostID, _ := hr.LastInsertId()
	dr, _ := d.SQL.Exec(`INSERT INTO dns_records (domain) VALUES ('x.com')`)
	dnsID, _ := dr.LastInsertId()
	d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svcID, hostID)
	d.SQL.Exec(`INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?)`, svcID, dnsID)

	detail, err := svc.Get(ctx, projID)
	if err != nil || detail == nil {
		t.Fatalf("get = %+v, %v", detail, err)
	}
	if detail.Project.Name != "Atlas" {
		t.Fatalf("detail project = %+v", detail.Project)
	}
	if len(detail.Tags) != 2 {
		t.Fatalf("detail tags = %v", detail.Tags)
	}
	if len(detail.Responsaveis) != 1 || !detail.Responsaveis[0].IsMain || detail.Responsaveis[0].Name != "Ada" {
		t.Fatalf("detail responsaveis = %+v", detail.Responsaveis)
	}
	if len(detail.Services) != 1 {
		t.Fatalf("detail services = %+v", detail.Services)
	}
	if len(detail.HostIDs) != 1 || detail.HostIDs[0] != hostID {
		t.Fatalf("detail host_ids = %v, want [%d]", detail.HostIDs, hostID)
	}
	if len(detail.DNSIDs) != 1 || detail.DNSIDs[0] != dnsID {
		t.Fatalf("detail dns_ids = %v, want [%d]", detail.DNSIDs, dnsID)
	}

	// Get on a missing id returns (nil, nil).
	if detail, err := svc.Get(ctx, 9999); detail != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", detail, err)
	}
}

func TestProjectService_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	svc, _ := newProjectService(t)

	w := &service.ProjectWrite{Project: models.Project{Name: "p1"}}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	id := w.Project.ID

	// Update with nil relations leaves them untouched; record fields change.
	newTags := []string{"z"}
	uw := &service.ProjectWrite{Project: models.Project{Name: "p1-renamed", Situacao: "archived"}, Tags: &newTags}
	found, err := svc.Update(ctx, id, uw)
	if err != nil || !found {
		t.Fatalf("update = found %v, %v", found, err)
	}
	detail, _ := svc.Get(ctx, id)
	if detail.Project.Name != "p1-renamed" || detail.Project.Situacao != "archived" {
		t.Fatalf("after update = %+v", detail.Project)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "z" {
		t.Fatalf("tags after update = %v", detail.Tags)
	}

	// Update on a missing id returns found=false.
	if found, _ := svc.Update(ctx, 9999, uw); found {
		t.Fatal("update(missing) should return found=false")
	}

	// Delete removes the project (cascade registry handles vault children).
	actor := models.ActorContext{UserID: 1, Role: "admin"}
	if err := svc.Delete(ctx, actor, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if detail, _ := svc.Get(ctx, id); detail != nil {
		t.Fatalf("after delete = %+v, want nil", detail)
	}
}
