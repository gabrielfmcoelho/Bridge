package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type DNSRecord struct {
	ID          int64     `json:"id"`
	Domain      string    `json:"domain"`
	HasHTTPS    bool      `json:"has_https"`
	Situacao    string    `json:"situacao"`
	Responsavel string    `json:"responsavel"`
	Observacoes string    `json:"observacoes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func CreateDNSRecord(db *sql.DB, d *DNSRecord) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO dns_records (domain, has_https, situacao, responsavel, observacoes) VALUES (?, ?, ?, ?, ?)`,
		d.Domain, d.HasHTTPS, d.Situacao, d.Responsavel, d.Observacoes,
	)
	if err != nil {
		return err
	}
	d.ID = id
	return nil
}

func GetDNSRecord(db *sql.DB, id int64) (*DNSRecord, error) {
	d := &DNSRecord{}
	err := db.QueryRow(
		`SELECT id, domain, has_https, situacao, responsavel, observacoes, created_at, updated_at FROM dns_records WHERE id = ?`, id,
	).Scan(&d.ID, &d.Domain, &d.HasHTTPS, &d.Situacao, &d.Responsavel, &d.Observacoes, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func ListDNSRecords(db *sql.DB) ([]DNSRecord, error) {
	rows, err := db.Query(`SELECT id, domain, has_https, situacao, responsavel, observacoes, created_at, updated_at FROM dns_records ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DNSRecord
	for rows.Next() {
		var d DNSRecord
		if err := rows.Scan(&d.ID, &d.Domain, &d.HasHTTPS, &d.Situacao, &d.Responsavel, &d.Observacoes, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, d)
	}
	return records, rows.Err()
}

func UpdateDNSRecord(db *sql.DB, d *DNSRecord) error {
	_, err := db.Exec(
		`UPDATE dns_records SET domain = ?, has_https = ?, situacao = ?, responsavel = ?, observacoes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		d.Domain, d.HasHTTPS, d.Situacao, d.Responsavel, d.Observacoes, d.ID,
	)
	return err
}

func DeleteDNSRecord(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM dns_records WHERE id = ?`, id)
	return err
}

// LinkDNSToHost creates a link between a DNS record and a host.
func LinkDNSToHost(db *sql.DB, dnsID, hostID int64) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dnsID, hostID)
	return err
}

// UnlinkDNSFromHost removes a link between a DNS record and a host.
func UnlinkDNSFromHost(db *sql.DB, dnsID, hostID int64) error {
	_, err := db.Exec(`DELETE FROM dns_host_links WHERE dns_id = ? AND host_id = ?`, dnsID, hostID)
	return err
}

// SetDNSHostLinks replaces all host links for a DNS record.
func SetDNSHostLinks(db *sql.DB, dnsID int64, hostIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM dns_host_links WHERE dns_id = ?`, dnsID); err != nil {
		return err
	}
	for _, hid := range hostIDs {
		if _, err := tx.Exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dnsID, hid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetDNSHostIDs returns all host IDs linked to a DNS record.
func GetDNSHostIDs(db *sql.DB, dnsID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT host_id FROM dns_host_links WHERE dns_id = ?`, dnsID)
	if err != nil {
		return nil, err
	}
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

// GetHostDNSRecords returns all DNS records linked to a host.
func GetHostDNSRecords(db *sql.DB, hostID int64) ([]DNSRecord, error) {
	rows, err := db.Query(
		`SELECT d.id, d.domain, d.has_https, d.situacao, d.responsavel, d.observacoes, d.created_at, d.updated_at
		FROM dns_records d
		JOIN dns_host_links l ON d.id = l.dns_id
		WHERE l.host_id = ?
		ORDER BY d.domain`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DNSRecord
	for rows.Next() {
		var d DNSRecord
		if err := rows.Scan(&d.ID, &d.Domain, &d.HasHTTPS, &d.Situacao, &d.Responsavel, &d.Observacoes, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, d)
	}
	return records, rows.Err()
}

// SetHostDNSLinks replaces all DNS links for a host.
func SetHostDNSLinks(db *sql.DB, hostID int64, dnsIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM dns_host_links WHERE host_id = ?`, hostID); err != nil {
		return err
	}
	for _, did := range dnsIDs {
		if _, err := tx.Exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, did, hostID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func DNSRecordCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM dns_records`).Scan(&n)
	return n, err
}

// GetDNSCountsByHost returns a map of host_id → number of linked DNS records.
func GetDNSCountsByHost(db *sql.DB) (map[int64]int, error) {
	rows, err := db.Query(`SELECT host_id, COUNT(*) FROM dns_host_links GROUP BY host_id`)
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
