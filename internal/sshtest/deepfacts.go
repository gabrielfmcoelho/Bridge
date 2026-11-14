package sshtest

import (
	"encoding/base64"
	"log"
	"net"
	"regexp"
	"sort"
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
//   - A2 Sites: per web site, is anyone using it — 30-day access-log and recent
//     error-log aggregates (a live site shows thousands of requests, an
//     abandoned VM zero).
// Only names, hosts, ports, dates and counts leave the VM — never a script
// line, a credential, a client's address, a query string or a User-Agent.
type DeepFacts struct {
	Outbound *OutboundFlows  `json:"outbound,omitempty"`
	Patch    *PatchFreshness `json:"patch,omitempty"`
	Sites    []SiteTraffic   `json:"sites,omitempty"`
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

// SiteTraffic is 30-day access-log and recent error-log aggregates for one web
// site — the signal behind "is anyone actually using it?". Discovered from
// Virtualmin's per-domain logs, Apache CustomLog and nginx access_log.
// PRIVACY: the probe aggregates on the VM and emits only counts and
// first-path-segment labels; a client IP, query string, User-Agent or request
// line never leaves the box.
type SiteTraffic struct {
	Site       string      `json:"site"`                  // Virtualmin domain / nginx server_name / log basename
	Server     string      `json:"server,omitempty"`      // virtualmin | apache | nginx
	AccessLog  string      `json:"access_log,omitempty"`  // the access-log path aggregated (no content)
	Requests   int         `json:"requests,omitempty"`    // requests in the window (~30 d)
	Days       int         `json:"days,omitempty"`        // distinct days seen in the window
	PerDay     int         `json:"per_day,omitempty"`     // requests / active day (the headline)
	Status2xx  int         `json:"status_2xx,omitempty"`  //
	Status3xx  int         `json:"status_3xx,omitempty"`  //
	Status4xx  int         `json:"status_4xx,omitempty"`  //
	Status5xx  int         `json:"status_5xx,omitempty"`  //
	Human      int         `json:"human,omitempty"`       // requests with a browser-like User-Agent
	Bot        int         `json:"bot,omitempty"`         // the rest (bots, tools, no UA)
	PrivateIPs int         `json:"private_ips,omitempty"` // requests from an RFC1918 client (count only)
	PublicIPs  int         `json:"public_ips,omitempty"`  // requests from a public client (count only)
	LastSeen   int64       `json:"last_seen,omitempty"`   // unix (day) of the newest request seen
	TopPaths   []PathCount `json:"top_paths,omitempty"`   // first path segment × count (query stripped)
	Errors     []ErrorSig  `json:"errors,omitempty"`      // error-log signatures (allowlisted categories)
	// Partial: the access log was larger than the byte cap or has compressed
	// rotations, so the window may be short.
	Partial bool `json:"partial,omitempty"`
}

// PathCount is one first-path-segment and how often it was requested.
type PathCount struct {
	Path  string `json:"path"` // first path segment, "/" for root; never a query string
	Count int    `json:"count"`
}

// ErrorSig counts error-log lines in one allowlisted category (disk-full,
// php-error, not-found…). The probe matches each line on the VM and emits only
// the fixed category label — never the message text.
type ErrorSig struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

const (
	maxFlowScripts = 20      // cron scripts read per scan
	maxFlowTargets = 200     // ScriptFlow rows kept
	maxConns       = 100     // OutboundConn rows kept
	maxTargetLen   = 512     // a destination longer than this is dropped
	staleAfter     = 1209600 // 14 days: index older than this with auto-updates on = broken

	maxSites       = 50       // SiteTraffic rows kept
	maxTopPaths    = 8        // top path segments kept per site
	maxErrSigs     = 8        // error categories kept per site
	siteWindowDays = 30       // access-log window
	accessTailCap  = 67108864 // 64 MiB: newest bytes of each access log read
	errorTailCap   = 4194304  // 4 MiB: newest bytes of each error log read
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

// collectDeepFacts runs every deep-facts probe and merges the results: A3/A4
// (outbound flows + patch freshness) and A2 (per-site traffic). nil = nothing
// ran. Each probe opens its own root-first/unprivileged session.
func collectDeepFacts(client *ssh.Client, sudoPassword string, cron *CronInfo, ports []string) *DeepFacts {
	df := collectFlowsPatch(client, sudoPassword, cron, ports)
	if sites := collectSiteTraffic(client, sudoPassword); len(sites) > 0 {
		if df == nil {
			df = &DeepFacts{}
		}
		df.Sites = sites
	}
	return df
}

// collectFlowsPatch runs the A3/A4 probe — root with the stored password (ss -p
// and some history files are root-only), else unprivileged. scripts are the
// cron jobs' binary paths; listen is the host's listening ports (to tell an
// inbound server connection from an outbound one). nil = the probe didn't run.
func collectFlowsPatch(client *ssh.Client, sudoPassword string, cron *CronInfo, ports []string) *DeepFacts {
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

// ── A2: per-site access/error-log traffic ──
//
// The whole aggregation runs on the VM. accessAwk reads a combined-log stream
// and emits ONLY aggregates: it uses the client IP ($1) solely to bump a
// private/public counter, strips every query string (sub(/\?.*/,"")) before
// taking the first path segment, and never prints the request line, referer or
// User-Agent. errorAwk maps each error line to one of a fixed set of category
// labels and emits only the label — never the message text. So the sole output
// crossing the SSH boundary is counts, dates and first-path-segment labels.

// accessAwk aggregates a combined-access-log stream. -v cut=<yyyymmdd> drops
// lines older than the window. Output: one "@@sa k=v …" line and one "@@sp
// <count> <segment>" line per first path segment.
const accessAwk = `BEGIN{m["Jan"]=1;m["Feb"]=2;m["Mar"]=3;m["Apr"]=4;m["May"]=5;m["Jun"]=6;m["Jul"]=7;m["Aug"]=8;m["Sep"]=9;m["Oct"]=10;m["Nov"]=11;m["Dec"]=12}
{
  ip=$1
  d=$4; sub(/^\[/,"",d)
  split(d,D,/[\/:]/)
  mo=m[D[2]]
  if(mo==""||D[3]==""||D[1]=="")next
  ymd=(D[3]*10000)+(mo*100)+(D[1]+0)
  if(ymd<cut+0)next
  total++; day[ymd]=1; if(ymd>last)last=ymd
  nq=split($0,Q,"\"")
  st=""; if(nq>=3){split(Q[3],SS," ");st=SS[1]}
  c=substr(st,1,1)
  if(c=="2")s2++;else if(c=="3")s3++;else if(c=="4")s4++;else if(c=="5")s5++
  if(nq>=2){
    split(Q[2],RQ," ");u=RQ[2]
    sub(/\?.*/,"",u); sub(/^\//,"",u); sub(/\/.*/,"",u)
    gsub(/[^A-Za-z0-9._~-]/,"",u); u=substr(u,1,40)
    if(u=="")u="/"
    seg[u]++
  }
  ua=""; if(nq>=6)ua=tolower(Q[nq-1])
  if(ua~/mozilla|webkit|gecko|chrome|firefox|safari|opera|edg|msie|trident/)human++;else bot++
  if(ip~/^10\./||ip~/^192\.168\./||ip~/^127\./||ip~/^172\.(1[6-9]|2[0-9]|3[01])\./||ip~/^169\.254\./||ip~/^::1/||ip~/^fe80/||ip~/^f[cd]/)priv++
  else if(ip!=""&&ip!="-")pub++
}
END{
  nd=0;for(k in day)nd++
  printf "@@sa req=%d s2=%d s3=%d s4=%d s5=%d human=%d bot=%d priv=%d pub=%d days=%d last=%d\n",total,s2,s3,s4,s5,human,bot,priv,pub,nd,last
  for(p in seg)printf "@@sp %d %s\n",seg[p],p
}`

// errorAwk categorises an error-log stream into a fixed allowlist and emits
// only "@@se <count> <category>". No message text ever leaves the VM.
const errorAwk = `{
  l=tolower($0)
  if(l~/no space left/)k="disk-full"
  else if(l~/out of memory|allowed memory size|memory exhausted/)k="php-oom"
  else if(l~/php (fatal|parse|compile) error|uncaught (error|exception)|call to (a member function|undefined)/)k="php-error"
  else if(l~/permission denied/)k="permission-denied"
  else if(l~/does not exist|primary script unknown|no such file|not found/)k="not-found"
  else if(l~/upstream|connect\(\) failed|failed to connect|connection refused|timed out/)k="upstream-error"
  else if(l~/ssl|certificate|handshake|\btls\b/)k="tls-error"
  else if(l~/client denied|forbidden|authenticat|authoriz/)k="access-denied"
  else if(l~/segmentation fault|segfault|exit signal|child .* exited/)k="worker-crash"
  else k="other"
  ec[k]++
}
END{for(k in ec)printf "@@se %d %s\n",ec[k],k}`

// siteTrafficScript is one root-capable session: it discovers each web site's
// access log (Virtualmin per-domain logs, Apache CustomLog, nginx access_log),
// then streams the newest bytes of each through accessAwk and its error log
// through errorAwk. cutoff is yyyymmdd; BRIDGE_ROOT prefixes reads (tests).
// Not root: SUDO=1 retries reads through sudo -n.
// shortcut: error lines are categorised from the log tail (recent bytes),
// not strictly the last 7 days; add a per-family date filter if precision is
// needed. Only host web logs are read, not container logs.
func siteTrafficScript(asRoot bool, cutoff string) string {
	head := "SUDO=1\n"
	if asRoot {
		head = ""
	}
	return head + `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
R="${BRIDGE_ROOT:-}"
CUT="` + cutoff + `"
ACAP=` + strconv.Itoa(accessTailCap) + `
ECAP=` + strconv.Itoa(errorTailCap) + `
S=""; if [ -n "$SUDO" ] && sudo -n true 2>/dev/null; then S="sudo -n"; fi
TAB=$(printf '\t')
{
  for a in "$R"/var/log/virtualmin/*_access_log; do
    [ -f "$a" ] || continue
    rel=${a#"$R"}
    dom=${a##*/}; dom=${dom%_access_log}
    printf 'virtualmin\t%s\t%s\n' "$dom" "$rel"
  done
  for d in "$R/etc/apache2" "$R/etc/httpd"; do
    [ -d "$d" ] || continue
    $S grep -rhsoE '^[[:space:]]*CustomLog[[:space:]]+[^|"[:space:]]+' "$d" 2>/dev/null \
      | sed 's#.*CustomLog[[:space:]]*##; s#[$]{APACHE_LOG_DIR}#/var/log/apache2#g'
  done | while IFS= read -r p; do
    case "$p" in ''|*'$'*) continue;; esac
    lbl=${p##*/}; lbl=${lbl%.log}; lbl=${lbl%_access_log}; lbl=${lbl%-access}; lbl=${lbl%_access}; lbl=${lbl%.access}
    [ -n "$lbl" ] || lbl=${p##*/}
    printf 'apache\t%s\t%s\n' "$lbl" "$p"
  done
  if command -v nginx >/dev/null 2>&1; then
    $S nginx -T 2>/dev/null | grep -hoE 'access_log[[:space:]]+[^;[:space:]]+' \
      | sed 's#access_log[[:space:]]*##' | while IFS= read -r p; do
      case "$p" in ''|off|*'$'*) continue;; esac
      lbl=${p##*/}; lbl=${lbl%.log}; lbl=${lbl%.access}; lbl=${lbl%-access}; lbl=${lbl%_access}
      [ -n "$lbl" ] || lbl=${p##*/}
      printf 'nginx\t%s\t%s\n' "$lbl" "$p"
    done
  fi
} | awk -F"$TAB" '!seen[$3]++' | while IFS="$TAB" read -r server label acc; do
  f="$R$acc"
  [ -f "$f" ] || continue
  part=0
  sz=$($S wc -c < "$f" 2>/dev/null); case "$sz" in ''|*[!0-9]*) sz=0;; esac
  [ "$sz" -gt "$ACAP" ] && part=1
  for g in "$f.1.gz" "$f.2.gz" "$f.2"; do [ -f "$g" ] && part=1; done
  lb=$(printf %s "$label" | base64 | tr -d '\n')
  pb=$(printf %s "$acc" | base64 | tr -d '\n')
  printf '@@site %s %s %s %d\n' "$server" "$lb" "$pb" "$part"
  { $S tail -c "$ACAP" "$f" 2>/dev/null; [ -f "$f.1" ] && $S tail -c "$ACAP" "$f.1" 2>/dev/null; } | awk -v cut="$CUT" '` + accessAwk + `'
  err=$(printf %s "$acc" | sed 's/\(.*\)access/\1error/')
  ef="$R$err"
  if [ "$err" != "$acc" ] && [ -f "$ef" ]; then
    $S tail -c "$ECAP" "$ef" 2>/dev/null | awk '` + errorAwk + `'
  fi
done
echo "@@sites-done"
`
}

// collectSiteTraffic runs the A2 probe — root with the stored password (web
// logs are usually root-only), else unprivileged. Returns one SiteTraffic per
// discovered access log, or nil when nothing ran or no web logs exist.
func collectSiteTraffic(client *ssh.Client, sudoPassword string) []SiteTraffic {
	cutoff := siteCutoff()
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, siteTrafficScript(true, cutoff))
		if strings.Contains(out, "@@sites-done") {
			return parseSiteTraffic(out)
		}
		log.Printf("[sshscan] site traffic as root failed, falling back to unprivileged: %v", err)
	}
	raw := runCmd(client, encodedScript(siteTrafficScript(false, cutoff), "sh"))
	return parseSiteTraffic(raw)
}

// siteCutoff is the yyyymmdd of the start of the access-log window, from nowUnix
// (overridable so tests are deterministic).
func siteCutoff() string {
	return time.Unix(nowUnix(), 0).UTC().AddDate(0, 0, -siteWindowDays).Format("20060102")
}

// parseSiteTraffic turns the probe markers into SiteTraffic rows. @@site opens a
// row; @@sa carries its aggregate counters; @@sp and @@se its path and error
// tallies. Each row's top paths and error categories are sorted and capped.
func parseSiteTraffic(raw string) []SiteTraffic {
	var sites []SiteTraffic
	var cur *SiteTraffic
	flush := func() {
		if cur != nil {
			finalizeSite(cur)
			if len(sites) < maxSites {
				sites = append(sites, *cur)
			}
			cur = nil
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "@@site "):
			flush()
			f := strings.Fields(line[7:])
			if len(f) < 4 {
				continue
			}
			site, acc := decodeToken(f[1]), decodeToken(f[2])
			if !printable(site) || !printable(acc) {
				continue
			}
			cur = &SiteTraffic{Server: f[0], Site: site, AccessLog: acc, Partial: f[3] == "1"}
		case strings.HasPrefix(line, "@@sa "):
			if cur != nil {
				parseSiteAgg(line[5:], cur)
			}
		case strings.HasPrefix(line, "@@sp "):
			if cur != nil {
				if p, n := parseCount(line[5:]); n > 0 && len(cur.TopPaths) < 4*maxTopPaths && validSeg(p) {
					if p != "/" {
						p = "/" + p
					}
					cur.TopPaths = append(cur.TopPaths, PathCount{Path: p, Count: n})
				}
			}
		case strings.HasPrefix(line, "@@se "):
			if cur != nil {
				if cat, n := parseCount(line[5:]); n > 0 && len(cur.Errors) < maxErrSigs && validSeg(cat) {
					cur.Errors = append(cur.Errors, ErrorSig{Category: cat, Count: n})
				}
			}
		}
	}
	flush()
	if len(sites) == 0 {
		return nil
	}
	return sites
}

// parseSiteAgg reads the "k=v …" aggregate line into st.
func parseSiteAgg(s string, st *SiteTraffic) {
	for _, kv := range strings.Fields(s) {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(v)
		switch k {
		case "req":
			st.Requests = n
		case "s2":
			st.Status2xx = n
		case "s3":
			st.Status3xx = n
		case "s4":
			st.Status4xx = n
		case "s5":
			st.Status5xx = n
		case "human":
			st.Human = n
		case "bot":
			st.Bot = n
		case "priv":
			st.PrivateIPs = n
		case "pub":
			st.PublicIPs = n
		case "days":
			st.Days = n
		case "last":
			if t, err := time.Parse("20060102", v); err == nil {
				st.LastSeen = t.Unix()
			}
		}
	}
}

// finalizeSite derives per-day rate and keeps the busiest path segments and
// error categories.
func finalizeSite(st *SiteTraffic) {
	if st.Days > 0 {
		st.PerDay = st.Requests / st.Days
	}
	sort.SliceStable(st.TopPaths, func(i, j int) bool { return st.TopPaths[i].Count > st.TopPaths[j].Count })
	if len(st.TopPaths) > maxTopPaths {
		st.TopPaths = st.TopPaths[:maxTopPaths]
	}
	sort.SliceStable(st.Errors, func(i, j int) bool { return st.Errors[i].Count > st.Errors[j].Count })
	if len(st.Errors) > maxErrSigs {
		st.Errors = st.Errors[:maxErrSigs]
	}
}

// parseCount reads a "<count> <label>" marker body.
func parseCount(s string) (string, int) {
	n, label, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return "", 0
	}
	c, err := strconv.Atoi(n)
	if err != nil {
		return "", 0
	}
	return strings.TrimSpace(label), c
}

var segOK = regexp.MustCompile(`^[A-Za-z0-9._~/-]{1,40}$`)

func validSeg(s string) bool { return segOK.MatchString(s) }

func printable(s string) bool {
	if s == "" || len(s) > maxTargetLen {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || r == ' ' })
}
