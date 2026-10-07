// Package signoz reads request telemetry (count, errors, p50/p95/p99, recent
// requests, whole traces) from SigNoz's ClickHouse. Two span sources feed it:
// the APISIX gateway (APIs, with the caller's Keycloak azp) and Coolify's
// Traefik (apps, by the domain each request asked for).
//
// Every SigNoz schema detail lives in the constants below: when SigNoz
// changes its trace table, this is the one place to update.
package signoz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	spansTable = "signoz_traces.distributed_signoz_index_v3"
	svcCol     = "resource_string_service$$name"

	gatewayService = "apisix-gateway" // resource.service.name set in APISIX's plugin_metadata
	traefikService = "traefik"

	// APISIX opentelemetry span attributes. bridge.client is the caller's azp:
	// a serverless-post-function (access phase, after openid-connect validated
	// the token) sets it on the open span — additional_attributes can't carry
	// it, the plugin reads those in the rewrite phase, before auth.
	attrGatewayPath   = "attributes_string['http.target']"
	attrGatewayClient = "attributes_string['bridge.client']"
	// Traefik (v3.6) makes two spans per request: its entry point's Server
	// span (server.address = the requested domain, url.path, url.query) and a
	// ReverseProxy Client span to the app, plus Client spans polling Docker.
	// Only the Server span counts; it carries no router name, so an app is
	// matched by its domains. server.address may carry a :port.
	attrTraefikDomain = "lower(splitByChar(':', attributes_string['server.address'])[1])"
	attrTraefikPath   = "attributes_string['url.path']"
	// APISIX writes http.method; Traefik (new semconv) http.request.method.
	attrMethod = "if(attributes_string['http.method'] != '', attributes_string['http.method'], attributes_string['http.request.method'])"

	// "Users" per source: an API's distinct keys (Keycloak clients); an app's
	// distinct visitors, approximated by client address + user agent (NAT and
	// proxies merge people; it counts devices, never stores who).
	usersGateway = "uniqIf(attributes_string['bridge.client'], attributes_string['bridge.client'] != '')"
	// Traefik's client.address is always the direct peer (ETIPI's proxy for
	// public domains): the visitor is the first IP of X-Forwarded-For, which
	// Traefik records only with forwardedHeaders.trustedIPs plus
	// tracing.capturedRequestHeaders=X-Forwarded-For. Without it, the peer.
	// (?:…): extract returns the first capture group when there is one.
	attrVisitorIP = "if(extract(attributes_string['http.request.header.x-forwarded-for'], '[0-9]+(?:\\\\.[0-9]+){3}') != '', " +
		"extract(attributes_string['http.request.header.x-forwarded-for'], '[0-9]+(?:\\\\.[0-9]+){3}'), attributes_string['client.address'])"
	usersTraefik = "uniq(" + attrVisitorIP + ", attributes_string['user_agent.original'])"

	// The aggregate every stats query selects (plus the source's users).
	agg = `count() AS count,
		countIf(has_error OR toUInt16OrZero(response_status_code) >= 500) AS errors,
		quantile(0.5)(duration_nano) / 1e6 AS p50,
		quantile(0.95)(duration_nano) / 1e6 AS p95,
		quantile(0.99)(duration_nano) / 1e6 AS p99`

	// Lines drawn per key over time; the rest is folded into OtherKey.
	seriesKeys = 5
	OtherKey   = "__outros__"
)

// Filter picks the spans of one asset. Exactly one of PathPrefix (an API
// behind the gateway) or Domains (apps behind Traefik, lower-case, no port).
//
// Clients is a restriction (a share link sees only the keys it shares): it
// applies to everything, facets included. Route, Key and Status are the
// viewer's own filters: facets ignore them, so the options stay complete.
type Filter struct {
	PathPrefix string
	Domains    []string
	Clients    []string
	Route      string // normalized route, as group_by=route reports it
	Key        string // one Keycloak client (azp); APIs only
	Status     string // HTTP status code, e.g. "404"
}

// Stats is one aggregate; latencies in milliseconds. Errors = 5xx or span
// error. Users: distinct keys (APIs) or visitors (apps), see usersGateway.
type Stats struct {
	Count  int64   `json:"count"`
	Users  int64   `json:"users"`
	Errors int64   `json:"errors"`
	P50    float64 `json:"p50"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
}

// Bucket is Stats for one time slot.
type Bucket struct {
	T time.Time `json:"t"`
	Stats
}

// Row is Stats for one group (route, key, status or domain).
type Row struct {
	Key string `json:"key"`
	Stats
}

// KeySeries is one key's line over time (Key OtherKey = every other key).
type KeySeries struct {
	Key    string   `json:"key"`
	Points []Bucket `json:"points"`
}

// Facets are the filter options of the window: top routes and, for APIs, keys.
type Facets struct {
	Routes []string `json:"routes"`
	Keys   []string `json:"keys,omitempty"`
}

// Result is what Requests returns.
type Result struct {
	Summary     Stats       `json:"summary"`
	Series      []Bucket    `json:"series"`
	Top         []Row       `json:"top"`
	SeriesByKey []KeySeries `json:"series_by_key,omitempty"`
	Facets      Facets      `json:"facets"`
}

// Span is one request: the gateway's (or Traefik's) span for it.
type Span struct {
	Time       time.Time `json:"time"`
	TraceID    string    `json:"trace_id"`
	SpanID     string    `json:"span_id"`
	Method     string    `json:"method"`
	Route      string    `json:"route"`
	Path       string    `json:"path"`
	Status     string    `json:"status"`
	DurationMS float64   `json:"duration_ms"`
	Client     string    `json:"client,omitempty"`
}

// TraceSpan is one span of a whole trace, any service. StartMS is the offset
// from the trace's first span.
type TraceSpan struct {
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Service    string            `json:"service"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	StartMS    float64           `json:"start_ms"`
	DurationMS float64           `json:"duration_ms"`
	Status     string            `json:"status,omitempty"`
	Error      bool              `json:"error,omitempty"`
	Attributes map[string]string `json:"attributes"`
}

// Ranges maps the accepted range names to (window, bucket width).
var Ranges = map[string][2]time.Duration{
	"1h":  {time.Hour, 2 * time.Minute},
	"24h": {24 * time.Hour, time.Hour},
	"7d":  {7 * 24 * time.Hour, 6 * time.Hour},
	"30d": {30 * 24 * time.Hour, 24 * time.Hour},
}

// GroupBys are the accepted group_by values.
var GroupBys = map[string]bool{"route": true, "key": true, "status": true, "service": true}

// MaxTraceSpans bounds one trace; longer traces come back cut (Trace's bool).
const MaxTraceSpans = 500

var traceIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Client struct {
	url, user, pass string
	http            *http.Client
	now             func() time.Time
}

func NewClient(chURL, user, pass string) *Client {
	return &Client{url: chURL, user: user, pass: pass, http: &http.Client{Timeout: 20 * time.Second}, now: time.Now}
}

// scope is a filter turned into SQL over one window: the WHERE clause, its
// bound parameters and the asset's path/route expressions.
type scope struct {
	where      string
	params     url.Values
	route      string // normalized-route expression
	path       string // path expression (gateway: without the API prefix)
	from, step int64
	gateway    bool
	agg        string // agg plus this source's users
}

func (c *Client) scope(f Filter, rng string) (*scope, error) {
	win, ok := Ranges[rng]
	if !ok {
		return nil, fmt.Errorf("invalid range %q", rng)
	}
	if (f.PathPrefix == "") == (len(f.Domains) == 0) {
		return nil, errors.New("filter needs a path prefix or domains")
	}
	if (len(f.Clients) > 0 || f.Key != "") && f.PathPrefix == "" {
		return nil, errors.New("clients and keys only filter an API")
	}
	for _, v := range append(append([]string{}, f.Domains...), f.Clients...) {
		if v == "" || strings.Contains(v, ",") {
			return nil, fmt.Errorf("invalid domain or client %q", v)
		}
	}
	s := &scope{from: c.now().Add(-win[0]).Unix(), step: int64(win[1].Seconds()), gateway: f.PathPrefix != ""}
	s.agg = agg + ", " + usersTraefik + " AS users"
	if s.gateway {
		s.agg = agg + ", " + usersGateway + " AS users"
	}
	s.params = url.Values{"param_from": {fmt.Sprint(s.from)}, "param_step": {fmt.Sprint(s.step)}}
	s.where = "timestamp >= toDateTime64({from:Int64}, 9) AND ts_bucket_start >= {from:Int64} - 1800"
	if s.gateway {
		// The gateway's own Server span, root or not: callers that are traced
		// themselves (visualizador-front) make it a child of their span.
		s.where += fmt.Sprintf(" AND kind_string = 'Server' AND %s = '%s' AND startsWith(%s, {prefix:String})", svcCol, gatewayService, attrGatewayPath)
		s.params.Set("param_prefix", f.PathPrefix)
		if len(f.Clients) > 0 {
			s.where += fmt.Sprintf(" AND has(%s, %s)", csvParam("clients"), attrGatewayClient)
			s.params.Set("param_clients", strings.Join(f.Clients, ","))
		}
		s.path = fmt.Sprintf("substring(%s, %d)", attrGatewayPath, len(f.PathPrefix)+1)
	} else {
		s.where += fmt.Sprintf(" AND kind_string = 'Server' AND %s = '%s' AND has(%s, %s)", svcCol, traefikService, csvParam("domains"), attrTraefikDomain)
		s.params.Set("param_domains", strings.ToLower(strings.Join(f.Domains, ",")))
		s.path = attrTraefikPath
	}
	// ponytail: ids are collapsed by pattern (digits, uuids); true route
	// templates need OTel inside each app (http.route).
	s.route = fmt.Sprintf(`replaceRegexpAll(replaceRegexpAll(%s, '/[0-9a-fA-F]{8}-[0-9a-fA-F-]{27}', '/:id'), '/[0-9]+', '/:id')`, s.path)
	if f.Route != "" {
		s.where += fmt.Sprintf(" AND %s = {route:String}", s.route)
		s.params.Set("param_route", f.Route)
	}
	if f.Key != "" {
		s.where += fmt.Sprintf(" AND %s = {key:String}", attrGatewayClient)
		s.params.Set("param_key", f.Key)
	}
	if f.Status != "" {
		s.where += " AND response_status_code = {status:String}"
		s.params.Set("param_status", f.Status)
	}
	return s, nil
}

// slots fills a series to every slot of the window, empty ones as zero:
// ClickHouse only returns slots with spans. Slots start at multiples of step
// since the epoch, as toStartOfInterval's.
func (c *Client) slots(s *scope, got map[int64]Stats) []Bucket {
	out := []Bucket{}
	for t := s.from - s.from%s.step; t <= c.now().Unix(); t += s.step {
		out = append(out, Bucket{T: time.Unix(t, 0).UTC(), Stats: got[t]})
	}
	return out
}

// Requests runs the summary, the time series, the top-20 groups and the
// facets for f over the last rng ("1h", "24h", "7d", "30d"). byKey adds one
// line per top key (APIs only).
func (c *Client) Requests(ctx context.Context, f Filter, rng, groupBy string, byKey bool) (*Result, error) {
	if !GroupBys[groupBy] {
		return nil, fmt.Errorf("invalid group_by %q", groupBy)
	}
	s, err := c.scope(f, rng)
	if err != nil {
		return nil, err
	}
	if byKey && !s.gateway {
		return nil, errors.New("series by key is for APIs")
	}
	var key string
	switch groupBy {
	case "route":
		key = s.route
	case "key":
		key = attrGatewayClient
	case "status":
		key = "response_status_code"
	case "service":
		key = attrTraefikDomain
	}

	res := &Result{Top: []Row{}, Facets: Facets{Routes: []string{}}}
	var sum []Stats
	if err := c.query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s", s.agg, spansTable, s.where), s.params, &sum); err != nil {
		return nil, err
	}
	if len(sum) > 0 {
		res.Summary = sum[0]
	}
	var series []struct {
		T int64 `json:"t"`
		Stats
	}
	q := fmt.Sprintf("SELECT toUnixTimestamp(toStartOfInterval(timestamp, toIntervalSecond({step:Int64}))) AS t, %s FROM %s WHERE %s GROUP BY t ORDER BY t", s.agg, spansTable, s.where)
	if err := c.query(ctx, q, s.params, &series); err != nil {
		return nil, err
	}
	got := make(map[int64]Stats, len(series))
	for _, b := range series {
		got[b.T] = b.Stats
	}
	res.Series = c.slots(s, got)
	q = fmt.Sprintf("SELECT %s AS key, %s FROM %s WHERE %s GROUP BY key ORDER BY count DESC LIMIT 20", key, s.agg, spansTable, s.where)
	if err := c.query(ctx, q, s.params, &res.Top); err != nil {
		return nil, err
	}

	// Facets: the same asset and window without the viewer's own filters.
	base := f
	base.Route, base.Key, base.Status = "", "", ""
	bs, _ := c.scope(base, rng)
	if res.Facets.Routes, err = c.topKeys(ctx, bs, bs.route, 20); err != nil {
		return nil, err
	}
	if bs.gateway {
		if res.Facets.Keys, err = c.topKeys(ctx, bs, attrGatewayClient, 20); err != nil {
			return nil, err
		}
	}

	if byKey {
		if res.SeriesByKey, err = c.seriesByKey(ctx, s); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// topKeys is the n most frequent values of expr in s.
func (c *Client) topKeys(ctx context.Context, s *scope, expr string, n int) ([]string, error) {
	var rows []struct {
		K string `json:"k"`
	}
	q := fmt.Sprintf("SELECT %s AS k FROM %s WHERE %s GROUP BY k ORDER BY count() DESC LIMIT %d", expr, spansTable, s.where, n)
	if err := c.query(ctx, q, s.params, &rows); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.K)
	}
	return out, nil
}

// seriesByKey draws the top seriesKeys keys as their own lines and folds the
// rest into OtherKey. Keys with a comma can't travel in the csv parameter;
// they fall into OtherKey.
func (c *Client) seriesByKey(ctx context.Context, s *scope) ([]KeySeries, error) {
	top, err := c.topKeys(ctx, s, attrGatewayClient, seriesKeys)
	if err != nil {
		return nil, err
	}
	keep := []string{}
	for _, k := range top {
		if !strings.Contains(k, ",") {
			keep = append(keep, k)
		}
	}
	params := url.Values{}
	for k, v := range s.params {
		params[k] = v
	}
	// A leading comma marks the csv as present even when every key is "".
	params.Set("param_topkeys", ","+strings.Join(keep, ","))
	var rows []struct {
		T int64  `json:"t"`
		K string `json:"k"`
		Stats
	}
	q := fmt.Sprintf(`SELECT toUnixTimestamp(toStartOfInterval(timestamp, toIntervalSecond({step:Int64}))) AS t,
		if(has(arraySlice(%s, 2), %s), %s, '%s') AS k, %s
		FROM %s WHERE %s GROUP BY t, k ORDER BY t`,
		csvParam("topkeys"), attrGatewayClient, attrGatewayClient, OtherKey, s.agg, spansTable, s.where)
	if err := c.query(ctx, q, params, &rows); err != nil {
		return nil, err
	}
	by := map[string]map[int64]Stats{}
	for _, r := range rows {
		if by[r.K] == nil {
			by[r.K] = map[int64]Stats{}
		}
		by[r.K][r.T] = r.Stats
	}
	out := []KeySeries{}
	for _, k := range append(keep, OtherKey) {
		if by[k] != nil {
			out = append(out, KeySeries{Key: k, Points: c.slots(s, by[k])})
		}
	}
	return out, nil
}

// Spans lists the most recent requests matching f (newest first), at most limit.
func (c *Client) Spans(ctx context.Context, f Filter, rng string, limit int) ([]Span, error) {
	s, err := c.scope(f, rng)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	client := "''"
	if s.gateway {
		client = attrGatewayClient
	}
	var rows []struct {
		MS int64 `json:"ms"`
		Span
	}
	q := fmt.Sprintf(`SELECT toUnixTimestamp64Milli(timestamp) AS ms, trace_id, span_id, %s AS method,
		%s AS route, %s AS path, response_status_code AS status, duration_nano / 1e6 AS duration_ms, %s AS client
		FROM %s WHERE %s ORDER BY timestamp DESC LIMIT %d`,
		attrMethod, s.route, s.path, client, spansTable, s.where, limit)
	if err := c.query(ctx, q, s.params, &rows); err != nil {
		return nil, err
	}
	out := make([]Span, len(rows))
	for i, r := range rows {
		out[i] = r.Span
		out[i].Time = time.UnixMilli(r.MS).UTC()
	}
	return out, nil
}

// TraceMatches says whether trace traceID holds a span of f's asset within
// the last 30 days — what lets an asset's viewer open that trace.
func (c *Client) TraceMatches(ctx context.Context, f Filter, traceID string) (bool, error) {
	if !traceIDRe.MatchString(traceID) {
		return false, nil
	}
	s, err := c.scope(f, "30d")
	if err != nil {
		return false, err
	}
	s.params.Set("param_trace", traceID)
	var rows []struct {
		N int64 `json:"n"`
	}
	q := fmt.Sprintf("SELECT count() AS n FROM %s WHERE trace_id = {trace:String} AND %s", spansTable, s.where)
	if err := c.query(ctx, q, s.params, &rows); err != nil {
		return false, err
	}
	return len(rows) > 0 && rows[0].N > 0, nil
}

// Trace returns every span of traceID (any service, last 30 days) ordered by
// start, at most MaxTraceSpans; cut reports a longer trace. Attributes are
// raw: the caller decides what each viewer may see.
func (c *Client) Trace(ctx context.Context, traceID string) (spans []TraceSpan, cut bool, err error) {
	if !traceIDRe.MatchString(traceID) {
		return nil, false, fmt.Errorf("invalid trace id %q", traceID)
	}
	from := c.now().Add(-30 * 24 * time.Hour).Unix()
	params := url.Values{"param_trace": {traceID}, "param_from": {fmt.Sprint(from)}}
	var rows []struct {
		StartNS int64              `json:"start_ns"`
		Str     map[string]string  `json:"attrs"`
		Num     map[string]float64 `json:"nums"`
		TraceSpan
	}
	q := fmt.Sprintf(`SELECT span_id, parent_span_id AS parent_id, %s AS service, name, kind_string AS kind,
		toUnixTimestamp64Nano(timestamp) AS start_ns, duration_nano / 1e6 AS duration_ms,
		response_status_code AS status, has_error AS error, attributes_string AS attrs, attributes_number AS nums
		FROM %s WHERE trace_id = {trace:String} AND timestamp >= toDateTime64({from:Int64}, 9)
		ORDER BY timestamp LIMIT %d`, svcCol, spansTable, MaxTraceSpans+1)
	if err := c.query(ctx, q, params, &rows); err != nil {
		return nil, false, err
	}
	if len(rows) > MaxTraceSpans {
		rows, cut = rows[:MaxTraceSpans], true
	}
	spans = make([]TraceSpan, len(rows))
	for i, r := range rows {
		sp := r.TraceSpan
		sp.StartMS = float64(r.StartNS-rows[0].StartNS) / 1e6
		sp.Attributes = make(map[string]string, len(r.Str)+len(r.Num))
		for k, v := range r.Str {
			sp.Attributes[k] = v
		}
		for k, v := range r.Num {
			sp.Attributes[k] = fmt.Sprint(v)
		}
		spans[i] = sp
	}
	return spans, cut, nil
}

// query POSTs sql (FORMAT JSON appended) and decodes its data rows into out.
// Values are bound server-side as {name:Type} query parameters, never spliced.
func (c *Client) query(ctx context.Context, sql string, params url.Values, out any) error {
	p := url.Values{"output_format_json_quote_64bit_integers": {"0"}}
	for k, v := range params {
		p[k] = v
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/?"+p.Encode(), strings.NewReader(sql+" FORMAT JSON"))
	if err != nil {
		return err
	}
	req.Header.Set("X-ClickHouse-User", c.user)
	req.Header.Set("X-ClickHouse-Key", c.pass)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("signoz clickhouse: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		// A failure mid-stream comes after a partial JSON body: keep the
		// exception ("Code: …"), not the opening braces.
		msg := string(body)
		if i := strings.Index(msg, "Code: "); i >= 0 {
			msg = msg[i:]
		}
		return fmt.Errorf("signoz clickhouse: %d %s", resp.StatusCode, strings.TrimSpace(msg[:min(len(msg), 400)]))
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("signoz clickhouse: decode: %w", err)
	}
	return json.Unmarshal(env.Data, out)
}

// csvParam is a list bound as one comma-joined String parameter, split in
// SQL. Not Array(String): on SigNoz's Distributed table an Array parameter
// reaches the shard as a String ("Bad get: has String, requested Array"),
// while String and Int64 parameters survive.
func csvParam(name string) string {
	return fmt.Sprintf("splitByChar(',', {%s:String})", name)
}
