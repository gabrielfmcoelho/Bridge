package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/proxmox"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// ProxmoxSyncSummary is what one Proxmox → hosts sync did.
type ProxmoxSyncSummary struct {
	Found       int `json:"found"`
	Created     int `json:"created"`
	Updated     int `json:"updated"`
	Deactivated int `json:"deactivated"`
	NoIP        int `json:"no_ip"`
}

// proxmoxDefaultEntidade owns synced hosts that have no node to inherit from
// (v82: every VM is created by ETIPI).
const proxmoxDefaultEntidade = "etipi"

// SyncFromProxmox upserts one host per Proxmox node / guest. A machine matches
// a host by proxmox_id, then by IP or name (= hosts.hostname, or its first DNS
// label); each host is claimed by one machine at most. The match is linked and
// gets its resources, situacao and parent node refreshed, nothing else. New
// hosts inherit their node host's entidade grants, else ETIPI's. Linked hosts
// whose machine is gone become inactive — skipped when ms is empty, so a
// broken fetch can't retire the fleet. Nodes must precede guests in ms
// (proxmox.Machines does). Runs unscoped — the caller must be admin.
func (s *HostService) SyncFromProxmox(ctx context.Context, ms []proxmox.Machine) (ProxmoxSyncSummary, error) {
	ctx = store.WithSystemScope(ctx)
	sum := ProxmoxSyncSummary{Found: len(ms)}
	byPID, byHostname, byLabel, err := s.hosts.ProxmoxIndex(ctx)
	if err != nil {
		return sum, err
	}
	nodeHost := map[string]int64{}
	claimed := map[int64]bool{}
	seen := make([]string, 0, len(ms))
	for _, m := range ms {
		seen = append(seen, m.ProxmoxID)
		if m.IP == "" {
			sum.NoIP++
		}
		st := store.ProxmoxState{
			ProxmoxID: m.ProxmoxID, IP: m.IP,
			RecursoCPU: m.CPU, RecursoRAM: m.RAM, RecursoArmazenamento: m.Disk,
			Situacao: "inactive",
		}
		if m.Running {
			st.Situacao = "active"
		}
		var parent int64
		if m.Kind != "node" {
			if id, ok := nodeHost[m.Node]; ok {
				parent = id
				st.ParentHostID = &parent
			}
		}

		id, ok := byPID[m.ProxmoxID]
		if !ok {
			name := strings.ToLower(m.Name)
			for _, c := range []int64{byHostname[m.IP], byHostname[name], byLabel[name]} {
				if c != 0 && !claimed[c] {
					id, ok, claimed[c] = c, true, true
					break
				}
			}
		}
		if ok {
			if err := s.hosts.SetProxmoxSync(ctx, id, st); err != nil {
				return sum, err
			}
			sum.Updated++
		} else {
			if id, err = s.createProxmoxHost(ctx, m, parent, st); err != nil {
				return sum, fmt.Errorf("create host for %s: %w", m.ProxmoxID, err)
			}
			sum.Created++
		}
		if m.Kind == "node" {
			nodeHost[m.Node] = id
		}
	}
	if len(seen) > 0 {
		if sum.Deactivated, err = s.hosts.DeactivateMissingProxmox(ctx, seen); err != nil {
			return sum, err
		}
	}
	return sum, nil
}

// createProxmoxHost inserts the host row for m under a free slug, with grants
// copied from its node host (parent != 0) or, failing that, ETIPI as creator,
// and links it (st) — all in one transaction, so a failure never leaves an
// unlinked or grant-less (admin-only) host for the next run to duplicate.
func (s *HostService) createProxmoxHost(ctx context.Context, m proxmox.Machine, parent int64, st store.ProxmoxState) (int64, error) {
	slug, err := s.freeSlug(ctx, store.Slugify(m.Name))
	if err != nil {
		return 0, err
	}
	h := models.Host{
		Nickname: m.Name, OficialSlug: slug, Hostname: m.IP,
		Hospedagem: "ETIPI", TipoMaquina: m.TipoMaquina(), Port: "22",
		Description: "Proxmox: " + m.ProxmoxID + " @ " + m.Node,
	}
	var g models.AssetGrants
	if parent != 0 {
		if g, err = s.grants.Get(ctx, store.AssetHost, parent); err != nil {
			return 0, err
		}
	}
	if g.CreatorEntidadeID == nil && !g.IsGlobal && len(g.ResponsibleEntidadeIDs) == 0 {
		etipi, err := store.NewEntidadeRepo(s.sqlDB).IDBySlug(ctx, proxmoxDefaultEntidade)
		if err != nil {
			return 0, err
		}
		if etipi != 0 {
			g.CreatorEntidadeID = &etipi
		}
	}
	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	hosts := s.hosts.WithTx(tx)
	if err := hosts.Create(ctx, &h); err != nil {
		return 0, err
	}
	if err := s.grants.Replace(ctx, tx, store.AssetHost, h.ID, g); err != nil {
		return 0, err
	}
	if err := hosts.SetProxmoxSync(ctx, h.ID, st); err != nil {
		return 0, err
	}
	return h.ID, tx.Commit()
}

// freeSlug returns base, or base-2, base-3, ... — the first slug not taken
// (soft-deleted hosts keep theirs reserved).
func (s *HostService) freeSlug(ctx context.Context, base string) (string, error) {
	if base == "" {
		base = "proxmox"
	}
	for i := 1; ; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		taken, err := s.hosts.SlugExists(ctx, slug, 0)
		if err != nil || !taken {
			return slug, err
		}
	}
}
