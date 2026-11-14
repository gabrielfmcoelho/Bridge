package sshtest

import (
	"encoding/base64"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
)

// Deep facts (VMInfo.DeepFacts): non-secret facts that enrich a scan but cost
// more than the base probes, so they run only in a background scan (CaptureJob)
// and stay nil in the synchronous test (CaptureSync). Today:
//   - A3 OutboundFlows: where this VM sends data — the destinations in the
//     scripts cron calls (rsync/scp/ssh/curl/wget/lftp to a remote host, any
//     password stripped) and its established outbound TCP peers.
//   - A4 PatchFreshness: whether patching is broken (no successful index update
//     in weeks although auto-updates are on) or just outdated.
// Only names, hosts, ports and dates leave the VM — never a script line, a
// credential, or a client's address.
type DeepFacts struct {
	Outbound *OutboundFlows  `json:"outbound,omitempty"`
	Patch    *PatchFreshness `json:"patch,omitempty"`
}

// OutboundFlows is where the VM pushes data off-box.
type OutboundFlows struct {
	// Scripts: transfer destinations found in the scripts cron jobs call.
	Scripts []ScriptFlow `json:"scripts,omitempty"`
	// Connections: established outbound TCP peers (not the host's own listeners).
	Connections []OutboundConn `json:"connections,omitempty"`
}

// ScriptFlow is one transfer destination inside a script a cron job runs.
type ScriptFlow struct {
	Script  string `json:"script"`  // the script path the cron job runs
	Program string `json:"program"` // rsync | scp | sftp | ssh | curl | wget | lftp | mysqldump
	Target  string `json:"target"`  // destination host[:path] or URL host, credentials stripped
}

// OutboundConn is one established outbound TCP connection.
type OutboundConn struct {
	Peer    string `json:"peer"`              // remote address
	Port    int    `json:"port,omitempty"`    // remote service port
	Process string `json:"process,omitempty"` // owning process (root only)
	Public  bool   `json:"public,omitempty"`  // peer is a public (non-RFC1918) address
}

// PatchFreshness answers "broken vs outdated" per OS family.
type PatchFreshness struct {
	Manager         string `json:"manager,omitempty"`           // apt | dnf | yum | apk
	LastIndexUpdate int64  `json:"last_index_update,omitempty"` // unix: last successful package-index refresh
	LastUpgrade     int64  `json:"last_upgrade,omitempty"`      // unix: last package-upgrade run
	AutoUpdates     string `json:"auto_updates,omitempty"`      // mechanism: unattended-upgrades | dnf-automatic
	AutoUpdatesOn   bool   `json:"auto_updates_on,omitempty"`   // that mechanism is enabled
	Upgradable      int    `json:"upgradable,omitempty"`        // upgradable package count (offline estimate)
	// Stale: auto-updates are on but the index hasn't refreshed in staleAfter —
	// "patching is broken", not merely outdated.
	Stale bool `json:"stale,omitempty"`
}

const (
	maxFlowScripts = 20      // cron scripts read per scan
	maxFlowTargets = 200     // ScriptFlow rows kept
	maxConns       = 100     // OutboundConn rows kept
	maxTargetLen   = 512     // a destination longer than this is dropped
	staleAfter     = 1209600 // 14 days: index older than this with auto-updates on = broken
)

// deepFactsScript is one root-capable session emitting A4 (@@pm/@@idx/@@upg/
// @@auto/@@upgradable) then A3 (@@script/@@flow, @@conns). scripts are the
// absolute paths of the binaries cron jobs run; each is read only if it is a
// text script (shebang) and capped. BRIDGE_ROOT prefixes system reads (tests).
// Not root: SUDO=1 retries root-only reads through sudo -n.
func deepFactsScript(asRoot bool, scripts []string) string {
	head := "SUDO=1\n"
	if asRoot {
		head = ""
	}
	var sb strings.Builder
	sb.WriteString(head)
	sb.WriteString(`export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
R="${BRIDGE_ROOT:-}"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 10"
S=""; if [ -n "$SUDO" ] && sudo -n true 2>/dev/null; then S="sudo -n"; fi
stampY() { [ -e "$R$1" ] && stat -c %Y "$R$1" 2>/dev/null; }
# ── A4 patch freshness ──
if command -v apt-get >/dev/null 2>&1 || [ -d "$R/etc/apt" ]; then
  echo "@@pm apt"
  s=$(stampY /var/lib/apt/periodic/update-success-stamp); [ -z "$s" ] && s=$(stampY /var/lib/apt/lists)
  [ -n "$s" ] && echo "@@idx $s"
  u=$({ cat "$R/var/log/apt/history.log" "$R/var/log/apt/history.log.1" 2>/dev/null; } | awk '
    /^Commandline:/ { up=($0 ~ /upgrade/) }
    /^End-Date:/ { if (up) last=$2" "$3; up=0 }
    END { if (last!="") print last }')
  [ -n "$u" ] && u=$(date -d "$u" +%s 2>/dev/null) && [ -n "$u" ] && echo "@@upg $u"
  on=0
  cat "$R"/etc/apt/apt.conf.d/*auto-upgrades "$R"/etc/apt/apt.conf.d/*unattended* 2>/dev/null | grep -qE '^[[:space:]]*APT::Periodic::Unattended-Upgrade[[:space:]]+"1"' && on=1
  echo "@@auto unattended-upgrades $on"
  n=$($T apt-get -s -o Debug::NoLocking=true upgrade 2>/dev/null | grep -c '^Inst')
  [ "$n" -gt 0 ] 2>/dev/null && echo "@@upgradable $n"
elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then
  PM=dnf; command -v dnf >/dev/null 2>&1 || PM=yum
  echo "@@pm $PM"
  s=$(stampY /var/lib/dnf/history.sqlite); [ -z "$s" ] && s=$(stampY /var/cache/$PM)
  [ -n "$s" ] && echo "@@idx $s"
  on=0; grep -hs -E '^[[:space:]]*apply_updates[[:space:]]*=[[:space:]]*yes' "$R/etc/dnf/automatic.conf" >/dev/null 2>&1 && on=1
  echo "@@auto dnf-automatic $on"
  n=$($T $PM -q check-update 2>/dev/null | awk 'NF==3 && $1 ~ /\./ && $2 ~ /[0-9]/' | grep -c .)
  [ "$n" -gt 0 ] 2>/dev/null && echo "@@upgradable $n"
elif command -v apk >/dev/null 2>&1; then
  echo "@@pm apk"
  s=$(stampY /var/lib/apk/db/installed); [ -n "$s" ] && echo "@@idx $s"
fi
# ── A3 outbound flows: destinations in the scripts cron calls ──
`)
	// One @@script block per readable cron script, then @@flow lines: the
	// destination tokens (url, user@host[:path], host::module) with any
	// embedded credential stripped in Go. grep -oE pulls only destinations,
	// so a `-psecret` / `PGPASSWORD=` / `sshpass -p x` token is never emitted.
	for _, p := range scripts {
		q := shQuote(p)
		sb.WriteString(`f="$R"` + q + `
if [ -f "$f" ] && [ "$($S head -c2 "$f" 2>/dev/null)" = "#!" ]; then
  echo "@@script $(printf %s ` + q + ` | base64 | tr -d '\n')"
  $S $T head -c 65536 "$f" 2>/dev/null | grep -E '(rsync|scp|sftp|ssh|curl|wget|lftp|mysqldump)' | head -n 50 | while IFS= read -r line; do
    prog=$(printf '%s\n' "$line" | grep -oE '(rsync|scp|sftp|ssh|curl|wget|lftp|mysqldump)' | head -n1)
    printf '%s\n' "$line" | grep -oE '[A-Za-z][A-Za-z0-9+.-]*://[^[:space:]]+|[A-Za-z0-9_.-]+@[^[:space:]]+|[A-Za-z0-9_.-]+::[^[:space:]]+' | head -n 10 | while IFS= read -r tok; do
      printf '@@flow %s %s\n' "$prog" "$(printf %s "$tok" | base64 | tr -d '\n')"
    done
  done
fi
`)
	}
	// Established outbound TCP peers + owning process (root). ss first, netstat
	// fallback; the whole line goes to Go, which keeps only peer/port/proc.
	sb.WriteString(`echo "@@conns"
if command -v ss >/dev/null 2>&1; then
  $S $T ss -tnp state established 2>/dev/null | head -n 500
elif command -v netstat >/dev/null 2>&1; then
  $S $T netstat -tnp 2>/dev/null | grep ESTABLISHED | head -n 500
fi
`)
	return sb.String()
}

// collectDeepFacts runs the probe — root with the stored password (ss -p and
// some history files are root-only), else unprivileged. scripts are the cron
// jobs' binary paths; listen is the host's listening ports (to tell an inbound
// server connection from an outbound one). nil = the probe didn't run.
func collectDeepFacts(client *ssh.Client, sudoPassword string, cron *CronInfo, ports []string) *DeepFacts {
	scripts := cronScriptPaths(cron)
	var raw string
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, deepFactsScript(true, scripts))
		if strings.Contains(out, "@@pm") || strings.Contains(out, "@@conns") {
			raw = out
		} else {
			log.Printf("[sshscan] deep facts as root failed, falling back to unprivileged: %v", err)
		}
	}
	if raw == "" {
		raw = runCmd(client, encodedScript(deepFactsScript(false, scripts), "sh"))
	}
	if !strings.Contains(raw, "@@pm") && !strings.Contains(raw, "@@conns") {
		return nil
	}
	return parseDeepFacts(raw, listenPortSet(ports))
}

// cronScriptPaths is the absolute binary of each cron job, deduped and capped —
// the scripts whose contents A3 reads for transfer destinations.
func cronScriptPaths(cron *CronInfo) []string {
	if cron == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, j := range cron.Jobs {
		b := cronCommandBinary(j.Command)
		if b == "" || seen[b] || len(b) > maxTargetLen {
			continue
		}
		seen[b] = true
		out = append(out, b)
		if len(out) >= maxFlowScripts {
			break
		}
	}
	return out
}

// listenPortSet is the set of the host's listening ports, from info.Ports
// ("<port> <proc>" rows). A connection whose local port is here is inbound.
func listenPortSet(ports []string) map[int]bool {
	set := map[int]bool{}
	for _, line := range ports {
		if f := strings.Fields(line); len(f) > 0 {
			if p, err := strconv.Atoi(f[0]); err == nil {
				set[p] = true
			}
		}
	}
	return set
}

var ssProcRe = regexp.MustCompile(`users:\(\("([^"]+)"`)
var netstatProcRe = regexp.MustCompile(`\s\d+/(\S+)\s*$`)

func parseDeepFacts(raw string, listen map[int]bool) *DeepFacts {
	df := &DeepFacts{}
	patch := &PatchFreshness{}
	out := &OutboundFlows{}
	seenFlow := map[ScriptFlow]bool{}
	seenConn := map[OutboundConn]bool{}
	script := ""
	inConns := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "@@pm "):
			patch.Manager = strings.TrimSpace(line[5:])
		case strings.HasPrefix(line, "@@idx "):
			patch.LastIndexUpdate = parseUnix(line[6:])
		case strings.HasPrefix(line, "@@upg "):
			patch.LastUpgrade = parseUnix(line[6:])
		case strings.HasPrefix(line, "@@auto "):
			f := strings.Fields(line[7:])
			if len(f) == 2 {
				patch.AutoUpdates, patch.AutoUpdatesOn = f[0], f[1] == "1"
			}
		case strings.HasPrefix(line, "@@upgradable "):
			if n, err := strconv.Atoi(strings.TrimSpace(line[13:])); err == nil {
				patch.Upgradable = n
			}
		case strings.HasPrefix(line, "@@script "):
			script = decodeToken(line[9:])
		case strings.HasPrefix(line, "@@flow "):
			if f := parseFlow(line[7:], script); f != nil && !seenFlow[*f] && len(out.Scripts) < maxFlowTargets {
				seenFlow[*f] = true
				out.Scripts = append(out.Scripts, *f)
			}
		case line == "@@conns":
			inConns = true
		case inConns:
			if c := parseConn(line, listen); c != nil && !seenConn[*c] && len(out.Connections) < maxConns {
				seenConn[*c] = true
				out.Connections = append(out.Connections, *c)
			}
		}
	}
	if patch.Manager != "" {
		patch.Stale = patch.AutoUpdatesOn && patch.LastIndexUpdate > 0 && nowUnix()-patch.LastIndexUpdate > staleAfter
		df.Patch = patch
	}
	if len(out.Scripts) > 0 || len(out.Connections) > 0 {
		df.Outbound = out
	}
	if df.Patch == nil && df.Outbound == nil {
		return nil
	}
	return df
}

// parseFlow turns a "@@flow <program> <b64 token>" line into a ScriptFlow,
// stripping any credential from the destination (coolify.SanitizeRepoURL for
// "scheme://user:pass@host", stripSCPPassword for "user:pass@host:path").
func parseFlow(s, script string) *ScriptFlow {
	prog, b64, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok || script == "" {
		return nil
	}
	tok := strings.Trim(decodeToken(b64), `"'()`)
	target := stripSCPPassword(coolify.SanitizeRepoURL(tok))
	if target == "" || len(target) > maxTargetLen || strings.ContainsFunc(target, unicode.IsControl) {
		return nil
	}
	// A destination must still name a host after stripping: reject a bare "@host"
	// or a leftover credential-looking token with no dot/host part.
	if strings.ContainsAny(target, " \t") {
		return nil
	}
	return &ScriptFlow{Script: script, Program: prog, Target: target}
}

// parseConn reads one ss / netstat established-connection line into an
// OutboundConn. A connection whose local port is a listener is inbound and
// dropped; a loopback / unspecified peer is dropped. Process name comes from
// ss's users:(("name"…)) or netstat's pid/name tail (root only).
func parseConn(line string, listen map[int]bool) *OutboundConn {
	var endpoints []struct {
		ip   string
		port int
	}
	for _, f := range strings.Fields(line) {
		if ip, port, ok := splitHostPort(f); ok {
			endpoints = append(endpoints, struct {
				ip   string
				port int
			}{ip, port})
		}
	}
	if len(endpoints) < 2 {
		return nil
	}
	local, peer := endpoints[0], endpoints[1]
	if listen[local.port] {
		return nil // inbound: the local side is one of our listeners
	}
	ip := net.ParseIP(peer.ip)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return nil
	}
	proc := ""
	if m := ssProcRe.FindStringSubmatch(line); m != nil {
		proc = m[1]
	} else if m := netstatProcRe.FindStringSubmatch(line); m != nil {
		proc = m[1]
	}
	return &OutboundConn{Peer: peer.ip, Port: peer.port, Process: proc, Public: !ip.IsPrivate()}
}

// splitHostPort parses "ip:port" / "[ipv6]:port" where port is numeric.
func splitHostPort(tok string) (string, int, bool) {
	i := strings.LastIndexByte(tok, ':')
	if i < 0 {
		return "", 0, false
	}
	host, portStr := tok[:i], tok[i+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if host == "" || host == "*" {
		return "", 0, false
	}
	return host, port, true
}

func decodeToken(b64 string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return ""
	}
	return string(raw)
}

// nowUnix is overridable so tests can assert the stale cutoff deterministically.
var nowUnix = func() int64 { return time.Now().Unix() }

func parseUnix(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 || n > 4102444800 { // after 2100 = forged
		return 0
	}
	return n
}
