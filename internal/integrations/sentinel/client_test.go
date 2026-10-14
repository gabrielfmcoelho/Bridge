package sentinel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Bodies captured from Sentinel 1.0.2 on a Coolify server (trimmed).
var fixtures = map[string]string{
	"/api/summary":            `{"cpu":{"time":"1791988041266","percent":60.31},"memory":{"time":"1791988041266","total":10432704512,"available":5757960192,"used":4674744320,"usedPercent":44.81,"free":1755013120},"disk":[{"time":"1791987921266","mount":"/app/db","usedPercent":10.23},{"time":"1791987921266","mount":"/etc/hostname","usedPercent":23.55}],"network":{"time":"1791988041266","rxBytesPerSec":3693.87,"txBytesPerSec":8808.07},"load":{"time":"1791988041266","load1":1.89,"load5":2.15,"load15":2.17},"host":{"time":"1791988041266","uptimeSeconds":33185050}}`,
	"/api/cpu/history":        `[{"time":"1791981329918","percent":"30.57"},{"time":"1791981347309","percent":"26.28"}]`,
	"/api/memory/history":     `[{"time":"1791981329918","total":25159274496,"available":12556664832,"used":12602609664,"usedPercent":50.09,"free":2428452864}]`,
	"/api/containers/current": `[{"id":"airflow-apiserver-rtb37u69bbkepzukr9vclrd7-153118077568","cpu":null,"memory":null,"disk":{"time":"1791553500000","writableLayer":42357277,"volumesTotal":0},"network":null,"status":null,"time":"1791553500000"},{"id":"flower-rtb37u69bbkepzukr9vclrd7-20261013t214301","cpu":{"time":"1791988041266","percent":"0.08"},"memory":{"time":"1791988041266","total":10432704512,"used":239681536,"usedPercent":2.3},"status":{"state":"running","health":"healthy","restartCount":0}}]`,
	"/api/container/flower-rtb37u69bbkepzukr9vclrd7-20261013t214301/cpu/history": `[{"time":"1791985531266","percent":"0.17"}]`,
	"/api/resource/rtb37u69bbkepzukr9vclrd7/traffic/overview":                    `{"requests":144,"bytes_in":126,"bytes_out":2221810,"status":{"s2xx":142,"s3xx":1,"s4xx":1,"s5xx":0},"latency":{"p50":96.67,"p95":322.95,"p99":843.71},"unique_visitors":1}`,
	"/api/resource/rtb37u69bbkepzukr9vclrd7/traffic/series":                      `[{"bucket":1791900000000,"requests":3,"bytes_in":0,"bytes_out":0,"s2xx":2,"s3xx":0,"s4xx":1,"s5xx":0,"unique_visitors":1,"p95":12.5}]`,
	"/api/resource/rtb37u69bbkepzukr9vclrd7/traffic/paths":                       `[{"path":"/ui/grid/runs/x","app":"rtb37u69bbkepzukr9vclrd7-airflow-apiserver-0c35","requests":25,"bytes_out":28359,"s4xx":0,"s5xx":0,"p50":95.35,"p95":376.47}]`,
	"/api/resource/rtb37u69bbkepzukr9vclrd7/traffic/breakdown/status":            `[{"value":"200","requests":142,"bytes_out":2221789},{"value":"499","requests":1,"bytes_out":21}]`,
	"/api/traffic/apps": `["rtb37u69bbkepzukr9vclrd7-airflow-apiserver-0c35","rtb37u69bbkepzukr9vclrd7-flower-608f"]`,
	"/api/app/rtb37u69bbkepzukr9vclrd7-flower-608f/traffic/overview": `{"requests":2,"bytes_in":0,"bytes_out":10,"status":{"s2xx":2,"s3xx":0,"s4xx":0,"s5xx":0},"latency":{"p50":2.3,"p95":2.3,"p99":2.3},"unique_visitors":1}`,
	"/api/version":     "1.0.2\n",
	"/api/push-status": `{"last_attempt_at":"2026-10-14T13:54:21Z","last_success_at":"2026-10-14T13:54:21Z","last_error":null,"last_status":null,"consecutive_failures":0}`,
}

func fakeSentinel(t *testing.T) (*Client, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := fixtures[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")
	return NewClient(addr, "tok", (&net.Dialer{}).DialContext), addr
}

func TestClient(t *testing.T) {
	c, _ := fakeSentinel(t)
	ctx := context.Background()
	now := time.Now()

	s, err := c.Summary(ctx)
	if err != nil || float64(s.CPU.Percent) != 60.31 || float64(s.Memory.UsedPercent) != 44.81 || s.DiskPct() != 23.55 {
		t.Fatalf("summary: %+v %v", s, err)
	}
	cpu, err := c.CPUHistory(ctx, now)
	if err != nil || len(cpu) != 2 || cpu[0] != (Point{1791981329918, 30.57}) {
		t.Fatalf("cpu history (string percent): %+v %v", cpu, err)
	}
	cs, err := c.ContainersCurrent(ctx)
	if err != nil || len(cs) != 1 || cs[0].Status.Health != "healthy" {
		t.Fatalf("containers: stale entries must drop: %+v %v", cs, err)
	}
	// Container ids are the lowercased name.
	cc, err := c.ContainerCPU(ctx, "flower-rtb37u69bbkepzukr9vclrd7-20261013T214301", now)
	if err != nil || len(cc) != 1 || cc[0].V != 0.17 {
		t.Fatalf("container cpu: %+v %v", cc, err)
	}
	o, err := c.Overview(ctx, ScopeResource, "rtb37u69bbkepzukr9vclrd7", now.Add(-time.Hour), now)
	if err != nil || o.Requests != 144 || o.Status.S4xx != 1 || o.Latency.P95 != 322.95 {
		t.Fatalf("overview: %+v %v", o, err)
	}
	ser, err := c.Series(ctx, ScopeResource, "rtb37u69bbkepzukr9vclrd7", "24h")
	if err != nil || len(ser) != 1 || ser[0].S4xx != 1 || ser[0].P95 != 12.5 {
		t.Fatalf("series: %+v %v", ser, err)
	}
	if p, err := c.Paths(ctx, ScopeResource, "rtb37u69bbkepzukr9vclrd7", now, now, 10); err != nil || p[0].Requests != 25 {
		t.Fatalf("paths: %+v %v", p, err)
	}
	if b, err := c.Breakdown(ctx, ScopeResource, "rtb37u69bbkepzukr9vclrd7", "status", now, now, 10); err != nil || b[1].Value != "499" {
		t.Fatalf("breakdown: %+v %v", b, err)
	}
	if v, err := c.Version(ctx); err != nil || v != "1.0.2" {
		t.Fatalf("version (plain text) = %q, %v", v, err)
	}
	if keys, err := c.TrafficApps(ctx); err != nil || len(keys) != 2 {
		t.Fatalf("apps: %v %v", keys, err)
	}
	if o, err := c.Overview(ctx, ScopeApp, "rtb37u69bbkepzukr9vclrd7-flower-608f", now, now); err != nil || o.Requests != 2 {
		t.Fatalf("app overview: %+v %v", o, err)
	}
	if _, err := c.Overview(ctx, Scope("server"), "x", now, now); err == nil {
		t.Fatal("unknown scope must be rejected")
	}
	if ps, err := c.PushStatus(ctx); err != nil || ps.LastSuccessAt == nil || ps.ConsecutiveFailures != 0 {
		t.Fatalf("push status: %+v %v", ps, err)
	}
}

func TestClientRejects(t *testing.T) {
	c, addr := fakeSentinel(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := c.Overview(ctx, ScopeApp, "../summary", now, now); err == nil {
		t.Fatal("path traversal id must be rejected")
	}
	if _, err := c.Breakdown(ctx, ScopeApp, "abc", "ip", now, now, 5); err == nil {
		t.Fatal("ip breakdown must be rejected (identifies visitors)")
	}
	if _, err := c.Series(ctx, ScopeApp, "abc", "1y"); err == nil {
		t.Fatal("unknown range must be rejected")
	}
	bad := NewClient(addr, "wrong", (&net.Dialer{}).DialContext)
	if _, err := bad.Summary(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestDownsampleAndStats(t *testing.T) {
	var pts []Point
	for i := 0; i < 1000; i++ {
		pts = append(pts, Point{int64(i * 10_000), float64(i % 100)})
	}
	d := Downsample(pts, 100)
	if len(d) == 0 || len(d) > 100 {
		t.Fatalf("downsample len %d", len(d))
	}
	if got := Downsample(pts[:5], 100); len(got) != 5 {
		t.Fatal("short series must pass through")
	}
	if p := P95(pts); p != 94 {
		t.Fatalf("p95 = %v, want 94", p)
	}
	if m := Mean([]Point{{0, 1}, {1, 3}}); m != 2 {
		t.Fatalf("mean = %v", m)
	}
	if P95(nil) != 0 || Mean(nil) != 0 {
		t.Fatal("empty stats must be 0")
	}
}
