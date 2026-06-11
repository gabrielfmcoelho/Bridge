package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

func newDNSService(t *testing.T) (*service.DNSService, *database.DB) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return service.NewDNSService(d.SQL), d
}

// The whole point of Phase 2: the list-enrichment composition (record + tags +
// host links) is now testable without spinning up HTTP.
func TestDNSService_CreateEnrichesListAndGet(t *testing.T) {
	ctx := context.Background()
	svc, d := newDNSService(t)

	// dns_host_links.host_id FKs hosts; seed two hosts.
	var host1 int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&host1); err != nil {
		t.Fatalf("seed host1: %v", err)
	}
	var host2 int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h2','h2') RETURNING id`).Scan(&host2); err != nil {
		t.Fatalf("seed host2: %v", err)
	}

	tags := []string{"prod", "edge"}
	hostIDs := []int64{host1, host2}
	w := &service.DNSWrite{
		Record:  models.DNSRecord{Domain: "example.com", HasHTTPS: true, Situacao: "ativo"},
		Tags:    &tags,
		HostIDs: &hostIDs,
	}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	if w.Record.ID == 0 {
		t.Fatal("create did not set ID")
	}

	// List is enriched with tags (sorted) + host links.
	list, err := svc.List(ctx, models.DNSFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	got := list[0]
	if got.Domain != "example.com" {
		t.Fatalf("domain = %q", got.Domain)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "edge" || got.Tags[1] != "prod" {
		t.Fatalf("tags = %v, want sorted [edge prod]", got.Tags)
	}
	if len(got.HostIDs) != 2 {
		t.Fatalf("host_ids = %v, want 2", got.HostIDs)
	}

	// Get returns the detail view.
	detail, err := svc.Get(ctx, w.Record.ID)
	if err != nil || detail == nil || detail.Record.Domain != "example.com" {
		t.Fatalf("get = %+v, %v", detail, err)
	}
	if len(detail.Tags) != 2 || len(detail.HostIDs) != 2 {
		t.Fatalf("detail relations = %+v", detail)
	}

	// Get on a missing id returns (nil, nil).
	if detail, err := svc.Get(ctx, 9999); detail != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", detail, err)
	}
}

func TestDNSService_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	svc, _ := newDNSService(t)

	w := &service.DNSWrite{Record: models.DNSRecord{Domain: "a.com"}}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	id := w.Record.ID

	// Update with nil relations leaves them untouched; record fields change.
	newTags := []string{"x"}
	uw := &service.DNSWrite{Record: models.DNSRecord{Domain: "a2.com", Situacao: "inativo"}, Tags: &newTags}
	found, err := svc.Update(ctx, id, uw)
	if err != nil || !found {
		t.Fatalf("update = found %v, %v", found, err)
	}
	detail, _ := svc.Get(ctx, id)
	if detail.Record.Domain != "a2.com" || detail.Record.Situacao != "inativo" {
		t.Fatalf("after update = %+v", detail.Record)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "x" {
		t.Fatalf("tags after update = %v", detail.Tags)
	}

	// Update on a missing id returns found=false.
	if found, _ := svc.Update(ctx, 9999, uw); found {
		t.Fatal("update(missing) should return found=false")
	}

	// Delete removes the record (and its tags).
	if err := svc.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if detail, _ := svc.Get(ctx, id); detail != nil {
		t.Fatalf("after delete = %+v, want nil", detail)
	}
}
