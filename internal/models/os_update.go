package models

import "time"

// OS update run statuses.
const (
	OSRunRunning     = "running"
	OSRunDone        = "done"
	OSRunCancelled   = "cancelled"
	OSRunInterrupted = "interrupted"
)

// OS update result statuses.
const (
	OSResultPending = "pending"
	OSResultRunning = "running"
	OSResultSuccess = "success"
	OSResultFailed  = "failed"
	OSResultSkipped = "skipped"
)

// OSUpdateRun is one check / upgrade over a set of hosts (os_update_runs).
// Counts are per result status, filled by the list and get queries.
type OSUpdateRun struct {
	ID            int64            `json:"id"`
	Kind          string           `json:"kind"`
	Reboot        bool             `json:"reboot"`
	Concurrency   int              `json:"concurrency"`
	Target        string           `json:"target,omitempty"`
	Status        string           `json:"status"`
	RequestedBy   *int64           `json:"requested_by,omitempty"`
	RequestedName string           `json:"requested_name,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	FinishedAt    *time.Time       `json:"finished_at,omitempty"`
	Counts        map[string]int   `json:"counts"`
	Results       []OSUpdateResult `json:"results,omitempty"`
}

// OSUpdateResult is one host of a run (os_update_results).
type OSUpdateResult struct {
	RunID              int64      `json:"run_id"`
	HostID             int64      `json:"host_id"`
	HostNickname       string     `json:"host_nickname"`
	HostSlug           string     `json:"host_slug"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	PkgManager         string     `json:"pkg_manager,omitempty"`
	OSBefore           string     `json:"os_before,omitempty"`
	OSAfter            string     `json:"os_after,omitempty"`
	Upgradable         *int       `json:"upgradable,omitempty"`
	SecurityUpgradable *int       `json:"security_upgradable,omitempty"`
	Upgraded           *int       `json:"upgraded,omitempty"`
	RebootRequired     bool       `json:"reboot_required"`
	Rebooted           bool       `json:"rebooted"`
	Output             string     `json:"output,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
}
