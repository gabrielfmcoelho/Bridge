package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCronDaemon(t *testing.T) {
	cases := []struct {
		name, active, enabled string
		i, a, e               bool
	}{
		{"crond", "active", "enabled", true, true, true},
		{"", "", "", false, false, false},
		{"cron", "inactive", "disabled", true, false, false},
	}
	for _, c := range cases {
		i, a, e := parseCronDaemon(c.name, c.active, c.enabled)
		if i != c.i || a != c.a || e != c.e {
			t.Errorf("parseCronDaemon(%q,%q,%q) = %v,%v,%v", c.name, c.active, c.enabled, i, a, e)
		}
	}
}

func TestCronCommandBinary(t *testing.T) {
	cases := map[string]string{
		"/ust/bin/certbot renew --quiet --no-self-upgrade": "/ust/bin/certbot",
		"PGPASSWORD=x /usr/bin/pg_dump db":                 "/usr/bin/pg_dump",
		"cd /tmp && ./run.sh":                              "",
		"certbot renew":                                    "",
		"  /opt/x.sh>/dev/null 2>&1":                       "/opt/x.sh",
		// Shell and cron expansions are not literal paths.
		"/home/$USER/bin/backup.sh":  "",
		"/opt/backup-$(date +%F).sh": "",
		"/opt/app/`hostname`.sh":     "",
		"/opt/run.sh%input":          "",
		// Glob patterns are not one path.
		"/opt/backup/*.sh": "",
		"/opt/run?.sh":     "",
		"/opt/[ab].sh":     "",
		// A quoted path keeps its spaces.
		`"/opt/my app/run.sh" --x`:          "/opt/my app/run.sh",
		`'/opt/my app/run.sh'>/dev/null`:    "/opt/my app/run.sh",
		`PGPASSWORD=x "/opt/my app/b.sh" y`: "/opt/my app/b.sh",
		`"/opt/unterminated x`:              "",
		`"/opt/my app"/run.sh`:              "",
		`A="/x" "/opt/b c/d.sh"`:            "/opt/b c/d.sh",
	}
	for in, want := range cases {
		if got := cronCommandBinary(in); got != want {
			t.Errorf("cronCommandBinary(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkMissingBinaries(t *testing.T) {
	jobs := []CronJob{
		{Command: "/ust/bin/certbot renew"},
		{Command: "/usr/bin/pg_dump db"},
		{Command: "certbot renew"},
	}
	markMissingBinaries(jobs, map[string]bool{"/ust/bin/certbot": true})
	if !jobs[0].MissingBinary || jobs[1].MissingBinary || jobs[2].MissingBinary {
		t.Errorf("got %+v", jobs)
	}
}

// Without root a path is reported only when it doesn't exist and the scan user
// can reach where it would be: a 0700 home or a root-only script is not a
// missing binary just because the scan user can't see it.
func TestMissingBinariesScriptRuns(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("runs the non-root branch")
	}
	root := t.TempDir()
	for _, f := range []struct {
		rel  string
		mode os.FileMode
	}{{"open/run.sh", 0o755}, {"open/noexec.sh", 0o644}, {"locked/run.sh", 0o755}} {
		p := filepath.Join(root, f.rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, f.mode); err != nil {
			t.Fatal(err)
		}
	}
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	paths := []string{"open/run.sh", "open/noexec.sh", "open/gone.sh", "open/it's gone.sh", "nodir/sub/gone.sh", "locked/run.sh", "locked/gone.sh", "locked/sub/gone.sh", `open/back\cslash.sh`}
	for i := range paths {
		paths[i] = filepath.Join(root, paths[i])
	}
	out, err := exec.Command("sh", "-c", missingBinariesScript(paths)).CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	// dash's echo stops at "\c": the path is printed with printf, whole.
	want := []string{paths[2], paths[3], paths[4], paths[8]}
	if got := strings.Split(strings.TrimSpace(string(out)), "\n"); !reflect.DeepEqual(got, want) {
		t.Fatalf("reported %q, want %q", got, want)
	}
}
