package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Situação roles survive an admin renaming the options: the code keeps
// finding "active"/"maintenance" hosts and writes the renamed values.
func TestSituacaoRoles_FollowRenamedOptions(t *testing.T) {
	ctx := context.Background()
	hosts, d := newHostRepo(t)
	enums := store.NewEnumOptionRepo(d.SQL)

	// Migration v88 pinned the roles to the factory options.
	opts, err := enums.List(ctx, "situacao")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	roles := map[string]string{}
	for _, o := range opts {
		roles[o.Value] = o.Role
	}
	if roles["active"] != "active" || roles["inactive"] != "inactive" || roles["maintenance"] != "maintenance" {
		t.Fatalf("roles after migration = %v", roles)
	}

	// The admin renames them, as on the real install.
	for old, renamed := range map[string]string{"active": "Ativa", "maintenance": "Em manutenção", "inactive": "Desligada"} {
		if err := enums.Update(ctx, "situacao", old, renamed, ""); err != nil {
			t.Fatalf("rename %s: %v", old, err)
		}
	}
	if v, err := enums.SituacaoValue(ctx, store.SituacaoActive); err != nil || v != "Ativa" {
		t.Fatalf("SituacaoValue(active) = %q, %v; want Ativa", v, err)
	}

	mk := func(slug, situacao string) int64 {
		t.Helper()
		h := &models.Host{Nickname: slug, OficialSlug: slug, Hostname: "10.0.0.1", Situacao: situacao}
		if err := hosts.Create(ctx, h); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
		return h.ID
	}
	renamed := mk("renamed", "Ativa")
	legacy := mk("legacy", "active") // a row written before roles
	mk("off", "Desligada")
	maint := mk("maint", "Em manutenção")

	ssh, err := hosts.ListForSSHConfig(ctx)
	if err != nil {
		t.Fatalf("ssh config: %v", err)
	}
	got := map[int64]bool{}
	for _, h := range ssh {
		got[h.ID] = true
	}
	if len(ssh) != 2 || !got[renamed] || !got[legacy] {
		t.Fatalf("SSH config hosts = %v, want the Ativa and legacy active ones", got)
	}

	// Proxmox writes the renamed value, and never over a manual maintenance.
	pve := &models.ProxmoxServer{Name: "pve", Enabled: true}
	if err := store.NewProxmoxServerRepo(d.SQL).Create(ctx, pve); err != nil {
		t.Fatalf("server: %v", err)
	}
	if err := hosts.SetProxmoxSync(ctx, renamed, store.ProxmoxState{ServerID: pve.ID, ProxmoxID: "qemu/1", SituacaoRole: store.SituacaoInactive}); err != nil {
		t.Fatalf("proxmox sync: %v", err)
	}
	if err := hosts.SetProxmoxSync(ctx, maint, store.ProxmoxState{ServerID: pve.ID, ProxmoxID: "qemu/2", SituacaoRole: store.SituacaoActive}); err != nil {
		t.Fatalf("proxmox sync maint: %v", err)
	}
	if h, _ := hosts.GetByID(ctx, renamed); h.Situacao != "Desligada" {
		t.Fatalf("stopped guest situação = %q, want Desligada", h.Situacao)
	}
	if h, _ := hosts.GetByID(ctx, maint); h.Situacao != "Em manutenção" {
		t.Fatalf("maintenance overwritten: %q", h.Situacao)
	}
	if n, err := hosts.DeactivateMissingProxmox(ctx, pve.ID, []string{"qemu/1"}); err != nil || n != 0 {
		t.Fatalf("DeactivateMissingProxmox = %d, %v; want 0 (qemu/2 is in maintenance)", n, err)
	}
}
