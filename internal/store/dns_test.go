package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func newDNSRepo(t *testing.T) (*store.DNSRepo, *database.DB) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.NewDNSRepo(d.SQL), d
}

// ListFiltered / CountFiltered: server-side search, situacao, tag, responsavel,
// has_https filters plus sort + window. dns_records has no deleted_at column, so
// (unlike projects) there is no soft-delete predicate. Mirrors the project
// filter test shape.
func TestDNSRepo_ListFiltered(t *testing.T) {
	ctx := context.Background()
	repo, d := newDNSRepo(t)

	// Seed 4 dns_records with distinct domain / situacao / responsavel and a mix
	// of has_https. RETURNING id to capture the generated ids for tagging.
	type seedRow struct {
		domain      string
		situacao    string
		responsavel string
		hasHTTPS    bool
	}
	seeds := []seedRow{
		{"alpha.com", "active", "alice", true},
		{"beta.com", "archived", "bob", false},
		{"gamma-alpha.net", "active", "alice", true},
		{"delta.org", "active", "carol", false},
	}
	ids := make([]int64, len(seeds))
	for i, s := range seeds {
		if err := d.SQL.QueryRow(
			`INSERT INTO dns_records (domain, situacao, responsavel, has_https) VALUES (?, ?, ?, ?) RETURNING id`,
			s.domain, s.situacao, s.responsavel, s.hasHTTPS,
		).Scan(&ids[i]); err != nil {
			t.Fatalf("seed %s: %v", s.domain, err)
		}
	}
	// Tag exactly one record (beta.com) with 'x'.
	if _, err := d.SQL.Exec(`INSERT INTO tags (entity_type, entity_id, tag) VALUES ('dns', ?, 'x')`, ids[1]); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	domains := func(rs []models.DNSRecord) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Domain
		}
		return out
	}

	// Search matches domain OR responsavel (case-insensitive ILIKE). "alpha"
	// hits alpha.com (domain) and gamma-alpha.net (domain). Sort domain asc.
	got, err := repo.ListFiltered(ctx, models.DNSFilter{Search: "alpha", SortBy: "domain"})
	if err != nil {
		t.Fatalf("ListFiltered(search): %v", err)
	}
	if g := domains(got); len(g) != 2 || g[0] != "alpha.com" || g[1] != "gamma-alpha.net" {
		t.Fatalf("search 'alpha' = %v, want [alpha.com gamma-alpha.net]", g)
	}
	if n, err := repo.CountFiltered(ctx, models.DNSFilter{Search: "alpha"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(search) = %d, %v; want 2", n, err)
	}

	// Situacao filter.
	if n, err := repo.CountFiltered(ctx, models.DNSFilter{Situacao: "active"}); err != nil || n != 3 {
		t.Fatalf("CountFiltered(situacao=active) = %d, %v; want 3", n, err)
	}

	// Tag filter returns only the tagged record.
	got, err = repo.ListFiltered(ctx, models.DNSFilter{Tag: "x"})
	if err != nil {
		t.Fatalf("ListFiltered(tag): %v", err)
	}
	if g := domains(got); len(g) != 1 || g[0] != "beta.com" {
		t.Fatalf("tag 'x' = %v, want [beta.com]", g)
	}

	// Responsavel filter (exact match) — alice owns two records.
	got, err = repo.ListFiltered(ctx, models.DNSFilter{Responsavel: "alice", SortBy: "domain"})
	if err != nil {
		t.Fatalf("ListFiltered(responsavel): %v", err)
	}
	if g := domains(got); len(g) != 2 || g[0] != "alpha.com" || g[1] != "gamma-alpha.net" {
		t.Fatalf("responsavel 'alice' = %v, want [alpha.com gamma-alpha.net]", g)
	}

	// HasHTTPS tri-state: "yes" → has_https = true (alpha.com, gamma-alpha.net).
	got, err = repo.ListFiltered(ctx, models.DNSFilter{HasHTTPS: "yes", SortBy: "domain"})
	if err != nil {
		t.Fatalf("ListFiltered(has_https=yes): %v", err)
	}
	if g := domains(got); len(g) != 2 || g[0] != "alpha.com" || g[1] != "gamma-alpha.net" {
		t.Fatalf("has_https=yes = %v, want [alpha.com gamma-alpha.net]", g)
	}
	// "no" → has_https = false (beta.com, delta.org).
	if n, err := repo.CountFiltered(ctx, models.DNSFilter{HasHTTPS: "no"}); err != nil || n != 2 {
		t.Fatalf("CountFiltered(has_https=no) = %d, %v; want 2", n, err)
	}

	// Sort by domain desc.
	got, err = repo.ListFiltered(ctx, models.DNSFilter{SortBy: "domain", SortDir: "desc"})
	if err != nil {
		t.Fatalf("ListFiltered(sort desc): %v", err)
	}
	if g := domains(got); len(g) != 4 || g[0] != "gamma-alpha.net" || g[3] != "alpha.com" {
		t.Fatalf("sort domain desc = %v, want [gamma-alpha.net delta.org beta.com alpha.com]", g)
	}

	// Pagination: domain asc → [alpha.com beta.com delta.org gamma-alpha.net];
	// PerPage=2 Page=2 → [delta.org gamma-alpha.net].
	got, err = repo.ListFiltered(ctx, models.DNSFilter{SortBy: "domain", PerPage: 2, Page: 2})
	if err != nil {
		t.Fatalf("ListFiltered(page2): %v", err)
	}
	if g := domains(got); len(g) != 2 || g[0] != "delta.org" || g[1] != "gamma-alpha.net" {
		t.Fatalf("page2 = %v, want [delta.org gamma-alpha.net]", g)
	}
	// CountFiltered ignores the window: full match count.
	if n, err := repo.CountFiltered(ctx, models.DNSFilter{SortBy: "domain", PerPage: 2, Page: 2}); err != nil || n != 4 {
		t.Fatalf("CountFiltered(page2) = %d, %v; want 4", n, err)
	}
}

// Cert buckets and the expiry sort. Expiries sit an hour either side of each
// bucket edge (NOW() at query time is later than at seed time, so an exact
// edge would be racy).
func TestDNSRepo_CertFilter(t *testing.T) {
	ctx := context.Background()
	repo, d := newDNSRepo(t)

	seeds := []struct {
		domain, expires, checked, certErr string // expires/checked are SQL expressions
		hasHTTPS                          bool
	}{
		{"a-expired.com", "NOW() - INTERVAL '1 hour'", "NOW()", "x509: certificate has expired", true},
		{"b-soon.com", "NOW() + INTERVAL '1 hour'", "NOW()", "", true},
		{"c-week-edge.com", "NOW() + INTERVAL '7 days' - INTERVAL '1 hour'", "NOW()", "", true},
		{"d-past-week.com", "NOW() + INTERVAL '7 days' + INTERVAL '1 hour'", "NOW()", "", true},
		{"e-month-edge.com", "NOW() + INTERVAL '30 days' - INTERVAL '1 hour'", "NOW()", "", true},
		{"f-far.com", "NOW() + INTERVAL '30 days' + INTERVAL '1 hour'", "NOW()", "", true},
		{"g-unreachable.com", "NULL", "NOW()", "dial tcp: i/o timeout", true},
		{"h-unscanned.com", "NULL", "NULL", "", true},
		{"i-http-only.com", "NULL", "NULL", "", false},
	}
	for _, s := range seeds {
		if _, err := d.SQL.Exec(
			`INSERT INTO dns_records (domain, has_https, cert_error, cert_expires_at, cert_checked_at) VALUES (?, ?, ?, `+s.expires+`, `+s.checked+`)`,
			s.domain, s.hasHTTPS, s.certErr,
		); err != nil {
			t.Fatalf("seed %s: %v", s.domain, err)
		}
	}

	domains := func(rs []models.DNSRecord) string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Domain
		}
		return strings.Join(out, " ")
	}

	for cert, want := range map[string]string{
		"expired":   "a-expired.com",
		"7":         "b-soon.com c-week-edge.com",
		"30":        "b-soon.com c-week-edge.com d-past-week.com e-month-edge.com",
		"error":     "a-expired.com g-unreachable.com",
		"unscanned": "h-unscanned.com",
		"":          "a-expired.com b-soon.com c-week-edge.com d-past-week.com e-month-edge.com f-far.com g-unreachable.com h-unscanned.com i-http-only.com",
	} {
		got, err := repo.ListFiltered(ctx, models.DNSFilter{Cert: cert, SortBy: "domain"})
		if err != nil {
			t.Fatalf("ListFiltered(cert=%q): %v", cert, err)
		}
		if g := domains(got); g != want {
			t.Errorf("cert=%q = [%s], want [%s]", cert, g, want)
		}
		if n, err := repo.CountFiltered(ctx, models.DNSFilter{Cert: cert}); err != nil || n != len(got) {
			t.Errorf("CountFiltered(cert=%q) = %d, %v; want %d", cert, n, err, len(got))
		}
	}

	// The cert columns round-trip through scanDNS.
	got, err := repo.ListFiltered(ctx, models.DNSFilter{Cert: "expired"})
	if err != nil || len(got) != 1 {
		t.Fatalf("ListFiltered(expired) = %v, %v", got, err)
	}
	if c := got[0].DNSCert; c.CertExpiresAt == nil || c.CertCheckedAt == nil || c.CertError != "x509: certificate has expired" {
		t.Fatalf("scanned cert = %+v", c)
	}

	// Expiry sort: NULLs last in both directions.
	for dir, want := range map[string]string{
		"asc":  "a-expired.com b-soon.com c-week-edge.com d-past-week.com e-month-edge.com f-far.com",
		"desc": "f-far.com e-month-edge.com d-past-week.com c-week-edge.com b-soon.com a-expired.com",
	} {
		got, err := repo.ListFiltered(ctx, models.DNSFilter{SortBy: "cert_expires_at", SortDir: dir})
		if err != nil || len(got) != len(seeds) {
			t.Fatalf("ListFiltered(sort %s) = %d rows, %v", dir, len(got), err)
		}
		if g := domains(got[:6]); g != want {
			t.Errorf("sort cert_expires_at %s = [%s], want [%s] first", dir, g, want)
		}
		for _, r := range got[6:] {
			if r.CertExpiresAt != nil {
				t.Errorf("sort %s: %s (non-NULL expiry) after the NULLs", dir, r.Domain)
			}
		}
	}
}
