// Package signoz reads request telemetry (count, errors, p50/p95/p99) from
// SigNoz's ClickHouse. Two span sources feed it (see the plan in the Bridge
// telemetry docs): the APISIX gateway (APIs, with the caller's Keycloak azp)
// and Coolify's Traefik (apps, by Coolify resource uuid).
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
	"strings"
	"time"
)

const (
	spansTable = "signoz_traces.distributed_signoz_index_v3"
	svcCol     = "resource_string_service$$name"

	gatewayService = "apisix-gateway" // resource.service.name set in APISIX's plugin_attr
	traefikService = "traefik"

	// APISIX opentelemetry span attributes. bridge.client is the caller's azp:
	// a serverless-post-function (access phase, after openid-connect validated
	// the token) sets it on the open span — additional_attributes can't carry
	// it, the plugin reads those in the rewrite phase, before auth.
	attrGatewayPath   = "attributes_string['http.target']"
	attrGatewayClient = "attributes_string['bridge.client']"
	// Traefik router spans carry the router name, which Coolify builds from
	// the resource uuid (e.g. "https-0-<uuid>@docker").
	attrTraefikRouter = "attributes_string['traefik.router.name']"
	attrTraefikPath   = "attributes_string['url.path']"
)

// Filter picks the spans of one asset. Exactly one of PathPrefix (an API
// behind the gateway) or RouterUUIDs (Coolify resources behind Traefik).
// Clients narrows an API to those Keycloak clients (azp) — a share link
// shows only the keys it shares.
type Filter struct {
	PathPrefix  string
	RouterUUIDs []string
	Clients     []string
}

// Stats is one aggregate; latencies in milliseconds. Errors = 5xx or span error.
type Stats struct {
	Count  int64   `json:"count"`
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

// Row is Stats for one group (route, key, status or service uuid).
type Row struct {
	Key string `json:"key"`
	Stats
}

// Result is what Requests returns.
type Result struct {
	Summary Stats    `json:"summary"`
	Series  []Bucket `json:"series"`
	Top     []Row    `json:"top"`
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

type Client struct {
	url, user, pass string
	http            *http.Client
	now             func() time.Time
}

func NewClient(chURL, user, pass string) *Client {
	return &Client{url: chURL, user: user, pass: pass, http: &http.Client{Timeout: 20 * time.Second}, now: time.Now}
}

// Requests runs the summary, the time series and the top-20 groups for f
// over the last rng ("1h", "24h", "7d", "30d").
func (c *Client) Requests(ctx context.Context, f Filter, rng, groupBy string) (*Result, error) {
	win, ok := Ranges[rng]
	if !ok {
		return nil, fmt.Errorf("invalid range %q", rng)
	}
	if !GroupBys[groupBy] {
		return nil, fmt.Errorf("invalid group_by %q", groupBy)
	}
	if (f.PathPrefix == "") == (len(f.RouterUUIDs) == 0) {
		return nil, errors.New("filter needs a path prefix or router uuids")
	}
	if len(f.Clients) > 0 && f.PathPrefix == "" {
		return nil, errors.New("clients only filter an API")
	}
	for _, v := range append(append([]string{}, f.RouterUUIDs...), f.Clients...) {
		if v == "" || strings.Contains(v, ",") {
			return nil, fmt.Errorf("invalid uuid or client %q", v)
		}
	}
	from := c.now().Add(-win[0]).Unix()
	params := url.Values{"param_from": {fmt.Sprint(from)}, "param_step": {fmt.Sprint(int64(win[1].Seconds()))}}

	where := "timestamp >= toDateTime64({from:Int64}, 9) AND ts_bucket_start >= {from:Int64} - 1800"
	path, key := attrGatewayPath, ""
	if f.PathPrefix != "" {
		where += fmt.Sprintf(" AND parent_span_id = '' AND %s = '%s' AND startsWith(%s, {prefix:String})", svcCol, gatewayService, attrGatewayPath)
		params.Set("param_prefix", f.PathPrefix)
		if len(f.Clients) > 0 {
			where += fmt.Sprintf(" AND has(%s, %s)", csvParam("clients"), attrGatewayClient)
			params.Set("param_clients", strings.Join(f.Clients, ","))
		}
		path = fmt.Sprintf("substring(%s, %d)", attrGatewayPath, len(f.PathPrefix)+1)
	} else {
		// Traefik's router span is a child of the entrypoint span: no root filter.
		where += fmt.Sprintf(" AND %s = '%s' AND arrayExists(u -> position(%s, u) > 0, %s)", svcCol, traefikService, attrTraefikRouter, csvParam("uuids"))
		params.Set("param_uuids", strings.Join(f.RouterUUIDs, ","))
		path = attrTraefikPath
	}
	switch groupBy {
	case "route":
		// ponytail: ids are collapsed by pattern (digits, uuids); true route
		// templates need OTel inside each app (http.route).
		key = fmt.Sprintf(`replaceRegexpAll(replaceRegexpAll(%s, '/[0-9a-fA-F]{8}-[0-9a-fA-F-]{27}', '/:id'), '/[0-9]+', '/:id')`, path)
	case "key":
		key = attrGatewayClient
	case "status":
		key = "response_status_code"
	case "service":
		key = fmt.Sprintf("arrayFirst(u -> position(%s, u) > 0, %s)", attrTraefikRouter, csvParam("uuids"))
	}

	const agg = `count() AS count,
		countIf(has_error OR toUInt16OrZero(response_status_code) >= 500) AS errors,
		quantile(0.5)(duration_nano) / 1e6 AS p50,
		quantile(0.95)(duration_nano) / 1e6 AS p95,
		quantile(0.99)(duration_nano) / 1e6 AS p99`

	res := &Result{Series: []Bucket{}, Top: []Row{}}
	var sum []Stats
	if err := c.query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s", agg, spansTable, where), params, &sum); err != nil {
		return nil, err
	}
	if len(sum) > 0 {
		res.Summary = sum[0]
	}
	var series []struct {
		T int64 `json:"t"`
		Stats
	}
	q := fmt.Sprintf("SELECT toUnixTimestamp(toStartOfInterval(timestamp, toIntervalSecond({step:Int64}))) AS t, %s FROM %s WHERE %s GROUP BY t ORDER BY t", agg, spansTable, where)
	if err := c.query(ctx, q, params, &series); err != nil {
		return nil, err
	}
	for _, b := range series {
		res.Series = append(res.Series, Bucket{T: time.Unix(b.T, 0).UTC(), Stats: b.Stats})
	}
	q = fmt.Sprintf("SELECT %s AS key, %s FROM %s WHERE %s GROUP BY key ORDER BY count DESC LIMIT 20", key, agg, spansTable, where)
	if err := c.query(ctx, q, params, &res.Top); err != nil {
		return nil, err
	}
	return res, nil
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
