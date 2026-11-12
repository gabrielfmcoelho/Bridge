package service

import (
	"strings"
	"testing"
	"time"
)

// Each list comes from the first level that sets it: run, host, defaults.
func TestEffectivePaths(t *testing.T) {
	d := BackupDefaults{Include: []string{"/"}, Exclude: []string{"/tmp"}}
	host := BackupPaths{Exclude: []string{"/tmp", "/var/lib/postgresql"}}
	run := BackupPaths{Include: []string{"/etc", "/opt"}}

	got := EffectivePaths(d, BackupPaths{}, BackupPaths{})
	if strings.Join(got.Include, " ") != "/" || strings.Join(got.Exclude, " ") != "/tmp" {
		t.Fatalf("defaults = %+v", got)
	}
	got = EffectivePaths(d, host, BackupPaths{})
	if strings.Join(got.Include, " ") != "/" || strings.Join(got.Exclude, " ") != "/tmp /var/lib/postgresql" {
		t.Fatalf("host = %+v", got)
	}
	got = EffectivePaths(d, host, run)
	if strings.Join(got.Include, " ") != "/etc /opt" || strings.Join(got.Exclude, " ") != "/tmp /var/lib/postgresql" {
		t.Fatalf("run = %+v", got)
	}
}

func TestBackupKeyAndExpiry(t *testing.T) {
	d := BackupDefaults{Prefix: DefaultBackupPrefix, RetentionDays: 30}
	at := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	if k := d.BackupKey("vm-web-01", 42, "tar.zst", at); k != "bridge/os-backups/vm-web-01/20261112-093000-run42.tar.zst" {
		t.Fatalf("key = %q", k)
	}
	if e := d.ExpiresAt(at); e == nil || !e.Equal(at.AddDate(0, 0, 30)) {
		t.Fatalf("expires = %v", e)
	}
	d.RetentionDays = 0
	if d.ExpiresAt(at) != nil {
		t.Fatal("retention 0 never expires")
	}
}

func TestCleanPaths(t *testing.T) {
	got, err := CleanPaths([]string{" /etc  /opt/app ", "", "/srv"})
	if err != nil || strings.Join(got, "|") != "/etc|/opt/app|/srv" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := CleanPaths([]string{"etc"}); err == nil {
		t.Fatal("a relative path must be refused")
	}
}
