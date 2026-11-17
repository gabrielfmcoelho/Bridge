package sshtest

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// repairStubs is SAPASP-CONCURSO after its failed release: dpkg fails on
// proftpd-core once (its postinst), then configures; apt-get check fails
// until apt-get -f install ran; fail2ban, logrotate and a user@ unit failed
// (one line with systemd's bullet); kernel -100 installed while -45 runs.
func repairStubs() map[string]string {
	return map[string]string{
		"dpkg": `echo "dpkg $*" >> "$LOG"
case "$*" in
--audit) [ -f "$BRIDGE_ROOT/configured" ] || printf 'The following packages are only half configured:\n proftpd-core   Versatile FTP server\n' ;;
*--configure*)
  if [ ! -f "$BRIDGE_ROOT/failed-once" ]; then
    touch "$BRIDGE_ROOT/failed-once"
    printf 'dpkg: error processing package proftpd-core (--configure):\n installed proftpd-core package post-installation script subprocess returned error exit status 1\n' >> "$BRIDGE_ROOT/var/log/apt/term.log"
    exit 1
  fi
  touch "$BRIDGE_ROOT/configured" ;;
esac`,
		"apt-get": `echo "apt-get $*" >> "$LOG"
case "$*" in check) [ -f "$BRIDGE_ROOT/fixed" ] || exit 100 ;; *"-f install"*) touch "$BRIDGE_ROOT/fixed" ;; esac`,
		"systemctl": `echo "systemctl $*" >> "$LOG"
case "$1" in
--failed) printf 'fail2ban.service loaded failed failed Fail2Ban Service\n\342\227\217 logrotate.service loaded failed failed Rotate log files\nuser@1000.service loaded failed failed User Manager for UID 1000\n' ;;
restart) [ "$2" = fail2ban.service ] && exit 1 ;;
is-active) [ "$2" = fail2ban.service ] && { echo active; exit 0; }; echo failed; exit 3 ;;
esac`,
		"sleep": "exit 0",
		"uname": "echo 6.8.0-45-generic",
	}
}

// awks are the awk implementations the scripts must run under: the host's
// own, plus mawk (Debian's default) and busybox's when installed.
func awks(t *testing.T) map[string]string {
	out := map[string]string{"default": ""}
	if p, err := exec.LookPath("mawk"); err == nil {
		out["mawk"] = `exec ` + p + ` "$@"`
	}
	if p, err := exec.LookPath("busybox"); err == nil {
		out["busybox"] = `exec ` + p + ` awk "$@"`
	}
	return out
}

func TestRepairScript(t *testing.T) {
	for name, awk := range awks(t) {
		t.Run(name, func(t *testing.T) {
			stubs := repairStubs()
			if awk != "" {
				stubs["awk"] = awk
			}
			h := newReleaseHost(t, "ID=ubuntu\nVERSION_ID=\"22.04\"\n", "9000000", stubs)
			h.write("var/log/apt/term.log", "")
			h.write("etc/proftpd/proftpd.conf", "ServerName x\nIdentLookups off\nsecret=hunter1\n")
			h.write("etc/proftpd/proftpd.conf.ucf-dist", "ServerName x\nIdentLookups off\nsecret=hunter2\n")
			h.write("etc/fail2ban/jail.conf.dpkg-dist", "[DEFAULT]\nbantime = 10m\n")
			h.write("etc/proftpd/modules.conf.dpkg-old", "kept, not listed\n")
			h.write("boot/vmlinuz-6.8.0-45-generic", "")
			h.write("boot/vmlinuz-6.8.0-100-generic", "")
			script, err := OSUpdateInnerScript(OSUpdateRepair)
			if err != nil {
				t.Fatal(err)
			}

			// First run: dpkg fails on proftpd's postinst. It says why and stops
			// before apt, the dist files and the units.
			r, out, err := h.runScript(script)
			if err == nil || r.Error != "dpkg --configure -a failed" || r.PkgManager != "apt" {
				t.Fatalf("first run: err=%v report=%+v\n%s", err, r, out)
			}
			if strings.Join(r.DpkgErrors, "|") != "proftpd-core: installed proftpd-core package post-installation script subprocess returned error exit status 1" ||
				strings.Join(r.DpkgBroken, " ") != "proftpd-core" {
				t.Fatalf("first run: errors=%q broken=%q\n%s", r.DpkgErrors, r.DpkgBroken, out)
			}
			if calls := h.read("calls"); strings.Contains(calls, "apt-get") || strings.Contains(calls, "restart") || len(r.DistFiles) > 0 {
				t.Fatalf("first run went on after dpkg failed: %q\n%s", calls, out)
			}
			if !strings.Contains(h.read("calls"), "dpkg --force-confdef --force-confold --configure -a") {
				t.Fatalf("dpkg args: %q", h.read("calls"))
			}

			// Second run: dpkg configures, apt fixes the dependencies, the
			// dist files are listed with a count only, failed units restart.
			r, out, err = h.runScript(script)
			if err != nil || r.Error != "" || len(r.DpkgErrors) > 0 || !r.RebootRequired {
				t.Fatalf("second run: err=%v report=%+v\n%s", err, r, out)
			}
			calls := h.read("calls")
			if !strings.Contains(calls, "apt-get check") || !strings.Contains(calls, "--force-confold -f install") {
				t.Fatalf("apt calls: %q", calls)
			}
			if got := fmt.Sprint(r.DistFiles); got != "[{/etc/fail2ban/jail.conf.dpkg-dist 2} {/etc/proftpd/proftpd.conf.ucf-dist 2}]" {
				t.Fatalf("dist files = %s\n%s", got, out)
			}
			if strings.Contains(out, "hunter") || strings.Contains(out, "bantime") {
				t.Fatalf("a config file's contents left the host:\n%s", out)
			}
			// Judged by is-active: fail2ban's restart exited 1 but it runs;
			// logrotate's exited 0 but it doesn't. user@ waits for the reboot.
			if got := fmt.Sprint(r.UnitRestarts); got != "[{fail2ban.service true} {logrotate.service false}]" {
				t.Fatalf("unit restarts = %s\n%s", got, out)
			}
			if strings.Contains(calls, "user@") || !strings.Contains(calls, "systemctl reset-failed logrotate.service") {
				t.Fatalf("systemctl calls: %q", calls)
			}
		})
	}
}

// Nothing to fix on a host without dpkg: no apt steps, no unit restarted,
// and no reboot while the running kernel is the newest (or isn't in /boot).
// A pending reboot is still said.
func TestRepairScript_NothingToDo(t *testing.T) {
	script, _ := OSUpdateInnerScript(OSUpdateRepair)
	h := newReleaseHost(t, "ID=rocky\n", "9000000", map[string]string{
		"systemctl": `echo "systemctl $*" >> "$LOG"`, "uname": "echo 5.14.0-427.el9.x86_64",
	})
	h.write("boot/vmlinuz-5.14.0-427.el9.x86_64", "")
	h.write("boot/vmlinuz-0-rescue-abc", "")
	r, out, err := h.runScript(script)
	if err != nil || r.PkgManager != "" || r.RebootRequired || len(r.UnitRestarts) > 0 || strings.Contains(h.read("calls"), "restart") {
		t.Fatalf("err=%v report=%+v\n%s", err, r, out)
	}

	h = newReleaseHost(t, "ID=alpine\n", "9000000", map[string]string{"uname": "echo 6.6.31-0-lts"})
	h.write("boot/vmlinuz-lts", "")
	if r, out, _ := h.runScript(script); r.RebootRequired {
		t.Fatalf("alpine's vmlinuz-lts is not a newer kernel: %+v\n%s", r, out)
	}
	h.write("var/run/reboot-required", "")
	if r, out, _ := h.runScript(script); !r.RebootRequired {
		t.Fatalf("pending reboot not said: %+v\n%s", r, out)
	}
}

func TestParseOSUpdateLog_Repair(t *testing.T) {
	r := ParseOSUpdateLog("@@unit_restart fail2ban.service ok\n@@unit_restart bad;unit ok\n@@unit_restart logrotate.service failed\n" +
		"@@dist_file /etc/a b.conf.ucf-dist 3\n@@dist_file relative.dpkg-dist 1\n@@dist_file /etc/x.dpkg-dist many\n")
	if got := fmt.Sprint(r.UnitRestarts, r.DistFiles); got != "[{fail2ban.service true} {logrotate.service false}] [{/etc/a b.conf.ucf-dist 3}]" {
		t.Fatalf("parsed = %s", got)
	}
}
