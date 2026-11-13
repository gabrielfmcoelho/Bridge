package store_test

import (
	"context"
	"slices"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A link is only returned when the caller can see both of its ends.
func TestRelationRepo_All_Scoped(t *testing.T) {
	d := openDB(t)
	ctx := context.Background()
	id := func(q string, args ...any) int64 {
		var v int64
		if err := d.SQL.QueryRow(q, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return v
	}
	exec := func(q string, args ...any) {
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	sga := id(`SELECT id FROM entidades WHERE slug = 'sga'`)
	mine := id(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('mine', 'rel-mine') RETURNING id`)
	hidden := id(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('hidden', 'rel-hidden') RETURNING id`)
	dns := id(`INSERT INTO dns_records (domain) VALUES ('rel.example.org') RETURNING id`)
	contact := id(`INSERT INTO contacts (name) VALUES ('Ana') RETURNING id`)
	exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?), (?, ?)`, dns, mine, dns, hidden)
	exec(`INSERT INTO responsaveis (entity_type, entity_id, contact_id) VALUES ('host', ?, ?), ('host', ?, ?)`, mine, contact, hidden, contact)
	for _, g := range []struct {
		typ string
		id  int64
	}{{"host", mine}, {"dns", dns}, {"contact", contact}} {
		exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES (?, ?, ?, 'creator')`, g.typ, g.id, sga)
	}

	// Contacts are grouped by, never grouped, so their own entidade grants are
	// not links.
	repo := store.NewRelationRepo(d.SQL)
	scoped, err := repo.All(store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}}))
	if err != nil {
		t.Fatal(err)
	}
	want := []store.Relation{
		{A: "contact", AID: contact, B: "host", BID: mine},
		{A: "dns", AID: dns, B: "host", BID: mine},
		{A: "entidade", AID: sga, B: "dns", BID: dns},
		{A: "entidade", AID: sga, B: "host", BID: mine},
	}
	if !slices.Equal(scoped, want) {
		t.Fatalf("scoped relations =\n%+v\nwant\n%+v", scoped, want)
	}

	all, err := repo.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(all, store.Relation{A: "dns", AID: dns, B: "host", BID: hidden}) {
		t.Fatalf("unscoped relations miss the hidden host's link: %+v", all)
	}
}

// A project's indirect links (to the hosts, DNS records and APIs of its
// services) need the service in between visible too. SGA and SGP both see the
// project, host, record and API; the only path is SGP's service, so SGP gets
// the three edges and SGA none.
func TestRelationRepo_All_IndirectNeedsVisibleService(t *testing.T) {
	d := openDB(t)
	ctx := context.Background()
	id := func(q string, args ...any) int64 {
		var v int64
		if err := d.SQL.QueryRow(q, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return v
	}
	exec := func(q string, args ...any) {
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	sga, sgp := id(`SELECT id FROM entidades WHERE slug = 'sga'`), id(`SELECT id FROM entidades WHERE slug = 'sgp'`)
	user := id(`INSERT INTO users (username, password_hash, role) VALUES ('rel-indirect', 'x', 'admin') RETURNING id`)
	proj := id(`INSERT INTO projects (name) VALUES ('rel-shared') RETURNING id`)
	host := id(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('rel-h', 'rel-h') RETURNING id`)
	dns := id(`INSERT INTO dns_records (domain) VALUES ('rel-indirect.example.org') RETURNING id`)
	api := id(`INSERT INTO api_catalog (scope, name, source_type, spec_json, owner_user_id, created_by)
		VALUES ('avulso', 'rel-api', 'url', '{}', ?, ?) RETURNING id`, user, user)
	svc := id(`INSERT INTO services (nickname, project_id) VALUES ('rel-sgp', ?) RETURNING id`, proj)
	exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svc, host)
	exec(`INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?)`, svc, dns)
	exec(`INSERT INTO api_service_links (api_id, service_id) VALUES (?, ?)`, api, svc)
	for _, g := range []struct {
		typ string
		id  int64
	}{{"project", proj}, {"host", host}, {"dns", dns}, {"api_catalog", api}} {
		exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES (?, ?, ?, 'creator'), (?, ?, ?, 'responsible')`,
			g.typ, g.id, sga, g.typ, g.id, sgp)
	}
	exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES ('service', ?, ?, 'creator')`, svc, sgp)

	repo := store.NewRelationRepo(d.SQL)
	indirect := []store.Relation{
		{A: "project", AID: proj, B: "api_catalog", BID: api},
		{A: "project", AID: proj, B: "dns", BID: dns},
		{A: "project", AID: proj, B: "host", BID: host},
	}
	for _, c := range []struct {
		ent  int64
		name string
		want bool
	}{{sgp, "SGP", true}, {sga, "SGA", false}} {
		rels, err := repo.All(store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{c.ent}}))
		if err != nil {
			t.Fatal(err)
		}
		for _, rel := range indirect {
			if slices.Contains(rels, rel) != c.want {
				t.Errorf("%s has %+v = %v; want %v", c.name, rel, !c.want, c.want)
			}
		}
	}
}
