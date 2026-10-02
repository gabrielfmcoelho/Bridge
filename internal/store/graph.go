package store

import (
	"context"
	"database/sql"
)

// LinkPair is one row of a two-column link table: (from, to).
type LinkPair struct{ From, To int64 }

// GraphLinks holds every two-column link table between inventory assets. The
// topology graph draws edges from it, and list endpoints enrich a page of rows
// from it instead of querying links per row.
type GraphLinks struct {
	DNSHost        []LinkPair // dns_id -> host_id
	ServiceHost    []LinkPair // service_id -> host_id
	ServiceDNS     []LinkPair // service_id -> dns_id
	ProjectHost    []LinkPair // project_id -> host_id
	ServiceDepends []LinkPair // service_id -> depends_on_id
	ProjectDNS     []LinkPair // project_id -> dns_id
	APIService     []LinkPair // api_id -> service_id
	APIConsumer    []LinkPair // api_id -> consumer service_id
	APIProject     []LinkPair // api_id -> project_id
}

// ByFrom groups pairs as from -> []to.
func ByFrom(pairs []LinkPair) map[int64][]int64 {
	m := make(map[int64][]int64)
	for _, p := range pairs {
		m[p.From] = append(m[p.From], p.To)
	}
	return m
}

// ByTo groups pairs as to -> []from.
func ByTo(pairs []LinkPair) map[int64][]int64 {
	m := make(map[int64][]int64)
	for _, p := range pairs {
		m[p.To] = append(m[p.To], p.From)
	}
	return m
}

// GraphRepo reads the graph's edges in one query per link table, instead of
// one query per entity (the per-id HostIDs/DNSIDs helpers made /api/graph
// issue ~1k round-trips). Rows are unscoped: callers only use ids of assets
// they already listed through VisibleExpr, so a link to an invisible asset is
// never followed (graph edges need both ends in the scoped node maps; list
// enrichment keys by the listed row's own id). Links from soft-deleted
// services/projects are dropped where a count reads them (DNS list), matching
// ServiceRepo.CountsByHost.
type GraphRepo struct{ db *sql.DB }

func NewGraphRepo(db *sql.DB) *GraphRepo { return &GraphRepo{db: db} }

func (r *GraphRepo) Links(ctx context.Context) (GraphLinks, error) {
	var g GraphLinks
	for _, q := range []struct {
		dst  *[]LinkPair
		from string
	}{
		{&g.DNSHost, `SELECT l.dns_id, l.host_id FROM dns_host_links l JOIN dns_records d ON d.id = l.dns_id WHERE d.deleted_at IS NULL`},
		{&g.ServiceHost, `SELECT service_id, host_id FROM service_host_links`},
		{&g.ServiceDNS, `SELECT l.service_id, l.dns_id FROM service_dns_links l JOIN services s ON s.id = l.service_id JOIN dns_records d ON d.id = l.dns_id WHERE s.deleted_at IS NULL AND d.deleted_at IS NULL`},
		{&g.ProjectHost, `SELECT project_id, host_id FROM project_host_links`},
		{&g.ServiceDepends, `SELECT service_id, depends_on_id FROM service_dependencies`},
		{&g.ProjectDNS, `SELECT l.project_id, l.dns_id FROM project_dns_links l JOIN projects p ON p.id = l.project_id JOIN dns_records d ON d.id = l.dns_id WHERE p.deleted_at IS NULL AND d.deleted_at IS NULL`},
		{&g.APIService, `SELECT api_id, service_id FROM api_service_links`},
		{&g.APIConsumer, `SELECT api_id, service_id FROM api_consumer_links`},
		{&g.APIProject, `SELECT api_id, project_id FROM api_project_links`},
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
