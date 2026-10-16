package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const hitJSON = `{"_source":{"agent":{"id":"001","name":"web-01"},"package":{"name":"openssl","version":"3.0.2"},
	"vulnerability":{"id":"CVE-2024-0001","severity":"High","description":"d","reference":"https://x.test","score":{"base":7.5}}}}`

// fakeIndexer reports `total` hits but serves `served`: 2 on the first
// page, the rest on the first scroll, none after. It records the requests.
func fakeIndexer(t *testing.T, total, served int) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	scrolls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if u, p, ok := r.BasicAuth(); !ok || u != "reader" || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		hits := func(n int) string { return strings.TrimSuffix(strings.Repeat(hitJSON+",", n), ",") }
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/_count"):
			_, _ = w.Write([]byte(`{"count":3}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/_search"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["track_total_hits"] != true {
				t.Errorf("track_total_hits not set: %v", body)
			}
			_, _ = w.Write([]byte(`{"_scroll_id":"s1","hits":{"total":{"value":` + itoa(total) + `},"hits":[` + hits(2) + `]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/_search/scroll":
			n := served - 2
			if scrolls++; scrolls > 1 {
				n = 0 // the indexer ran dry
			}
			_, _ = w.Write([]byte(`{"_scroll_id":"s1","hits":{"total":{"value":` + itoa(total) + `},"hits":[` + hits(n) + `]}}`))
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestVulnerabilities_ScrollsAllPages(t *testing.T) {
	srv, seen := fakeIndexer(t, 3, 3)
	hits, err := NewClient(srv.URL, "reader", "pw", false).Vulnerabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 || hits[0].Agent.Name != "web-01" || hits[0].Vulnerability.ID != "CVE-2024-0001" ||
		hits[0].Package.Version != "3.0.2" || hits[0].Vulnerability.Score.Base == nil || *hits[0].Vulnerability.Score.Base != 7.5 {
		t.Fatalf("hits = %+v", hits)
	}
	if last := (*seen)[len(*seen)-1]; last != "DELETE /_search/scroll" {
		t.Fatalf("scroll not released: %v", *seen)
	}
}

// A short read (the indexer reports more than it pages out) must fail:
// the sync would otherwise auto-resolve what it never saw.
func TestVulnerabilities_ShortReadFails(t *testing.T) {
	srv, _ := fakeIndexer(t, 5, 3)
	if _, err := NewClient(srv.URL, "reader", "pw", false).Vulnerabilities(context.Background()); err == nil {
		t.Fatal("short read accepted")
	}
}

func TestCountAndAuth(t *testing.T) {
	srv, _ := fakeIndexer(t, 3, 3)
	if n, err := NewClient(srv.URL, "reader", "pw", false).Count(context.Background()); err != nil || n != 3 {
		t.Fatalf("count = %d, %v", n, err)
	}
	if _, err := NewClient(srv.URL, "reader", "wrong", false).Count(context.Background()); err == nil {
		t.Fatal("bad credentials accepted")
	}
}
