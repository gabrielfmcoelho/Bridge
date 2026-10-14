package service

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

func TestSentinelAlerts(t *testing.T) {
	th := models.DefaultAlertThresholds()
	f := func(v float64) *float64 { return &v }
	n := func(v int64) *int64 { return &v }
	i := func(v int) *int { return &v }
	no := false

	host := &models.SentinelHostSnapshot{State: SentinelOK, PushOK: &no, CPUP95_24h: f(85), MemP95_24h: f(65), DiskPct: f(20)}
	svcs := []models.SentinelServiceSnapshot{
		{ServiceName: "api", Req24h: n(1000), S5xx24h: n(60), P95Ms: f(1500)}, // 6% critical, latency warning
		{ServiceName: "quiet", Req24h: n(10), S5xx24h: n(5), P95Ms: f(10)},    // under the request floor
		{ServiceName: "flower", ContainerState: "running", ContainerHealth: "unhealthy", RestartCount: i(3), PrevRestartCount: i(1), TraefikBypass: true},
		{ServiceName: "old-postgres", ContainerState: "exited", ContainerHealth: "unhealthy"}, // stale health of a stopped container
		{ServiceName: "fresh", RestartCount: i(4)},                                            // first sync: no previous count
	}
	got := map[string]string{}
	for _, a := range SentinelAlerts(host, svcs, &th) {
		got[a.Type+"|"+strings.SplitN(a.Message, ":", 2)[0]] = a.Level
		if a.Source != "sentinel" {
			t.Errorf("source = %q", a.Source)
		}
	}
	want := map[string]string{
		"sentinel_push|Sentinel is not reaching Coolify": "warning",
		"sentinel_cpu|CPU p95 24h at 85% (critical":      "critical",
		"sentinel_ram|RAM p95 24h at 65% (warning":       "warning",
		"sentinel_5xx|api":                               "critical",
		"sentinel_latency|api":                           "warning",
		"sentinel_unhealthy|flower":                      "critical",
		"sentinel_restarts|flower":                       "warning",
		"sentinel_bypass|flower":                         "info",
	}
	if len(got) != len(want) {
		t.Errorf("got %d alerts, want %d: %v", len(got), len(want), got)
	}
	for k, lvl := range want {
		if got[k] != lvl {
			t.Errorf("%s = %q, want %q (all: %v)", k, got[k], lvl, got)
		}
	}

	if a := SentinelAlerts(&models.SentinelHostSnapshot{State: SentinelReasonSSHFailed}, nil, &th); len(a) != 1 || a[0].Type != "sentinel_unreadable" {
		t.Fatalf("unreadable host = %+v", a)
	}
	if SentinelAlerts(nil, nil, &th) != nil {
		t.Fatal("never-synced host has no alerts")
	}
}
