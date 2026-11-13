package sshtest

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// Release risks (VMInfo.ReleaseRisks): what a release upgrade trips over or
// silently breaks. These are SAPASP-CONCURSO's 20.04 → 22.04 lessons:
// - packages from another release or a PPA, and disabled sources;
// - locally modified conffiles and the *.dist files they leave;
// - failed units and packages dpkg left half configured;
// - Apache sites whose PHP is gone or whose port Apache doesn't listen on.
// Only names, paths and ports leave the VM, never a file's contents.

// ReleaseRisks is the probe's result; nil in VMInfo when it didn't run.
type ReleaseRisks struct {
	ForeignCount      int           `json:"foreign_count"`
	ForeignPackages   []string      `json:"foreign_packages,omitempty"` // "name version", the first 100
	DisabledSources   []string      `json:"disabled_sources,omitempty"`
	ModifiedConffiles []string      `json:"modified_conffiles,omitempty"`
	PendingDistFiles  []string      `json:"pending_dist_files,omitempty"`
	FailedUnits       []string      `json:"failed_units,omitempty"`
	DpkgBroken        []string      `json:"dpkg_broken,omitempty"`
	ApacheListen      []int         `json:"apache_listen,omitempty"`
	PHPHandlers       []PHPHandler  `json:"php_handlers,omitempty"`
	Warnings          []RiskWarning `json:"warnings,omitempty"`
}

// PHPHandler is one Apache line that hands PHP to something: a FastCGI
// wrapper (FCGIWrapper / FcgidWrapper) or a php-fpm socket (SetHandler
// proxy:unix:…), with the <Directory> it sits in. State: ok, missing (the
// binary the wrapper execs, or the socket, isn't there) or unknown (the
// wrapper's directory can't be read).
type PHPHandler struct {
	Source  string `json:"source"` // file:line
	Dir     string `json:"dir,omitempty"`
	Kind    string `json:"kind"`           // fcgi | fpm
	Target  string `json:"target"`         // the wrapper or the socket
	Runs    string `json:"runs,omitempty"` // fcgi: the binary the wrapper execs
	Version string `json:"version,omitempty"`
	State   string `json:"state"`
}

// RiskWarning is one thing to look at before a release; Detail lists the
// first few items. The UI words it from Code.
type RiskWarning struct {
	Code   string `json:"code"`
	Count  int    `json:"count"`
	Detail string `json:"detail,omitempty"`
}

const releaseRiskTimeout = 30 * time.Second

// releaseRiskScript sections (headers have no space):
// - @@os: ID VERSION_ID; @@foreign: "name version" of PPA / other-release builds;
// - @@local: [installed,local] packages; @@conffiles: modified conffiles;
// - @@audit: packages dpkg --audit names; @@disabled: source files;
// - @@dist: pending *.dist files; @@failed: systemctl --failed;
// - @@apache: grep -n of Listen / Directory / handler lines;
// - @@fcgi: "wrapper binary|- ok|missing|unknown"; @@sockets: "socket ok|missing".
// Every command is bounded by timeout(1): the root run has no deadline of
// its own. Not root: SUDO=1 tries sudo -n for the reads that need it.
// BRIDGE_ROOT prefixes every path (tests only).
func releaseRiskScript(asRoot bool) string {
	head := "SUDO=1\n"
	if asRoot {
		head = ""
	}
	return head + `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
R="${BRIDGE_ROOT:-}"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 8"
S=""; if [ -n "$SUDO" ] && sudo -n true 2>/dev/null; then S="sudo -n"; fi
x() { $S test "$@"; }
strip() { if [ -n "$R" ]; then sed "s#^$R##"; else cat; fi; }
echo @@os; ( . "$R/etc/os-release" 2>/dev/null; echo "${ID:-?} ${VERSION_ID:-?}" )
if command -v dpkg-query >/dev/null 2>&1; then
  echo @@foreign
  $T dpkg-query -W -f='${db:Status-Abbrev} ${Package} ${Version}\n' 2>/dev/null | awk '$1 ~ /^ii/ && $3 ~ /[+~]ubuntu[0-9]|deb\.sury\.org|~bpo/ {print $2, $3}' | head -n 2000
  echo @@local
  $T apt list --installed 2>/dev/null | awk -F'[/ ]' '/\[installed,local\]/ {print $1, $3}' | head -n 2000
  echo @@conffiles
  $T dpkg-query -W -f='${Conffiles}\n' 2>/dev/null | awk -v r="$R" 'NF == 2 && $1 ~ /^\// && length($2) == 32 {print $2 "  " r $1}' |
    $S $T md5sum -c 2>/dev/null | awk '/: FAILED$/ {sub(/: FAILED$/, ""); print}' | strip | head -n 200
  echo @@audit
  $T dpkg --audit 2>/dev/null | awk '/^ [a-z0-9]/ {print $1}' | head -n 100
fi
echo @@disabled
ls -1 "$R"/etc/apt/sources.list.d/*.distUpgrade "$R"/etc/apt/sources.list.d/*.save "$R"/etc/apt/sources.list.distUpgrade "$R"/etc/apt/sources.list.save 2>/dev/null | strip
grep -l -E '^#[[:space:]]*deb[[:space:]]' "$R"/etc/apt/sources.list.d/*.list 2>/dev/null | strip
grep -l -i -E '^Enabled:[[:space:]]*no' "$R"/etc/apt/sources.list.d/*.sources 2>/dev/null | strip
echo @@dist
$S $T find "$R/etc" -xdev \( -name '*.dpkg-dist' -o -name '*.ucf-dist' -o -name '*.dpkg-new' -o -name '*.dpkg-old' \) -type f 2>/dev/null | strip | head -n 200
if command -v systemctl >/dev/null 2>&1; then
  echo @@failed
  $T systemctl --failed --plain --no-legend --no-pager 2>/dev/null | awk '{print $1}' | head -n 100
fi
A=$(ls "$R"/etc/apache2/ports.conf "$R"/etc/apache2/apache2.conf "$R"/etc/apache2/sites-enabled/* "$R"/etc/apache2/conf-enabled/*.conf "$R"/etc/httpd/conf/httpd.conf "$R"/etc/httpd/conf.d/*.conf 2>/dev/null)
if [ -n "$A" ]; then
  echo @@apache
  $T grep -H -n -i -E '^[[:space:]]*(Listen|</?Directory|FCGIWrapper|FcgidWrapper|SetHandler)[[:space:]>]' $A 2>/dev/null | strip | head -n 3000
  echo @@fcgi
  grep -h -i -E '^[[:space:]]*(FCGIWrapper|FcgidWrapper)[[:space:]]' $A 2>/dev/null | awk '{gsub(/"/, "", $2); print $2}' | sort -u | head -n 200 |
  while read -r w; do
    b=""; st=unknown
    if x -f "$R$w"; then
      b=$($S $T grep -m1 -E '^[[:space:]]*exec[[:space:]]' "$R$w" 2>/dev/null | awk '{print $2}')
      case "$b" in /*) if x -x "$R$b"; then st=ok; else st=missing; fi ;; *) b=""; st=ok ;; esac
    elif x -x "$R${w%/*}"; then st=missing
    fi
    echo "$w ${b:--} $st"
  done
  echo @@sockets
  grep -h -o -i -E 'proxy:unix:[^|" ]+' $A 2>/dev/null | cut -d: -f3- | sort -u | head -n 200 |
  while read -r s; do if x -S "$R$s"; then echo "$s ok"; else echo "$s missing"; fi; done
fi
`
}

// collectReleaseRisks runs the probe: as root with the stored password
// (conffiles and wrappers may be root-only), else unprivileged.
func collectReleaseRisks(client *ssh.Client, sudoPassword string) string {
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, releaseRiskScript(true))
		if strings.Contains(out, "@@os") {
			return out
		}
		log.Printf("[sshscan] release risks as root failed, falling back to unprivileged: %v", err)
	}
	out, _ := runCmdWithDeadline(client, encodedScript(releaseRiskScript(false), "sh"), releaseRiskTimeout)
	return out
}

const (
	maxRiskList   = 100
	maxRiskPath   = 512
	maxRiskDetail = 5
)

var (
	ubuntuSeries = regexp.MustCompile(`[+~]ubuntu(\d\d\.\d\d)`)
	unitName     = regexp.MustCompile(`^[A-Za-z0-9@._:\\-]{1,256}$`)
	phpVersion   = regexp.MustCompile(`php(?:-cgi|-fpm)?(\d+(?:\.\d+)?)`)
	apacheLine   = regexp.MustCompile(`^(.+?):(\d+):\s*(\S+)\s*(.*)$`)
)

// parseReleaseRisks reads the probe; nil when it didn't run (no @@os).
func parseReleaseRisks(raw string) *ReleaseRisks {
	sec := map[string][]string{}
	cur := ""
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "@@") && !strings.Contains(t, " ") {
			cur = strings.TrimPrefix(t, "@@")
			if sec[cur] == nil {
				sec[cur] = []string{}
			}
			continue
		}
		if cur != "" && t != "" {
			sec[cur] = append(sec[cur], t)
		}
	}
	osLine, ok := sec["os"]
	if !ok {
		return nil
	}
	versionID := ""
	if len(osLine) > 0 {
		if f := strings.Fields(osLine[0]); len(f) == 2 {
			versionID = strings.Trim(f[1], `"`)
		}
	}
	r := &ReleaseRisks{
		DisabledSources:   riskPaths(sec["disabled"]),
		ModifiedConffiles: riskPaths(sec["conffiles"]),
		PendingDistFiles:  riskPaths(sec["dist"]),
	}
	r.ForeignCount, r.ForeignPackages = foreignPackages(sec["foreign"], sec["local"], versionID)
	for _, u := range sec["failed"] {
		if u = strings.TrimLeft(u, "●* "); unitName.MatchString(u) && len(r.FailedUnits) < maxRiskList {
			r.FailedUnits = append(r.FailedUnits, u)
		}
	}
	for _, p := range sec["audit"] {
		if safeToken.MatchString(p) && len(r.DpkgBroken) < maxRiskList {
			r.DpkgBroken = append(r.DpkgBroken, p)
		}
	}
	r.ApacheListen, r.PHPHandlers = parseApacheHandlers(sec["apache"], sec["fcgi"], sec["sockets"])
	return r
}

// foreignPackages: the PPA builds (deb.sury.org), Debian backports and builds
// for another Ubuntu release, plus what no source offers any more
// ([installed,local]). Each package once, sorted; the list is capped.
func foreignPackages(candidates, local []string, versionID string) (int, []string) {
	seen := map[string]bool{}
	var out []string
	add := func(line string, check bool) {
		f := strings.Fields(line)
		if len(f) != 2 || !safeToken.MatchString(f[0]) || !safeToken.MatchString(f[1]) || seen[f[0]] {
			return
		}
		if check && !foreignVersion(f[1], versionID) {
			return
		}
		seen[f[0]] = true
		out = append(out, f[0]+" "+f[1])
	}
	for _, l := range candidates {
		add(l, true)
	}
	for _, l := range local {
		add(l, false)
	}
	sort.Strings(out)
	n := len(out)
	if n > maxRiskList {
		out = out[:maxRiskList]
	}
	return n, out
}

func foreignVersion(v, versionID string) bool {
	if strings.Contains(v, "deb.sury.org") || strings.Contains(v, "~bpo") {
		return true
	}
	m := ubuntuSeries.FindStringSubmatch(v)
	return m != nil && m[1] != versionID
}

// riskPaths keeps absolute, printable, bounded paths, once each, sorted.
func riskPaths(lines []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range lines {
		if !validRiskPath(p) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	if len(out) > maxRiskList {
		out = out[:maxRiskList]
	}
	return out
}

func validRiskPath(p string) bool {
	return strings.HasPrefix(p, "/") && len(p) <= maxRiskPath && !strings.ContainsFunc(p, unicode.IsControl)
}

// parseApacheHandlers walks the grep -n lines of each Apache file in order:
// Listen ports, the <Directory> each handler sits in, and each handler with
// the state the probe found for its wrapper or socket.
func parseApacheHandlers(lines, fcgi, sockets []string) ([]int, []PHPHandler) {
	type state struct{ runs, st string }
	targets := map[string]state{}
	for _, l := range fcgi {
		if f := strings.Fields(l); len(f) == 3 && validRiskPath(f[0]) {
			runs := f[1]
			if !validRiskPath(runs) {
				runs = ""
			}
			targets["fcgi "+f[0]] = state{runs, f[2]}
		}
	}
	for _, l := range sockets {
		if f := strings.Fields(l); len(f) == 2 && validRiskPath(f[0]) {
			targets["fpm "+f[0]] = state{"", f[1]}
		}
	}
	listen := map[int]bool{}
	var handlers []PHPHandler
	dir := map[string]string{} // file → the <Directory> open at this line
	for _, l := range lines {
		m := apacheLine.FindStringSubmatch(l)
		if m == nil || !validRiskPath(m[1]) {
			continue
		}
		file, directive, args := m[1], strings.ToLower(m[3]), strings.Fields(m[4])
		h := PHPHandler{Source: file + ":" + m[2], Dir: dir[file]}
		switch {
		case directive == "listen" && len(args) > 0:
			addr := args[0]
			if i := strings.LastIndex(addr, ":"); i >= 0 {
				addr = addr[i+1:]
			}
			if p, err := strconv.Atoi(addr); err == nil && p > 0 && p < 65536 {
				listen[p] = true
			}
			continue
		case strings.HasPrefix(directive, "<directory"):
			d := strings.Trim(strings.TrimSuffix(strings.Join(args, " "), ">"), `"'`)
			if validRiskPath(d) {
				dir[file] = d
			}
			continue
		case strings.HasPrefix(directive, "</directory"):
			dir[file] = ""
			continue
		case (directive == "fcgiwrapper" || directive == "fcgidwrapper") && len(args) > 0:
			h.Kind, h.Target = "fcgi", strings.Trim(args[0], `"`)
		case directive == "sethandler" && len(args) > 0:
			arg := strings.Trim(args[0], `"`)
			sock, ok := strings.CutPrefix(strings.ToLower(arg), "proxy:unix:")
			if !ok {
				continue
			}
			sock = arg[len(arg)-len(sock):] // the path as written
			if i := strings.Index(sock, "|"); i >= 0 {
				sock = sock[:i]
			}
			h.Kind, h.Target = "fpm", sock
		default:
			continue
		}
		if !validRiskPath(h.Target) || len(handlers) >= maxRiskList {
			continue
		}
		s, ok := targets[h.Kind+" "+h.Target]
		if !ok {
			s.st = "unknown"
		}
		h.Runs, h.State = s.runs, s.st
		if m := phpVersion.FindStringSubmatch(h.Runs + " " + h.Target); m != nil {
			h.Version = m[1]
		}
		handlers = append(handlers, h)
	}
	ports := make([]int, 0, len(listen))
	for p := range listen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	if len(ports) == 0 {
		ports = nil
	}
	return ports, handlers
}

// deriveReleaseWarnings turns the probe (and the web servers it is checked
// against) into the warnings the UI lists. Computed at capture time and
// stored in release_risks, so the frontend only renders them.
func deriveReleaseWarnings(info *VMInfo) []RiskWarning {
	r := info.ReleaseRisks
	if r == nil {
		return nil
	}
	var out []RiskWarning
	add := func(code string, count int, items []string) {
		if count == 0 {
			return
		}
		w := RiskWarning{Code: code, Count: count}
		if len(items) > maxRiskDetail {
			w.Detail = strings.Join(items[:maxRiskDetail], ", ") + ", …"
		} else {
			w.Detail = strings.Join(items, ", ")
		}
		out = append(out, w)
	}
	add("dpkg_broken", len(r.DpkgBroken), r.DpkgBroken)
	add("foreign_packages", r.ForeignCount, r.ForeignPackages)
	add("disabled_sources", len(r.DisabledSources), r.DisabledSources)
	add("modified_conffiles", len(r.ModifiedConffiles), r.ModifiedConffiles)
	add("pending_dist_files", len(r.PendingDistFiles), r.PendingDistFiles)
	add("failed_units", len(r.FailedUnits), r.FailedUnits)

	var missing []string
	seen := map[string]bool{}
	for _, h := range r.PHPHandlers {
		if h.State == "missing" && !seen[h.Target] {
			seen[h.Target] = true
			what := h.Target
			if h.Runs != "" {
				what += " → " + h.Runs
			}
			missing = append(missing, what)
		}
	}
	add("php_handler_missing", len(missing), missing)

	// A vhost on a port Apache doesn't Listen on is never served. Only the
	// host's Apache, and only when its Listen lines were read.
	if len(r.ApacheListen) > 0 {
		listen := map[int]bool{}
		for _, p := range r.ApacheListen {
			listen[p] = true
		}
		var unheard []string
		seen := map[string]bool{}
		for _, ws := range info.WebServers {
			if ws.Kind != "apache" || ws.Container != "" {
				continue
			}
			for _, s := range ws.Sites {
				for _, p := range s.Listen {
					item := fmt.Sprintf("%s:%d", strings.Join(s.Names, " "), p)
					if !listen[p] && !seen[item] {
						seen[item] = true
						unheard = append(unheard, item)
					}
				}
			}
		}
		add("vhost_port_unlistened", len(unheard), unheard)
	}
	return out
}
