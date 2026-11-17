package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestRecordSiteServices covers C.3 end to end: only host nginx vhosts with a
// docroot and a DNS-linkable name become "site" services; the A7 code
// fingerprint attaches the framework; and RecordWebDNS links the vhost's DNS
// record to the site service.
func TestRecordSiteServices(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	sys := store.WithSystemScope(ctx)

	h := &models.Host{Nickname: "vm-site", OficialSlug: "vm-site", Hostname: "10.0.0.8", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	var entID int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades ORDER BY id LIMIT 1`).Scan(&entID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES ('host', ?, ?, 'creator') ON CONFLICT DO NOTHING`, h.ID, entID); err != nil {
		t.Fatal(err)
	}

	dns := store.NewDNSRepo(d.SQL)
	rec := models.DNSRecord{Domain: "portal.sead.pi.gov.br", Situacao: "active"}
	if err := dns.Create(ctx, &rec); err != nil {
		t.Fatal(err)
	}

	info := sshtest.VMInfo{
		WebServers: []sshtest.WebServer{
			{Kind: "nginx", Sites: []sshtest.WebSite{
				{Names: []string{"portal.sead.pi.gov.br", "_"}, Listen: []int{443}, TLS: true, Root: "/var/www/portal"},
				{Names: []string{"noroot.sead.pi.gov.br"}, Listen: []int{80}},                      // no docroot → skip
				{Names: []string{"disabled.sead.pi.gov.br"}, Root: "/var/www/dis", Disabled: true}, // virtualmin-disabled → skip
				{Names: []string{"_"}, Root: "/var/www/default"},                                  // no linkable name → skip
			}},
			{Kind: "nginx", Container: "web", Sites: []sshtest.WebSite{{Names: []string{"c.sead.pi.gov.br"}, Root: "/app"}}}, // container → skip
			{Kind: "apache", Stopped: true, Sites: []sshtest.WebSite{{Names: []string{"s.sead.pi.gov.br"}, Root: "/s"}}},     // stopped → skip
		},
		DeepFacts: &sshtest.DeepFacts{Code: []sshtest.CodeFingerprint{
			{Docroot: "/var/www/portal", Apps: []sshtest.CodeApp{{Path: "", Framework: "laravel", Version: "10.3"}}},
		}},
	}

	sum, err := service.RecordSiteServices(ctx, d.SQL, h, info)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if sum.Sites != 1 {
		t.Fatalf("sites = %d, want 1 (only the host nginx vhost with a docroot and a linkable name)", sum.Sites)
	}

	svcs := mustListDiscovered(t, d, sys, h.ID)
	var site *models.Service
	for i := range svcs {
		if svcs[i].DiscoveryKind == "site" {
			site = &svcs[i]
		}
	}
	if site == nil {
		t.Fatalf("no site service created; discovered = %+v", svcs)
	}
	if site.Nickname != "portal.sead.pi.gov.br" || site.TechnologyStack != "laravel" || site.Version != "10.3" ||
		site.ServiceKind != "web" || site.Source != "auto" || site.ContainerStatus != "online" {
		t.Fatalf("site service = %+v (want portal/laravel/10.3/web/auto/online)", site)
	}

	// RecordWebDNS links the vhost's DNS record to its site service.
	lookup := func(_ context.Context, _ string) ([]string, error) { return []string{"200.1.2.3"}, nil }
	if _, err := service.RecordWebDNS(ctx, d.SQL, h, info, lookup); err != nil {
		t.Fatalf("webdns: %v", err)
	}
	ids, err := dns.ServiceIDs(sys, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	linked := false
	for _, id := range ids {
		if id == site.ID {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("DNS record %q not linked to site service %d; links = %v", rec.Domain, site.ID, ids)
	}
}
