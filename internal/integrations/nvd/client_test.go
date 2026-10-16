package nvd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const fixture = `{"totalResults":1,"vulnerabilities":[{"cve":{
 "id":"CVE-2021-44228","published":"2021-12-10T10:15:09.143",
 "descriptions":[{"lang":"es","value":"no"},{"lang":"en","value":"Apache Log4j2 JNDI features ..."}],
 "metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":10.0,"vectorString":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H"}}],
            "cvssMetricV2":[{"cvssData":{"baseScore":9.3,"vectorString":"AV:N/AC:M/Au:N/C:C/I:C/A:C"}}]},
 "weaknesses":[{"description":[{"value":"NVD-CWE-noinfo"}]},{"description":[{"value":"CWE-502"},{"value":"CWE-20"}]}],
 "references":[{"url":"https://logging.apache.org/log4j/2.x/security.html"},{"url":"https://example.org/b"}]}}]}`

func fake(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cveId") == "" {
			t.Errorf("missing cveId query: %s", r.URL)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, HTTP: srv.Client()}
}

func TestCVEParses(t *testing.T) {
	c, err := fake(t, 200, fixture).CVE(context.Background(), "CVE-2021-44228")
	if err != nil {
		t.Fatal(err)
	}
	if c.Description != "Apache Log4j2 JNDI features ..." {
		t.Errorf("description = %q", c.Description)
	}
	if c.CVSS == nil || *c.CVSS != 10.0 || c.CVSSVector[:8] != "CVSS:3.1" {
		t.Errorf("cvss = %v %q, want v3.1 10.0 over v2", c.CVSS, c.CVSSVector)
	}
	if c.CWE != "CWE-502" {
		t.Errorf("cwe = %q, want first real CWE", c.CWE)
	}
	if len(c.References) != 2 || c.Published == nil || c.Published.Year() != 2021 {
		t.Errorf("refs = %v published = %v", c.References, c.Published)
	}
}

func TestCVENotFound(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{200, `{"totalResults":0,"vulnerabilities":[]}`}, {404, ``}} {
		if _, err := fake(t, tc.status, tc.body).CVE(context.Background(), "CVE-2099-0001"); !errors.Is(err, ErrNotFound) {
			t.Errorf("status %d: err = %v, want ErrNotFound", tc.status, err)
		}
	}
	if _, err := fake(t, 503, "down").CVE(context.Background(), "CVE-2099-0001"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("503: err = %v, want a non-NotFound error", err)
	}
}
