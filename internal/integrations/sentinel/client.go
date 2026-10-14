// Package sentinel reads Coolify's Sentinel agent (coollabsio/sentinel 1.x):
// host and container metrics and the Traefik/Caddy traffic analytics.
//
// Sentinel listens on :8888 inside its container and publishes no port, so
// the caller supplies the transport (Bridge dials through the host's SSH
// connection). The bearer token comes from Coolify's API; never log it.
package sentinel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Dialer opens the connection to Sentinel's address (ssh.Client.DialContext,
// or net.Dialer.DialContext in tests).
type Dialer func(ctx context.Context, network, addr string) (net.Conn, error)

// ErrUnauthorized means Sentinel rejected the token (rotated by Coolify).
var ErrUnauthorized = errors.New("sentinel: unauthorized")

// Client is one Sentinel instance.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// NewClient builds a client for Sentinel at addr ("172.17.0.3:8888").
func NewClient(addr, token string, dial Dialer) *Client {
	return &Client{
		base:  "http://" + addr + "/api",
		token: token,
		http: &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{DialContext: dial, MaxIdleConnsPerHost: 4, IdleConnTimeout: time.Minute},
		},
	}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("sentinel %s: %d %.200s", path, resp.StatusCode, body)
	}
	return json.Unmarshal(body, out)
}

// safeSegment is what Sentinel ids look like: resource uuids, container
// names, app keys. Anything else never reaches a URL.
var safeSegment = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,250}$`)

func segment(s string) (string, error) {
	if !safeSegment.MatchString(s) {
		return "", fmt.Errorf("sentinel: invalid id %q", s)
	}
	return s, nil
}

// Dimensions are the traffic breakdowns Bridge exposes. Sentinel also has
// ip/agent/useragent; they identify visitors, so they stay out.
var Dimensions = map[string]bool{
	"status": true, "method": true, "country": true, "referer": true, "browser": true,
	"os": true, "device": true, "protocol": true, "scheme": true, "tls": true, "cache": true, "bot": true,
}

// Ranges are the windows Sentinel's series endpoints accept.
var Ranges = map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}

func iso(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

func window(from, to time.Time) url.Values {
	return url.Values{"from": {iso(from)}, "to": {iso(to)}}
}

// Num decodes a JSON number or numeric string: Sentinel sends cpu percent and
// timestamps as strings, everything else as numbers.
type Num float64

func (n *Num) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	*n = Num(f)
	return err
}

// Point is one sample: t in Unix milliseconds.
type Point struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

type cpuRow struct {
	Time    Num `json:"time"`
	Percent Num `json:"percent"`
}

type memRow struct {
	Time        Num `json:"time"`
	Total       Num `json:"total"`
	Used        Num `json:"used"`
	UsedPercent Num `json:"usedPercent"`
}

type loadRow struct {
	Time  Num `json:"time"`
	Load1 Num `json:"load1"`
}

// Summary is GET /summary: the latest host sample. Any part can be nil.
type Summary struct {
	CPU    *cpuRow  `json:"cpu"`
	Memory *memRow  `json:"memory"`
	Load   *loadRow `json:"load"`
	Disk   []struct {
		Mount       string `json:"mount"`
		UsedPercent Num    `json:"usedPercent"`
	} `json:"disk"`
	Host *struct {
		UptimeSeconds Num `json:"uptimeSeconds"`
	} `json:"host"`
}

// DiskPct is the fullest filesystem Sentinel sees.
// ponytail: max over Sentinel's mounts (its data dir and the Docker root);
// per-mount breakdown when someone needs to know which one.
func (s *Summary) DiskPct() float64 {
	var m float64
	for _, d := range s.Disk {
		m = max(m, float64(d.UsedPercent))
	}
	return m
}

func (c *Client) Summary(ctx context.Context) (*Summary, error) {
	var s Summary
	return &s, c.get(ctx, "/summary", nil, &s)
}

func (c *Client) Version(ctx context.Context) (string, error) {
	var v any
	if err := c.get(ctx, "/version", nil, &v); err != nil {
		return "", err
	}
	return fmt.Sprint(v), nil
}

func cpuPoints(rows []cpuRow) []Point {
	out := make([]Point, len(rows))
	for i, r := range rows {
		out[i] = Point{int64(r.Time), float64(r.Percent)}
	}
	return out
}

func memPoints(rows []memRow) []Point {
	out := make([]Point, len(rows))
	for i, r := range rows {
		out[i] = Point{int64(r.Time), float64(r.UsedPercent)}
	}
	return out
}

// CPUHistory is the host CPU percent since from.
func (c *Client) CPUHistory(ctx context.Context, from time.Time) ([]Point, error) {
	var rows []cpuRow
	err := c.get(ctx, "/cpu/history", url.Values{"from": {iso(from)}}, &rows)
	return cpuPoints(rows), err
}

// MemoryHistory is the host memory used percent since from.
func (c *Client) MemoryHistory(ctx context.Context, from time.Time) ([]Point, error) {
	var rows []memRow
	err := c.get(ctx, "/memory/history", url.Values{"from": {iso(from)}}, &rows)
	return memPoints(rows), err
}

// LoadHistory is the host 1-minute load average since from.
func (c *Client) LoadHistory(ctx context.Context, from time.Time) ([]Point, error) {
	var rows []loadRow
	err := c.get(ctx, "/load/history", url.Values{"from": {iso(from)}}, &rows)
	out := make([]Point, len(rows))
	for i, r := range rows {
		out[i] = Point{int64(r.Time), float64(r.Load1)}
	}
	return out, err
}

// Container is one entry of GET /containers/current. ID is the container name
// lowercased. Containers of old deploys linger with a nil Status.
type Container struct {
	ID     string  `json:"id"`
	CPU    *cpuRow `json:"cpu"`
	Memory *memRow `json:"memory"`
	Status *struct {
		State        string `json:"state"`
		Health       string `json:"health"`
		RestartCount int    `json:"restartCount"`
	} `json:"status"`
}

// ContainersCurrent returns the containers Sentinel still sees (Status set).
func (c *Client) ContainersCurrent(ctx context.Context) ([]Container, error) {
	var all []Container
	if err := c.get(ctx, "/containers/current", nil, &all); err != nil {
		return nil, err
	}
	live := all[:0]
	for _, ct := range all {
		if ct.Status != nil {
			live = append(live, ct)
		}
	}
	return live, nil
}

// ContainerCPU is one container's CPU percent (of one core) since from.
func (c *Client) ContainerCPU(ctx context.Context, name string, from time.Time) ([]Point, error) {
	id, err := segment(strings.ToLower(name))
	if err != nil {
		return nil, err
	}
	var rows []cpuRow
	err = c.get(ctx, "/container/"+id+"/cpu/history", url.Values{"from": {iso(from)}}, &rows)
	return cpuPoints(rows), err
}

// ContainerMemory is one container's memory used percent since from.
func (c *Client) ContainerMemory(ctx context.Context, name string, from time.Time) ([]Point, error) {
	id, err := segment(strings.ToLower(name))
	if err != nil {
		return nil, err
	}
	var rows []memRow
	err = c.get(ctx, "/container/"+id+"/memory/history", url.Values{"from": {iso(from)}}, &rows)
	return memPoints(rows), err
}

// StatusCounts are requests per status class.
type StatusCounts struct {
	S2xx int64 `json:"s2xx"`
	S3xx int64 `json:"s3xx"`
	S4xx int64 `json:"s4xx"`
	S5xx int64 `json:"s5xx"`
}

// Overview is a traffic aggregate; latencies in milliseconds.
type Overview struct {
	Requests int64        `json:"requests"`
	BytesIn  int64        `json:"bytes_in"`
	BytesOut int64        `json:"bytes_out"`
	Status   StatusCounts `json:"status"`
	Latency  struct {
		P50 float64 `json:"p50"`
		P95 float64 `json:"p95"`
		P99 float64 `json:"p99"`
	} `json:"latency"`
	UniqueVisitors int64 `json:"unique_visitors"`
}

// Bucket is one slot of a traffic series (hourly for 24h, daily beyond).
type Bucket struct {
	Bucket   int64 `json:"bucket"` // Unix ms
	Requests int64 `json:"requests"`
	StatusCounts
	UniqueVisitors int64   `json:"unique_visitors"`
	P95            float64 `json:"p95"`
}

// Path is one top path; App is the Sentinel app key (router) it came from.
type Path struct {
	Path     string  `json:"path"`
	App      string  `json:"app"`
	Requests int64   `json:"requests"`
	BytesOut int64   `json:"bytes_out"`
	S4xx     int64   `json:"s4xx"`
	S5xx     int64   `json:"s5xx"`
	P50      float64 `json:"p50"`
	P95      float64 `json:"p95"`
}

// BreakdownRow is one value of a dimension.
type BreakdownRow struct {
	Value    string `json:"value"`
	Requests int64  `json:"requests"`
	BytesOut int64  `json:"bytes_out"`
}

// Scope picks what a traffic query covers: one app key (a Traefik router,
// e.g. `<uuid>-airflow-apiserver-0c35`), or a whole Coolify resource
// (Sentinel merges `{uuid}` and every `{uuid}-*` key).
type Scope string

const (
	ScopeApp      Scope = "app"
	ScopeResource Scope = "resource"
)

func (c *Client) scoped(scope Scope, id, suffix string) (string, error) {
	if scope != ScopeApp && scope != ScopeResource {
		return "", fmt.Errorf("sentinel: invalid scope %q", scope)
	}
	id, err := segment(id)
	if err != nil {
		return "", err
	}
	return "/" + string(scope) + "/" + id + "/traffic/" + suffix, nil
}

// TrafficApps lists every app key Sentinel has recorded traffic for.
func (c *Client) TrafficApps(ctx context.Context) ([]string, error) {
	var keys []string
	return keys, c.get(ctx, "/traffic/apps", nil, &keys)
}

func (c *Client) Overview(ctx context.Context, scope Scope, id string, from, to time.Time) (*Overview, error) {
	p, err := c.scoped(scope, id, "overview")
	if err != nil {
		return nil, err
	}
	var o Overview
	return &o, c.get(ctx, p, window(from, to), &o)
}

func (c *Client) Series(ctx context.Context, scope Scope, id, rng string) ([]Bucket, error) {
	p, err := c.scoped(scope, id, "series")
	if err != nil {
		return nil, err
	}
	if _, ok := Ranges[rng]; !ok {
		return nil, fmt.Errorf("sentinel: invalid range %q", rng)
	}
	var b []Bucket
	return b, c.get(ctx, p, url.Values{"range": {rng}}, &b)
}

func (c *Client) Paths(ctx context.Context, scope Scope, id string, from, to time.Time, limit int) ([]Path, error) {
	p, err := c.scoped(scope, id, "paths")
	if err != nil {
		return nil, err
	}
	q := window(from, to)
	q.Set("limit", strconv.Itoa(limit))
	var out []Path
	return out, c.get(ctx, p, q, &out)
}

func (c *Client) Breakdown(ctx context.Context, scope Scope, id, dim string, from, to time.Time, limit int) ([]BreakdownRow, error) {
	if !Dimensions[dim] {
		return nil, fmt.Errorf("sentinel: invalid dimension %q", dim)
	}
	p, err := c.scoped(scope, id, "breakdown/"+dim)
	if err != nil {
		return nil, err
	}
	q := window(from, to)
	q.Set("limit", strconv.Itoa(limit))
	var rows []BreakdownRow
	return rows, c.get(ctx, p, q, &rows)
}

// PushStatus is Sentinel's heartbeat to Coolify.
type PushStatus struct {
	LastAttemptAt       *time.Time `json:"last_attempt_at"`
	LastSuccessAt       *time.Time `json:"last_success_at"`
	LastError           *string    `json:"last_error"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

func (c *Client) PushStatus(ctx context.Context) (*PushStatus, error) {
	var p PushStatus
	return &p, c.get(ctx, "/push-status", nil, &p)
}

// Downsample averages points into at most n buckets of equal time span, so a
// 7d history (60k samples at 10s) charts as a few hundred.
// ponytail: plain bucket mean flattens short spikes; LTTB if the charts need them.
func Downsample(pts []Point, n int) []Point {
	if n <= 0 || len(pts) <= n {
		return pts
	}
	first, last := pts[0].T, pts[len(pts)-1].T
	span := (last - first) / int64(n)
	if span <= 0 {
		return pts[len(pts)-n:]
	}
	out := make([]Point, 0, n)
	var sumT int64
	var sumV float64
	var cnt int64
	slot := int64(0)
	flush := func() {
		if cnt > 0 {
			out = append(out, Point{sumT / cnt, sumV / float64(cnt)})
		}
		sumT, sumV, cnt = 0, 0, 0
	}
	for _, p := range pts {
		s := min((p.T-first)/span, int64(n-1))
		if s != slot {
			flush()
			slot = s
		}
		sumT += p.T
		sumV += p.V
		cnt++
	}
	flush()
	return out
}

// P95 is the 95th percentile of the values (nearest rank); 0 when empty.
func P95(pts []Point) float64 {
	if len(pts) == 0 {
		return 0
	}
	v := make([]float64, len(pts))
	for i, p := range pts {
		v[i] = p.V
	}
	sort.Float64s(v)
	return v[(len(v)*95+99)/100-1]
}

// Mean of the values; 0 when empty.
func Mean(pts []Point) float64 {
	if len(pts) == 0 {
		return 0
	}
	var s float64
	for _, p := range pts {
		s += p.V
	}
	return s / float64(len(pts))
}
