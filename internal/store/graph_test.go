package store_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Links returns each link table as (from, to) pairs in the documented direction.
func TestGraphRepo_Links(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	id := func(q string, args ...any) int64 {
		t.Helper()
		var v int64
		if err := d.SQL.QueryRow(q, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return v
	}
	host := id(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h') RETURNING id`)
	dns := id(`INSERT INTO dns_records (domain) VALUES ('a.gov') RETURNING id`)
	proj := id(`INSERT INTO projects (name) VALUES ('p') RETURNING id`)
	svc := id(`INSERT INTO services (nickname) VALUES ('s1') RETURNING id`)
	dep := id(`INSERT INTO services (nickname) VALUES ('s2') RETURNING id`)
	for _, q := range []struct {
		sql  string
		a, b int64
	}{
		{`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dns, host},
		{`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svc, host},
		{`INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?)`, svc, dns},
		{`INSERT INTO project_host_links (project_id, host_id) VALUES (?, ?)`, proj, host},
		{`INSERT INTO service_dependencies (service_id, depends_on_id) VALUES (?, ?)`, svc, dep},
		{`INSERT INTO project_dns_links (project_id, dns_id) VALUES (?, ?)`, proj, dns},
	} {
		if _, err := d.SQL.Exec(q.sql, q.a, q.b); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}

	got, err := store.NewGraphRepo(d.SQL).Links(ctx)
	if err != nil {
		t.Fatalf("Links: %v", err)
	}
	want := store.GraphLinks{
		DNSHost:        []store.LinkPair{{dns, host}},
		ServiceHost:    []store.LinkPair{{svc, host}},
		ServiceDNS:     []store.LinkPair{{svc, dns}},
		ProjectHost:    []store.LinkPair{{proj, host}},
		ServiceDepends: []store.LinkPair{{svc, dep}},
		ProjectDNS:     []store.LinkPair{{proj, dns}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Links:\n got %+v\nwant %+v", got, want)
	}
	if m := store.ByTo(got.ServiceHost); !reflect.DeepEqual(m[host], []int64{svc}) {
		t.Fatalf("ByTo(ServiceHost)[host] = %v, want [%d]", m[host], svc)
	}
	if m := store.ByFrom(got.ServiceDNS); !reflect.DeepEqual(m[svc], []int64{dns}) {
		t.Fatalf("ByFrom(ServiceDNS)[svc] = %v, want [%d]", m[svc], dns)
	}
}
