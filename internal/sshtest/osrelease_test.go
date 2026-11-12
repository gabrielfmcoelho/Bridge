package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseHost is a fake root (os-release, apt sources / apk repositories)
// plus a PATH with the given stubs; df reports freeKB free on /.
type releaseHost struct {
	t    *testing.T
	root string
	path string
}

func newReleaseHost(t *testing.T, osRelease string, freeKB string, stubs map[string]string) *releaseHost {
	t.Helper()
	root := t.TempDir()
	h := &releaseHost{t: t, root: root}
	h.write("etc/os-release", osRelease)
	all := map[string]string{"df": "printf 'Filesystem 1024-blocks Used Available Capacity Mounted\\n/dev/sda1 100 50 " + freeKB + " 50%% /\\n'"}
	for k, v := range stubs {
		all[k] = v
	}
	h.path = fakeHost(t, all)
	return h
}

func (h *releaseHost) write(rel, body string) {
	h.t.Helper()
	p := filepath.Join(h.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *releaseHost) read(rel string) string {
	b, _ := os.ReadFile(filepath.Join(h.root, rel))
	return string(b)
}

// run executes the release script; the exit code is part of the result.
func (h *releaseHost) run() (OSUpdateReport, string, error) {
	script, err := OSUpdateInnerScript(OSUpdateRelease)
	if err != nil {
		h.t.Fatal(err)
	}
	return h.runScript(script)
}

func (h *releaseHost) runScript(script string) (OSUpdateReport, string, error) {
	cmd := exec.Command(filepath.Join(h.path, "sh"), "-c", script)
	cmd.Env = []string{"PATH=" + h.path, "BRIDGE_ROOT=" + h.root, "LOG=" + filepath.Join(h.root, "calls")}
	out, err := cmd.CombinedOutput()
	return ParseOSUpdateLog(string(out)), string(out), err
}

const bookworm = "ID=debian\nVERSION_ID=\"12\"\nVERSION_CODENAME=bookworm\n"

// apt-get records every call; "update" fails while a source names failOn.
func aptGetStub(failOn string) string {
	fail := ""
	if failOn != "" {
		fail = `grep -rq ` + failOn + ` "$BRIDGE_ROOT/etc/apt" && exit 100; `
	}
	return `echo "apt-get $*" >> "$LOG"; case "$*" in *update*) ` + fail + `exit 0 ;; esac`
}

func TestReleaseScript_Debian(t *testing.T) {
	h := newReleaseHost(t, bookworm, "9000000", map[string]string{"apt-get": aptGetStub(""), "apt": "exit 0"})
	h.write("etc/apt/sources.list", "deb http://deb.debian.org/debian bookworm main\ndeb http://security.debian.org/debian-security bookworm-security main\n")
	h.write("etc/apt/sources.list.d/extra.sources", "Types: deb\nSuites: bookworm bookworm-updates\n")

	r, out, err := h.run()
	if err != nil || r.Skip != "" || r.Error != "" || !r.RebootRequired {
		t.Fatalf("debian: err=%v report=%+v\n%s", err, r, out)
	}
	if got := h.read("etc/apt/sources.list"); strings.Contains(got, "bookworm") || !strings.Contains(got, "trixie-security") {
		t.Fatalf("sources.list = %q", got)
	}
	if got := h.read("etc/apt/sources.list.d/extra.sources"); got != "Types: deb\nSuites: trixie trixie-updates\n" {
		t.Fatalf("deb822 sources = %q", got)
	}
	if got := h.read("etc/apt.bridge-backup-bookworm/sources.list"); !strings.Contains(got, "bookworm main") {
		t.Fatalf("backup = %q", got)
	}
	calls := h.read("calls")
	if !strings.Contains(calls, "upgrade --without-new-pkgs") || !strings.Contains(calls, "full-upgrade") || !strings.Contains(out, "@@release_from debian 12") {
		t.Fatalf("calls = %q\n%s", calls, out)
	}
}

// Buster's security suite was "buster/updates"; from bullseye on it's "-security".
func TestReleaseScript_DebianBusterSecuritySuite(t *testing.T) {
	h := newReleaseHost(t, "ID=debian\nVERSION_ID=\"10\"\nVERSION_CODENAME=buster\n", "9000000", map[string]string{"apt-get": aptGetStub(""), "apt": "exit 0"})
	h.write("etc/apt/sources.list", "deb http://security.debian.org/debian-security buster/updates main\n")
	if _, out, err := h.run(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := h.read("etc/apt/sources.list"); got != "deb http://security.debian.org/debian-security bullseye-security main\n" {
		t.Fatalf("sources.list = %q", got)
	}
}

// The new release's lists can't be fetched: sources restored, nothing upgraded.
func TestReleaseScript_DebianRestoresSources(t *testing.T) {
	h := newReleaseHost(t, bookworm, "9000000", map[string]string{"apt-get": aptGetStub("trixie"), "apt": "exit 0"})
	h.write("etc/apt/sources.list", "deb http://deb.debian.org/debian bookworm main\n")
	r, out, err := h.run()
	if err == nil || !strings.Contains(r.Error, "sources restored") || r.RebootRequired {
		t.Fatalf("err=%v report=%+v\n%s", err, r, out)
	}
	if got := h.read("etc/apt/sources.list"); got != "deb http://deb.debian.org/debian bookworm main\n" {
		t.Fatalf("sources not restored: %q", got)
	}
	if strings.Contains(h.read("calls"), "full-upgrade") {
		t.Fatal("must not upgrade after a failed update")
	}
}

func TestReleaseScript_Alpine(t *testing.T) {
	h := newReleaseHost(t, "ID=alpine\nVERSION_ID=3.19.1\n", "9000000", map[string]string{"apk": `echo "apk $*" >> "$LOG"`})
	h.write("etc/apk/repositories", "https://dl-cdn.alpinelinux.org/alpine/v3.19/main\nhttps://dl-cdn.alpinelinux.org/alpine/v3.19/community\n")
	r, out, err := h.run()
	if err != nil || r.Error != "" || r.Skip != "" || !r.RebootRequired {
		t.Fatalf("alpine: %v %+v\n%s", err, r, out)
	}
	if got := h.read("etc/apk/repositories"); strings.Contains(got, "v3.19") || strings.Count(got, "/v3.20/") != 2 {
		t.Fatalf("repositories = %q", got)
	}
	if !strings.Contains(h.read("calls"), "apk upgrade --available") {
		t.Fatalf("calls = %q", h.read("calls"))
	}
}

func TestReleaseScript_Ubuntu(t *testing.T) {
	h := newReleaseHost(t, "ID=ubuntu\nVERSION_ID=\"22.04\"\nVERSION_CODENAME=jammy\n", "9000000", map[string]string{
		"apt-get": aptGetStub(""), "apt": "exit 0",
		"do-release-upgrade": `echo "do-release-upgrade $*" >> "$LOG"`,
	})
	h.write("etc/update-manager/release-upgrades", "[DEFAULT]\nPrompt=normal\n")
	r, out, err := h.run()
	if err != nil || r.Skip != "" || !r.RebootRequired {
		t.Fatalf("ubuntu: %v %+v\n%s", err, r, out)
	}
	if !strings.Contains(h.read("calls"), "do-release-upgrade -f DistUpgradeViewNonInteractive") || !strings.Contains(h.read("etc/update-manager/release-upgrades"), "Prompt=lts") {
		t.Fatalf("calls = %q", h.read("calls"))
	}
}

// Every refusal skips without touching anything and without a reboot.
func TestReleaseScript_Skips(t *testing.T) {
	pending := `case "$*" in *-s*) printf 'Inst curl [0] (1 jammy-updates [amd64])\n' ;; esac`
	cases := []struct {
		name, osRelease, free, want string
		stubs                       map[string]string
		reboot                      bool
	}{
		{"non-LTS", "ID=ubuntu\nVERSION_ID=\"23.10\"\n", "9000000", "only LTS", nil, false},
		{"odd year", "ID=ubuntu\nVERSION_ID=\"23.04\"\n", "9000000", "only LTS", nil, false},
		{"no upgrader", "ID=ubuntu\nVERSION_ID=\"22.04\"\n", "9000000", "not installed", nil, false},
		{"no newer LTS", "ID=ubuntu\nVERSION_ID=\"24.04\"\n", "9000000", "no newer LTS", map[string]string{"apt-get": "exit 0", "apt": "exit 0", "do-release-upgrade": "exit 1"}, false},
		{"pending packages", bookworm, "9000000", "1 packages waiting", map[string]string{"apt-get": pending}, false},
		{"low disk", bookworm, "1000", "5 GB", nil, false},
		{"reboot pending", bookworm, "9000000", "reboot is pending", nil, true},
		{"unknown debian", "ID=debian\nVERSION_ID=\"13\"\nVERSION_CODENAME=trixie\n", "9000000", "no supported next release", nil, false},
		{"rocky", "ID=rocky\nVERSION_ID=\"9.3\"\n", "9000000", "RHEL family", nil, false},
		{"arch", "ID=arch\n", "9000000", "not supported", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newReleaseHost(t, c.osRelease, c.free, c.stubs)
			if c.reboot {
				h.write("var/run/reboot-required", "")
			}
			r, out, err := h.run()
			if err != nil || !strings.Contains(r.Skip, c.want) || r.RebootRequired {
				t.Fatalf("err=%v report=%+v\n%s", err, r, out)
			}
		})
	}
}

// The check-only pass changes nothing and ends with @@ready. A package that
// "apt list" still shows but an update wouldn't install (Ubuntu's phased
// updates) doesn't count as waiting.
func TestReleasePrecheck(t *testing.T) {
	const sources = "deb mirror bookworm main\n"
	listed := `case "$1" in list) printf 'curl/jammy-updates 1 amd64 [upgradable from: 0]\n' ;; esac`
	h := newReleaseHost(t, bookworm, "9000000", map[string]string{"apt-get": aptGetStub(""), "apt": listed})
	h.write("etc/apt/sources.list", sources)
	r, out, err := h.runScript(ReleasePrecheck(false))
	if err != nil || !r.Ready || r.Skip != "" || r.RebootRequired {
		t.Fatalf("precheck: %v %+v\n%s", err, r, out)
	}
	if h.read("etc/apt/sources.list") != sources || h.read("etc/apt.bridge-backup-bookworm/sources.list") != "" {
		t.Fatal("the check changed the host")
	}
	if calls := h.read("calls"); strings.Contains(calls, "--without-new-pkgs") || strings.Contains(calls, "full-upgrade") {
		t.Fatalf("calls = %q", calls)
	}

	// Waiting packages still refuse, before anything is backed up.
	pending := `echo "apt-get $*" >> "$LOG"; case "$*" in *-s*) printf 'Inst curl [0] (1 x [amd64])\n' ;; esac`
	h = newReleaseHost(t, bookworm, "9000000", map[string]string{"apt-get": pending})
	if r, out, _ := h.runScript(ReleasePrecheck(false)); r.Ready || !strings.Contains(r.Skip, "packages waiting") {
		t.Fatalf("pending: %+v\n%s", r, out)
	}

	// Updating first: waiting packages and a pending reboot are the update's
	// job, so the check doesn't even refresh the lists.
	h = newReleaseHost(t, bookworm, "9000000", map[string]string{"apt-get": pending})
	h.write("var/run/reboot-required", "")
	if r, out, err := h.runScript(ReleasePrecheck(true)); err != nil || !r.Ready || r.Skip != "" {
		t.Fatalf("update first: %v %+v\n%s", err, r, out)
	}
	if calls := h.read("calls"); calls != "" {
		t.Fatalf("update first called apt-get: %q", calls)
	}

	// The other refusals stay.
	h = newReleaseHost(t, "ID=debian\nVERSION_ID=\"13\"\nVERSION_CODENAME=trixie\n", "9000000", nil)
	if r, _, _ := h.runScript(ReleasePrecheck(true)); r.Ready || !strings.Contains(r.Skip, "no supported next release") {
		t.Fatalf("trixie: %+v", r)
	}
}
