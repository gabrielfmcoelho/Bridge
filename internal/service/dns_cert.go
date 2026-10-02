package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// certScanConcurrency bounds the parallel TLS probes of a bulk scan.
const certScanConcurrency = 10

// certTarget turns a DNS record's domain into the TLS server name and the
// address to dial: an explicit host:port is honoured, anything else gets :443.
func certTarget(domain string) (host, addr string) {
	domain = strings.TrimSpace(domain)
	if h, _, err := net.SplitHostPort(domain); err == nil {
		return h, domain
	}
	return domain, net.JoinHostPort(domain, "443")
}

// probeCert reads the TLS certificate the domain serves and verifies it by
// hand. It never fails: a dial/handshake error lands in CertError with no
// dates (unreachable); a cert that was read but does not verify keeps its
// dates and gets the verification error.
func probeCert(ctx context.Context, domain string, now time.Time) models.DNSCert {
	c := models.DNSCert{CertCheckedAt: &now}
	host, addr := certTarget(domain)

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	d := tls.Dialer{Config: &tls.Config{
		ServerName: host,
		// Verification is done by hand below, so a self-signed or expired cert
		// is still recorded (with its error) instead of aborting the handshake.
		InsecureSkipVerify: true,
		// Go's client default minimum is TLS 1.2; legacy servers would hide
		// their cert behind a handshake failure. We only read a date and send
		// nothing secret, so the old versions are fine here.
		// ponytail: servers offering only RSA key exchange still fail the
		// handshake; add tls.InsecureCipherSuites() to CipherSuites if seen.
		MinVersion: tls.VersionTLS10,
	}}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		c.CertError = err.Error()
		return c
	}
	defer conn.Close()

	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	leaf := certs[0] // the client handshake rejects an empty chain
	c.CertNotBefore = &leaf.NotBefore
	c.CertExpiresAt = &leaf.NotAfter
	c.CertIssuer, c.CertSubject = leaf.Issuer.CommonName, leaf.Subject.CommonName
	if c.CertIssuer == "" { // no CN: fall back to the full DN
		c.CertIssuer = leaf.Issuer.String()
	}
	if c.CertSubject == "" {
		c.CertSubject = leaf.Subject.String()
	}
	c.CertSANs = strings.Join(leaf.DNSNames, ", ")

	inter := x509.NewCertPool()
	for _, ic := range certs[1:] {
		inter.AddCert(ic)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: host, Intermediates: inter, CurrentTime: now}); err != nil {
		c.CertError = err.Error()
	}
	return c
}

// ScanCert probes one record's certificate (regardless of has_https) and its
// DNS/HTTP observation, and stores both. Returns (nil, nil) when the record is absent or invisible.
func (s *DNSService) ScanCert(ctx context.Context, id int64) (*models.DNSRecord, error) {
	rec, err := s.dns.Get(ctx, id)
	if err != nil || rec == nil {
		return nil, err
	}
	now := time.Now()
	rec.DNSCert = probeCert(ctx, rec.Domain, now)
	rec.DNSObservation = probeObservation(ctx, rec.Domain, now)
	if err := s.dns.SetCert(ctx, id, rec.DNSCert); err != nil {
		return nil, err
	}
	if err := s.dns.SetObservation(ctx, id, rec.DNSObservation); err != nil {
		return nil, err
	}
	return rec, nil
}

// CertScanSummary is the bulk scan outcome. Scanned/OK/Failed count the cert
// probes (has_https records): Failed = left a cert_error (unreachable or not
// verifying), OK = Scanned - Failed. Observed counts every record probed for
// DNS/HTTP, Online those whose obs_status came out "online".
type CertScanSummary struct {
	Scanned  int `json:"scanned"`
	OK       int `json:"ok"`
	Failed   int `json:"failed"`
	Observed int `json:"observed"`
	Online   int `json:"online"`
}

// ScanCerts observes every visible record (DNS/HTTP) and probes the
// certificate of the has_https ones, certScanConcurrency records at a time,
// storing each result. Store errors don't stop the scan; they are joined and
// returned with the summary once every probe is done.
func (s *DNSService) ScanCerts(ctx context.Context) (CertScanSummary, error) {
	records, err := s.dns.ListFiltered(ctx, models.DNSFilter{})
	if err != nil {
		return CertScanSummary{}, err
	}
	var (
		sum  CertScanSummary
		errs []error
		mu   sync.Mutex
		wg   sync.WaitGroup
		sem  = make(chan struct{}, certScanConcurrency)
		now  = time.Now()
	)
	for _, rec := range records {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			o := probeObservation(ctx, rec.Domain, now)
			oErr := s.dns.SetObservation(ctx, rec.ID, o)
			var c models.DNSCert
			var cErr error
			if rec.HasHTTPS {
				c = probeCert(ctx, rec.Domain, now)
				cErr = s.dns.SetCert(ctx, rec.ID, c)
			}
			mu.Lock()
			defer mu.Unlock()
			sum.Observed++
			if o.ObsStatus == "online" {
				sum.Online++
			}
			if rec.HasHTTPS {
				sum.Scanned++
				if c.CertError != "" {
					sum.Failed++
				} else {
					sum.OK++
				}
			}
			if err := errors.Join(oErr, cErr); err != nil {
				errs = append(errs, err)
			}
		})
	}
	wg.Wait()
	return sum, errors.Join(errs...)
}
