package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A route is its proxy host's config: a caller sees it only with the proxy
// host, and the other ends it can't see are blanked (a hidden backend reads
// unknown_host, a hidden record has no domain).
func TestWebRouteRepo_EntidadeScope(t *testing.T) {
	bg := context.Background()
	d := openDB(t)
	sga := entidadeID(t, d, "sga")
	grants := store.NewAssetEntidadeRepo(d.SQL)
	host := func(slug, addr string, granted bool) int64 {
		t.Helper()
		h := &models.Host{Nickname: slug, OficialSlug: slug, Hostname: addr, Situacao: "active"}
		if err := store.NewHostRepo(d.SQL).Create(bg, h); err != nil {
			t.Fatal(err)
		}
		if granted {
			if err := grants.Replace(bg, d.SQL, store.AssetHost, h.ID, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
				t.Fatal(err)
			}
		}
		return h.ID
	}
	proxy, backend := host("proxy", "10.0.0.10", true), host("backend", "10.0.0.11", true)
	hiddenProxy, hiddenBackend := host("hidden-proxy", "10.0.0.20", false), host("hidden-backend", "10.0.0.21", false)
	var rec int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('hidden.example') RETURNING id`).Scan(&rec); err != nil {
		t.Fatal(err)
	}
	repo := store.NewWebRouteRepo(d.SQL)
	route := func(from, to int64, addr string) store.WebRoute {
		return store.WebRoute{ProxyHostID: from, ServerName: "hidden.example", DNSID: rec, Path: "/" + addr,
			Target: addr + ":8080", TargetAddr: addr, TargetPort: 8080, TargetHostID: to}
	}
	for from, rs := range map[int64][]store.WebRoute{
		proxy:       {route(proxy, backend, "10.0.0.11"), route(proxy, hiddenBackend, "10.0.0.21")},
		hiddenProxy: {route(hiddenProxy, backend, "10.0.0.11")},
	} {
		if err := repo.ReplaceForHost(bg, from, nil, rs); err != nil {
			t.Fatal(err)
		}
	}
	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})

	in, err := repo.Inbound(scoped, backend)
	if err != nil || len(in) != 1 || in[0].ProxyHostID != proxy || in[0].DNSID != 0 || in[0].Domain != "" {
		t.Fatalf("scoped inbound = %+v, %v; want only the visible proxy's route, record blanked", in, err)
	}
	if all, _ := repo.Inbound(bg, backend); len(all) != 2 {
		t.Fatalf("unscoped inbound = %d rows, want 2", len(all))
	}
	out, err := repo.Outbound(scoped, proxy)
	if err != nil || len(out) != 2 {
		t.Fatalf("scoped outbound = %+v, %v; want 2 rows", out, err)
	}
	for _, v := range out {
		hidden := v.TargetAddr == "10.0.0.21"
		if hidden && (v.TargetHostID != 0 || v.TargetHostSlug != "" || v.Status != store.RouteUnknownHost) {
			t.Fatalf("hidden backend leaked: %+v", v)
		}
		if !hidden && (v.TargetHostSlug != "backend" || v.Status != store.RouteNotScanned) {
			t.Fatalf("visible backend = %+v, want backend / not_scanned", v)
		}
	}
	if got, err := repo.Outbound(scoped, hiddenProxy); err != nil || len(got) != 0 {
		t.Fatalf("scoped outbound of a hidden proxy = %+v, %v; want none", got, err)
	}
}
