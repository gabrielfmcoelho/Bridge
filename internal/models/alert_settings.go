package models

// AlertThresholds are the resource-usage alert cutoffs (percent). Persistence
// lives in internal/store.AlertSettingsRepo (backed by the alert_resource_* keys
// in app_settings) — this file is the pure data type only.
type AlertThresholds struct {
	ResourceCritical int `json:"resource_critical"`
	ResourceWarning  int `json:"resource_warning"`
	ResourceInfoLow  int `json:"resource_info_low"`
}
