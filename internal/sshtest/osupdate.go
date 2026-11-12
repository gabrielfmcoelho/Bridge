package sshtest

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// OS updates (os_update_runs): one script, run as root, that finds the
// package manager itself and reports through "@@key value" lines in its log.
//
// It runs detached on the host (setsid, log and exit code in
// /var/lib/bridge-osupdate): a dropped SSH session, a Bridge restart or the
// proxy's 2-minute cut must never kill apt halfway through dpkg. Bridge only
// launches it, then reads the exit file and the log tail on fresh sessions.

// OS update kinds.
const (
	OSUpdateCheck    = "check"    // refresh the package lists, count what's upgradable
	OSUpdatePackages = "packages" // every upgrade inside the current release
	OSUpdateSecurity = "security" // security upgrades only
	OSUpdateRelease  = "release"  // next release (osReleaseScript)
	OSUpdateRestore  = "restore"  // a kept backup back onto its host (osrestore.go)
)

// osUpdateDir holds each run's script, log and exit code on the host.
const osUpdateDir = "/var/lib/bridge-osupdate"

// osUpdateScript is the root script for one kind. Markers:
//
//	@@pm apt|dnf|yum|apk              package manager found
//	@@upgradable N / @@security N     before the upgrade
//	@@remaining N / @@remaining_security N   after an upgrade (both kinds)
//	@@reboot yes                      a reboot is pending
//	@@skip <reason>                   nothing was done, on purpose
//	@@error <reason>                  it failed (exit code non-zero too)
//
// Security counts on apt are the upgradable packages from a *-security
// suite; on dnf/yum, the packages named by a security advisory.
const osUpdateScript = `MODE='%s'
export DEBIAN_FRONTEND=noninteractive
say() { echo "@@$*"; }
reboot_flag() {
  if [ -f /var/run/reboot-required ]; then say reboot yes
  elif command -v needs-restarting >/dev/null 2>&1; then needs-restarting -r >/dev/null 2>&1; [ $? -eq 1 ] && say reboot yes
  fi
  return 0
}
APTOPTS='-y -q -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold'
apt_up() { apt list --upgradable 2>/dev/null | grep '\[upgradable'; }
if command -v apt-get >/dev/null 2>&1; then
  say pm apt
  apt-get update -q || { say error apt-get update failed; exit 1; }
  say upgradable "$(apt_up | grep -c .)"
  say security "$(apt_up | grep -c -- '-security')"
  case "$MODE" in
  packages)
    apt-get $APTOPTS upgrade --with-new-pkgs || { say error apt-get upgrade failed; exit 1; }
    say remaining "$(apt_up | grep -c .)"
    say remaining_security "$(apt_up | grep -c -- '-security')" ;;
  security)
    PKGS=$(apt_up | grep -- '-security' | cut -d/ -f1)
    if [ -n "$PKGS" ]; then
      apt-get $APTOPTS install --only-upgrade $PKGS || { say error apt-get install --only-upgrade failed; exit 1; }
    fi
    say remaining "$(apt_up | grep -c .)"
    say remaining_security "$(apt_up | grep -c -- '-security')" ;;
  esac
elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then
  PM=dnf; command -v dnf >/dev/null 2>&1 || PM=yum
  say pm $PM
  up() { $PM -q check-update 2>/dev/null | awk 'NF==3 && $1 ~ /\./ && $2 ~ /[0-9]/' | wc -l; }
  sec() { $PM -q updateinfo list --security 2>/dev/null | awk 'NF>=3 {print $3}' | sort -u | wc -l; }
  $PM -q makecache >/dev/null 2>&1 || { say error $PM makecache failed; exit 1; }
  say upgradable "$(up)"
  say security "$(sec)"
  case "$MODE" in
  packages)
    $PM -y upgrade || { say error $PM upgrade failed; exit 1; }
    say remaining "$(up)"
    say remaining_security "$(sec)" ;;
  security)
    $PM -y upgrade --security || { say error $PM upgrade --security failed; exit 1; }
    say remaining "$(up)"
    say remaining_security "$(sec)" ;;
  esac
elif command -v apk >/dev/null 2>&1; then
  say pm apk
  apk update -q || { say error apk update failed; exit 1; }
  say upgradable "$(apk version -l '<' 2>/dev/null | grep -c '<')"
  case "$MODE" in
  packages)
    apk upgrade -q || { say error apk upgrade failed; exit 1; }
    say remaining "$(apk version -l '<' 2>/dev/null | grep -c '<')" ;;
  security)
    say skip "Alpine has no separate security channel: use a package update" ;;
  esac
else
  say skip "no supported package manager (apt, dnf, yum, apk)"
fi
reboot_flag
exit 0
`

// osReleaseScript moves the host to its next release, as root. Supported:
// Ubuntu LTS → next LTS (do-release-upgrade), Debian N → N+1 (codename
// rewritten in the apt sources, then full-upgrade) and Alpine 3.x → 3.x+1
// (repositories bumped, apk upgrade --available). It skips, changing
// nothing, when a reboot is pending, / has under 5 GB free, packages are
// still waiting for a package update, or the path isn't supported (RHEL
// family: Leapp/ELevate). Debian / Alpine sources are backed up first and
// restored when the new release's lists can't be fetched; once the upgrade
// itself has started nothing is rolled back (a half-restored system is
// worse). It always ends with "@@reboot yes": the release finishes on boot.
// BRIDGE_ROOT prefixes the files it reads and writes (tests only).
const osReleaseScript = `R="${BRIDGE_ROOT:-}"
export DEBIAN_FRONTEND=noninteractive
say() { echo "@@$*"; }
APTOPTS='-y -q -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold'
osr() ( . "$R/etc/os-release" 2>/dev/null; echo "$ID $VERSION_ID" )
. "$R/etc/os-release" 2>/dev/null || { say skip "no /etc/os-release"; exit 0; }
say release_from "$(osr)"
if [ -f "$R/var/run/reboot-required" ]; then say skip "a reboot is pending: reboot the host first"; exit 0; fi
FREE=$(df -Pk / 2>/dev/null | awk 'NR==2 {print $4}')
if [ "${FREE:-0}" -lt 5242880 ]; then say skip "less than 5 GB free on /"; exit 0; fi
apt_pending() {
  apt-get update -q || { say error apt-get update failed; exit 1; }
  N=$(apt list --upgradable 2>/dev/null | grep -c '\[upgradable')
  if [ "$N" -gt 0 ]; then say skip "$N packages waiting: run a package update first"; exit 0; fi
}
case "$ID" in
ubuntu)
  MAJ=${VERSION_ID%%.*}
  if [ "${VERSION_ID#*.}" != 04 ] || [ $((MAJ % 2)) -ne 0 ]; then say skip "only LTS to LTS upgrades are supported"; exit 0; fi
  command -v do-release-upgrade >/dev/null 2>&1 || { say skip "do-release-upgrade is not installed (ubuntu-release-upgrader-core)"; exit 0; }
  apt_pending
  [ -f "$R/etc/update-manager/release-upgrades" ] && sed -i 's/^Prompt=.*/Prompt=lts/' "$R/etc/update-manager/release-upgrades"
  do-release-upgrade -c >/dev/null 2>&1 || { say skip "no newer LTS release is available yet"; exit 0; }
  do-release-upgrade -f DistUpgradeViewNonInteractive || { say error do-release-upgrade failed; exit 1; } ;;
debian)
  case "$VERSION_CODENAME" in
  buster) NEXT=bullseye ;; bullseye) NEXT=bookworm ;; bookworm) NEXT=trixie ;;
  *) say skip "no supported next release for Debian ${VERSION_CODENAME:-$VERSION_ID}"; exit 0 ;;
  esac
  apt_pending
  B="$R/etc/apt.bridge-backup-$VERSION_CODENAME"
  rm -rf "$B" && cp -a "$R/etc/apt" "$B" || { say error could not back up /etc/apt; exit 1; }
  say backup "$B"
  for f in "$R/etc/apt/sources.list" "$R"/etc/apt/sources.list.d/*.list "$R"/etc/apt/sources.list.d/*.sources; do
    [ -f "$f" ] && sed -i -e "s/\b$VERSION_CODENAME\b/$NEXT/g" -e "s#$NEXT/updates#$NEXT-security#g" "$f"
  done
  apt-get update -q || { rm -rf "$R/etc/apt" && cp -a "$B" "$R/etc/apt"; say error "apt-get update failed on $NEXT: sources restored"; exit 1; }
  apt-get $APTOPTS upgrade --without-new-pkgs || { say error "apt-get upgrade failed halfway to $NEXT: fix it on the host (backup in $B)"; exit 1; }
  apt-get $APTOPTS full-upgrade || { say error "apt-get full-upgrade failed halfway to $NEXT: fix it on the host (backup in $B)"; exit 1; } ;;
alpine)
  CUR=$(echo "$VERSION_ID" | cut -d. -f1,2); MIN=${CUR#*.}; NEXT="${CUR%%.*}.$((MIN + 1))"
  REPOS="$R/etc/apk/repositories"
  grep -q "/v$CUR/" "$REPOS" 2>/dev/null || { say skip "the repositories don't pin v$CUR (edge?)"; exit 0; }
  cp "$REPOS" "$REPOS.bridge-backup" || { say error could not back up the repositories; exit 1; }
  say backup "$REPOS.bridge-backup"
  sed -i "s#/v$CUR/#/v$NEXT/#g" "$REPOS"
  apk update -q || { cp "$REPOS.bridge-backup" "$REPOS"; say error "apk update failed on v$NEXT (not released yet?): repositories restored"; exit 1; }
  apk upgrade --available -q || { say error "apk upgrade failed halfway to v$NEXT: fix it on the host"; exit 1; } ;;
rhel|rocky|almalinux|centos|ol|fedora)
  say skip "major upgrades of the RHEL family (Leapp / ELevate) are not supported"; exit 0 ;;
*)
  say skip "release upgrades of $ID are not supported"; exit 0 ;;
esac
say release_to "$(osr)"
say reboot yes
exit 0
`

// OSUpdateInnerScript returns the root script for kind, or an error for an
// unknown kind.
func OSUpdateInnerScript(kind string) (string, error) {
	switch kind {
	case OSUpdateCheck, OSUpdatePackages, OSUpdateSecurity:
		return fmt.Sprintf(osUpdateScript, kind), nil
	case OSUpdateRelease:
		return osReleaseScript, nil
	}
	return "", fmt.Errorf("unknown OS update kind %q", kind)
}

// osUpdateFile is the run's path prefix on the host (.sh, .log, .exit). The
// script is root-only (a restore's carries a download link to the backup);
// the log and exit code are world-readable so polling needs no sudo, and
// never contain the link.
func osUpdateFile(runID int64) string {
	return fmt.Sprintf("%s/run-%d", osUpdateDir, runID)
}

// osUpdateLauncher writes inner to the host and starts it detached. It runs
// as root and returns at once ("@@started").
func osUpdateLauncher(runID int64, inner string) string {
	f := osUpdateFile(runID)
	b64 := base64.StdEncoding.EncodeToString([]byte(inner))
	return fmt.Sprintf(`umask 022
mkdir -p %[1]s && chmod 755 %[1]s || exit 1
(umask 077; echo %[3]s | base64 -d > %[2]s.sh) || exit 1
rm -f %[2]s.log %[2]s.exit
RUN="sh %[2]s.sh > %[2]s.log 2>&1; echo \$? > %[2]s.exit"
if command -v setsid >/dev/null 2>&1; then
  setsid sh -c "$RUN" < /dev/null > /dev/null 2>&1 &
else
  nohup sh -c "$RUN" < /dev/null > /dev/null 2>&1 &
fi
echo @@started
`, osUpdateDir, f, b64)
}

// Privilege is how root is reached on a host: already root, sudo with the
// stored password, or passwordless sudo.
type Privilege struct {
	User     string
	Password string
}

// RunAsRoot runs script as root on client: directly for root, through
// sudo -S with the stored password, else through sudo -n (passwordless
// sudo). Errors when none of them works.
func RunAsRoot(client *ssh.Client, p Privilege, script string) (string, error) {
	switch {
	case p.User == "root":
		return RunScript(client, script)
	case p.Password != "":
		return RunPrivilegedScript(client, p.Password, script)
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(script))
	out, err := runCmdRaw(client, fmt.Sprintf("echo %s | base64 -d | sudo -n sh", b64))
	if err != nil && strings.Contains(out+err.Error(), "password is required") {
		return out, fmt.Errorf("sudo needs a password: store the host's password or enable passwordless sudo")
	}
	return out, err
}

// StartOSUpdate launches kind as run runID on the host, detached.
func StartOSUpdate(client *ssh.Client, p Privilege, runID int64, kind string) error {
	inner, err := OSUpdateInnerScript(kind)
	if err != nil {
		return err
	}
	return StartDetached(client, p, runID, inner)
}

// StartDetached launches a root script as run runID on the host, detached.
func StartDetached(client *ssh.Client, p Privilege, runID int64, inner string) error {
	out, err := RunAsRoot(client, p, osUpdateLauncher(runID, inner))
	if err != nil {
		return err
	}
	if !strings.Contains(out, "@@started") {
		return fmt.Errorf("launcher did not start: %s", out)
	}
	return nil
}

// osUpdateLogTail caps the log Bridge reads (and stores) per poll.
const osUpdateLogTail = 64 << 10

// PollOSUpdate reads run runID's state on the host as the SSH user (the
// files are world-readable): done once the exit file exists.
func PollOSUpdate(client *ssh.Client, runID int64) (done bool, exitCode int, logTail string, err error) {
	f := osUpdateFile(runID)
	out, err := runCmdWithDeadline(client, fmt.Sprintf(
		`if [ -f %[1]s.exit ]; then echo "@@exit $(cat %[1]s.exit)"; fi; [ -f %[1]s.log ] || echo "@@nolog"; tail -c %[2]d %[1]s.log 2>/dev/null`,
		f, osUpdateLogTail), 30*time.Second)
	if err != nil {
		return false, 0, "", err
	}
	first, rest, _ := strings.Cut(out, "\n")
	if code, ok := strings.CutPrefix(strings.TrimSpace(first), "@@exit "); ok {
		n, convErr := strconv.Atoi(strings.TrimSpace(code))
		if convErr != nil {
			n = -1
		}
		return true, n, rest, nil
	}
	if strings.Contains(out, "@@nolog") {
		return false, 0, "", fmt.Errorf("run log not found on the host")
	}
	return false, 0, out, nil
}

// OSUpdateReport is what the script's markers say.
type OSUpdateReport struct {
	PkgManager        string
	Upgradable        *int
	Security          *int
	Remaining         *int
	RemainingSecurity *int
	RebootRequired    bool
	Skip              string
	Error             string
}

// ParseOSUpdateLog reads the "@@key value" markers of a run's log; the last
// one of each key wins.
func ParseOSUpdateLog(logText string) OSUpdateReport {
	var r OSUpdateReport
	num := func(v string) *int {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil
		}
		return &n
	}
	for _, line := range strings.Split(logText, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "@@")
		if !ok {
			continue
		}
		key, val, _ := strings.Cut(rest, " ")
		val = strings.TrimSpace(val)
		switch key {
		case "pm":
			r.PkgManager = val
		case "upgradable":
			r.Upgradable = num(val)
		case "security":
			r.Security = num(val)
		case "remaining":
			r.Remaining = num(val)
		case "remaining_security":
			r.RemainingSecurity = num(val)
		case "reboot":
			r.RebootRequired = val == "yes"
		case "skip":
			r.Skip = val
		case "error":
			r.Error = val
		}
	}
	return r
}

// Reboot schedules a reboot in one minute (so this session ends cleanly).
func Reboot(client *ssh.Client, p Privilege) error {
	_, err := RunAsRoot(client, p, `shutdown -r +1 "Bridge: reboot after OS update" 2>&1 || { (sleep 60; reboot) >/dev/null 2>&1 & }`)
	return err
}
