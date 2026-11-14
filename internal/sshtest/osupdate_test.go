package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeHost is a PATH with only the base tools the script uses plus the given
// stub commands (name → shell body), so each package manager's branch runs
// for real without touching the machine's own apt. A stub replaces the base
// tool of the same name (never written through its symlink).
func fakeHost(t *testing.T, stubs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range []string{"sh", "grep", "cut", "awk", "wc", "sort", "cat", "touch", "cp", "rm", "sed", "tar", "gzip", "mkdir", "chmod", "head", "tail", "find", "diff", "timeout"} {
		if _, stubbed := stubs[tool]; stubbed {
			continue
		}
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s not available", tool)
		}
		if err := os.Symlink(p, filepath.Join(dir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!"+filepath.Join(dir, "sh")+"\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runInner(t *testing.T, kind, path string) OSUpdateReport {
	t.Helper()
	script, err := OSUpdateInnerScript(kind)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(path, "sh"), "-c", script)
	cmd.Env = []string{"PATH=" + path}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", kind, err, out)
	}
	return ParseOSUpdateLog(string(out))
}

// apt lists three upgrades, one from a -security suite; once "upgraded"
// (a marker file) the list is empty.
const aptStub = `
case "$1" in
list) [ -f "$STATE" ] || printf 'Listing...\nopenssl/jammy-updates,jammy-security 3.0.2-0ubuntu1.15 amd64 [upgradable from: 3.0.2-0ubuntu1.14]\ncurl/jammy-updates 7.81.0-1ubuntu1.16 amd64 [upgradable from: 7.81.0-1ubuntu1.15]\nvim/jammy-updates 2:8.2 amd64 [upgradable from: 2:8.1]\n' ;;
esac`

func TestOSUpdateScript_Apt(t *testing.T) {
	state := filepath.Join(t.TempDir(), "upgraded")
	path := fakeHost(t, map[string]string{
		"apt":     "STATE=" + state + aptStub,
		"apt-get": `case "$*" in *update*) exit 0 ;; *upgrade*|*install*) echo "$*" >> ` + state + `.args; touch ` + state + ` ;; esac`,
	})

	r := runInner(t, OSUpdateCheck, path)
	if r.PkgManager != "apt" || *r.Upgradable != 3 || *r.Security != 1 || r.Remaining != nil || r.Error != "" {
		t.Fatalf("check = %+v", r)
	}
	if _, err := os.Stat(state); err == nil {
		t.Fatal("check must not upgrade anything")
	}

	r = runInner(t, OSUpdateSecurity, path)
	if *r.Security != 1 || r.RemainingSecurity == nil || *r.RemainingSecurity != 0 {
		t.Fatalf("security = %+v", r)
	}
	args, _ := os.ReadFile(state + ".args")
	if !strings.Contains(string(args), "install --only-upgrade openssl") || strings.Contains(string(args), "curl") {
		t.Fatalf("security upgraded %q, want openssl only", args)
	}

	os.Remove(state)
	r = runInner(t, OSUpdatePackages, path)
	if *r.Upgradable != 3 || *r.Remaining != 0 {
		t.Fatalf("packages = %+v", r)
	}
	if args, _ := os.ReadFile(state + ".args"); !strings.Contains(string(args), "--force-confold upgrade --with-new-pkgs") {
		t.Fatalf("apt-get args = %q", args)
	}
}

func TestOSUpdateScript_FailureAndOthers(t *testing.T) {
	// apt-get update failing: error marker and a non-zero exit.
	path := fakeHost(t, map[string]string{"apt-get": "exit 100", "apt": "exit 0"})
	script, _ := OSUpdateInnerScript(OSUpdatePackages)
	cmd := exec.Command(filepath.Join(path, "sh"), "-c", script)
	cmd.Env = []string{"PATH=" + path}
	out, err := cmd.CombinedOutput()
	if err == nil || ParseOSUpdateLog(string(out)).Error != "apt-get update failed" {
		t.Fatalf("failed update: err=%v out=%s", err, out)
	}

	// dnf: two upgrades, one named by (two) security advisories.
	path = fakeHost(t, map[string]string{"dnf": `case "$*" in
*check-update*) printf 'openssl.x86_64   1:3.0.7-27.el9   baseos\nbash.x86_64   5.1.8-9.el9   baseos\n'; exit 100 ;;
*updateinfo*) printf 'RLSA-2024:1 Important/Sec. openssl-1:3.0.7-27.el9.x86_64\nRLSA-2024:2 Moderate/Sec. openssl-1:3.0.7-27.el9.x86_64\n' ;;
esac`})
	if r := runInner(t, OSUpdateCheck, path); r.PkgManager != "dnf" || *r.Upgradable != 2 || *r.Security != 1 {
		t.Fatalf("dnf check = %+v", r)
	}

	// apk has no security channel; an unknown manager is skipped.
	path = fakeHost(t, map[string]string{"apk": `[ "$1" = version ] && printf 'musl-1.2.4-r1 < 1.2.4-r2\n'; exit 0`})
	if r := runInner(t, OSUpdateSecurity, path); r.PkgManager != "apk" || *r.Upgradable != 1 || !strings.Contains(r.Skip, "security channel") {
		t.Fatalf("apk security = %+v", r)
	}
	if r := runInner(t, OSUpdateCheck, fakeHost(t, nil)); r.Skip == "" || r.PkgManager != "" {
		t.Fatalf("no manager = %+v", r)
	}
	if _, err := OSUpdateInnerScript("bogus"); err == nil {
		t.Fatal("unknown kind must error")
	}
}

// The launcher runs the script detached and leaves the log and exit code
// where PollOSUpdate reads them (paths swapped to a temp dir).
func TestOSUpdateLauncher_Detached(t *testing.T) {
	dir := t.TempDir()
	launcher := strings.ReplaceAll(osUpdateLauncher(7, "echo @@upgradable 2\nexit 3\n"), osUpdateDir, dir)
	out, err := exec.Command("sh", "-c", launcher).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "@@started") {
		t.Fatalf("launcher: %v %s", err, out)
	}
	exitFile := filepath.Join(dir, "run-7.exit")
	for i := 0; i < 50; i++ {
		if b, err := os.ReadFile(exitFile); err == nil && len(b) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	code, _ := os.ReadFile(exitFile)
	logText, _ := os.ReadFile(filepath.Join(dir, "run-7.log"))
	if r := ParseOSUpdateLog(string(logText)); strings.TrimSpace(string(code)) != "3" || r.Upgradable == nil || *r.Upgradable != 2 {
		t.Fatalf("exit=%q log=%q", code, logText)
	}
}
