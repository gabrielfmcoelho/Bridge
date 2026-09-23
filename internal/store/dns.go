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

const dnsCols = `id, domain, has_https, situacao, responsavel, observacoes, created_at, updated_at,
	cert_not_before, cert_expires_at, cert_issuer, cert_subject, cert_sans, cert_error, cert_checked_at`

func scanDNS(scanner interface{ Scan(...any) error }, d *models.DNSRecord) error {
	return scanner.Scan(&d.ID, &d.Domain, &d.HasHTTPS, &d.Situacao, &d.Responsavel, &d.Observacoes, &d.CreatedAt, &d.UpdatedAt,
		&d.CertNotBefore, &d.CertExpiresAt, &d.CertIssuer, &d.CertSubject, &d.CertSANs, &d.CertError, &d.CertCheckedAt)
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
	// Cert buckets: literal SQL only. "30" includes the "7" rows; both exclude
	// already-expired certs. The frontend's matchesCertFilter mirrors these.
	switch f.Cert {
	case "expired":
		where = append(where, "cert_expires_at < NOW()")
	case "7":
		where = append(where, "cert_expires_at >= NOW() AND cert_expires_at < NOW() + INTERVAL '7 days'")
	case "30":
		where = append(where, "cert_expires_at >= NOW() AND cert_expires_at < NOW() + INTERVAL '30 days'")
	case "error":
		where = append(where, "cert_error <> ''")
	case "unscanned":
		where = append(where, "has_https = true AND cert_checked_at IS NULL")
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
		"domain":          "domain",
		"situacao":        "situacao",
		"responsavel":     "responsavel",
		"cert_expires_at": "cert_expires_at",
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
	if sortCol == "cert_expires_at" { // never-scanned rows sink in both directions
		query += " NULLS LAST"
	}

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

// SetCert stores the latest cert scan result for a DNS record. Invisible rows
// are untouched. updated_at is left alone: it tracks edits, not scans.
func (r *DNSRepo) SetCert(ctx context.Context, id int64, c models.DNSCert) error {
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	_, err := r.db.ExecContext(ctx,
		`UPDATE dns_records SET cert_not_before = ?, cert_expires_at = ?, cert_issuer = ?, cert_subject = ?, cert_sans = ?, cert_error = ?, cert_checked_at = ? WHERE id = ? AND `+vis,
		append([]any{c.CertNotBefore, c.CertExpiresAt, c.CertIssuer, c.CertSubject, c.CertSANs, c.CertError, c.CertCheckedAt, id}, vargs...)...,
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

// AddHostLink links a host to a DNS record without touching its other links.
// Reports whether a new row was added.
func (r *DNSRepo) AddHostLink(ctx context.Context, dnsID, hostID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, dnsID, hostID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// IDByDomain finds a record by domain, case-insensitively and unscoped (the
// Coolify sync must see every row to avoid duplicates).
func (r *DNSRepo) IDByDomain(ctx context.Context, domain string) (int64, bool, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM dns_records WHERE lower(domain) = lower(?) ORDER BY id LIMIT 1`, domain).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return id, err == nil, err
}

// SetServiceLinks replaces all service links for a DNS record (one tx).
func (r *DNSRepo) SetServiceLinks(ctx context.Context, dnsID int64, serviceIDs []int64) error {
	return replaceLinks(ctx, r.db, `service_dns_links`, `dns_id`, `service_id`, dnsID, serviceIDs)
}

// ServiceIDs returns all service ids linked to a DNS record.
func (r *DNSRepo) ServiceIDs(ctx context.Context, dnsID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT service_id FROM service_dns_links WHERE dns_id = ? ORDER BY service_id`, dnsID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// SetProjectLinks replaces all direct project links for a DNS record (one tx).
func (r *DNSRepo) SetProjectLinks(ctx context.Context, dnsID int64, projectIDs []int64) error {
	return replaceLinks(ctx, r.db, `project_dns_links`, `dns_id`, `project_id`, dnsID, projectIDs)
}

// ProjectIDs returns all project ids directly linked to a DNS record.
func (r *DNSRepo) ProjectIDs(ctx context.Context, dnsID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT project_id FROM project_dns_links WHERE dns_id = ? ORDER BY project_id`, dnsID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
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
	vis, vargs := VisibleExpr(ctx, AssetDNS, "dns_records.id")
	// dns_host_links has only (dns_id, host_id), so dnsCols stays unambiguous.
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+dnsCols+` FROM dns_records JOIN dns_host_links l ON l.dns_id = dns_records.id
		 WHERE l.host_id = ? AND `+vis+` ORDER BY domain`, append([]any{hostID}, vargs...)...)
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
