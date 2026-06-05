package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newHostService(t *testing.T) (*service.HostService, *database.DB) {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return service.NewHostService(d.SQL), d
}

func TestHostService_GetComposesRelations(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	hostRepo := store.NewHostRepo(d.SQL)

	h := &models.Host{Nickname: "web", OficialSlug: "web-01", Hostname: "10.0.0.1", User: "root", Situacao: "active"}
	if err := hostRepo.Create(ctx, h); err != nil {
		t.Fatalf("create host: %v", err)
	}
	store.NewTagRepo(d.SQL).Set(ctx, "host", h.ID, []string{"prod"})

	// Link a service and a dns record to the host.
	sr, _ := d.SQL.Exec(`INSERT INTO services (nickname) VALUES ('svc')`)
	svcID, _ := sr.LastInsertId()
	d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svcID, h.ID)
	dr, _ := d.SQL.Exec(`INSERT INTO dns_records (domain) VALUES ('web.example.com')`)
	dnsID, _ := dr.LastInsertId()
	d.SQL.Exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dnsID, h.ID)

	// A scan so last_scan is populated.
	store.NewHostScanRepo(d.SQL).Create(ctx, h.ID, `{"cpu":"4","services":[],"containers":[]}`)

	detail, err := svc.Get(ctx, "web-01")
	if err != nil || detail == nil {
		t.Fatalf("get = %+v, %v", detail, err)
	}
	if detail.Host.OficialSlug != "web-01" {
		t.Fatalf("host = %+v", detail.Host)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "prod" {
		t.Fatalf("tags = %v", detail.Tags)
	}
	if len(detail.Services) != 1 || len(detail.DNSRecords) != 1 {
		t.Fatalf("relations: services=%d dns=%d", len(detail.Services), len(detail.DNSRecords))
	}
	if detail.LastScan == nil {
		t.Fatal("last_scan should be populated")
	}

	// Missing host → (nil, nil).
	if got, err := svc.Get(ctx, "nope"); got != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", got, err)
	}
}

func TestHostService_ListEnrichesAndComputesAlerts(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	hostRepo := store.NewHostRepo(d.SQL)

	h := &models.Host{Nickname: "box", OficialSlug: "box-01", Hostname: "h", User: "root", Situacao: "active"}
	if err := hostRepo.Create(ctx, h); err != nil {
		t.Fatalf("create: %v", err)
	}
	store.NewTagRepo(d.SQL).Set(ctx, "host", h.ID, []string{"edge"})

	// Link a service so ServicesCount is enriched.
	sr, _ := d.SQL.Exec(`INSERT INTO services (nickname) VALUES ('s1')`)
	svcID, _ := sr.LastInsertId()
	d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svcID, h.ID)

	// Scan data with critical CPU usage (90% ≥ default critical 80%).
	scan := `{"cpu":"4 cores","cpu_usage":"90%","ram":"8G","ram_percent":"50%","storage":"100G","disk_percent":"40%","services":[],"containers":["c1"]}`
	store.NewHostScanRepo(d.SQL).Create(ctx, h.ID, scan)

	items, err := svc.List(ctx, models.HostFilter{})
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %d items, %v", len(items), err)
	}
	it := items[0]
	if it.OficialSlug != "box-01" {
		t.Fatalf("item host = %+v", it.Host)
	}
	if len(it.Tags) != 1 || it.Tags[0] != "edge" {
		t.Fatalf("tags = %v", it.Tags)
	}
	if !it.HasScan || it.ScanRes == nil {
		t.Fatalf("scan enrichment missing: hasScan=%v res=%v", it.HasScan, it.ScanRes)
	}
	if it.ServicesCount != 1 {
		t.Fatalf("services_count = %d, want 1", it.ServicesCount)
	}
	if it.ContainersCount != 1 {
		t.Fatalf("containers_count = %d, want 1", it.ContainersCount)
	}
	// A critical CPU alert should have been computed.
	var foundCPUCritical bool
	for _, a := range it.Alerts {
		if a.Type == "resource_cpu" && a.Level == "critical" {
			foundCPUCritical = true
		}
	}
	if !foundCPUCritical {
		t.Fatalf("expected a critical resource_cpu alert, got %+v", it.Alerts)
	}
}
