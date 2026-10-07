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

// now is fixed so windows and slots are predictable; 1699999200 is an hour boundary.
var testNow = time.Unix(1_700_086_400, 0)

// fakeCH answers ClickHouse's HTTP interface: it records each query and its
// parameters and returns canned rows shaped for the query it recognizes.
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
		case strings.Contains(q, "GROUP BY t, k"):
			io.WriteString(w, `{"data":[{"t":1699999200,"k":"servidores-painel",`+stats+`},{"t":1699999200,"k":"__outros__",`+stats+`}]}`)
		case strings.Contains(q, "GROUP BY t"):
			io.WriteString(w, `{"data":[{"t":1699999200,`+stats+`}]}`)
		case strings.Contains(q, "GROUP BY key"):
			io.WriteString(w, `{"data":[{"key":"servidores-painel",`+stats+`}]}`)
		case strings.Contains(q, "GROUP BY k "):
			io.WriteString(w, `{"data":[{"k":"servidores-painel"},{"k":"/api/x"}]}`)
		case strings.Contains(q, "count() AS n"):
			io.WriteString(w, `{"data":[{"n":2}]}`)
		case strings.Contains(q, "AS start_ns"):
			io.WriteString(w, `{"data":[
				{"span_id":"a","parent_id":"","service":"vp-front","name":"GET","kind":"Server","start_ns":1000000000,"duration_ms":30,"status":"200","error":false,"attrs":{"http.target":"/x?cpf=1"},"nums":{"http.status_code":200}},
				{"span_id":"b","parent_id":"a","service":"apisix-gateway","name":"GET /x/*","kind":"Server","start_ns":1005000000,"duration_ms":20,"status":"200","error":false,"attrs":{},"nums":{}}]}`)
		case strings.Contains(q, "ORDER BY timestamp DESC"):
			io.WriteString(w, `{"data":[{"ms":1700000000123,"trace_id":"t1","span_id":"s1","method":"GET","route":"/api/:id","path":"/api/7","status":"200","duration_ms":12.5,"client":"servidores-painel"}]}`)
		default:
			io.WriteString(w, `{"data":[{`+stats+`}]}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &queries, &params
}

func newTestClient(t *testing.T) (*Client, *[]string, *[]map[string]string) {
	srv, queries, params := fakeCH(t)
	c := NewClient(srv.URL, "bridge_ro", "pw")
	c.now = func() time.Time { return testNow }
	return c, queries, params
}

func find(qs []string, sub string) (string, int) {
	for i, q := range qs {
		if strings.Contains(q, sub) {
			return q, i
		}
	}
	return "", -1
}

func TestRequestsGateway(t *testing.T) {
	c, queries, params := newTestClient(t)
	res, err := c.Requests(context.Background(), Filter{PathPrefix: "/datalakehouse/servidores"}, "24h", "key", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Count != 10 || res.Summary.P99 != 120.25 || res.Top[0].Key != "servidores-painel" {
		t.Fatalf("result = %+v", res)
	}
	// 24h in 1h slots, empty ones filled: the slot the fake answered keeps its numbers.
	if len(res.Series) != 25 {
		t.Fatalf("%d slots, want 25 (24h of 1h, both ends)", len(res.Series))
	}
	var hit int
	for _, b := range res.Series {
		if b.T.Unix()%3600 != 0 {
			t.Fatalf("slot %v not aligned to the hour", b.T)
		}
		if b.Count > 0 {
			hit++
			if !b.T.Equal(time.Unix(1699999200, 0)) {
				t.Errorf("data landed in slot %v", b.T)
			}
		}
	}
	if hit != 1 {
		t.Errorf("%d slots with data, want 1", hit)
	}
	// Facets: routes and keys of the window.
	if len(res.Facets.Routes) != 2 || len(res.Facets.Keys) != 2 {
		t.Errorf("facets = %+v", res.Facets)
	}
	top, _ := find(*queries, "GROUP BY key")
	for _, want := range []string{"'apisix-gateway'", "startsWith(", "kind_string = 'Server'", "bridge.client", "FORMAT JSON"} {
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

// The viewer's route/key/status filters narrow the stats, are bound as String
// parameters, and are left out of the facets so the options stay complete.
func TestRequestsFilters(t *testing.T) {
	c, queries, params := newTestClient(t)
	f := Filter{PathPrefix: "/sei", Route: "/api/v1/processos/:id", Key: "sei-painel", Status: "404"}
	if _, err := c.Requests(context.Background(), f, "1h", "route", false); err != nil {
		t.Fatal(err)
	}
	sum := (*queries)[0]
	for _, want := range []string{"= {route:String}", "attributes_string['bridge.client'] = {key:String}", "response_status_code = {status:String}"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary lacks %q:\n%s", want, sum)
		}
	}
	if p := (*params)[0]; p["param_route"] != "/api/v1/processos/:id" || p["param_key"] != "sei-painel" || p["param_status"] != "404" {
		t.Errorf("params = %v", p)
	}
	facet, i := find(*queries, "GROUP BY k ")
	if i < 0 || strings.Contains(facet, "{route:String}") || strings.Contains(facet, "{key:String}") || strings.Contains(facet, "{status:String}") {
		t.Errorf("facet query keeps the viewer's filters:\n%s", facet)
	}
}

// The share link's client restriction applies to the facets too.
func TestRequestsClients(t *testing.T) {
	c, queries, params := newTestClient(t)
	if _, err := c.Requests(context.Background(), Filter{PathPrefix: "/sei", Clients: []string{"sei-painel"}}, "7d", "route", false); err != nil {
		t.Fatal(err)
	}
	for _, q := range *queries {
		if !strings.Contains(q, "has(splitByChar(',', {clients:String}), attributes_string['bridge.client'])") {
			t.Errorf("query without the clients restriction:\n%s", q)
		}
	}
	if got := (*params)[0]["param_clients"]; got != "sei-painel" {
		t.Errorf("clients param = %s", got)
	}
}

func TestRequestsSeriesByKey(t *testing.T) {
	c, queries, params := newTestClient(t)
	res, err := c.Requests(context.Background(), Filter{PathPrefix: "/sei"}, "24h", "route", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SeriesByKey) != 2 || res.SeriesByKey[0].Key != "servidores-painel" || res.SeriesByKey[1].Key != OtherKey {
		t.Fatalf("series by key = %+v", res.SeriesByKey)
	}
	for _, s := range res.SeriesByKey {
		if len(s.Points) != 25 {
			t.Errorf("%s: %d points, want the full window (25)", s.Key, len(s.Points))
		}
	}
	q, i := find(*queries, "GROUP BY t, k")
	if !strings.Contains(q, "arraySlice(splitByChar(',', {topkeys:String}), 2)") || strings.Contains(q, "Array(String)") {
		t.Errorf("series-by-key query:\n%s", q)
	}
	if got := (*params)[i]["param_topkeys"]; got != ",servidores-painel,/api/x" {
		t.Errorf("topkeys = %q", got)
	}
	if _, err := c.Requests(context.Background(), Filter{Domains: []string{"app.example"}}, "24h", "route", true); err == nil {
		t.Error("series by key accepted for a service")
	}
}

func TestRequestsTraefik(t *testing.T) {
	c, queries, params := newTestClient(t)
	if _, err := c.Requests(context.Background(), Filter{Domains: []string{"Bridge.Example", "o'q.example"}}, "1h", "service", false); err != nil {
		t.Fatal(err)
	}
	// One span per request (Traefik's Server span), matched on the requested
	// domain without its port; grouped by that domain for a host.
	q, _ := find(*queries, "GROUP BY key")
	for _, want := range []string{"'traefik'", "kind_string = 'Server'", "has(splitByChar(',', {domains:String}), lower(splitByChar(':', attributes_string['server.address'])[1]))",
		"SELECT lower(splitByChar(':', attributes_string['server.address'])[1]) AS key"} {
		if !strings.Contains(q, want) {
			t.Errorf("traefik query lacks %q:\n%s", want, q)
		}
	}
	if strings.Contains(q, "Array(String)") {
		t.Errorf("Array parameter in:\n%s", q)
	}
	if got := (*params)[0]["param_domains"]; got != "bridge.example,o'q.example" {
		t.Errorf("domains param = %s", got)
	}
	if _, i := find(*queries, "bridge.client"); i >= 0 {
		t.Errorf("a service query reads bridge.client:\n%s", (*queries)[i])
	}
}

func TestSpans(t *testing.T) {
	c, queries, _ := newTestClient(t)
	spans, err := c.Spans(context.Background(), Filter{PathPrefix: "/sei", Key: "k1"}, "24h", 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].TraceID != "t1" || spans[0].Route != "/api/:id" || spans[0].Client != "servidores-painel" ||
		!spans[0].Time.Equal(time.UnixMilli(1700000000123)) {
		t.Fatalf("spans = %+v", spans)
	}
	q := (*queries)[0]
	if !strings.Contains(q, "LIMIT 50") || !strings.Contains(q, "{key:String}") {
		t.Errorf("limit not clamped or filter missing:\n%s", q)
	}
}

func TestTrace(t *testing.T) {
	c, queries, params := newTestClient(t)
	id := "0123456789abcdef0123456789abcdef"
	ok, err := c.TraceMatches(context.Background(), Filter{PathPrefix: "/sei"}, id)
	if err != nil || !ok {
		t.Fatalf("matches = %v %v", ok, err)
	}
	if !strings.Contains((*queries)[0], "trace_id = {trace:String}") || (*params)[0]["param_trace"] != id {
		t.Errorf("match query: %s %v", (*queries)[0], (*params)[0])
	}
	if ok, _ := c.TraceMatches(context.Background(), Filter{PathPrefix: "/sei"}, "x' OR 1=1"); ok || len(*queries) != 1 {
		t.Error("a malformed trace id reached ClickHouse")
	}
	spans, cut, err := c.Trace(context.Background(), id)
	if err != nil || cut || len(spans) != 2 {
		t.Fatalf("trace = %+v %v %v", spans, cut, err)
	}
	if spans[0].StartMS != 0 || spans[1].StartMS != 5 || spans[1].ParentID != "a" {
		t.Errorf("offsets/parents = %+v", spans)
	}
	if spans[0].Attributes["http.target"] != "/x?cpf=1" || spans[0].Attributes["http.status_code"] != "200" {
		t.Errorf("attributes (raw, numbers as text) = %v", spans[0].Attributes)
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
		{Filter{PathPrefix: "/x", Domains: []string{"a"}}, "1h", "route"},
		{Filter{Domains: []string{"a"}, Clients: []string{"c"}}, "1h", "route"},
		{Filter{Domains: []string{"a"}, Key: "c"}, "1h", "route"},
		{Filter{Domains: []string{"a,b"}}, "1h", "route"},
		{Filter{PathPrefix: "/x", Clients: []string{""}}, "1h", "route"},
	} {
		if _, err := c.Requests(context.Background(), tc.f, tc.rng, tc.grp, false); err == nil {
			t.Errorf("%+v accepted", tc)
		}
	}
}

func TestRequestsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "{\n\t\"meta\":\n\t[\n\t],\n\t\"data\":\n\t[\nCode: 60. Table does not exist")
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, "", "").Requests(context.Background(), Filter{PathPrefix: "/x"}, "1h", "route", false)
	if err == nil || !strings.Contains(err.Error(), "500 Code: 60. Table does not exist") {
		t.Fatalf("err = %v", err)
	}
}
