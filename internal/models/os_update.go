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

// Backup statuses of a result.
const (
	OSBackupDone    = "done"
	OSBackupFailed  = "failed"
	OSBackupDeleted = "deleted"
)

// OSUpdateRun is one check / upgrade over a set of hosts (os_update_runs).
// Counts are per result status, filled by the list and get queries.
type OSUpdateRun struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Reboot      bool   `json:"reboot"`
	Concurrency int    `json:"concurrency"`
	Target      string `json:"target,omitempty"`
	Backup      bool   `json:"backup"`      // back each host up to MinIO first
	KeepBackup  bool   `json:"keep_backup"` // keep it after a success too
	// Restores: the kept backup (source run and host) and where it goes.
	RestoreMode   string           `json:"restore_mode,omitempty"`  // staging | paths
	RestorePaths  []string         `json:"restore_paths,omitempty"` // absolute paths
	SourceRunID   *int64           `json:"source_run_id,omitempty"`
	SourceHostID  *int64           `json:"source_host_id,omitempty"`
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
	BackupKey          string     `json:"backup_key,omitempty"`
	BackupSize         *int64     `json:"backup_size,omitempty"`
	BackupStatus       string     `json:"backup_status,omitempty"` // done | failed | deleted
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
}
