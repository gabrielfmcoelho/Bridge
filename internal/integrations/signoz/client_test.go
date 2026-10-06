package signoz

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeCH answers ClickHouse's HTTP interface: it records each query and its
// parameters and returns one canned row shaped for summary, series or top.
func fakeCH(t *testing.T) (*httptest.Server, *[]string, *[]map[string]string) {
	var queries []string
	var params []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-ClickHouse-User") != "bridge_ro" || r.Header.Get("X-ClickHouse-Key") != "pw" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		b, _ := io.ReadAll(r.Body)
		q := string(b)
		queries = append(queries, q)
		p := map[string]string{}
		for k, v := range r.URL.Query() {
			p[k] = v[0]
		}
		params = append(params, p)
		const stats = `"count":10,"errors":1,"p50":12.5,"p95":80,"p99":120.25`
		switch {
		case strings.Contains(q, "GROUP BY t"):
			io.WriteString(w, `{"data":[{"t":1700000000,`+stats+`}]}`)
		case strings.Contains(q, "GROUP BY key"):
			io.WriteString(w, `{"data":[{"key":"servidores-painel",`+stats+`}]}`)
		default:
			io.WriteString(w, `{"data":[{`+stats+`}]}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &queries, &params
}

func TestRequestsGateway(t *testing.T) {
	srv, queries, params := fakeCH(t)
	c := NewClient(srv.URL, "bridge_ro", "pw")
	c.now = func() time.Time { return time.Unix(1_700_086_400, 0) }

	res, err := c.Requests(context.Background(), Filter{PathPrefix: "/datalakehouse/servidores"}, "24h", "key")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Count != 10 || res.Summary.P99 != 120.25 || len(res.Series) != 1 || res.Top[0].Key != "servidores-painel" {
		t.Fatalf("result = %+v", res)
	}
	if !res.Series[0].T.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("bucket time = %v", res.Series[0].T)
	}
	if len(*queries) != 3 {
		t.Fatalf("%d queries, want 3", len(*queries))
	}
	top := (*queries)[2]
	for _, want := range []string{"'apisix-gateway'", "startsWith(", "parent_span_id = ''", "bridge.client", "FORMAT JSON"} {
		if !strings.Contains(top, want) {
			t.Errorf("top query lacks %q:\n%s", want, top)
		}
	}
	p := (*params)[0]
	if p["param_prefix"] != "/datalakehouse/servidores" || p["param_from"] != "1700000000" || p["param_step"] != "3600" {
		t.Errorf("params = %v", p)
	}
	// The prefix is bound, never spliced into the SQL.
	if strings.Contains(top, "/datalakehouse") {
		t.Errorf("prefix spliced into SQL")
	}
}

func TestRequestsTraefik(t *testing.T) {
	srv, queries, params := fakeCH(t)
	c := NewClient(srv.URL, "bridge_ro", "pw")
	if _, err := c.Requests(context.Background(), Filter{RouterUUIDs: []string{"abc123", "o'q"}}, "1h", "service"); err != nil {
		t.Fatal(err)
	}
	q := (*queries)[2]
	if !strings.Contains(q, "'traefik'") || !strings.Contains(q, "multiSearchAny(") || strings.Contains(q, "parent_span_id") {
		t.Errorf("traefik query:\n%s", q)
	}
	if got := (*params)[0]["param_uuids"]; got != `['abc123','o\'q']` {
		t.Errorf("uuids param = %s", got)
	}
}

func TestRequestsClients(t *testing.T) {
	srv, queries, params := fakeCH(t)
	c := NewClient(srv.URL, "bridge_ro", "pw")
	if _, err := c.Requests(context.Background(), Filter{PathPrefix: "/sei", Clients: []string{"sei-painel"}}, "7d", "route"); err != nil {
		t.Fatal(err)
	}
	if q := (*queries)[0]; !strings.Contains(q, "has({clients:Array(String)}, attributes_string['bridge.client'])") {
		t.Errorf("summary query does not filter clients:\n%s", q)
	}
	if got := (*params)[0]["param_clients"]; got != "['sei-painel']" {
		t.Errorf("clients param = %s", got)
	}
}

func TestRequestsRejects(t *testing.T) {
	c := NewClient("http://unused", "", "")
	for _, tc := range []struct {
		f        Filter
		rng, grp string
	}{
		{Filter{PathPrefix: "/x"}, "2h", "route"},
		{Filter{PathPrefix: "/x"}, "1h", "user"},
		{Filter{}, "1h", "route"},
		{Filter{PathPrefix: "/x", RouterUUIDs: []string{"a"}}, "1h", "route"},
		{Filter{RouterUUIDs: []string{"a"}, Clients: []string{"c"}}, "1h", "route"},
	} {
		if _, err := c.Requests(context.Background(), tc.f, tc.rng, tc.grp); err == nil {
			t.Errorf("%+v accepted", tc)
		}
	}
}

func TestRequestsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Code: 60. Table does not exist", http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, "", "").Requests(context.Background(), Filter{PathPrefix: "/x"}, "1h", "route")
	if err == nil || !strings.Contains(err.Error(), "Table does not exist") {
		t.Fatalf("err = %v", err)
	}
}
