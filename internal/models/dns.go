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
	DNSCert
}

// DNSCert is the domain's TLS certificate as last read by the cert scan
// (embedded in DNSRecord, so the JSON stays flat). CertCheckedAt nil = never
// scanned. CertError with nil CertExpiresAt = unreachable (dial/handshake
// failed); CertError with CertExpiresAt set = the cert was read but did not
// verify (self-signed, expired, wrong name, ...). CertSANs is the DNS names
// joined with ", ".
type DNSCert struct {
	CertNotBefore *time.Time `json:"cert_not_before"`
	CertExpiresAt *time.Time `json:"cert_expires_at"`
	CertIssuer    string     `json:"cert_issuer"`
	CertSubject   string     `json:"cert_subject"`
	CertSANs      string     `json:"cert_sans"`
	CertError     string     `json:"cert_error"`
	CertCheckedAt *time.Time `json:"cert_checked_at"`
}

// DNSFilter is the value object describing list/count predicates, sort, and
// pagination for DNS records. Consumed by store.DNSRepo.ListFiltered /
// CountFiltered. HasHTTPS is tri-state: "" (any), "yes", "no". Cert is a
// certificate bucket: "" (any), "expired", "7" / "30" (expires within that
// many days, not yet expired), "error", "unscanned" (has_https, never scanned).
type DNSFilter struct {
	Search      string
	Situacao    string
	Tag         string
	Responsavel string
	HasHTTPS    string
	Cert        string
	SortBy      string
	SortDir     string
	Page        int
	PerPage     int
}
