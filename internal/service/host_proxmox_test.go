package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/proxmox"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostService_SyncFromProxmox(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)

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
	sum, err := svc.SyncFromProxmox(ctx, ms)
	if err != nil {
		t.Fatal(err)
	}
	if want := (service.ProxmoxSyncSummary{Found: 4, Created: 3, Updated: 1, NoIP: 1}); sum != want {
		t.Fatalf("first sync = %+v, want %+v", sum, want)
	}

	hosts := store.NewHostRepo(d.SQL)
	byPID, _, _, err := hosts.ProxmoxIndex(ctx)
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
	sum, err = svc.SyncFromProxmox(ctx, ms)
	if err != nil {
		t.Fatal(err)
	}
	if want := (service.ProxmoxSyncSummary{Found: 4, Created: 1, Updated: 3, Deactivated: 1, NoIP: 1}); sum != want {
		t.Fatalf("second sync = %+v, want %+v", sum, want)
	}
	if web, _ = hosts.GetByID(ctx, web.ID); web.Situacao != "inactive" {
		t.Fatalf("vanished web situacao = %q", web.Situacao)
	}
	if cache, _ = hosts.GetByID(ctx, cache.ID); cache.Situacao != "maintenance" {
		t.Fatalf("maintenance overwritten: %q", cache.Situacao)
	}
	byPID, _, _, _ = hosts.ProxmoxIndex(ctx)
	if g, _ := grants.Get(ctx, store.AssetHost, byPID["qemu/104"]); g.CreatorEntidadeID == nil || *g.CreatorEntidadeID != other {
		t.Fatalf("new guest grants = %+v, want node's creator %d", g, other)
	}

	// A vanished host in maintenance keeps it.
	if _, err = svc.SyncFromProxmox(ctx, ms[:2]); err != nil {
		t.Fatal(err)
	}
	if cache, _ = hosts.GetByID(ctx, cache.ID); cache.Situacao != "maintenance" {
		t.Fatalf("vanished maintenance host situacao = %q", cache.Situacao)
	}

	// An empty fetch retires nothing.
	if sum, err = svc.SyncFromProxmox(ctx, nil); err != nil || sum.Deactivated != 0 {
		t.Fatalf("empty sync = %+v, %v", sum, err)
	}
	if m, _ = hosts.GetByID(ctx, manual); m.Situacao != "active" {
		t.Fatalf("empty sync deactivated manual host: %q", m.Situacao)
	}
}

func TestHostService_SyncFromProxmox_Matching(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
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
	sum, err := svc.SyncFromProxmox(ctx, ms)
	if err != nil {
		t.Fatal(err)
	}
	if want := (service.ProxmoxSyncSummary{Found: 3, Created: 1, Updated: 2, NoIP: 1}); sum != want {
		t.Fatalf("sync = %+v, want %+v", sum, want)
	}
	hosts := store.NewHostRepo(d.SQL)
	byPID, _, _, err := hosts.ProxmoxIndex(ctx)
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
	if _, err := svc.SyncFromProxmox(ctx, ms); err != nil {
		t.Fatal(err)
	}
	if h, _ := hosts.GetByID(ctx, dhcp); h.Hostname != "10.0.0.51" {
		t.Fatalf("IP hostname = %q, want 10.0.0.51", h.Hostname)
	}
	ms[2].IP = ""
	if _, err := svc.SyncFromProxmox(ctx, ms); err != nil {
		t.Fatal(err)
	}
	if h, _ := hosts.GetByID(ctx, dhcp); h.Hostname != "10.0.0.51" {
		t.Fatalf("IP hostname cleared by a missing IP: %q", h.Hostname)
	}
}
