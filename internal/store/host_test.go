package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newHostRepo(t *testing.T) (*store.HostRepo, *database.DB) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewHostRepo(d.SQL), d
}

func TestHostRepo_CRUDAndLookups(t *testing.T) {
	ctx := context.Background()
	repo, _ := newHostRepo(t)

	h := &models.Host{Nickname: "web", OficialSlug: "web-01", Hostname: "10.0.0.1", User: "root", Situacao: "active", HasPassword: true}
	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("create: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("create did not set ID")
	}

	// GetByID / GetBySlug
	byID, err := repo.GetByID(ctx, h.ID)
	if err != nil || byID == nil || byID.OficialSlug != "web-01" || !byID.HasPassword {
		t.Fatalf("GetByID = %+v, %v", byID, err)
	}
	bySlug, err := repo.GetBySlug(ctx, "web-01")
	if err != nil || bySlug == nil || bySlug.ID != h.ID {
		t.Fatalf("GetBySlug = %+v, %v", bySlug, err)
	}
	if got, _ := repo.GetBySlug(ctx, "nope"); got != nil {
		t.Fatalf("GetBySlug(missing) = %+v, want nil", got)
	}

	// SlugExists (exclude self).
	if ex, _ := repo.SlugExists(ctx, "web-01", 0); !ex {
		t.Fatal("SlugExists(web-01,0) = false, want true")
	}
	if ex, _ := repo.SlugExists(ctx, "web-01", h.ID); ex {
		t.Fatal("SlugExists excluding self = true, want false")
	}

	// Update.
	h.Hostname = "10.0.0.2"
	h.Situacao = "inactive"
	if err := repo.Update(ctx, h); err != nil {
		t.Fatalf("update: %v", err)
	}
	byID, _ = repo.GetByID(ctx, h.ID)
	if byID.Hostname != "10.0.0.2" || byID.Situacao != "inactive" {
		t.Fatalf("after update = %+v", byID)
	}

	// Flag-only updaters.
	if err := repo.UpdateHasPassword(ctx, h.ID, false); err != nil {
		t.Fatalf("UpdateHasPassword: %v", err)
	}
	if err := repo.UpdateKeyMeta(ctx, h.ID, true, "/k", "yes"); err != nil {
		t.Fatalf("UpdateKeyMeta: %v", err)
	}
	byID, _ = repo.GetByID(ctx, h.ID)
	if byID.HasPassword || !byID.HasKey || byID.KeyPath != "/k" || byID.IdentitiesOnly != "yes" {
		t.Fatalf("after flag updates = %+v", byID)
	}

	// Coolify uuid.
	uuid := "srv-123"
	if err := repo.SetCoolifyUUID(ctx, h.ID, &uuid); err != nil {
		t.Fatalf("SetCoolifyUUID: %v", err)
	}
	byID, _ = repo.GetByID(ctx, h.ID)
	if byID.CoolifyServerUUID == nil || *byID.CoolifyServerUUID != "srv-123" {
		t.Fatalf("coolify uuid = %v", byID.CoolifyServerUUID)
	}

	// Delete.
	if err := repo.Delete(ctx, h.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.GetByID(ctx, h.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestHostRepo_ListFilterAndCounts(t *testing.T) {
	ctx := context.Background()
	repo, d := newHostRepo(t)

	seed := []models.Host{
		{Nickname: "alpha", OficialSlug: "alpha", Hostname: "a.internal", Situacao: "active", Hospedagem: "aws", PrecisaManutencao: true},
		{Nickname: "beta", OficialSlug: "beta", Hostname: "b.internal", Situacao: "inactive", Hospedagem: "gcp"},
		{Nickname: "gamma", OficialSlug: "gamma", Hostname: "c.internal", Situacao: "active", Hospedagem: "aws"},
	}
	for i := range seed {
		if err := repo.Create(ctx, &seed[i]); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	// Tag one host so the tag filter has something to hit.
	d.SQL.Exec(`INSERT INTO tags (entity_type, entity_id, tag) VALUES ('host', ?, 'prod')`, seed[0].ID)

	// List all (default sort by nickname asc).
	all, err := repo.List(ctx, models.HostFilter{})
	if err != nil || len(all) != 3 || all[0].Nickname != "alpha" {
		t.Fatalf("list all = %+v, %v", all, err)
	}

	// Filter by situacao.
	active, _ := repo.List(ctx, models.HostFilter{Situacao: "active"})
	if len(active) != 2 {
		t.Fatalf("active = %d, want 2", len(active))
	}

	// Filter by search (matches hostname).
	srch, _ := repo.List(ctx, models.HostFilter{Search: "a.internal"})
	if len(srch) != 1 || srch[0].Nickname != "alpha" {
		t.Fatalf("search = %+v, want [alpha]", srch)
	}

	// Filter by tag.
	tagged, _ := repo.List(ctx, models.HostFilter{Tag: "prod"})
	if len(tagged) != 1 || tagged[0].ID != seed[0].ID {
		t.Fatalf("tag filter = %+v", tagged)
	}

	// Pagination: per_page 2, page 1 → 2 rows; page 2 → 1 row.
	p1, _ := repo.List(ctx, models.HostFilter{PerPage: 2, Page: 1})
	p2, _ := repo.List(ctx, models.HostFilter{PerPage: 2, Page: 2})
	if len(p1) != 2 || len(p2) != 1 {
		t.Fatalf("pagination p1=%d p2=%d, want 2,1", len(p1), len(p2))
	}

	// Count (filtered) + CountAll.
	if n, _ := repo.Count(ctx, models.HostFilter{Situacao: "active"}); n != 2 {
		t.Fatalf("Count(active) = %d, want 2", n)
	}
	if n, _ := repo.CountAll(ctx); n != 3 {
		t.Fatalf("CountAll = %d, want 3", n)
	}
	if m, _ := repo.CountBySituacao(ctx); m["active"] != 2 || m["inactive"] != 1 {
		t.Fatalf("CountBySituacao = %v", m)
	}
	if m, _ := repo.CountByHospedagem(ctx); m["aws"] != 2 || m["gcp"] != 1 {
		t.Fatalf("CountByHospedagem = %v", m)
	}
	if n, _ := repo.NeedingMaintenanceCount(ctx); n != 1 {
		t.Fatalf("NeedingMaintenanceCount = %d, want 1", n)
	}

	// ListForSSHConfig returns only active hosts.
	cfg, _ := repo.ListForSSHConfig(ctx)
	if len(cfg) != 2 {
		t.Fatalf("ListForSSHConfig = %d, want 2 active", len(cfg))
	}
}

// TestHostRepo_SortByScanMetric verifies server-side ORDER BY on the latest
// scan's numeric resource metrics (P2/P3), with NULLS-LAST for hosts that have
// no scan.
func TestHostRepo_SortByScanMetric(t *testing.T) {
	ctx := context.Background()
	repo, d := newHostRepo(t)
	scans := store.NewHostScanRepo(d.SQL)

	mk := func(slug string, cpu string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES (?, ?) RETURNING id`, slug, slug).Scan(&id); err != nil {
			t.Fatalf("seed %s: %v", slug, err)
		}
		if cpu != "" {
			if err := scans.Create(ctx, id, `{"cpu_usage":"`+cpu+`"}`); err != nil {
				t.Fatalf("scan %s: %v", slug, err)
			}
		}
		return id
	}
	mk("low", "10%")
	mk("high", "90%")
	mk("mid", "50%")
	mk("noscan", "") // no scan → NULL cpu_pct → sorts last

	got, err := repo.List(ctx, models.HostFilter{SortBy: "resource_cpu", SortDir: "desc"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	order := make([]string, len(got))
	for i, h := range got {
		order[i] = h.OficialSlug
	}
	// desc by CPU: high, mid, low, then the scan-less host last (NULLS LAST).
	want := []string{"high", "mid", "low", "noscan"}
	if len(order) != 4 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] || order[3] != want[3] {
		t.Fatalf("order = %v, want %v", order, want)
	}
}
