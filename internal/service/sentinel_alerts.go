package service

import (
	"fmt"
	"sort"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// sentinelMin5xxRequests: below this many requests a 5xx share is noise
// (1 error in 3 requests is 33%).
const sentinelMin5xxRequests = 20

// SentinelAlerts derives alerts from a host's latest Sentinel snapshot and
// its services' — recomputed on every read, so an alert clears itself once a
// sync no longer sees the problem.
// ponytail: computed, not stored, so they can't be linked to an issue the way
// Grafana's are; store them with HostAlertRepo.UpsertExternal
// (external_source "sentinel") if that's needed.
func SentinelAlerts(h *models.SentinelHostSnapshot, svcs []models.SentinelServiceSnapshot, t *models.AlertThresholds) []AlertView {
	var out []AlertView
	add := func(typ, level, msg string) {
		out = append(out, AlertView{Type: typ, Level: level, Message: msg, Source: "sentinel", Status: "active"})
	}
	if h == nil {
		return nil
	}
	switch h.State {
	case SentinelOK, SentinelReasonDisabled, SentinelReasonNotLinked:
	default:
		add("sentinel_unreadable", "warning", "Sentinel could not be read: "+h.State)
	}
	if h.PushOK != nil && !*h.PushOK {
		add("sentinel_push", "warning", "Sentinel is not reaching Coolify")
	}
	for _, r := range []struct {
		typ, label string
		v          *float64
	}{
		{"sentinel_cpu", "CPU", h.CPUP95_24h},
		{"sentinel_ram", "RAM", h.MemP95_24h},
		{"sentinel_disk", "Disk", h.DiskPct},
	} {
		if r.v == nil {
			continue
		}
		switch pct := *r.v; {
		case pct >= float64(t.ResourceCritical):
			add(r.typ, "critical", fmt.Sprintf("%s p95 24h at %.0f%% (critical: %d%%)", r.label, pct, t.ResourceCritical))
		case pct >= float64(t.ResourceWarning):
			add(r.typ, "warning", fmt.Sprintf("%s p95 24h at %.0f%% (warning: %d%%)", r.label, pct, t.ResourceWarning))
		}
	}

	sorted := append([]models.SentinelServiceSnapshot(nil), svcs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ServiceName < sorted[j].ServiceName })
	for _, v := range sorted {
		name := v.ServiceName
		if v.Req24h != nil && v.S5xx24h != nil && *v.Req24h >= sentinelMin5xxRequests {
			pct := float64(*v.S5xx24h) * 100 / float64(*v.Req24h)
			switch {
			case t.Traffic5xxCritical > 0 && pct >= float64(t.Traffic5xxCritical):
				add("sentinel_5xx", "critical", fmt.Sprintf("%s: %.1f%% of requests failed with 5xx in 24h (critical: %d%%)", name, pct, t.Traffic5xxCritical))
			case t.Traffic5xxWarning > 0 && pct >= float64(t.Traffic5xxWarning):
				add("sentinel_5xx", "warning", fmt.Sprintf("%s: %.1f%% of requests failed with 5xx in 24h (warning: %d%%)", name, pct, t.Traffic5xxWarning))
			}
		}
		if v.P95Ms != nil {
			switch ms := *v.P95Ms; {
			case t.LatencyP95Critical > 0 && ms >= float64(t.LatencyP95Critical):
				add("sentinel_latency", "critical", fmt.Sprintf("%s: p95 latency %.0f ms in 24h (critical: %d ms)", name, ms, t.LatencyP95Critical))
			case t.LatencyP95Warning > 0 && ms >= float64(t.LatencyP95Warning):
				add("sentinel_latency", "warning", fmt.Sprintf("%s: p95 latency %.0f ms in 24h (warning: %d ms)", name, ms, t.LatencyP95Warning))
			}
		}
		// A stopped container keeps the health it had when it stopped: only a
		// running one's health means anything.
		if v.ContainerHealth == "unhealthy" && v.ContainerState == "running" {
			add("sentinel_unhealthy", "critical", name+": container is unhealthy")
		}
		if v.RestartCount != nil && v.PrevRestartCount != nil && *v.RestartCount > *v.PrevRestartCount {
			add("sentinel_restarts", "warning", fmt.Sprintf("%s: restarted %d time(s) since the previous sync", name, *v.RestartCount-*v.PrevRestartCount))
		}
		if v.TraefikBypass {
			add("sentinel_bypass", "info", name+": also published on a host port; traffic through it isn't counted")
		}
	}
	return out
}
