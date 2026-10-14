package models

// AlertThresholds are the alert cutoffs: resource usage (percent, scan and
// Sentinel), and Sentinel traffic — 5xx share (percent of requests) and p95
// latency (ms). Persistence lives in internal/store.AlertSettingsRepo (backed
// by alert_* keys in app_settings) — this file is the pure data type only.
type AlertThresholds struct {
	ResourceCritical   int `json:"resource_critical"`
	ResourceWarning    int `json:"resource_warning"`
	ResourceInfoLow    int `json:"resource_info_low"`
	Traffic5xxWarning  int `json:"traffic_5xx_warning"`
	Traffic5xxCritical int `json:"traffic_5xx_critical"`
	LatencyP95Warning  int `json:"latency_p95_warning"`
	LatencyP95Critical int `json:"latency_p95_critical"`
}

// DefaultAlertThresholds are used for any key never saved.
func DefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{ResourceCritical: 80, ResourceWarning: 60, ResourceInfoLow: 5,
		Traffic5xxWarning: 2, Traffic5xxCritical: 5, LatencyP95Warning: 1000, LatencyP95Critical: 3000}
}
