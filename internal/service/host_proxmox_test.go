package service_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/proxmox"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostService_SyncFromProxmox(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	pve := newProxmoxServer(t, d, "pve")

	// A hand-registered host whose IP a guest reports, and one in maintenance.
	var manual, etipi, other int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, situacao) VALUES ('Portal','portal','10.0.0.10','active') RETURNING id`).Scan(&manual); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'etipi'`).Scan(&etipi); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sead-pi'`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	// A soft-deleted host still reserves the slug "web".
	if _, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug, deleted_at) VALUES ('old','web', NOW())`); err != nil {
		t.Fatal(err)
	}

	ms := []proxmox.Machine{
		{ProxmoxID: "node/pve1", Kind: "node", Name: "pve1", Node: "pve1", IP: "10.0.0.1", Running: true, CPU: "32 vCPU"},
		{ProxmoxID: "qemu/101", Kind: "qemu", Name: "portal-vm", Node: "pve1", VMID: 101, IP: "10.0.0.10", Running: true, CPU: "4 vCPU", RAM: "8 GB"},
		{ProxmoxID: "qemu/102", Kind: "qemu", Name: "web", Node: "pve1", VMID: 102, Running: true},
		{ProxmoxID: "lxc/103", Kind: "lxc", Name: "cache", Node: "pve1", VMID: 103, IP: "10.0.0.13", Running: true},
	}
	sum, err := svc.SyncFromProxmox(ctx, pve, ms)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"unknown: web"}; !reflect.DeepEqual(sum.NoIPReasons, want) {
		t.Fatalf("NoIPReasons = %q, want %q", sum.NoIPReasons, want)
	}
	sum.NoIPReasons = nil
	if want := (service.ProxmoxSyncSummary{Found: 4, Created: 3, Updated: 1, NoIP: 1}); !reflect.DeepEqual(sum, want) {
		t.Fatalf("first sync = %+v, want %+v", sum, want)
	}

	hosts := store.NewHostRepo(d.SQL)
	byPID, _, _, err := hosts.ProxmoxIndex(ctx, pve)
	if err != nil {
		t.Fatal(err)
	}
	if byPID["qemu/101"] != manual {
		t.Fatalf("qemu/101 linked to %d, want manual host %d", byPID["qemu/101"], manual)
	}
	m, _ := hosts.GetByID(ctx, manual)
	if m.Nickname != "Portal" || m.OficialSlug != "portal" || m.RecursoCPU != "4 vCPU" || m.RecursoRAM != "8 GB" {
		t.Fatalf("manual host = %+v: identity must stay, resources refresh", m)
	}
	node := byPID["node/pve1"]
	if m.ParentHostID == nil || *m.ParentHostID != node {
		t.Fatalf("manual host parent = %v, want node %d", m.ParentHostID, node)
	}
	web, _ := hosts.GetByID(ctx, byPID["qemu/102"])
	if web.OficialSlug != "web-2" || web.Situacao != "active" || web.TipoMaquina != "VM" {
		t.Fatalf("web = slug %q situacao %q tipo %q", web.OficialSlug, web.Situacao, web.TipoMaquina)
	}
	cache, _ := hosts.GetByID(ctx, byPID["lxc/103"])
	if cache.Hostname != "10.0.0.13" || cache.TipoMaquina != "Container" || cache.Situacao != "active" {
		t.Fatalf("cache = %+v", cache)
	}

	// Grants: the node falls back to ETIPI; guests copy the node's.
	grants := store.NewAssetEntidadeRepo(d.SQL)
	if g, _ := grants.Get(ctx, store.AssetHost, node); g.CreatorEntidadeID == nil || *g.CreatorEntidadeID != etipi {
		t.Fatalf("node grants = %+v, want ETIPI creator", g)
	}
	if _, err := d.SQL.Exec(`UPDATE asset_entidades SET entidade_id = ? WHERE asset_type = 'host' AND asset_id = ?`, other, node); err != nil {
		t.Fatal(err)
	}

	// Second run: web gone, cache in maintenance, a new guest inherits the node's grants.
	if _, err := d.SQL.Exec(`UPDATE hosts SET situacao = 'maintenance' WHERE id = ?`, cache.ID); err != nil {
		t.Fatal(err)
	}
	ms = append(ms[:2], ms[3], proxmox.Machine{ProxmoxID: "qemu/104", Kind: "qemu", Name: "db", Node: "pve1", VMID: 104, Running: true})
	sum, err = svc.SyncFromProxmox(ctx, pve, ms)
	if err != nil {
		t.Fatal(err)
	}
	if want := (service.ProxmoxSyncSummary{Found: 4, Created: 1, Updated: 3, Deactivated: 1, NoIP: 1}); !reflect.DeepEqual(withoutReasons(sum), want) {
		t.Fatalf("second sync = %+v, want %+v", sum, want)
	}
	if web, _ = hosts.GetByID(ctx, web.ID); web.Situacao != "inactive" {
		t.Fatalf("vanished web situacao = %q", web.Situacao)
	}
	if cache, _ = hosts.GetByID(ctx, cache.ID); cache.Situacao != "maintenance" {
		t.Fatalf("maintenance overwritten: %q", cache.Situacao)
	}
	byPID, _, _, _ = hosts.ProxmoxIndex(ctx, pve)
	if g, _ := grants.Get(ctx, store.AssetHost, byPID["qemu/104"]); g.CreatorEntidadeID == nil || *g.CreatorEntidadeID != other {
		t.Fatalf("new guest grants = %+v, want node's creator %d", g, other)
	}

	// A vanished host in maintenance keeps it.
	if _, err = svc.SyncFromProxmox(ctx, pve, ms[:2]); err != nil {
		t.Fatal(err)
	}
	if cache, _ = hosts.GetByID(ctx, cache.ID); cache.Situacao != "maintenance" {
		t.Fatalf("vanished maintenance host situacao = %q", cache.Situacao)
	}

	// An empty fetch retires nothing.
	if sum, err = svc.SyncFromProxmox(ctx, pve, nil); err != nil || sum.Deactivated != 0 {
		t.Fatalf("empty sync = %+v, %v", sum, err)
	}
	if m, _ = hosts.GetByID(ctx, manual); m.Situacao != "active" {
		t.Fatalf("empty sync deactivated manual host: %q", m.Situacao)
	}
}

func TestHostService_SyncFromProxmox_Matching(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	pve := newProxmoxServer(t, d, "pve")
	insert := func(slug, hostname string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES (?, ?, ?) RETURNING id`, slug, slug, hostname).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	gitlab := insert("gitlab-prod", "GitLab.sead.pi.gov.br") // matched by first DNS label, case-insensitive
	insert("app-a", "app.a.gov.br")                          // "app" is ambiguous: never matched
	insert("app-b", "app.b.gov.br")
	dhcp := insert("dhcp", "10.0.0.50")

	ms := []proxmox.Machine{
		{ProxmoxID: "qemu/1", Kind: "qemu", Name: "gitlab", Node: "pve1", IP: "10.0.0.9", Running: true},
		{ProxmoxID: "qemu/2", Kind: "qemu", Name: "app", Node: "pve1", Running: true},
		{ProxmoxID: "qemu/3", Kind: "qemu", Name: "dhcp-vm", Node: "pve1", IP: "10.0.0.50", Running: true},
	}
	sum, err := svc.SyncFromProxmox(ctx, pve, ms)
	if err != nil {
		t.Fatal(err)
	}
	if want := (service.ProxmoxSyncSummary{Found: 3, Created: 1, Updated: 2, NoIP: 1}); !reflect.DeepEqual(withoutReasons(sum), want) {
		t.Fatalf("sync = %+v, want %+v", sum, want)
	}
	hosts := store.NewHostRepo(d.SQL)
	byPID, _, _, err := hosts.ProxmoxIndex(ctx, pve)
	if err != nil {
		t.Fatal(err)
	}
	if byPID["qemu/1"] != gitlab || byPID["qemu/3"] != dhcp {
		t.Fatalf("links = %v, want gitlab %d, dhcp %d", byPID, gitlab, dhcp)
	}
	if h, _ := hosts.GetByID(ctx, gitlab); h.Hostname != "GitLab.sead.pi.gov.br" {
		t.Fatalf("DNS hostname overwritten: %q", h.Hostname)
	}

	// The guest's IP moves (DHCP): an IP hostname follows it; an unknown IP keeps it.
	ms[2].IP = "10.0.0.51"
	if _, err := svc.SyncFromProxmox(ctx, pve, ms); err != nil {
		t.Fatal(err)
	}
	if h, _ := hosts.GetByID(ctx, dhcp); h.Hostname != "10.0.0.51" {
		t.Fatalf("IP hostname = %q, want 10.0.0.51", h.Hostname)
	}
	ms[2].IP = ""
	if _, err := svc.SyncFromProxmox(ctx, pve, ms); err != nil {
		t.Fatal(err)
	}
	if h, _ := hosts.GetByID(ctx, dhcp); h.Hostname != "10.0.0.51" {
		t.Fatalf("IP hostname cleared by a missing IP: %q", h.Hostname)
	}
}

// Two independent clusters: "qemu/101" exists on both and each server keeps
// its own host; a sync only retires its own server's hosts; deleting a server
// orphans its hosts, which the next sync re-links by IP instead of duplicating.
func TestHostService_SyncFromProxmox_TwoServers(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	a := newProxmoxServer(t, d, "pve-a")
	b := newProxmoxServer(t, d, "pve-b")
	vmA := proxmox.Machine{ProxmoxID: "qemu/101", Kind: "qemu", Name: "app-a", Node: "pve1", VMID: 101, IP: "10.1.0.101", Running: true}
	vmB := proxmox.Machine{ProxmoxID: "qemu/101", Kind: "qemu", Name: "app-b", Node: "pve1", VMID: 101, IP: "10.2.0.101", Running: true}
	if sum, err := svc.SyncFromProxmox(ctx, a, []proxmox.Machine{vmA}); err != nil || sum.Created != 1 {
		t.Fatalf("sync A = %+v, %v", sum, err)
	}
	if sum, err := svc.SyncFromProxmox(ctx, b, []proxmox.Machine{vmB}); err != nil || sum.Created != 1 {
		t.Fatalf("sync B = %+v, %v (B must not claim A's qemu/101)", sum, err)
	}
	hosts := store.NewHostRepo(d.SQL)
	pidA, _, _, _ := hosts.ProxmoxIndex(ctx, a)
	pidB, _, _, _ := hosts.ProxmoxIndex(ctx, b)
	hostA, hostB := pidA["qemu/101"], pidB["qemu/101"]
	if hostA == 0 || hostB == 0 || hostA == hostB {
		t.Fatalf("qemu/101 hosts = A %d, B %d; want two distinct", hostA, hostB)
	}

	// A re-sync of A that no longer sees qemu/101 retires A's host only.
	other := proxmox.Machine{ProxmoxID: "qemu/102", Kind: "qemu", Name: "other", Node: "pve1", VMID: 102, Running: true}
	if sum, err := svc.SyncFromProxmox(ctx, a, []proxmox.Machine{other}); err != nil || sum.Deactivated != 1 {
		t.Fatalf("sync A without 101 = %+v, %v", sum, err)
	}
	if h, _ := hosts.GetByID(ctx, hostA); h.Situacao != "inactive" {
		t.Fatalf("A's host situacao = %q, want inactive", h.Situacao)
	}
	if h, _ := hosts.GetByID(ctx, hostB); h.Situacao != "active" {
		t.Fatalf("B's host situacao = %q: another server's sync deactivated it", h.Situacao)
	}

	// B is deleted and registered again: its host is re-linked, not duplicated.
	if err := store.NewProxmoxServerRepo(d.SQL).Delete(ctx, b); err != nil {
		t.Fatal(err)
	}
	b2 := newProxmoxServer(t, d, "pve-b-again")
	sum, err := svc.SyncFromProxmox(ctx, b2, []proxmox.Machine{vmB})
	if err != nil || sum.Created != 0 || sum.Updated != 1 {
		t.Fatalf("sync after re-adding B = %+v, %v; want the orphan re-linked", sum, err)
	}
	if pid, _, _, _ := hosts.ProxmoxIndex(ctx, b2); pid["qemu/101"] != hostB {
		t.Fatalf("re-linked to %d, want %d", pid["qemu/101"], hostB)
	}
}

func newProxmoxServer(t *testing.T, d *database.DB, name string) int64 {
	t.Helper()
	s := &models.ProxmoxServer{Name: name, Enabled: true}
	if err := store.NewProxmoxServerRepo(d.SQL).Create(context.Background(), s); err != nil {
		t.Fatalf("server %s: %v", name, err)
	}
	return s.ID
}

func withoutReasons(s service.ProxmoxSyncSummary) service.ProxmoxSyncSummary {
	s.NoIPReasons = nil
	return s
}
