package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestRebind_AnyPlaceholder(t *testing.T) {
	if got := database.Rebind("x = ANY(?) AND y = ?"); got != "x = ANY($1) AND y = $2" {
		t.Fatalf("Rebind = %q", got)
	}
}

// entidadeID looks up a seeded (migration v82) entidade by slug.
func entidadeID(t *testing.T, d *database.DB, slug string) int64 {
	t.Helper()
	var id int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = ?`, slug).Scan(&id); err != nil {
		t.Fatalf("entidade %s: %v", slug, err)
	}
	return id
}

func seedHost(t *testing.T, d *database.DB, slug string) int64 {
	t.Helper()
	var id int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES (?, ?) RETURNING id`, slug, slug).Scan(&id); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	return id
}

func countVisibleHosts(t *testing.T, ctx context.Context, d *database.DB) int {
	t.Helper()
	vis, args := store.VisibleExpr(ctx, store.AssetHost, "hosts.id")
	var n int
	if err := d.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE deleted_at IS NULL AND `+vis, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestVisibleExpr_GrantsGlobalAndAncestors(t *testing.T) {
	d := openDB(t)
	bg := context.Background()
	etipi := entidadeID(t, d, "etipi")
	govpi := entidadeID(t, d, "govpi")
	sga := entidadeID(t, d, "sga")

	hEtipi := seedHost(t, d, "h-etipi")   // creator = ETIPI
	hSga := seedHost(t, d, "h-sga")       // responsible = SGA
	hGlobal := seedHost(t, d, "h-global") // global
	_ = seedHost(t, d, "h-none")          // unassigned ⇒ admin-only
	grants := store.NewAssetEntidadeRepo(d.SQL)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(grants.Replace(bg, d.SQL, store.AssetHost, hEtipi, models.AssetGrants{CreatorEntidadeID: &etipi}))
	must(grants.Replace(bg, d.SQL, store.AssetHost, hSga, models.AssetGrants{ResponsibleEntidadeIDs: []int64{sga}}))
	must(grants.Replace(bg, d.SQL, store.AssetHost, hGlobal, models.AssetGrants{IsGlobal: true}))

	cases := []struct {
		name string
		ctx  context.Context
		want int
	}{
		{"absent scope = unscoped", bg, 4},
		{"system scope", store.WithSystemScope(bg), 4},
		{"admin", store.WithScope(bg, store.Scope{Admin: true}), 4},
		{"empty scope sees only global", store.WithScope(bg, store.Scope{EntidadeIDs: []int64{}}), 1},
		{"nil ids behaves like empty", store.WithScope(bg, store.Scope{}), 1},
		{"etipi sees own + global", store.WithScope(bg, store.Scope{EntidadeIDs: []int64{etipi}}), 2},
		{"sga responsible + global", store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}}), 2},
		{"union etipi+sga", store.WithScope(bg, store.Scope{EntidadeIDs: []int64{etipi, sga}}), 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countVisibleHosts(t, tc.ctx, d); got != tc.want {
				t.Fatalf("visible = %d, want %d", got, tc.want)
			}
		})
	}

	// Ancestor rule via ScopeForUser: a GovPI member's visible set is the whole
	// tree, so ETIPI- and SGA-granted hosts are both visible; an SGA member
	// sees neither ETIPI's host nor anything above.
	var uGov, uSga int64
	must(d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('gov','x','viewer') RETURNING id`).Scan(&uGov))
	must(d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('sga','x','viewer') RETURNING id`).Scan(&uSga))
	members := store.NewUserEntidadeRepo(d.SQL)
	must(members.Replace(bg, uGov, []int64{govpi}, govpi))
	must(members.Replace(bg, uSga, []int64{sga}, sga))
	ents := store.NewEntidadeRepo(d.SQL)

	scGov, err := ents.ScopeForUser(bg, uGov)
	must(err)
	if scGov.PrimaryEntidadeID != govpi || len(scGov.EntidadeIDs) != 11 {
		t.Fatalf("gov scope = %+v, want primary=%d and 11 ids", scGov, govpi)
	}
	if got := countVisibleHosts(t, store.WithScope(bg, scGov), d); got != 3 {
		t.Fatalf("gov visible = %d, want 3", got)
	}
	scSga, err := ents.ScopeForUser(bg, uSga)
	must(err)
	if got := countVisibleHosts(t, store.WithScope(bg, scSga), d); got != 2 {
		t.Fatalf("sga visible = %d, want 2", got)
	}

	// CanSee mirrors the predicate for single ids.
	if ok, _ := store.CanSee(store.WithScope(bg, scSga), d.SQL, store.AssetHost, hEtipi); ok {
		t.Fatal("sga must not see etipi host")
	}
	if ok, _ := store.CanSee(store.WithScope(bg, scGov), d.SQL, store.AssetHost, hEtipi); !ok {
		t.Fatal("gov must see etipi host")
	}

	// Get round-trips the grants; Replace is idempotent and clears old rows.
	g, err := grants.Get(bg, store.AssetHost, hEtipi)
	must(err)
	if g.CreatorEntidadeID == nil || *g.CreatorEntidadeID != etipi || g.IsGlobal || len(g.ResponsibleEntidadeIDs) != 0 {
		t.Fatalf("get = %+v", g)
	}
	must(grants.Replace(bg, d.SQL, store.AssetHost, hEtipi, models.AssetGrants{CreatorEntidadeID: &etipi, ResponsibleEntidadeIDs: []int64{sga, govpi}, IsGlobal: true}))
	g, _ = grants.Get(bg, store.AssetHost, hEtipi)
	if !g.IsGlobal || len(g.ResponsibleEntidadeIDs) != 2 {
		t.Fatalf("after replace = %+v", g)
	}
}
