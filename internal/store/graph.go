package store

import (
	"context"
	"database/sql"
)

// LinkPair is one row of a two-column link table: (from, to).
type LinkPair struct{ From, To int64 }

// GraphLinks holds every link table the topology graph draws edges from.
type GraphLinks struct {
	DNSHost        []LinkPair // dns_id -> host_id
	ServiceHost    []LinkPair // service_id -> host_id
	ServiceDNS     []LinkPair // service_id -> dns_id
	ProjectHost    []LinkPair // project_id -> host_id
	ServiceDepends []LinkPair // service_id -> depends_on_id
}

// GraphRepo reads the graph's edges in one query per link table, instead of
// one query per entity (the per-id HostIDs/DNSIDs helpers made /api/graph
// issue ~1k round-trips). Visibility is applied by the caller, which only
// draws an edge when both ends are in its (already scoped) node lists.
type GraphRepo struct{ db *sql.DB }

func NewGraphRepo(db *sql.DB) *GraphRepo { return &GraphRepo{db: db} }

func (r *GraphRepo) Links(ctx context.Context) (GraphLinks, error) {
	var g GraphLinks
	for _, q := range []struct {
		dst  *[]LinkPair
		from string
	}{
		{&g.DNSHost, `SELECT dns_id, host_id FROM dns_host_links`},
		{&g.ServiceHost, `SELECT service_id, host_id FROM service_host_links`},
		{&g.ServiceDNS, `SELECT service_id, dns_id FROM service_dns_links`},
		{&g.ProjectHost, `SELECT project_id, host_id FROM project_host_links`},
		{&g.ServiceDepends, `SELECT service_id, depends_on_id FROM service_dependencies`},
	} {
		rows, err := r.db.QueryContext(ctx, q.from)
		if err != nil {
			return g, err
		}
		for rows.Next() {
			var p LinkPair
			if err := rows.Scan(&p.From, &p.To); err != nil {
				rows.Close()
				return g, err
			}
			*q.dst = append(*q.dst, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return g, err
		}
	}
	return g, nil
}
