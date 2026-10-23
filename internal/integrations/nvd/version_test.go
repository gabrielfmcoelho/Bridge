package nvd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestVersionCVEs_PagesAndSendsKey(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("virtualMatchString"); got != "cpe:2.3:a:php:php:7.4.33" {
			t.Errorf("virtualMatchString = %q", got)
		}
		if r.Header.Get("apiKey") != "k" {
			t.Errorf("apiKey header missing")
		}
		start, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
		// Two pages: the first returns 2 of 3 results.
		ids := []string{"CVE-2024-1", "CVE-2024-2"}
		if start > 0 {
			ids = []string{"CVE-2024-3"}
		}
		w.Write([]byte(`{"totalResults":3,"vulnerabilities":[`))
		for i, id := range ids {
			if i > 0 {
				w.Write([]byte(","))
			}
			fmt.Fprintf(w, `{"cve":{"id":%q,"descriptions":[{"lang":"en","value":"bug in %s"}],"metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":9.8}}]}}}`, id, id)
		}
		w.Write([]byte(`]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), APIKey: "k"}
	got, err := c.VersionCVEs(context.Background(), "php:php", "7.4.33")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || calls != 2 || got[2].ID != "CVE-2024-3" || got[0].CVSS != 9.8 || got[0].Summary != "bug in CVE-2024-1" {
		t.Fatalf("got %+v after %d calls", got, calls)
	}
}

func TestVersionCVEs_NoneIsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"totalResults":0,"vulnerabilities":[]}`))
	}))
	defer srv.Close()
	got, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).VersionCVEs(context.Background(), "redis:redis", "7.2.4")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}
