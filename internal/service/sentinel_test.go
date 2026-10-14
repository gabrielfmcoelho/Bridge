package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/sentinel"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// fakeSentinelBodies mirror Sentinel 1.0.2 on a host running an Airflow
// compose app (Nodo F): flower has traffic, the worker only a container.
var fakeSentinelBodies = map[string]string{
	"/api/version":            `"1.0.2"`,
	"/api/push-status":        `{"last_success_at":"2026-10-14T13:54:21Z","consecutive_failures":0}`,
	"/api/cpu/history":        `[{"time":"1","percent":"10"},{"time":"2","percent":"30"},{"time":"3","percent":"95"}]`,
	"/api/memory/history":     `[{"time":"1","usedPercent":40},{"time":"2","usedPercent":44}]`,
	"/api/summary":            `{"cpu":{"time":"3","percent":95},"disk":[{"mount":"/app/db","usedPercent":10.2},{"mount":"/etc/hosts","usedPercent":23.5}],"load":{"time":"3","load1":1.9}}`,
	"/api/containers/current": `[{"id":"flower-rtb37u69bbkepzukr9vclrd7-20261013t214301","cpu":{"time":"3","percent":"0.08"},"memory":{"time":"3","usedPercent":2.3},"status":{"state":"running","health":"healthy","restartCount":1}},{"id":"airflow-worker-rtb37u69bbkepzukr9vclrd7-20261013t214301","cpu":{"time":"3","percent":"95.3"},"memory":{"time":"3","usedPercent":8.9},"status":{"state":"running","health":"healthy","restartCount":0}}]`,
	"/api/traffic/apps":       `["rtb37u69bbkepzukr9vclrd7-flower-608f"]`,
	"/api/app/rtb37u69bbkepzukr9vclrd7-flower-608f/traffic/overview": `{"requests":200,"status":{"s2xx":180,"s3xx":0,"s4xx":8,"s5xx":12},"latency":{"p50":20,"p95":1500,"p99":2000},"unique_visitors":3}`,
	"/api/app/rtb37u69bbkepzukr9vclrd7-flower-608f/traffic/series":   `[{"bucket":1791900000000,"requests":200,"s2xx":180,"s4xx":8,"s5xx":12,"p95":1500}]`,
	"/api/app/rtb37u69bbkepzukr9vclrd7-flower-608f/traffic/paths":    `[{"path":"/tasks","app":"rtb37u69bbkepzukr9vclrd7-flower-608f","requests":150,"p95":1400}]`,
}

func TestSentinelSync(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	var failOnce atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failOnce.CompareAndSwap(true, false) {
			http.Error(w, "gone", http.StatusBadGateway)
			return
		}
		if body, ok := fakeSentinelBodies[r.URL.Path]; ok {
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc := NewSentinelService(d)
	var dials atomic.Int32
	metrics, traffic := true, true
	svc.dial = func(ctx context.Context, host *models.Host) (*sentinelConn, error) {
		dials.Add(1)
		api := sentinel.NewClient(strings.TrimPrefix(srv.URL, "http://"), "tok", (&net.Dialer{}).DialContext)
		return &sentinelConn{api: api, metrics: metrics, traffic: traffic}, nil
	}

	var hostID, flower, worker, manual int64
	scan := func(q string, dst *int64, args ...any) {
		t.Helper()
		if err := d.SQL.QueryRow(q, args...).Scan(dst); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	scan(`INSERT INTO hosts (nickname, oficial_slug, coolify_server_uuid) VALUES ('nodo-f', 'nodo-f', 'h7t2q26dp5r8ty3oazurbv41') RETURNING id`, &hostID)
	// The scan last saw an older deploy (12-digit timestamp).
	scan(`INSERT INTO services (nickname, source, discovery_key, container_name, container_ports) VALUES ('flower', 'auto', 'flower-rtb37u69bbkepzukr9vclrd7', 'flower-rtb37u69bbkepzukr9vclrd7-123345302783', '0.0.0.0:5555->5555/tcp') RETURNING id`, &flower)
	scan(`INSERT INTO services (nickname, source, discovery_key, container_name) VALUES ('worker', 'auto', 'airflow-worker-rtb37u69bbkepzukr9vclrd7', 'airflow-worker-rtb37u69bbkepzukr9vclrd7-123345134951') RETURNING id`, &worker)
	scan(`INSERT INTO services (nickname) VALUES ('manual') RETURNING id`, &manual)
	for _, id := range []int64{flower, worker, manual} {
		if _, err := d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, id, hostID); err != nil {
			t.Fatal(err)
		}
	}

	// Integration off: nothing is read.
	if _, err := svc.Sync(ctx); err == nil || !strings.Contains(err.Error(), SentinelReasonDisabled) {
		t.Fatalf("sync with integration off: %v", err)
	}
	if err := store.NewAppSettingsRepo(d.SQL).Set(ctx, "sentinel_enabled", "true"); err != nil {
		t.Fatal(err)
	}

	rep, err := svc.Sync(ctx)
	if err != nil || rep.OK != 1 || rep.Hosts[0].Services != 2 {
		t.Fatalf("sync = %+v, %v", rep, err)
	}
	repo := store.NewSentinelRepo(d.SQL)
	h, _ := repo.GetHost(ctx, hostID)
	if h == nil || h.State != SentinelOK || *h.CPUP95_24h != 95 || *h.MemP95_24h != 44 || *h.DiskPct != 23.5 || h.PushOK == nil || !*h.PushOK {
		t.Fatalf("host snapshot = %+v", h)
	}
	f, _ := repo.GetService(ctx, flower)
	if f == nil || *f.Req24h != 200 || *f.S5xx24h != 12 || *f.P95Ms != 1500 || f.ContainerHealth != "healthy" || !f.TraefikBypass {
		t.Fatalf("flower snapshot = %+v", f)
	}
	w, _ := repo.GetService(ctx, worker)
	if w == nil || w.Req24h != nil || *w.CPUPct != 95.3 {
		t.Fatalf("worker snapshot (container, no traffic) = %+v", w)
	}
	if m, _ := repo.GetService(ctx, manual); m != nil {
		t.Fatalf("manual service must get no snapshot: %+v", m)
	}
	if n := dials.Load(); n != 1 {
		t.Fatalf("dials = %d, want 1 (connection pooled)", n)
	}

	// Live reads share the pooled connection.
	tr, err := svc.ServiceTraffic(ctx, &models.Service{ID: flower, ContainerName: "flower-rtb37u69bbkepzukr9vclrd7-123345302783"}, "")
	if err != nil || len(tr.Keys) != 1 || tr.Overview.Requests != 200 || len(tr.Series) != 1 || tr.Paths[0].Path != "/tasks" {
		t.Fatalf("traffic = %+v, %v", tr, err)
	}
	if n := dials.Load(); n != 1 {
		t.Fatalf("dials after a live read = %d, want 1", n)
	}

	// A pooled connection that fails (Sentinel recreated) is redialed once.
	failOnce.Store(true)
	if st := svc.Status(ctx, &models.Host{ID: hostID, CoolifyServerUUID: ptr("h7t2q26dp5r8ty3oazurbv41")}); !st.Available || st.Version != "1.0.2" {
		t.Fatalf("status after a failed pooled read = %+v", st)
	}
	if n := dials.Load(); n != 2 {
		t.Fatalf("dials after retry = %d, want 2", n)
	}

	// Traffic analytics off: the live read says why; the sync keeps metrics.
	svc.Close()
	traffic = false
	if _, err := svc.ServiceTraffic(ctx, &models.Service{ID: worker}, "7d"); err == nil || !strings.Contains(err.Error(), SentinelReasonTrafficOff) {
		t.Fatalf("traffic off: %v", err)
	}
	if _, err := svc.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if f, _ := repo.GetService(ctx, flower); f == nil || f.Req24h != nil || f.ContainerState != "running" {
		t.Fatalf("flower after traffic off = %+v", f)
	}

	// Unreachable host: state records why, service snapshots are dropped.
	svc.Close()
	svc.dial = func(context.Context, *models.Host) (*sentinelConn, error) {
		return nil, unavailable(SentinelReasonSSHFailed, "dial tcp: i/o timeout")
	}
	rep, _ = svc.Sync(ctx)
	if rep.OK != 0 || rep.Hosts[0].State != SentinelReasonSSHFailed {
		t.Fatalf("report = %+v", rep)
	}
	if h, _ := repo.GetHost(ctx, hostID); h.State != SentinelReasonSSHFailed || h.CPUP95_24h != nil {
		t.Fatalf("host after failure = %+v", h)
	}
	if all, _ := repo.ServicesBulk(ctx, hostID); len(all) != 0 {
		t.Fatalf("service snapshots must go with an unreadable host: %+v", all)
	}

	// Not linked to Coolify.
	if st := svc.Status(ctx, &models.Host{ID: 999}); st.Available || st.Reason != SentinelReasonNotLinked {
		t.Fatalf("status = %+v", st)
	}
}

func ptr(s string) *string { return &s }
