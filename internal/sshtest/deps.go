package sshtest

import (
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Dependency inventory: which OS, runtimes and packages (with versions) run
// on the host and inside each container, so their versions can be checked
// for CVEs. One probe script runs on the host and, piped through
// `docker exec -i <id> sh`, inside one running container per image.
//
// Everything that comes back is untrusted text: names and versions must
// match safeToken, container IDs validContainerID, and outputs are capped on
// both ends. The environment of a running container is never read (it holds
// secrets); versions from env come from the *image* config only, filtered
// to *_VERSION keys on the remote side.

// OSRelease is the /etc/os-release fields advisory matching needs.
type OSRelease struct {
	ID        string `json:"id"`
	IDLike    string `json:"id_like,omitempty"`
	VersionID string `json:"version_id"`
	Codename  string `json:"codename,omitempty"`
}

// RuntimeVersion is a version read from a binary (`php -v`).
type RuntimeVersion struct {
	Name    string `json:"name"` // php | node | python | nginx | openssl | apache | postgresql | mysql | mariadb | redis | java
	Version string `json:"version"`
}

// DepPackage is one installed OS package. SrcName/SrcVersion are the source
// package (Debian/Alpine advisories are keyed by it); "" when unknown.
type DepPackage struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	SrcName    string `json:"src_name,omitempty"`
	SrcVersion string `json:"src_version,omitempty"`
}

// DepScope is what one place (the host, or a container) runs. Container,
// ContainerID, Image and ImageID are all "" for the host.
type DepScope struct {
	Container   string            `json:"container,omitempty"`
	ContainerID string            `json:"container_id,omitempty"`
	Image       string            `json:"image,omitempty"`
	ImageID     string            `json:"image_id,omitempty"`
	OS          OSRelease         `json:"os"`
	PkgManager  string            `json:"pkg_manager,omitempty"` // dpkg | apk | rpm
	Runtimes    []RuntimeVersion  `json:"runtimes,omitempty"`
	Packages    []DepPackage      `json:"packages,omitempty"`
	ImageEnv    map[string]string `json:"image_env,omitempty"` // *_VERSION keys of the image config
	// Error is why this scope wasn't probed ("" = probed). A scope with an
	// error keeps whatever inventory it had before.
	Error string `json:"error,omitempty"`
}

const (
	// maxProbeImages caps how many distinct images get a docker exec per
	// scan; one container per image is probed.
	maxProbeImages = 12
	// maxDepPackages caps packages per scope (remote head + Go side).
	maxDepPackages = 3000
	// probeOutputCap caps one container's probe output on the remote side.
	probeOutputCap  = 262144
	hostDepTimeout  = 20 * time.Second
	execLoopTimeout = 45 * time.Second
)

// hostSecurityPackages are probed on the host besides the catalog packages:
// the usual CVE suspects the catalog (agents/services) doesn't name.
var hostSecurityPackages = []string{
	"openssl", "libssl*", "openssh-server", "sudo", "libc6", "glibc", "bash", "curl", "libcurl*",
	"php*", "nodejs", "python3*", "nginx*", "apache2", "httpd", "postgresql*", "mysql-server*",
	"mariadb-server*", "redis*", "docker-ce", "containerd*",
}

var (
	safeToken   = regexp.MustCompile(`^[A-Za-z0-9.+~:_-]{1,128}$`)
	containerID = regexp.MustCompile(`^[0-9a-f]{12,64}$`)
	envKeyRe    = regexp.MustCompile(`^[A-Z0-9_]+_VERSION$`)
	ociVersion  = "org.opencontainers.image.version"
	patternRe   = regexp.MustCompile(`^[A-Za-z0-9.+_-]+\*?$`)
)

func validContainerID(s string) bool { return containerID.MatchString(s) }

// depProbeScript is the POSIX/busybox sh script run on the host and inside
// containers. patterns narrow the package list (nil = every package); java
// is probed only when withJava (a JVM start-up in a memory-limited container
// risks the OOM killer).
func depProbeScript(patterns []string, withJava bool) string {
	var b strings.Builder
	b.WriteString(`P(){ printf '@@%s\n' "$1"; }
T(){ if command -v timeout >/dev/null 2>&1; then timeout 5 "$@"; else "$@"; fi; }
R(){ command -v "$1" >/dev/null 2>&1 && { P "rt:$2"; shift 2; T "$@" 2>&1 | head -n 3; }; }
P os; grep -E '^(ID|ID_LIKE|VERSION_ID|VERSION_CODENAME)=' /etc/os-release 2>/dev/null
R php php php -v
R node node node -v
R python3 python python3 -V
R nginx nginx nginx -v
R openssl openssl openssl version
R apache2 apache apache2 -v
R httpd apache httpd -v
R psql postgresql psql --version
R postgres postgresql postgres --version
R mysql mysql mysql --version
R mysqld mysql mysqld --version
R redis-server redis redis-server -v
`)
	if withJava {
		b.WriteString("R java java java -version\n")
	}
	pats := ""
	for _, p := range patterns {
		if patternRe.MatchString(p) {
			pats += " '" + p + "'"
		}
	}
	fmt.Fprintf(&b, `P pkg
if command -v dpkg-query >/dev/null 2>&1; then echo dpkg; dpkg-query -W -f='${db:Status-Abbrev}\t${Package}\t${Version}\t${source:Package}\t${source:Version}\n'%[1]s 2>/dev/null | head -n %[2]d
elif [ -f /lib/apk/db/installed ]; then echo apk; awk 'function out(){ if (p != "") print "ii\t" p "\t" v "\t" o "\t" v; p=""; v=""; o="" } /^P:/{p=substr($0,3)} /^V:/{v=substr($0,3)} /^o:/{o=substr($0,3)} /^$/{out()} END{out()}' /lib/apk/db/installed | head -n %[2]d
elif command -v rpm >/dev/null 2>&1; then echo rpm; rpm -qa --qf 'ii\t%%{NAME}\t%%{EPOCHNUM}:%%{VERSION}-%%{RELEASE}\t%%{SOURCERPM}\t\n'%[1]s 2>/dev/null | head -n %[2]d
fi
`, pats, maxDepPackages)
	return b.String()
}

// encodedScript wraps a script so it travels with no quoting layers:
// `printf %s <b64> | base64 -d | <runner>`.
func encodedScript(script, runner string) string {
	return "printf %s " + base64.StdEncoding.EncodeToString([]byte(script)) + " | base64 -d | " + runner
}

var runtimeRes = map[string]*regexp.Regexp{
	"php":        regexp.MustCompile(`PHP (\d+\.\d+\.\d+)`),
	"node":       regexp.MustCompile(`(?m)^v(\d+\.\d+\.\d+)`),
	"python":     regexp.MustCompile(`Python (\d+\.\d+\.\d+)`),
	"nginx":      regexp.MustCompile(`nginx/(\d+\.\d+\.\d+)`),
	"openssl":    regexp.MustCompile(`OpenSSL (\d+\.\d+\.\d+[a-z]?)`),
	"apache":     regexp.MustCompile(`Apache/(\d+\.\d+\.\d+)`),
	"postgresql": regexp.MustCompile(`\(PostgreSQL\) (\d+(?:\.\d+)?)`),
	"mysql":      regexp.MustCompile(`(?:Ver|Distrib|from) (\d+\.\d+\.\d+)`),
	"redis":      regexp.MustCompile(`v=(\d+\.\d+\.\d+)`),
	"java":       regexp.MustCompile(`version "([\d._]+)"`),
}

// parseDepProbe parses one probe script's output into a scope (the caller
// fills in container/image fields).
func parseDepProbe(raw string) DepScope {
	var s DepScope
	seen := map[string]bool{}
	section := ""
	var body []string
	flush := func() {
		switch {
		case section == "os":
			s.OS = parseOSRelease(body)
		case strings.HasPrefix(section, "rt:"):
			name := strings.TrimPrefix(section, "rt:")
			text := strings.Join(body, "\n")
			if name == "mysql" && strings.Contains(strings.ToLower(text), "mariadb") {
				name = "mariadb"
			}
			re := runtimeRes[strings.TrimPrefix(section, "rt:")]
			if re == nil {
				break
			}
			if m := re.FindStringSubmatch(text); m != nil && safeToken.MatchString(m[1]) && !seen[name] {
				seen[name] = true
				s.Runtimes = append(s.Runtimes, RuntimeVersion{Name: name, Version: m[1]})
			}
		case section == "pkg":
			s.PkgManager, s.Packages = parsePackages(body)
		}
		body = body[:0]
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "@@") {
			flush()
			section = strings.TrimSpace(strings.TrimPrefix(line, "@@"))
			continue
		}
		body = append(body, line)
	}
	flush()
	return s
}

func parseOSRelease(lines []string) OSRelease {
	var o OSRelease
	for _, l := range lines {
		k, v, ok := strings.Cut(strings.TrimSpace(l), "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		if v != "" && !safeToken.MatchString(strings.ReplaceAll(v, " ", "_")) {
			continue
		}
		switch k {
		case "ID":
			o.ID = strings.ToLower(v)
		case "ID_LIKE":
			o.IDLike = strings.ToLower(v)
		case "VERSION_ID":
			o.VersionID = v
		case "VERSION_CODENAME":
			o.Codename = v
		}
	}
	return o
}

// parsePackages reads the pkg section: the manager name, then
// "status\tname\tversion\tsrcname\tsrcversion" rows. Only installed ("ii")
// rows with safe tokens survive.
func parsePackages(lines []string) (string, []DepPackage) {
	if len(lines) == 0 {
		return "", nil
	}
	mgr := strings.TrimSpace(lines[0])
	switch mgr {
	case "dpkg", "apk", "rpm":
	default:
		return "", nil
	}
	var out []DepPackage
	seen := map[string]bool{}
	for _, l := range lines[1:] {
		f := strings.Split(l, "\t")
		if len(f) < 3 || !strings.HasPrefix(strings.TrimSpace(f[0]), "ii") {
			continue
		}
		p := DepPackage{Name: f[1], Version: f[2]}
		if len(f) > 3 {
			p.SrcName = f[3]
		}
		if len(f) > 4 {
			p.SrcVersion = f[4]
		}
		if mgr == "rpm" {
			p.Version = strings.TrimPrefix(p.Version, "0:")
			p.SrcName, p.SrcVersion = rpmSource(p.SrcName), p.Version
		}
		if p.SrcName == "" {
			p.SrcName = p.Name
		}
		if p.SrcVersion == "" {
			p.SrcVersion = p.Version
		}
		if !safeToken.MatchString(p.Name) || !safeToken.MatchString(p.Version) ||
			!safeToken.MatchString(p.SrcName) || !safeToken.MatchString(p.SrcVersion) || seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		out = append(out, p)
		if len(out) >= maxDepPackages {
			break
		}
	}
	return mgr, out
}

// rpmSource turns "openssl-3.0.7-27.el9.src.rpm" into "openssl".
func rpmSource(srpm string) string {
	s := strings.TrimSuffix(srpm, ".src.rpm")
	for i := 0; i < 2; i++ {
		if j := strings.LastIndex(s, "-"); j > 0 {
			s = s[:j]
		}
	}
	return s
}

// parseImageEnv reads `docker image inspect` output: a "sha256:…" line per
// image followed by KEY=VALUE lines. Only *_VERSION keys and the OCI version
// label with safe values survive, even if the remote grep didn't run.
func parseImageEnv(raw string) map[string]map[string]string {
	out := map[string]map[string]string{}
	cur := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "sha256:") {
			cur = line
			if _, ok := out[cur]; !ok {
				out[cur] = map[string]string{}
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || cur == "" || !safeToken.MatchString(v) || !(envKeyRe.MatchString(k) || k == ociVersion) {
			continue
		}
		out[cur][k] = v
	}
	return out
}

// runningContainer is one `docker ps` row with its image ID.
type runningContainer struct {
	ID, Name, Image, ImageID string
	Up                       bool
}

// pickProbeTargets returns one running container per image ID, in a stable
// order, at most maxProbeImages.
func pickProbeTargets(cs []runningContainer) []runningContainer {
	byImage := map[string]runningContainer{}
	var images []string
	for _, c := range cs {
		if !c.Up || c.ImageID == "" || !validContainerID(c.ID) {
			continue
		}
		if _, ok := byImage[c.ImageID]; !ok {
			byImage[c.ImageID] = c
			images = append(images, c.ImageID)
		}
	}
	sort.Strings(images)
	var out []runningContainer
	for _, img := range images {
		if len(out) == maxProbeImages {
			break
		}
		out = append(out, byImage[img])
	}
	return out
}

// splitExecLoop splits the exec loop's output into per-container probe
// outputs, keyed by container ID.
func splitExecLoop(raw string) map[string]string {
	out := map[string]string{}
	cur := ""
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "@@CTR ") {
			if cur != "" {
				out[cur] = b.String()
			}
			cur = strings.TrimSpace(strings.TrimPrefix(line, "@@CTR "))
			b.Reset()
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if cur != "" {
		out[cur] = b.String()
	}
	return out
}

// collectHostDeps probes the host itself: catalog + security packages,
// runtimes (java included) and the OS release.
func collectHostDeps(client *ssh.Client) DepScope {
	patterns := append(append([]string{}, catalogPackageNames()...), hostSecurityPackages...)
	out, err := runCmdWithDeadline(client, encodedScript(depProbeScript(patterns, true), "sh"), hostDepTimeout)
	if err != nil && strings.TrimSpace(out) == "" {
		return DepScope{Error: "probe failed"}
	}
	return parseDepProbe(out)
}

// collectContainerDeps runs after `docker ps` inside the docker goroutine:
// image IDs, image env versions, then one docker exec per distinct image.
// pfx is "" or "sudo -n " (whatever docker ps needed). Every container in
// psRaw gets a scope; unprobed ones carry Error.
func collectContainerDeps(client *ssh.Client, psRaw, pfx string) []DepScope {
	var cs []runningContainer
	for _, line := range splitLines(psRaw) {
		p := strings.SplitN(line, "\t", 5)
		if len(p) < 4 || !validContainerID(p[0]) {
			continue
		}
		cs = append(cs, runningContainer{ID: p[0], Name: p[1], Image: p[2], Up: strings.HasPrefix(p[3], "Up")})
	}
	if len(cs) == 0 {
		return nil
	}
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.ID
	}
	// Container → image ID. The template prints only these two fields, so
	// the container's environment never leaves the host.
	inspect, _ := runCmdWithDeadline(client, pfx+`docker inspect --format '{{.Id}} {{.Image}}' `+strings.Join(ids, " ")+` 2>/dev/null`, 8*time.Second)
	for _, line := range splitLines(inspect) {
		full, img, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || !strings.HasPrefix(img, "sha256:") {
			continue
		}
		for i := range cs {
			if strings.HasPrefix(full, cs[i].ID) {
				cs[i].ImageID = img
			}
		}
	}
	images := map[string]bool{}
	var imageIDs []string
	for _, c := range cs {
		if c.ImageID != "" && validContainerID(strings.TrimPrefix(c.ImageID, "sha256:")) && !images[c.ImageID] {
			images[c.ImageID] = true
			imageIDs = append(imageIDs, c.ImageID)
		}
	}
	env := map[string]map[string]string{}
	if len(imageIDs) > 0 {
		raw, _ := runCmdWithDeadline(client, pfx+`docker image inspect --format '{{.Id}}{{range .Config.Env}}{{"\n"}}{{.}}{{end}}{{range $k,$v := .Config.Labels}}{{"\n"}}{{$k}}={{$v}}{{end}}' `+
			strings.Join(imageIDs, " ")+` 2>/dev/null | grep -E '^(sha256:|[A-Z0-9_]+_VERSION=|org\.opencontainers\.image\.version=)'`, 8*time.Second)
		env = parseImageEnv(raw)
	}

	targets := pickProbeTargets(cs)
	probes := map[string]DepScope{} // image ID → probed scope
	if len(targets) > 0 {
		script := base64.StdEncoding.EncodeToString([]byte(depProbeScript(nil, false)))
		var loop strings.Builder
		loop.WriteString(`TO=""; command -v timeout >/dev/null 2>&1 && TO="timeout 6"; for c in`)
		for _, t := range targets {
			loop.WriteString(" " + t.ID)
		}
		fmt.Fprintf(&loop, `; do echo "@@CTR $c"; printf %%s %s | base64 -d | $TO %sdocker exec -i "$c" sh 2>/dev/null | head -c %d; echo; done`, script, pfx, probeOutputCap)
		raw, err := runCmdWithDeadline(client, loop.String(), execLoopTimeout)
		if err != nil {
			log.Printf("[sshscan] dependency exec loop: %v", err)
		}
		byCtr := splitExecLoop(raw)
		for _, t := range targets {
			if out, ok := byCtr[t.ID]; ok && strings.Contains(out, "@@") {
				probes[t.ImageID] = parseDepProbe(out)
			}
		}
	}

	scopes := make([]DepScope, 0, len(cs))
	for _, c := range cs {
		s, ok := probes[c.ImageID]
		if !ok {
			s = DepScope{Error: "not probed"}
		}
		s.Container, s.ContainerID, s.Image, s.ImageID = c.Name, c.ID, c.Image, c.ImageID
		s.ImageEnv = env[c.ImageID]
		scopes = append(scopes, s)
	}
	return scopes
}
