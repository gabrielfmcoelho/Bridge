package store

import (
	"context"
	"database/sql"

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

// Get returns a DNS record by id, or (nil, nil) if absent.
func (r *DNSRepo) Get(ctx context.Context, id int64) (*models.DNSRecord, error) {
	d := &models.DNSRecord{}
	err := scanDNS(r.db.QueryRowContext(ctx, `SELECT `+dnsCols+` FROM dns_records WHERE id = ?`, id), d)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// List returns all DNS records ordered by domain.
func (r *DNSRepo) List(ctx context.Context) ([]models.DNSRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+dnsCols+` FROM dns_records ORDER BY domain`)
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

// Update writes the mutable fields of a DNS record by id.
func (r *DNSRepo) Update(ctx context.Context, d *models.DNSRecord) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE dns_records SET domain = ?, has_https = ?, situacao = ?, responsavel = ?, observacoes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		d.Domain, d.HasHTTPS, d.Situacao, d.Responsavel, d.Observacoes, d.ID,
	)
	return err
}

// Delete removes a DNS record by id.
func (r *DNSRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM dns_records WHERE id = ?`, id)
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
	rows, err := r.db.QueryContext(ctx,
		`SELECT d.id, d.domain, d.has_https, d.situacao, d.responsavel, d.observacoes, d.created_at, d.updated_at
		 FROM dns_records d JOIN dns_host_links l ON d.id = l.dns_id
		 WHERE l.host_id = ? ORDER BY d.domain`, hostID)
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

// Count returns the total number of DNS records.
func (r *DNSRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dns_records`).Scan(&n)
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
