package models

import "time"

// SentinelHostSnapshot is the latest Coolify Sentinel summary of a host
// (sentinel_host_snapshots). State is "ok" or the reason Sentinel couldn't be
// read (service.SentinelReason*); nil metrics = Sentinel had no value.
type SentinelHostSnapshot struct {
	HostID         int64      `json:"host_id"`
	State          string     `json:"state"`
	Detail         string     `json:"detail,omitempty"`
	MetricsEnabled bool       `json:"metrics_enabled"`
	TrafficEnabled bool       `json:"traffic_enabled"`
	PushOK         *bool      `json:"push_ok"`
	LastPushAt     *time.Time `json:"last_push_at"`
	CPUAvg24h      *float64   `json:"cpu_avg_24h"`
	CPUP95_24h     *float64   `json:"cpu_p95_24h"`
	MemP95_24h     *float64   `json:"mem_p95_24h"`
	DiskPct        *float64   `json:"disk_pct"`
	Load1          *float64   `json:"load1"`
	SyncedAt       time.Time  `json:"synced_at"`
}

// SentinelServiceSnapshot is the latest Sentinel summary of a service: its
// traffic over 24h (nil when traffic analytics is off or no app key matches)
// and its container's live state. CPU is percent of one core (can pass 100).
type SentinelServiceSnapshot struct {
	ServiceID        int64    `json:"service_id"`
	HostID           int64    `json:"host_id"`
	Req24h           *int64   `json:"req_24h"`
	S4xx24h          *int64   `json:"s4xx_24h"`
	S5xx24h          *int64   `json:"s5xx_24h"`
	P50Ms            *float64 `json:"p50_ms"`
	P95Ms            *float64 `json:"p95_ms"`
	P99Ms            *float64 `json:"p99_ms"`
	Visitors24h      *int64   `json:"visitors_24h"`
	ContainerState   string   `json:"container_state"`
	ContainerHealth  string   `json:"container_health"`
	RestartCount     *int     `json:"restart_count"`
	PrevRestartCount *int     `json:"prev_restart_count"`
	CPUPct           *float64 `json:"cpu_pct"`
	MemPct           *float64 `json:"mem_pct"`
	// TraefikBypass: the service has traffic through the proxy and also
	// publishes a host port, so the counts miss whatever uses the port.
	TraefikBypass bool      `json:"traefik_bypass"`
	SyncedAt      time.Time `json:"synced_at"`
	// ServiceName is read-only (joined from services).
	ServiceName string `json:"service_name"`
}
