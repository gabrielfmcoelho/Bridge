package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// DNSRepo owns SQL for dns_records and the dns_host_links junction. Used by the
// DNS service (enrichment) and inline by host/service/tool/dashboard handlers.
type DNSRepo struct {
	db *sql.DB
}

// NewDNSRepo constructs a DNSRepo over the given DB handle.
func NewDNSRepo(db *sql.DB) *DNSRepo { return &DNSRepo{db: db} }

const dnsCols = `id, domain, has_https, situacao, responsavel, observacoes, created_at, updated_at`

func scanDNS(scanner interface{ Scan(...any) error }, d *models.DNSRecord) error {
	return scanner.Scan(&d.ID, &d.Domain, &d.HasHTTPS, &d.Situacao, &d.Responsavel, &d.Observacoes, &d.CreatedAt, &d.UpdatedAt)
}

// Create inserts a DNS record and sets d.ID.
func (r *DNSRepo) Create(ctx context.Context, d *models.DNSRecord) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO dns_records (domain, has_https, situacao, responsavel, observacoes) VALUES (?, ?, ?, ?, ?)`,
		d.Domain, d.HasHTTPS, d.Situacao, d.Responsavel, d.Observacoes,
	)
	if err != nil {
		return err
	}
	d.ID = id
	return nil
}

// Get returns a DNS record by id, or (nil, nil) if absent (or invisible to the
// caller's entidade scope).
func (r *DNSRepo) Get(ctx context.Context, id int64) (*models.DNSRecord, error) {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	d := &models.DNSRecord{}
	err := scanDNS(r.db.QueryRowContext(ctx, `SELECT `+dnsCols+` FROM dns_records WHERE id = ? AND `+vis, append([]any{id}, vargs...)...), d)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// List returns all DNS records ordered by domain.
func (r *DNSRepo) List(ctx context.Context) ([]models.DNSRecord, error) {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	rows, err := r.db.QueryContext(ctx, `SELECT `+dnsCols+` FROM dns_records WHERE `+vis+` ORDER BY domain`, vargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []models.DNSRecord
	for rows.Next() {
		var d models.DNSRecord
		if err := scanDNS(rows, &d); err != nil {
			return nil, err
		}
		records = append(records, d)
	}
	return records, rows.Err()
}

// dnsWhere builds the shared WHERE predicates (and bound args) for the filtered
// list/count queries. Mirrors projectWhere, minus the soft-delete clause:
// dns_records has no deleted_at column. Dynamic clauses use `?` placeholders
// (the driver rebinds `?`→$N), ILIKE search via database.LikeOp(), and a tag
// subquery against the unified tags table. Always ends with the entidade
// visibility predicate for the caller's scope.
func dnsWhere(ctx context.Context, f models.DNSFilter) ([]string, []any) {
	var where []string
	var args []any

	if f.Search != "" {
		op := database.LikeOp()
		where = append(where, "(domain "+op+" ? OR responsavel "+op+" ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s)
	}
	if f.Situacao != "" {
		where = append(where, "situacao = ?")
		args = append(args, f.Situacao)
	}
	if f.Tag != "" {
		where = append(where, "id IN (SELECT entity_id FROM tags WHERE entity_type = 'dns' AND tag = ?)")
		args = append(args, f.Tag)
	}
	if f.Responsavel != "" {
		where = append(where, "responsavel = ?")
		args = append(args, f.Responsavel)
	}
	switch f.HasHTTPS {
	case "yes":
		where = append(where, "has_https = true")
	case "no":
		where = append(where, "has_https = false")
	}
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	where = append(where, vis)
	args = append(args, vargs...)
	return where, args
}

// ListFiltered returns DNS records matching the filter, with sort + pagination
// applied. Mirrors ProjectRepo.ListFiltered. When f.PerPage <= 0 the result is
// unbounded (no LIMIT) — the path the frontend's full-list query uses.
func (r *DNSRepo) ListFiltered(ctx context.Context, f models.DNSFilter) ([]models.DNSRecord, error) {
	query := `SELECT ` + dnsCols + ` FROM dns_records`
	where, args := dnsWhere(ctx, f)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	allowedSorts := map[string]string{
		"domain":      "domain",
		"situacao":    "situacao",
		"responsavel": "responsavel",
	}
	sortCol := "domain"
	if col, ok := allowedSorts[f.SortBy]; ok {
		sortCol = col
	}
	sortDir := "ASC"
	if f.SortDir == "desc" {
		sortDir = "DESC"
	}
	query += " ORDER BY " + sortCol + " " + sortDir

	if f.PerPage > 0 {
		offset := 0
		if f.Page > 1 {
			offset = (f.Page - 1) * f.PerPage
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PerPage, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []models.DNSRecord
	for rows.Next() {
		var d models.DNSRecord
		if err := scanDNS(rows, &d); err != nil {
			return nil, err
		}
		records = append(records, d)
	}
	return records, rows.Err()
}

// CountFiltered returns the number of DNS records matching the same predicates
// ListFiltered paginates over (no order/limit).
func (r *DNSRepo) CountFiltered(ctx context.Context, f models.DNSFilter) (int, error) {
	query := `SELECT COUNT(*) FROM dns_records`
	where, args := dnsWhere(ctx, f)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// Update writes the mutable fields of a DNS record by id. Invisible rows are
// untouched (0 rows affected).
func (r *DNSRepo) Update(ctx context.Context, d *models.DNSRecord) error {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	_, err := r.db.ExecContext(ctx,
		`UPDATE dns_records SET domain = ?, has_https = ?, situacao = ?, responsavel = ?, observacoes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND `+vis,
		append([]any{d.Domain, d.HasHTTPS, d.Situacao, d.Responsavel, d.Observacoes, d.ID}, vargs...)...,
	)
	return err
}

// Delete removes a DNS record by id. Invisible rows are untouched.
func (r *DNSRepo) Delete(ctx context.Context, id int64) error {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	_, err := r.db.ExecContext(ctx, `DELETE FROM dns_records WHERE id = ? AND `+vis, append([]any{id}, vargs...)...)
	return err
}

// SetHostLinks replaces all host links for a DNS record (one tx).
func (r *DNSRepo) SetHostLinks(ctx context.Context, dnsID int64, hostIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM dns_host_links WHERE dns_id = ?`, dnsID); err != nil {
		return err
	}
	for _, hid := range hostIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dnsID, hid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetLinksForHost replaces all DNS links for a host (one tx).
func (r *DNSRepo) SetLinksForHost(ctx context.Context, hostID int64, dnsIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM dns_host_links WHERE host_id = ?`, hostID); err != nil {
		return err
	}
	for _, did := range dnsIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, did, hostID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HostIDs returns all host ids linked to a DNS record.
func (r *DNSRepo) HostIDs(ctx context.Context, dnsID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id FROM dns_host_links WHERE dns_id = ?`, dnsID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// RecordsByHost returns all DNS records linked to a host, ordered by domain.
func (r *DNSRepo) RecordsByHost(ctx context.Context, hostID int64) ([]models.DNSRecord, error) {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "d.id")
	rows, err := r.db.QueryContext(ctx,
		`SELECT d.id, d.domain, d.has_https, d.situacao, d.responsavel, d.observacoes, d.created_at, d.updated_at
		 FROM dns_records d JOIN dns_host_links l ON d.id = l.dns_id
		 WHERE l.host_id = ? AND `+vis+` ORDER BY d.domain`, append([]any{hostID}, vargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []models.DNSRecord
	for rows.Next() {
		var d models.DNSRecord
		if err := scanDNS(rows, &d); err != nil {
			return nil, err
		}
		records = append(records, d)
	}
	return records, rows.Err()
}

// Count returns the total number of DNS records visible to the caller.
func (r *DNSRepo) Count(ctx context.Context) (int, error) {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dns_records WHERE `+vis, vargs...).Scan(&n)
	return n, err
}

// CountsByHost returns host_id → number of linked DNS records.
func (r *DNSRepo) CountsByHost(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id, COUNT(*) FROM dns_host_links GROUP BY host_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]int)
	for rows.Next() {
		var hostID int64
		var cnt int
		if err := rows.Scan(&hostID, &cnt); err != nil {
			return nil, err
		}
		m[hostID] = cnt
	}
	return m, rows.Err()
}

func scanInt64s(rows *sql.Rows) ([]int64, error) {
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
