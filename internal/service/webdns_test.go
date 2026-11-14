package service_test

import (
	"encoding/json"
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestLinkableName(t *testing.T) {
	ok := map[string]string{"Portal.SEAD.pi.gov.br.": "portal.sead.pi.gov.br", "a-b.example.com": "a-b.example.com"}
	for in, want := range ok {
		if got, valid := service.LinkableName(in); !valid || got != want {
			t.Errorf("%q = %q,%v; want %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"_", "localhost", "app.localhost", "10.0.0.1", "::1", "*.x.gov.br", ".x.gov.br", `~^(?<s>.+)\.x\.br$`, "intranet", "a b.com", ""} {
		if _, valid := service.LinkableName(in); valid {
			t.Errorf("%q should not be linkable", in)
		}
	}
}

// One scan of a host with a host nginx (proxying to a published container
// port), an nginx container (fastcgi to a compose php service) and a stopped
// host Apache.
func TestRecordWebDNS(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	sys := store.WithSystemScope(ctx)

	h := &models.Host{Nickname: "vm-web", OficialSlug: "vm-web", Hostname: "10.0.0.7", Situacao: "active"}
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
	ctrs := []sshtest.ContainerInfo{
		{ID: "0123456789ab", Name: "web", Image: "nginx:1.25", Status: "Up 1 hour"},
		{ID: "123456789abc", Name: "proj-api-1", Image: "api:1", Status: "Up 1 hour"},
		{ID: "23456789abcd", Name: "proj-php-1", Image: "php:8.2-fpm", Status: "Up 1 hour"},
	}
	if err := store.NewServiceRepo(d.SQL).ReconcileDiscovered(ctx, h.ID, store.DiscoveredInventory{
		Containers: ctrs, ContainersKnown: true,
		Services: []sshtest.DiscoveredService{{Name: "nginx", Label: "Nginx", Kind: "web", HostRunning: true}},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	svcID := map[string]int64{}
	svcs, _ := store.NewServiceRepo(d.SQL).ListDiscoveredByHost(sys, h.ID)
	for _, s := range svcs {
		key := s.ContainerName
		if s.DiscoveryKind == "host" {
			key = "host:" + s.DiscoveryKey
		}
		svcID[key] = s.ID
	}
	if len(svcID) != 4 {
		t.Fatalf("services = %v, want 3 containers + host nginx", svcID)
	}

	dns := store.NewDNSRepo(d.SQL)
	known := models.DNSRecord{Domain: "known.x.gov.br", Situacao: "active"}
	trash := models.DNSRecord{Domain: "trash.x.gov.br", Situacao: "active"}
	for _, r := range []*models.DNSRecord{&known, &trash} {
		if err := dns.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := dns.Delete(sys, trash.ID); err != nil {
		t.Fatal(err)
	}

	info := sshtest.VMInfo{
		ParsedContainers: ctrs,
		PortOwners:       []sshtest.PortOwner{{Port: 8080, OwnerType: "container", OwnerName: "proj-api-1"}},
		WebServers: []sshtest.WebServer{
			{Kind: "nginx", Sites: []sshtest.WebSite{
				{Names: []string{"Known.x.gov.br", "new.x.gov.br", "_", "localhost", "*.x.gov.br"}, Listen: []int{443}, TLS: true, Proxy: []string{"http://127.0.0.1:8080"}},
				{Names: []string{"ghost.x.gov.br", "trash.x.gov.br"}, Listen: []int{80}},
			}},
			{Kind: "nginx", Container: "web", ContainerID: "0123456789ab", Sites: []sshtest.WebSite{
				{Names: []string{"new.x.gov.br", "app.x.gov.br"}, Listen: []int{80}, FastCGI: []string{"php:9000"}},
			}},
			{Kind: "apache", Stopped: true, Sites: []sshtest.WebSite{{Names: []string{"stopped.x.gov.br"}}}},
			{Kind: "apache", Sites: []sshtest.WebSite{{Names: []string{"disabled.x.gov.br"}, Listen: []int{80}, Disabled: true}}},
			{Kind: "nginx", Container: "broken", Error: "config unreadable", Sites: []sshtest.WebSite{{Names: []string{"broken.x.gov.br"}}}},
		},
	}
	var mu sync.Mutex
	var looked []string
	lookup := func(_ context.Context, n string) ([]string, error) {
		mu.Lock()
		looked = append(looked, n)
		mu.Unlock()
		if n == "ghost.x.gov.br" {
			return nil, errors.New("no such host")
		}
		return []string{"200.1.2.3"}, nil
	}

	sum, err := service.RecordWebDNS(ctx, d.SQL, h, info, lookup)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	want := service.WebDNSSummary{Names: 5, Created: 2, Linked: 3, Unresolved: 1, Trashed: 1, ServiceLinks: 8}
	if sum != want {
		t.Fatalf("summary = %+v, want %+v", sum, want)
	}
	sort.Strings(looked)
	if strings.Join(looked, ",") != "app.x.gov.br,ghost.x.gov.br,new.x.gov.br" {
		t.Fatalf("looked up %v; only names missing from Bridge should be", looked)
	}

	idOf := func(domain string) int64 {
		t.Helper()
		id, found, _, err := dns.IDByDomain(ctx, domain)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			return 0
		}
		return id
	}
	newID := idOf("new.x.gov.br")
	rec, err := dns.Get(sys, newID)
	if err != nil || rec == nil || !rec.HasHTTPS || !strings.HasPrefix(rec.Observacoes, "Scan: vm-web — nginx (host)") {
		t.Fatalf("new record = %+v %v", rec, err)
	}
	var grants int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM asset_entidades WHERE asset_type = 'dns' AND asset_id = ? AND entidade_id = ?`, newID, entID).Scan(&grants)
	if grants == 0 {
		t.Fatal("new record did not inherit the host's entidade")
	}
	for _, n := range []string{"ghost.x.gov.br", "stopped.x.gov.br", "disabled.x.gov.br", "broken.x.gov.br", "localhost"} {
		if idOf(n) != 0 {
			t.Errorf("%s should not have a record", n)
		}
	}

	services := func(domain string) []int64 {
		ids, err := dns.ServiceIDs(sys, idOf(domain))
		if err != nil {
			t.Fatal(err)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}
	sorted := func(ids ...int64) []int64 { sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] }); return ids }
	check := func(domain string, want []int64) {
		t.Helper()
		if got := services(domain); !reflect.DeepEqual(got, want) {
			t.Errorf("%s services = %v, want %v", domain, got, want)
		}
	}
	// known: host nginx + the container behind its proxy_pass port.
	check("known.x.gov.br", sorted(svcID["host:nginx"], svcID["proj-api-1"]))
	// new: served by both nginx — plus what each hands requests to.
	check("new.x.gov.br", sorted(svcID["host:nginx"], svcID["proj-api-1"], svcID["web"], svcID["proj-php-1"]))
	// app: the nginx container and the compose php service it fastcgi_passes to.
	check("app.x.gov.br", sorted(svcID["web"], svcID["proj-php-1"]))
	if hosts, _ := dns.HostIDs(sys, idOf("app.x.gov.br")); len(hosts) != 1 || hosts[0] != h.ID {
		t.Fatalf("app host links = %v", hosts)
	}

	// A second scan changes nothing and looks nothing up but the unresolved name.
	looked = nil
	sum, err = service.RecordWebDNS(ctx, d.SQL, h, info, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Created != 0 || sum.Linked != 0 || sum.ServiceLinks != 0 || strings.Join(looked, ",") != "ghost.x.gov.br" {
		t.Fatalf("second run = %+v, looked %v", sum, looked)
	}
}

// remoteSetup creates proxy host P (10.0.0.10) and a record for siace.example,
// and returns what RecordWebDNS(P) needs.
func remoteSetup(t *testing.T) (*database.DB, *models.Host, sshtest.VMInfo, int64) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	p := &models.Host{Nickname: "proxy", OficialSlug: "proxy", Hostname: "10.0.0.10", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	rec := models.DNSRecord{Domain: "siace.example", Situacao: "active"}
	if err := store.NewDNSRepo(d.SQL).Create(ctx, &rec); err != nil {
		t.Fatal(err)
	}
	info := sshtest.VMInfo{WebServers: []sshtest.WebServer{{Kind: "nginx", Sites: []sshtest.WebSite{
		{Names: []string{"siace.example"}, Listen: []int{80}, Proxy: []string{"10.0.0.11:8180"}},
	}}}}
	return d, p, info, rec.ID
}

func remoteLinks(t *testing.T, d *database.DB, recID int64) (hosts, svcs []int64) {
	t.Helper()
	for _, q := range []struct {
		sql string
		out *[]int64
	}{
		{`SELECT host_id FROM dns_host_links WHERE dns_id = ? ORDER BY 1`, &hosts},
		{`SELECT service_id FROM service_dns_links WHERE dns_id = ? ORDER BY 1`, &svcs},
	} {
		rows, err := d.SQL.Query(q.sql, recID)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			*q.out = append(*q.out, id)
		}
		rows.Close()
	}
	return
}

func addUpstream(t *testing.T, d *database.DB, withScan bool) (*models.Host, int64) {
	t.Helper()
	ctx := context.Background()
	u := &models.Host{Nickname: "up", OficialSlug: "up", Hostname: "10.0.0.11", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	if !withScan {
		return u, 0
	}
	inv := []sshtest.DiscoveredService{{Name: "jboss", Label: "JBoss", Kind: "app", Instance: "compras2.service",
		HostRunning: true, Ports: []int{8180, 1190}, HostPorts: []int{8180, 1190}}}
	b, _ := json.Marshal(sshtest.VMInfo{ServiceInventory: inv})
	if err := store.NewHostScanRepo(d.SQL).Create(ctx, u.ID, string(b)); err != nil {
		t.Fatal(err)
	}
	if err := store.NewServiceRepo(d.SQL).ReconcileDiscovered(ctx, u.ID, store.DiscoveredInventory{Services: inv}); err != nil {
		t.Fatal(err)
	}
	svcs, _ := store.NewServiceRepo(d.SQL).ListDiscoveredByHost(store.WithSystemScope(ctx), u.ID)
	for _, s := range svcs {
		if s.DiscoveryKey == "jboss@compras2.service" {
			return u, s.ID
		}
	}
	t.Fatalf("service not reconciled: %+v", svcs)
	return u, 0
}

func TestRecordWebDNSRemoteUpstream(t *testing.T) {
	d, p, info, rec := remoteSetup(t)
	u, svc := addUpstream(t, d, true)
	ctx := context.Background()
	if _, err := service.RecordWebDNS(ctx, d.SQL, p, info, nil); err != nil {
		t.Fatal(err)
	}
	hosts, svcs := remoteLinks(t, d, rec)
	if !reflect.DeepEqual(hosts, []int64{p.ID, u.ID}) || !reflect.DeepEqual(svcs, []int64{svc}) {
		t.Fatalf("hosts=%v svcs=%v, want [%d %d] [%d]", hosts, svcs, p.ID, u.ID, svc)
	}
	sum, err := service.RecordWebDNS(ctx, d.SQL, p, info, nil)
	if err != nil || sum.Linked != 0 || sum.ServiceLinks != 0 {
		t.Fatalf("second run %+v %v, want nothing added", sum, err)
	}
}

func TestRecordWebDNSRemoteHostOnly(t *testing.T) {
	d, p, info, rec := remoteSetup(t)
	u, _ := addUpstream(t, d, false)
	if _, err := service.RecordWebDNS(context.Background(), d.SQL, p, info, nil); err != nil {
		t.Fatal(err)
	}
	hosts, svcs := remoteLinks(t, d, rec)
	if !reflect.DeepEqual(hosts, []int64{p.ID, u.ID}) || len(svcs) != 0 {
		t.Fatalf("hosts=%v svcs=%v", hosts, svcs)
	}
}

func TestRecordWebDNSRemoteAmbiguous(t *testing.T) {
	d, p, info, rec := remoteSetup(t)
	addUpstream(t, d, false)
	dup := &models.Host{Nickname: "up2", OficialSlug: "up2", Hostname: "10.0.0.11", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(context.Background(), dup); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordWebDNS(context.Background(), d.SQL, p, info, nil); err != nil {
		t.Fatal(err)
	}
	hosts, _ := remoteLinks(t, d, rec)
	if !reflect.DeepEqual(hosts, []int64{p.ID}) {
		t.Fatalf("hosts=%v, want only the proxy", hosts)
	}
}
