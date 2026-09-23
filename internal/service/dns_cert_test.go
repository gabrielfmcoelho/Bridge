package service_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// A live self-signed TLS server: the cert is read (dates, names) and the
// verification error recorded; a dead port is unreachable (error, no dates);
// a has_https=false record is left alone by the bulk scan but still scanned
// on demand.
func TestDNSService_ScanCerts(t *testing.T) {
	ctx := context.Background()
	svc, d := newDNSService(t)

	srv := httptest.NewTLSServer(nil)
	defer srv.Close()
	live := strings.TrimPrefix(srv.URL, "https://") // 127.0.0.1:<port>
	_, port, _ := strings.Cut(live, ":")

	seed := func(domain string, https bool) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain, has_https) VALUES (?, ?) RETURNING id`, domain, https).Scan(&id); err != nil {
			t.Fatalf("seed %s: %v", domain, err)
		}
		return id
	}
	liveID := seed(live, true)
	deadID := seed("127.0.0.1:1", true)
	httpID := seed("localhost:"+port, false) // same server, but has_https=false
	reload := func(id int64) models.DNSRecord {
		detail, err := svc.Get(ctx, id)
		if err != nil || detail == nil {
			t.Fatalf("get %d = %v, %v", id, detail, err)
		}
		return *detail.Record
	}

	sum, err := svc.ScanCerts(ctx)
	if err != nil {
		t.Fatalf("ScanCerts: %v", err)
	}
	if sum != (service.CertScanSummary{Scanned: 2, OK: 0, Failed: 2}) {
		t.Fatalf("summary = %+v, want {2 0 2}", sum)
	}

	got := reload(liveID)
	want := srv.Certificate().NotAfter.Truncate(time.Second)
	if got.CertExpiresAt == nil || !got.CertExpiresAt.Truncate(time.Second).Equal(want) {
		t.Fatalf("live expires = %v, want %v", got.CertExpiresAt, want)
	}
	if !strings.Contains(got.CertError, "unknown authority") {
		t.Fatalf("live error = %q, want unknown authority", got.CertError)
	}
	if got.CertSubject == "" || got.CertIssuer == "" || got.CertNotBefore == nil || got.CertCheckedAt == nil {
		t.Fatalf("live cert = %+v", got.DNSCert)
	}

	if got := reload(deadID); got.CertError == "" || got.CertExpiresAt != nil || got.CertCheckedAt == nil {
		t.Fatalf("dead cert = %+v, want error, no expiry", got.DNSCert)
	}

	if got := reload(httpID); got.CertCheckedAt != nil {
		t.Fatalf("http-only record was scanned: %+v", got.DNSCert)
	}

	// On demand, has_https doesn't matter.
	rec, err := svc.ScanCert(ctx, httpID)
	if err != nil || rec == nil || rec.CertExpiresAt == nil || rec.CertCheckedAt == nil {
		t.Fatalf("ScanCert(http-only) = %+v, %v", rec, err)
	}
	if got := reload(httpID); got.CertExpiresAt == nil {
		t.Fatalf("ScanCert did not persist: %+v", got.DNSCert)
	}

	if rec, err := svc.ScanCert(ctx, 9999); rec != nil || err != nil {
		t.Fatalf("ScanCert(missing) = %+v, %v; want nil,nil", rec, err)
	}
}
