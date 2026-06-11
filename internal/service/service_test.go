package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

func newServiceService(t *testing.T) (*service.ServiceService, *database.DB) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return service.NewServiceService(d.SQL), d
}

func TestServiceService_CreateEnrichesListAndGet(t *testing.T) {
	ctx := context.Background()
	svc, d := newServiceService(t)

	var contactID int64
	if err := d.SQL.QueryRow(`INSERT INTO contacts (name, phone) VALUES ('Ada','555') RETURNING id`).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	var dnsID int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('x.com') RETURNING id`).Scan(&dnsID); err != nil {
		t.Fatalf("seed dns: %v", err)
	}

	// A dependency target.
	depW := &service.ServiceWrite{Service: models.Service{Nickname: "db"}}
	if err := svc.Create(ctx, depW); err != nil {
		t.Fatalf("create dep: %v", err)
	}

	tags := []string{"web", "core"}
	hosts := []int64{hostID}
	dnss := []int64{dnsID}
	deps := []int64{depW.Service.ID}
	resp := []models.ResponsavelInput{{ContactID: contactID, IsMain: true}}
	w := &service.ServiceWrite{
		Service:      models.Service{Nickname: "api"},
		Tags:         &tags,
		HostIDs:      &hosts,
		DNSIDs:       &dnss,
		DependsOnIDs: &deps,
		Responsaveis: &resp,
	}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	apiID := w.Service.ID

	list, err := svc.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %d items, %v", len(list), err)
	}
	var got service.ServiceListItem
	for _, it := range list {
		if it.ID == apiID {
			got = it
		}
	}
	if len(got.Tags) != 2 || got.Tags[0] != "core" || got.Tags[1] != "web" {
		t.Fatalf("tags = %v, want sorted [core web]", got.Tags)
	}
	if len(got.HostIDs) != 1 || len(got.DNSIDs) != 1 || len(got.DependsOnIDs) != 1 {
		t.Fatalf("list relations = %+v", got)
	}
	if got.MainResponsavelName != "Ada" {
		t.Fatalf("main responsavel = %q", got.MainResponsavelName)
	}

	detail, err := svc.Get(ctx, apiID)
	if err != nil || detail == nil {
		t.Fatalf("get = %+v, %v", detail, err)
	}
	if len(detail.HostIDs) != 1 || len(detail.DNSIDs) != 1 || len(detail.DependsOnIDs) != 1 {
		t.Fatalf("detail relations = %+v", detail)
	}
	if len(detail.Responsaveis) != 1 || detail.Responsaveis[0].Name != "Ada" {
		t.Fatalf("detail responsaveis = %+v", detail.Responsaveis)
	}
	// The dependency target should report `api` as a dependent.
	depDetail, _ := svc.Get(ctx, depW.Service.ID)
	if len(depDetail.DependentIDs) != 1 || depDetail.DependentIDs[0] != apiID {
		t.Fatalf("dep dependent_ids = %v, want [%d]", depDetail.DependentIDs, apiID)
	}

	if detail, err := svc.Get(ctx, 9999); detail != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", detail, err)
	}
}

func TestServiceService_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	svc, _ := newServiceService(t)

	w := &service.ServiceWrite{Service: models.Service{Nickname: "s1"}}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	id := w.Service.ID

	newTags := []string{"z"}
	uw := &service.ServiceWrite{Service: models.Service{Nickname: "s1-renamed"}, Tags: &newTags}
	found, err := svc.Update(ctx, id, uw)
	if err != nil || !found {
		t.Fatalf("update = found %v, %v", found, err)
	}
	detail, _ := svc.Get(ctx, id)
	if detail.Service.Nickname != "s1-renamed" || len(detail.Tags) != 1 || detail.Tags[0] != "z" {
		t.Fatalf("after update = %+v", detail)
	}

	if found, _ := svc.Update(ctx, 9999, uw); found {
		t.Fatal("update(missing) should return found=false")
	}

	actor := models.ActorContext{UserID: 1, Role: "admin"}
	if err := svc.Delete(ctx, actor, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if detail, _ := svc.Get(ctx, id); detail != nil {
		t.Fatalf("after delete = %+v, want nil", detail)
	}
}

func TestServiceService_FixateAndContainerRules(t *testing.T) {
	ctx := context.Background()
	svc, _ := newServiceService(t)

	// A manual service cannot be fixated.
	manual := &service.ServiceWrite{Service: models.Service{Nickname: "m", Source: "manual"}}
	if err := svc.Create(ctx, manual); err != nil {
		t.Fatalf("create manual: %v", err)
	}
	if _, err := svc.Fixate(ctx, manual.Service.ID); !errors.Is(err, service.ErrServiceNotAuto) {
		t.Fatalf("fixate(manual) err = %v, want ErrServiceNotAuto", err)
	}

	// An auto service can be fixated and then rebound.
	auto := &service.ServiceWrite{Service: models.Service{Nickname: "a", Source: "auto"}}
	if err := svc.Create(ctx, auto); err != nil {
		t.Fatalf("create auto: %v", err)
	}
	// Auto cannot be rebound until fixated.
	if _, err := svc.UpdateContainer(ctx, auto.Service.ID, "n", "i"); !errors.Is(err, service.ErrServiceCannotRebindAuto) {
		t.Fatalf("rebind(auto) err = %v, want ErrServiceCannotRebindAuto", err)
	}
	fixed, err := svc.Fixate(ctx, auto.Service.ID)
	if err != nil || fixed == nil || fixed.Source != "fixed" {
		t.Fatalf("fixate(auto) = %+v, %v", fixed, err)
	}
	rebound, err := svc.UpdateContainer(ctx, auto.Service.ID, "newname", "newid")
	if err != nil || rebound.ContainerName != "newname" {
		t.Fatalf("rebind = %+v, %v", rebound, err)
	}

	// Fixate on a missing service returns (nil, nil) → handler 404.
	if got, err := svc.Fixate(ctx, 9999); got != nil || err != nil {
		t.Fatalf("fixate(missing) = %+v, %v; want nil,nil", got, err)
	}
}
