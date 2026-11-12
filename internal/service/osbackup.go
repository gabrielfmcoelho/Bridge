package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Host backups (the feature; the MinIO connection is the integration's).
// What a backup takes resolves in three levels: the run's override, else the
// host's profile, else these defaults — each list on its own.
const (
	DefaultBackupInclude = "/"
	DefaultBackupExclude = "/tmp /var/tmp /var/cache /var/lib/docker /var/lib/containerd /swapfile /swap.img /var/lib/bridge-osupdate /lost+found"
	DefaultBackupPrefix  = "bridge/os-backups"
	// DefaultBackupRetentionDays: kept backups expire after this many days.
	DefaultBackupRetentionDays = 30
	// BackupRuleID names Bridge's lifecycle rule among the bucket's.
	BackupRuleID = "bridge-os-backups"
)

// BackupDefaults is the osbackup_* settings group.
type BackupDefaults struct {
	Prefix  string   `json:"prefix"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// RetentionDays: backups under Prefix expire after this many days (a
	// bucket lifecycle rule); 0 keeps them until deleted by hand.
	RetentionDays int `json:"retention_days"`
}

// LoadBackupDefaults reads the osbackup_* settings; an empty one is its default.
func LoadBackupDefaults(ctx context.Context, db *sql.DB) BackupDefaults {
	repo := store.NewAppSettingsRepo(db)
	get := func(k string) string { return strings.TrimSpace(repo.Value(ctx, k)) }
	d := BackupDefaults{
		Prefix:        strings.Trim(get("osbackup_prefix"), "/"),
		Include:       strings.Fields(get("osbackup_include")),
		Exclude:       strings.Fields(get("osbackup_exclude")),
		RetentionDays: DefaultBackupRetentionDays,
	}
	if d.Prefix == "" {
		d.Prefix = DefaultBackupPrefix
	}
	if len(d.Include) == 0 {
		d.Include = strings.Fields(DefaultBackupInclude)
	}
	if len(d.Exclude) == 0 {
		d.Exclude = strings.Fields(DefaultBackupExclude)
	}
	if n, err := strconv.Atoi(get("osbackup_retention_days")); err == nil && n >= 0 {
		d.RetentionDays = n
	}
	return d
}

// BackupPaths is an include/exclude pair; an empty list means "inherit".
type BackupPaths struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

// EffectivePaths resolves what a backup takes: each list from the first
// level that sets it (run override, host profile, defaults).
func EffectivePaths(d BackupDefaults, profile, override BackupPaths) BackupPaths {
	first := func(lists ...[]string) []string {
		for _, l := range lists {
			if len(l) > 0 {
				return l
			}
		}
		return nil
	}
	return BackupPaths{
		Include: first(override.Include, profile.Include, d.Include),
		Exclude: first(override.Exclude, profile.Exclude, d.Exclude),
	}
}

// BackupKey is where a host's backup of a run goes.
func (d BackupDefaults) BackupKey(hostSlug string, runID int64, ext string, now time.Time) string {
	return fmt.Sprintf("%s/%s/%s-run%d.%s", d.Prefix, hostSlug, now.UTC().Format("20060102-150405"), runID, ext)
}

// ExpiresAt is when the lifecycle rule removes a backup taken at t (nil when
// backups are kept until deleted by hand).
func (d BackupDefaults) ExpiresAt(t time.Time) *time.Time {
	if d.RetentionDays <= 0 {
		return nil
	}
	e := t.AddDate(0, 0, d.RetentionDays)
	return &e
}

// CleanPaths trims, drops empties and refuses anything not absolute: these
// become tar operands on the host.
func CleanPaths(in []string) ([]string, error) {
	var out []string
	for _, p := range in {
		for _, f := range strings.Fields(p) {
			if !strings.HasPrefix(f, "/") {
				return nil, fmt.Errorf("%q is not an absolute path", f)
			}
			out = append(out, f)
		}
	}
	return out, nil
}
