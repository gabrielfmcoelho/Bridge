package sshtest

import (
	"encoding/base64"
	"log"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// WebServer is one nginx, Apache or php-fpm found by the scan, on the host
// (Container "") or inside a running container. The scan stores it in the
// host_scans JSON; service.RecordWebDNS turns its site names into DNS links.
type WebServer struct {
	Kind           string    `json:"kind"` // nginx | apache | php-fpm
	Container      string    `json:"container,omitempty"`
	ContainerID    string    `json:"container_id,omitempty"`
	ContainerImage string    `json:"container_image,omitempty"`
	Sites          []WebSite `json:"sites,omitempty"`
	Pools          []FPMPool `json:"pools,omitempty"`
	// Stopped: on the host, its systemd unit isn't running (the config is
	// still there). Its names are shown but never create DNS records.
	Stopped bool   `json:"stopped,omitempty"`
	Error   string `json:"error,omitempty"`
}

// WebSite is one nginx server block or Apache virtual host.
type WebSite struct {
	Names   []string `json:"names,omitempty"` // as written, "_" included
	Listen  []int    `json:"listen,omitempty"`
	TLS     bool     `json:"tls,omitempty"`
	Root    string   `json:"root,omitempty"`
	Proxy   []string `json:"proxy,omitempty"`   // proxy_pass targets, upstreams expanded
	FastCGI []string `json:"fastcgi,omitempty"` // fastcgi_pass targets
	Source  string   `json:"source,omitempty"`  // apache: file:line
	// Locations keeps each nginx location with its own targets, in the order
	// each first received one; Proxy/FastCGI stay the flattened union.
	Locations []WebLocation `json:"locations,omitempty"`
}

// WebLocation is an nginx location that forwards somewhere. Path is the
// pattern token as written (a regex is not interpreted).
type WebLocation struct {
	Path    string   `json:"path"`
	Proxy   []string `json:"proxy,omitempty"`
	FastCGI []string `json:"fastcgi,omitempty"`
}

// FPMPool is one php-fpm pool and where it listens (socket or host:port).
type FPMPool struct {
	Name   string `json:"name"`
	Listen string `json:"listen,omitempty"`
}

const (
	maxWebContainers = 30
	webOutputCap     = 262144
	webProbeTimeout  = 60 * time.Second
)

// webInnerScript runs on the host (SUDO=1: retry with sudo -n) and inside
// each container. Sections: @@nginx (nginx -T), @@apache (apachectl -S),
// @@fpm (pool headers and listen lines). Every command is bounded by
// timeout(1) when present: the root run (runSudoCmd) has no deadline of its own.
const webInnerScript = `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 20"
P(){ printf '@@%s\n' "$1"; }
run(){ o=$($T "$@" 2>&1); r=$?; if [ "$r" -ne 0 ] && [ -n "$SUDO" ]; then o2=$(sudo -n $T "$@" 2>&1) && o=$o2; fi; printf '%s\n' "$o"; }
if command -v nginx >/dev/null 2>&1; then P nginx; run nginx -T | head -c 1048576; echo; fi
for b in apachectl apache2ctl httpd; do if command -v $b >/dev/null 2>&1; then P apache; run $b -S | head -n 500; break; fi; done
f=$(ls /etc/php/*/fpm/pool.d/*.conf /etc/php-fpm.d/*.conf /usr/local/etc/php-fpm.d/*.conf 2>/dev/null)
if [ -n "$f" ]; then P fpm; grep -H -E '^\[|^[[:space:]]*listen[[:space:]]*=' $f; fi
`

// webProbeScript is the whole remote script: the host part, then every
// running container (up to maxWebContainers) through docker exec. One SSH session.
// ponytail: every running container is probed, not only web-looking images —
// app images often embed nginx; cap and per-container timeout bound the cost.
func webProbeScript(asRoot bool) string {
	return hostAndContainersScript(webInnerScript, maxWebContainers, webOutputCap, 6, 0, asRoot)
}

// hostAndContainersScript runs inner on the host, then inside every running
// container (up to max) through docker exec, each framed by
// "@@CTR <id> <name> <image>" and closed by "@@exit <docker exec status>",
// its output capped at outCap bytes and its run at ctrTimeout seconds. budget > 0: a container reached that many seconds
// after the loop began isn't run, its frame says "@@skipped". asRoot: the
// script already runs as root (runSudoCmd), so no sudo -n retries; otherwise
// inner sees SUDO=1.
func hostAndContainersScript(inner string, max, outCap, ctrTimeout, budget int, asRoot bool) string {
	b64 := base64.StdEncoding.EncodeToString([]byte(inner))
	head, dockerFallback := "SUDO=1\n", `$D ps -q >/dev/null 2>&1 || D="sudo -n docker"`
	if asRoot {
		head, dockerFallback = "", ""
	}
	return head + inner + strings.NewReplacer("{MAX}", strconv.Itoa(max), "{B64}", b64,
		"{CAP}", strconv.Itoa(outCap), "{FALLBACK}", dockerFallback, "{TO}", strconv.Itoa(ctrTimeout),
		"{BUDGET}", strconv.Itoa(budget)).Replace(`
command -v docker >/dev/null 2>&1 || exit 0
TO=""; command -v timeout >/dev/null 2>&1 && TO="timeout {TO}"
D=docker; {FALLBACK}
B0=$(date +%s 2>/dev/null || echo 0)
$T $D ps --format '{{.ID}} {{.Names}} {{.Image}}' 2>/dev/null | head -n {MAX} | while read id name image; do
  echo "@@CTR $id $name $image"
  if [ {BUDGET} -gt 0 ] && [ $(( $(date +%s 2>/dev/null || echo 0) - B0 )) -ge {BUDGET} ]; then echo @@skipped; continue; fi
  { printf %s {B64} | base64 -d | $TO $D exec -i "$id" sh 2>/dev/null; r=$?; echo; echo "@@exit $r"; } | head -c {CAP}
  echo
done
`)
}

// collectWebServers runs the probe — as root with the stored password when
// there is one (apachectl -S / nginx -T often need to read root-only TLS
// files), else unprivileged with sudo -n retries. hostNginx is the host's raw
// nginx -T (for the port → server_name map); servers are host first, then
// containers.
func collectWebServers(client *ssh.Client, sudoPassword string) (servers []WebServer, hostNginx string) {
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, webProbeScript(true))
		if strings.Contains(out, "@@") {
			return parseWebProbe(out)
		}
		log.Printf("[sshscan] web-servers as root failed, falling back to unprivileged: %v", err)
	}
	raw, _ := runCmdWithDeadline(client, encodedScript(webProbeScript(false), "sh"), webProbeTimeout)
	return parseWebProbe(raw)
}

var ctrHeader = regexp.MustCompile(`^@@CTR ([0-9a-f]{12,64}) (\S+) (\S+)$`)

// ctrFrame is one container's part of a hostAndContainersScript output.
// Exit is the docker exec status the host shell saw (126/127: no shell in
// the image; 1: paused, restarting…), -1 when the frame has none (skipped,
// or cut by the output cap).
type ctrFrame struct {
	ID, Name, Image, Body string
	Exit                  int
}

var ctrExit = regexp.MustCompile(`\n@@exit ([0-9]{1,3})\s*$`)

// splitCtrFrames splits a hostAndContainersScript output into the host part
// and one frame per container, with the exec status taken off the body's
// end (only the last marker counts: the container prints whatever it likes
// before it). A frame whose header doesn't validate is dropped whole:
// container output is untrusted.
func splitCtrFrames(raw string) (string, []ctrFrame) {
	parts := strings.Split("\n"+raw, "\n@@CTR ")
	var frames []ctrFrame
	for _, p := range parts[1:] {
		head, body, _ := strings.Cut(p, "\n")
		m := ctrHeader.FindStringSubmatch("@@CTR " + strings.TrimSpace(head))
		if m == nil {
			continue
		}
		f := ctrFrame{ID: m[1], Name: m[2], Image: m[3], Body: body, Exit: -1}
		s := "\n" + body
		if e := ctrExit.FindStringSubmatchIndex(s); e != nil {
			f.Exit, _ = strconv.Atoi(s[e[2]:e[3]])
			f.Body = strings.TrimPrefix(s[:e[0]], "\n")
		}
		frames = append(frames, f)
	}
	return parts[0], frames
}

// parseWebProbe splits the probe output into the host part and one part per
// container and builds their servers.
func parseWebProbe(raw string) ([]WebServer, string) {
	host, frames := splitCtrFrames(raw)
	hostSec := webSections(host)
	servers := webServersFrom(hostSec)
	for _, f := range frames {
		for _, ws := range webServersFrom(webSections(f.Body)) {
			ws.ContainerID, ws.Container, ws.ContainerImage = f.ID, f.Name, f.Image
			servers = append(servers, ws)
		}
	}
	return servers, hostSec["nginx"]
}

// webSections splits "@@name" sections; text before the first is dropped.
func webSections(text string) map[string]string {
	out := map[string]string{}
	cur := ""
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		switch strings.TrimSpace(line) {
		case "@@nginx", "@@apache", "@@fpm":
			if cur != "" {
				out[cur] = b.String()
			}
			cur = strings.TrimPrefix(strings.TrimSpace(line), "@@")
			b.Reset()
			continue
		}
		if cur != "" {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if cur != "" {
		out[cur] = b.String()
	}
	return out
}

var probeFailure = regexp.MustCompile(`(?i)\[emerg\]|permission denied|not permitted|syntax error`)

func webServersFrom(sec map[string]string) []WebServer {
	var out []WebServer
	add := func(kind string, sites []WebSite, pools []FPMPool, raw string) {
		ws := WebServer{Kind: kind, Sites: sites, Pools: pools}
		if len(sites) == 0 && len(pools) == 0 {
			if l := probeFailure.FindString(raw); l != "" {
				ws.Error = "config unreadable: " + firstMatchingLine(raw, probeFailure)
			} else {
				return
			}
		}
		out = append(out, ws)
	}
	if cfg, ok := sec["nginx"]; ok {
		add("nginx", parseNginxSites(cfg), nil, cfg)
	}
	if s, ok := sec["apache"]; ok {
		add("apache", parseApacheS(s), nil, s)
	}
	if s, ok := sec["fpm"]; ok {
		add("php-fpm", nil, parseFPMPools(s), "")
	}
	return out
}

func firstMatchingLine(text string, re *regexp.Regexp) string {
	for _, l := range strings.Split(text, "\n") {
		if re.MatchString(l) {
			l = strings.TrimSpace(l)
			if len(l) > 160 {
				l = l[:160]
			}
			return l
		}
	}
	return ""
}

// markStoppedHostServers flags host servers whose systemd unit isn't among
// the running ones. No running units at all (no systemd, or unreadable)
// leaves everything as running.
func markStoppedHostServers(servers []WebServer, running []SystemdService) {
	if len(running) == 0 {
		return
	}
	unitRe := map[string]*regexp.Regexp{
		"nginx":   regexp.MustCompile(`^nginx\.service$`),
		"apache":  regexp.MustCompile(`^(apache2|httpd)\.service$`),
		"php-fpm": regexp.MustCompile(`^php[0-9.]*-fpm[0-9.]*\.service$`),
	}
	for i := range servers {
		if servers[i].Container != "" || unitRe[servers[i].Kind] == nil {
			continue
		}
		up := false
		for _, u := range running {
			if unitRe[servers[i].Kind].MatchString(u.Unit) {
				up = true
			}
		}
		servers[i].Stopped = !up
	}
}

var (
	nginxDirective = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?:^|[\s{;])` + name + `\s+([^;\s]+)`)
	}
	reProxyPass   = nginxDirective("proxy_pass")
	reFastCGIPass = nginxDirective("fastcgi_pass")
	reRoot        = nginxDirective("root")
	// The "{" may open on the next line; a path holding "{" ({3}) keeps it.
	reLocation    = regexp.MustCompile(`^location\s+(?:[=~^*]+\s+)?(?:(\S+)\s*\{|(\S+)$)`)
	reUpstream    = regexp.MustCompile(`^upstream\s+(\S+)\s*\{`)
	reUpstreamSrv = regexp.MustCompile(`^server\s+([^;\s]+)`)
)

// parseNginxSites reads `nginx -T` output: one WebSite per top-level
// `server { … }` block (listen ports, ssl, server_name, root, and the
// proxy_pass / fastcgi_pass targets of its locations). proxy_pass to an
// `upstream` name is expanded to that upstream's servers. Brace depth is
// tracked so `server x;` lines inside upstream blocks never start a site.
func parseNginxSites(cfg string) []WebSite {
	var sites []WebSite
	upstreams := map[string][]string{}
	var cur *WebSite
	var serverDepth, depth int
	upstream, upstreamDepth := "", 0
	type openLoc struct {
		path  string
		depth int
	}
	var locs []openLoc // open `location` blocks, innermost last
	// target records a pass on the innermost named location (an `if` block
	// belongs to its enclosing one; @named locations are skipped).
	target := func(proxy bool, v string) {
		if len(locs) == 0 || strings.HasPrefix(locs[len(locs)-1].path, "@") {
			return
		}
		path := locs[len(locs)-1].path
		i := 0
		for i < len(cur.Locations) && cur.Locations[i].Path != path {
			i++
		}
		if i == len(cur.Locations) {
			cur.Locations = append(cur.Locations, WebLocation{Path: path})
		}
		if l := &cur.Locations[i]; proxy {
			l.Proxy = appendUnique(l.Proxy, v)
		} else {
			l.FastCGI = appendUnique(l.FastCGI, v)
		}
	}

	for _, raw := range strings.Split(cfg, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if cur == nil && upstream == "" {
			if m := reUpstream.FindStringSubmatch(line); m != nil {
				upstream, upstreamDepth = m[1], depth
			} else if i := strings.IndexByte(line, '{'); i >= 0 && strings.TrimSpace(line[:i]) == "server" {
				cur, serverDepth = &WebSite{}, depth
			}
		} else if upstream != "" && depth == upstreamDepth+1 {
			if m := reUpstreamSrv.FindStringSubmatch(line); m != nil {
				upstreams[upstream] = append(upstreams[upstream], m[1])
			}
		}

		if cur != nil {
			if m := reLocation.FindStringSubmatch(line); m != nil {
				locs = append(locs, openLoc{m[1] + m[2], depth})
			}
			top := depth == serverDepth+1 || (depth == serverDepth && strings.Contains(line, "{"))
			switch {
			case top && (strings.HasPrefix(line, "listen ") || strings.HasPrefix(line, "listen\t")):
				f := strings.Fields(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "listen")), ";"))
				if len(f) > 0 {
					spec := f[0]
					if i := strings.LastIndex(spec, ":"); i >= 0 {
						spec = spec[i+1:]
					}
					if p, err := strconv.Atoi(spec); err == nil && p > 0 {
						cur.Listen = appendUniqueInt(cur.Listen, p)
					}
					for _, x := range f[1:] {
						if x == "ssl" {
							cur.TLS = true
						}
					}
				}
			case top && (strings.HasPrefix(line, "server_name ") || strings.HasPrefix(line, "server_name\t")):
				for _, n := range strings.Fields(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "server_name")), ";")) {
					cur.Names = appendUnique(cur.Names, n)
				}
			case top && (line == "ssl on;" || strings.HasPrefix(line, "ssl on;")):
				cur.TLS = true
			}
			for _, m := range reProxyPass.FindAllStringSubmatch(line, -1) {
				cur.Proxy = appendUnique(cur.Proxy, m[1])
				target(true, m[1])
			}
			for _, m := range reFastCGIPass.FindAllStringSubmatch(line, -1) {
				cur.FastCGI = appendUnique(cur.FastCGI, m[1])
				target(false, m[1])
			}
			if m := reRoot.FindStringSubmatch(line); m != nil && (cur.Root == "" || top) {
				cur.Root = m[1]
			}
		}

		for _, c := range line {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
				if n := len(locs); n > 0 && locs[n-1].depth == depth {
					locs = locs[:n-1]
				}
				if cur != nil && depth == serverDepth {
					sites = append(sites, *cur)
					cur, locs = nil, nil
				}
				if upstream != "" && depth == upstreamDepth {
					upstream = ""
				}
			}
		}
	}

	expand := func(targets []string) (out []string) {
		for _, p := range targets {
			if srvs, ok := upstreams[proxyHost(p)]; ok {
				for _, s := range srvs {
					out = appendUnique(out, s)
				}
			} else {
				out = appendUnique(out, p)
			}
		}
		return out
	}
	for i := range sites {
		sites[i].Proxy = expand(sites[i].Proxy)
		for j := range sites[i].Locations {
			sites[i].Locations[j].Proxy = expand(sites[i].Locations[j].Proxy)
		}
	}
	return sites
}

// proxyHost returns the host part of a proxy target ("http://app:3000/x" → "app").
func proxyHost(target string) string {
	t := target
	if i := strings.Index(t, "://"); i >= 0 {
		t = t[i+3:]
	}
	if i := strings.IndexAny(t, "/"); i >= 0 {
		t = t[:i]
	}
	if h, _, ok := strings.Cut(t, ":"); ok && !strings.HasPrefix(t, "[") {
		return h
	}
	return t
}

// parseNginxListens maps each listen port to the server_names answering on
// it ("_" for a block without names). Feeds the port-owner labels.
func parseNginxListens(cfg string) map[int][]string {
	out := map[int][]string{}
	for _, s := range parseNginxSites(cfg) {
		var names []string
		for _, n := range s.Names {
			if n != "_" {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			names = []string{"_"}
		}
		for _, p := range s.Listen {
			out[p] = append(out[p], names...)
		}
	}
	return out
}

var (
	reApacheNameVHost = regexp.MustCompile(`^\s*port (\d+) namevhost (\S+) \((.+)\)\s*$`)
	reApacheSingle    = regexp.MustCompile(`^(\S+):(\d+)\s+(\S+) \((.+)\)\s*$`)
	reApacheAlias     = regexp.MustCompile(`^\s*(?:wild )?alias (\S+)\s*$`)
)

// parseApacheS reads `apachectl -S`: one WebSite per virtual host
// (namevhost lines, or the single vhost on an address), aliases attached.
// ponytail: names, port and source file only — no per-vhost DocumentRoot or
// ProxyPass; parse the vhost files (apachectl -t -D DUMP_INCLUDES) if needed.
func parseApacheS(out string) []WebSite {
	var sites []WebSite
	for _, line := range strings.Split(out, "\n") {
		var port, name, src string
		if m := reApacheNameVHost.FindStringSubmatch(line); m != nil {
			port, name, src = m[1], m[2], m[3]
		} else if m := reApacheSingle.FindStringSubmatch(line); m != nil {
			port, name, src = m[2], m[3], m[4]
		} else if m := reApacheAlias.FindStringSubmatch(line); m != nil && len(sites) > 0 {
			sites[len(sites)-1].Names = appendUnique(sites[len(sites)-1].Names, m[1])
			continue
		} else {
			continue
		}
		p, _ := strconv.Atoi(port)
		sites = append(sites, WebSite{Names: []string{name}, Listen: []int{p}, TLS: p == 443, Source: src})
	}
	return sites
}

// parseFPMPools reads `grep -H` output of pool files: "[name]" headers and
// "listen = …" lines. php-fpm merges sections of the same name across the
// files of one pool directory, a later file's listen winning (the official
// image splits [www] over docker.conf / www.conf / zz-docker.conf); pools in
// different directories (/etc/php/7.4 vs 8.1) stay apart. [global] is not a pool.
func parseFPMPools(out string) []FPMPool {
	var pools []FPMPool
	byKey := map[string]int{} // pool dir + name → index
	last := map[string]int{}  // file → index of its current pool
	for _, line := range strings.Split(out, "\n") {
		file, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
			name := strings.Trim(rest, "[]")
			if name == "global" {
				delete(last, file)
				continue
			}
			key := path.Dir(file) + "\x00" + name
			i, seen := byKey[key]
			if !seen {
				pools = append(pools, FPMPool{Name: name})
				i = len(pools) - 1
				byKey[key] = i
			}
			last[file] = i
			continue
		}
		if k, v, ok := strings.Cut(rest, "="); ok && strings.TrimSpace(k) == "listen" {
			if i, ok := last[file]; ok {
				pools[i].Listen = strings.TrimSpace(v)
			}
		}
	}
	sort.SliceStable(pools, func(i, j int) bool { return pools[i].Name < pools[j].Name })
	return pools
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func appendUniqueInt(list []int, n int) []int {
	for _, v := range list {
		if v == n {
			return list
		}
	}
	return append(list, n)
}
