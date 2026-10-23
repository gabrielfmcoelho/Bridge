package endoflife

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCyclesAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openssl.json" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`[{"cycle":"3.0","eol":"2026-09-07"},{"cycle":"1.1.1","eol":true},{"cycle":"1.1.10","eol":false},{"cycle":18,"eol":"2025-04-30"}]`))
	}))
	defer srv.Close()
	cs, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).Cycles(context.Background(), "openssl")
	if err != nil || len(cs) != 4 || cs[1].EOL != "true" || cs[2].EOL != "" || cs[3].Cycle != "18" {
		t.Fatalf("cycles = %+v, %v", cs, err)
	}
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		version string
		eol     bool
		date    string
		ok      bool
	}{
		{"3.0.11", true, "2026-09-07", true},
		{"1.1.1w", true, "", true},
		{"1.1.10", false, "", true}, // its own cycle, not 1.1.1
		{"18.19.0", true, "2025-04-30", true},
		{"3.5.1", false, "", false},
	} {
		eol, date, ok := Status(cs, c.version, now)
		if eol != c.eol || date != c.date || ok != c.ok {
			t.Errorf("Status(%s) = %v %q %v, want %v %q %v", c.version, eol, date, ok, c.eol, c.date, c.ok)
		}
	}
	if _, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).Cycles(context.Background(), "nope"); err == nil {
		t.Fatal("404 not an error")
	}
}
