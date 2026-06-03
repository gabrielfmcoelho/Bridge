package models

import "time"

// DNSRecord is a managed domain entry, optionally linked to hosts via
// dns_host_links. Persistence lives in internal/store.DNSRepo — this file is
// the pure data type only.
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
