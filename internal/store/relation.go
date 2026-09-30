package store

import (
	"context"
	"database/sql"
)

// Relation is one undirected link between two inventory entities, e.g.
// {A: "service", AID: 3, B: "host", BID: 7}. Types are asset types, plus
// "entidade" for the org units an asset is created by or responsible to.
type Relation struct {
	A   string `json:"a"`
	AID int64  `json:"a_id"`
	B   string `json:"b"`
	BID int64  `json:"b_id"`
}

// RelationRepo reads every link between hosts, DNS records, services,
// projects, APIs, contacts and entidades in one query, for list pages that group
// their items by a related entity.
type RelationRepo struct{ db *sql.DB }

// NewRelationRepo constructs a RelationRepo over db.
func NewRelationRepo(db *sql.DB) *RelationRepo { return &RelationRepo{db: db} }

// relationEdges unions the link tables. Project links include the indirect
// ones through a service's project_id, matching the project counts on the
// host list. Soft-deleted rows are not filtered here: the caller only groups
// items it can name from its own (live) lists.
const relationEdges = `
	SELECT 'dns' AS a, dns_id AS a_id, 'host' AS b, host_id AS b_id FROM dns_host_links
	UNION SELECT 'service', service_id, 'host', host_id FROM service_host_links
	UNION SELECT 'service', service_id, 'dns', dns_id FROM service_dns_links
	UNION SELECT 'project', project_id, 'service', id FROM services WHERE project_id IS NOT NULL
	UNION SELECT 'project', project_id, 'host', host_id FROM project_host_links
	UNION SELECT 'project', s.project_id, 'host', l.host_id FROM services s JOIN service_host_links l ON l.service_id = s.id WHERE s.project_id IS NOT NULL
	UNION SELECT 'project', project_id, 'dns', dns_id FROM project_dns_links
	UNION SELECT 'project', s.project_id, 'dns', l.dns_id FROM services s JOIN service_dns_links l ON l.service_id = s.id WHERE s.project_id IS NOT NULL
	UNION SELECT 'api_catalog', l.api_id, 'service', l.service_id FROM api_service_links l JOIN api_catalog a ON a.id = l.api_id WHERE a.deleted_at IS NULL
	UNION SELECT 'api_catalog', l.api_id, 'project', l.project_id FROM api_project_links l JOIN api_catalog a ON a.id = l.api_id WHERE a.deleted_at IS NULL
	UNION SELECT 'project', s.project_id, 'api_catalog', l.api_id FROM services s JOIN api_service_links l ON l.service_id = s.id JOIN api_catalog a ON a.id = l.api_id WHERE s.project_id IS NOT NULL AND a.deleted_at IS NULL
	UNION SELECT 'contact', contact_id, entity_type, entity_id FROM responsaveis
	UNION SELECT 'entidade', entidade_id, asset_type, asset_id FROM asset_entidades
		WHERE relation IN ('creator', 'responsible') AND asset_type IN ('host', 'dns', 'service', 'project', 'api_catalog')`

// All returns every link whose ends the caller can see. Both ends are scoped
// (an entidade end is an org unit, not an asset, so only its asset end is),
// so a link never reveals an asset outside the caller's entidades.
func (r *RelationRepo) All(ctx context.Context) ([]Relation, error) {
	visA, argsA := VisibleExprDyn(ctx, "e.a", "e.a_id")
	visB, argsB := VisibleExprDyn(ctx, "e.b", "e.b_id")
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.a, e.a_id, e.b, e.b_id FROM (`+relationEdges+`) e
		WHERE (e.a = 'entidade' OR `+visA+`) AND `+visB+`
		ORDER BY e.a, e.a_id, e.b, e.b_id`,
		append(argsA, argsB...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Relation{}
	for rows.Next() {
		var rel Relation
		if err := rows.Scan(&rel.A, &rel.AID, &rel.B, &rel.BID); err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}
