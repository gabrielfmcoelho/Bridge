package sshtest

import (
	"encoding/base64"
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// credsources.go — the config-file secret probe. Root-only, CaptureJob-only.
// It streams the allowlisted config files that exist on the HOST (never inside
// containers — container env is never read; a container's compose/.env files
// live on the host) in base64 frames, which Go parses in memory into CredHits.
// The raw content and the plaintext in CredHit never serialize (VMInfo.CredHits
// is json:"-"); service.RecordScanSecrets puts each value in the vault and
// stores only the tokens + field names in VMInfo.CredentialSources.
//
// Modelled on gitremotes.go / webservers.go: one SSH session, output
// allowlisted (base64 so file content can never forge a frame marker), caps on
// per-file size, total size and file count. BRIDGE_ROOT prefixes every path in
// tests.

// CredHit is one credential a config-file detector found: exactly one vault
// secret plus one CredentialSource entry. Secret holds the plaintext payload
// (JSON for cred/sshkey, the raw value for env_var) and is transient.
type CredHit struct {
	Detector string            // "ci3_database", "dotenv", "root_ssh_keys", …
	Path     string            // absolute path on the VM
	Locator  string            // the secret within the file: "default", "DB_PASSWORD", "id_rsa"
	Kind     string            // "cred" | "env_var" | "sshkey"
	Admin    bool              // scan-admin- tier (DB-admin, superuser app login, root key)
	Site     string            // slug for the group label (app dir, domain id, or "host")
	Fields   map[string]string // engine/host/port/db/user/url — non-secret, shown + stored
	Secret   string            // plaintext payload to store (never serialized)
}

// CredentialSource is the stored provenance of a captured credential. The scan
// JSON keeps only this — detector, path, locator, field names and the vault
// token — never a value. nil (VMInfo.CredentialSources) = the probe didn't run.
type CredentialSource struct {
	Detector string            `json:"detector"`
	Path     string            `json:"path"`
	Locator  string            `json:"locator,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Admin    bool              `json:"admin,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
	Token    string            `json:"token,omitempty"` // {{secret:<id>}}; empty in the names-only commit
}

const (
	credPerFileCap = 262144  // 256 KiB per file
	credTotalCap   = 4194304 // 4 MiB across all files
	credMaxFiles   = 200
	credMaxPathLen = 512
	// credMaxInputs bounds the dynamic path lists (app roots, cron scripts).
	credMaxInputs = 64
)

// credFile is one file the probe streamed back, decoded. Content never leaves
// this package: detectCreds reads it and drops it.
type credFile struct {
	Path      string
	Owner     string
	Mtime     int64
	Content   string
	Truncated bool
}

// credScriptTmpl lists the allowlisted files that exist on the host and streams
// each (capped). Static absolute paths/globs are hardcoded; {ROOTS} (app roots
// to scan for app configs) and {FILES} (exact extra files — cron scripts and
// -i key paths) are baked in as base64, since the script runs through
// `base64 -d | sudo sh` and outer-shell env wouldn't survive. BRIDGE_ROOT
// prefixes every path (tests). POSIX sh + stat/head/base64 only, no awk.
const credScriptTmpl = `R="${BRIDGE_ROOT:-}"
echo @@CSBEGIN
TOTAL=0; N=0
emit() {
  f="$1"
  [ -f "$f" ] && [ -r "$f" ] || return 0
  p="${f#$R}"
  sz=$(stat -c %s "$f" 2>/dev/null); case "$sz" in ''|*[!0-9]*) sz=0;; esac
  rd=$sz; [ "$rd" -gt {PERFILE} ] && rd={PERFILE}
  TOTAL=$((TOTAL+rd)); [ "$TOTAL" -gt {TOTAL} ] && exit 0
  N=$((N+1)); [ "$N" -gt {MAXN} ] && exit 0
  tr=0; [ "$sz" -gt {PERFILE} ] && tr=1
  mt=$(stat -c %Y "$f" 2>/dev/null); case "$mt" in ''|*[!0-9]*) mt=0;; esac
  ow=$(stat -c %U "$f" 2>/dev/null); case "$ow" in ''|*[!A-Za-z0-9_.-]*) ow=-;; esac
  printf '@@F %s %s %s\n' "$(printf %s "$p" | base64 | tr -d '\n')" "$mt" "$ow"
  head -c {PERFILE} "$f" 2>/dev/null | base64 | tr -d '\n'; printf '\n'
  printf '@@E %s %s\n' "$rd" "$tr"
}
# erel: Java/Oracle app-server configs under one app root — exact names plus
# bounded globs (Spring, Tomcat conf, WildFly standalone, WebLogic jdbc/boot,
# hibernate/jdbc, and deployed WEB-INF apps). emit skips what doesn't exist.
erel() {
  d="$1"
  for rel in application.properties application.yml application.yaml config/application.properties config/application.yml conf/context.xml conf/server.xml conf/tomcat-users.xml standalone/configuration/standalone.xml hibernate.cfg.xml persistence.xml META-INF/persistence.xml jdbc.properties db.properties boot.properties ; do
    emit "$d/$rel"
  done
  for x in "$d"/conf/Catalina/*/*.xml "$d"/standalone/configuration/standalone*.xml "$d"/config/jdbc/*.xml "$d"/application-*.properties "$d"/application-*.yml "$d"/config/application-*.properties "$d"/config/application-*.yml "$d"/servers/*/security/boot.properties "$d"/webapps/*/WEB-INF/classes/hibernate.cfg.xml "$d"/webapps/*/WEB-INF/classes/jdbc.properties "$d"/webapps/*/WEB-INF/classes/db.properties "$d"/webapps/*/WEB-INF/classes/application.properties "$d"/webapps/*/META-INF/persistence.xml ; do
    emit "$x"
  done
}
# emitjar: a Spring Boot fat jar's config members only (job-mode only — the whole
# probe is). unzip -p | head -c caps each read; counted against N and TOTAL.
emitjar() {
  jar="$1"
  [ -f "$jar" ] && [ -r "$jar" ] || return 0
  command -v unzip >/dev/null 2>&1 || return 0
  for mem in BOOT-INF/classes/application.properties BOOT-INF/classes/application.yml BOOT-INF/classes/application.yaml ; do
    data=$(unzip -p "$jar" "$mem" 2>/dev/null | head -c {PERFILE} | base64 | tr -d '\n')
    [ -n "$data" ] || continue
    TOTAL=$((TOTAL + ${#data} * 3 / 4)); [ "$TOTAL" -gt {TOTAL} ] && exit 0
    N=$((N+1)); [ "$N" -gt {MAXN} ] && exit 0
    printf '@@F %s %s %s\n' "$(printf %s "${jar#$R}!/$mem" | base64 | tr -d '\n')" 0 -
    printf '%s\n' "$data"
    printf '@@E %s %s\n' 0 0
  done
}
for g in "$R"/etc/mysql/debian.cnf "$R"/root/.my.cnf "$R"/etc/my.cnf.d/*.cnf "$R"/var/log/mysqld.log "$R"/etc/postfix/sasl_passwd "$R"/etc/webmin/virtual-server/domains/* "$R"/etc/tnsnames.ora "$R"/var/opt/oracle/tnsnames.ora "$R"/u01/app/oracle/product/*/network/admin/tnsnames.ora "$R"/u01/app/oracle/product/*/*/network/admin/tnsnames.ora ; do emit "$g"; done
for k in "$R"/root/.ssh/id_* ; do case "$k" in *.pub) ;; *) emit "$k" ;; esac; done
OLDIFS=$IFS; IFS='
'
for x in $(printf %s {FILES} | base64 -d 2>/dev/null); do [ -n "$x" ] && emit "$R$x"; done
for j in $(printf %s {JARS} | base64 -d 2>/dev/null); do [ -n "$j" ] && emitjar "$R$j"; done
for r in $(printf %s {ROOTS} | base64 -d 2>/dev/null); do
  [ -n "$r" ] || continue
  for b in "$r" "$r/.." ; do for rel in .env wp-config.php application/config/database.php ; do emit "$R$b/$rel"; done; done
  for d in "$R$r"/*/ ; do [ -d "$d" ] || continue; dd="${d#$R}"; for rel in .env wp-config.php application/config/database.php ; do emit "$R$dd$rel"; done; done
  erel "$R$r"
  for d in "$R$r"/*/ ; do [ -d "$d" ] || continue; erel "${d%/}"; done
done
IFS=$OLDIFS
echo @@CSEND
`

// credProbeScript bakes roots, files and fat jars into the template.
func credProbeScript(roots, files, jars []string) string {
	enc := func(list []string) string {
		return base64.StdEncoding.EncodeToString([]byte(strings.Join(list, "\n")))
	}
	return strings.NewReplacer(
		"{ROOTS}", enc(roots), "{FILES}", enc(files), "{JARS}", enc(jars),
		"{PERFILE}", strconv.Itoa(credPerFileCap), "{TOTAL}", strconv.Itoa(credTotalCap),
		"{MAXN}", strconv.Itoa(credMaxFiles),
	).Replace(credScriptTmpl)
}

// collectCredSources runs the probe as root (it reads 0600 root-only files:
// debian.cnf, /root/.my.cnf, /root/.ssh/id_*). Without a stored password, or
// outside a job scan, it doesn't run and returns scanned=false — the
// credential_sources field stays null ("didn't run"), like git_remotes.
func collectCredSources(client *ssh.Client, sudoPassword string, roots, files, jars []string) ([]credFile, bool) {
	if sudoPassword == "" || !jobMode(client) {
		return nil, false
	}
	script := credProbeScript(roots, files, jars)
	out, err := runSudoCmd(client, sudoPassword, script)
	if !strings.Contains(out, "@@CSBEGIN") {
		// RHEL / CentOS 6–7 ship `Defaults requiretty`: sudo refuses without a
		// PTY. Retry the probe on a PTY session (Oracle Linux / CentOS 7 are
		// exactly the Java/Oracle targets). The first, cheaper attempt covers
		// every modern host; this fires only when sudo itself refused.
		out, err = runSudoCmdPTY(client, sudoPassword, script)
	}
	if !strings.Contains(out, "@@CSBEGIN") {
		log.Printf("[sshscan] cred-sources probe produced no output: %v", err)
		return nil, false
	}
	return parseCredProbe(out), true
}

// runSudoCmdPTY runs a script under sudo on a PTY session, for hosts with
// `Defaults requiretty`. ECHO off keeps the piped password out of the output;
// OPOST off stops the PTY translating \n→\r\n and corrupting the base64 frames.
// Any login banner lands before @@CSBEGIN, which parseCredProbe ignores.
func runSudoCmdPTY(client *ssh.Client, password, cmd string) (string, error) {
	session, err := newSession(client)
	if err != nil {
		return "", err
	}
	defer session.Close()
	modes := ssh.TerminalModes{ssh.ECHO: 0, ssh.OPOST: 0}
	if err := session.RequestPty("vt100", 200, 80, modes); err != nil {
		return "", err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}
	go func() {
		fmt.Fprintln(stdin, password)
		stdin.Close()
	}()
	encoded := base64.StdEncoding.EncodeToString([]byte(cmd))
	out, err := session.CombinedOutput(fmt.Sprintf("sudo -S -p '' sh -c 'echo %s | base64 -d | sh' 2>&1", encoded))
	return string(out), err
}

// parseCredProbe reads the base64 frames into credFiles, duplicate paths
// dropped (a file matched by a static glob and an app-root scan is read twice).
func parseCredProbe(raw string) []credFile {
	var out []credFile
	seen := map[string]bool{}
	lines := strings.Split(raw, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "@@F ") {
			continue
		}
		f := strings.Fields(strings.TrimPrefix(lines[i], "@@F "))
		if len(f) < 3 {
			continue
		}
		pb, err := base64.StdEncoding.DecodeString(f[0])
		if err != nil {
			continue
		}
		p := path.Clean(string(pb))
		mt, _ := strconv.ParseInt(f[1], 10, 64)
		content := ""
		if i+1 < len(lines) {
			if cb, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[i+1])); err == nil {
				content = string(cb)
			}
		}
		trunc := false
		if i+2 < len(lines) {
			if e := strings.Fields(strings.TrimPrefix(lines[i+2], "@@E ")); strings.HasPrefix(lines[i+2], "@@E ") && len(e) == 2 {
				trunc = e[1] == "1"
			}
		}
		i += 2
		if !validCredPath(p) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, credFile{Path: p, Owner: f[2], Mtime: mt, Content: content, Truncated: trunc})
	}
	return out
}

// validCredPath: an absolute path of printable characters, bounded.
func validCredPath(p string) bool {
	return strings.HasPrefix(p, "/") && len(p) <= credMaxPathLen && !strings.ContainsFunc(p, unicode.IsControl)
}

// credProbeInputs derives the dynamic path lists from the rest of the scan:
// the roots to scan for app configs (host web docroots, process cwds, Java
// homes), the scripts cron calls (cron_embedded reads their content) and the
// private keys cron passes with -i (root_ssh_keys reads them). Kept separate so
// detectCreds can dispatch each arbitrary-path file to the right detector.
// Each list is bounded and deduplicated.
func credProbeInputs(info *VMInfo) (roots, cronScripts, keyFiles, jars []string) {
	add := func(dst *[]string, seen map[string]bool, p string) {
		if p = strings.TrimSpace(p); strings.HasPrefix(p, "/") && !seen[p] && len(seen) < credMaxInputs {
			seen[p] = true
			*dst = append(*dst, p)
		}
	}
	rootSet, cronSet, keySet, jarSet := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	// Host web docroots (container roots live on the container fs — skipped).
	for _, ws := range info.WebServers {
		if ws.Container != "" {
			continue
		}
		for _, s := range ws.Sites {
			add(&roots, rootSet, s.Root)
		}
	}
	// Process cwds, Java app homes, Spring config-location dirs and fat jars.
	for _, p := range info.ProcessDetails {
		add(&roots, rootSet, p.CWD)
		if home := AppHome(p.Command); home != "" {
			add(&roots, rootSet, home)
		}
		for _, loc := range springConfigRoots(p.Command) {
			add(&roots, rootSet, loc)
		}
		for _, jar := range javaJars(p.Command) {
			add(&jars, jarSet, jar)
		}
	}
	// The scripts cron calls, and the -i keys in cron command lines.
	if info.Cron != nil {
		for _, j := range info.Cron.Jobs {
			for _, p := range cronScriptPaths(j.Command) {
				add(&cronScripts, cronSet, p)
			}
			for _, p := range cronIdentityFiles(j.Command) {
				add(&keyFiles, keySet, p)
			}
		}
	}
	return roots, cronScripts, keyFiles, jars
}

// javaJars returns the absolute `-jar <file>.jar` arguments of a java command
// line (Spring Boot fat jars). Relative jars are skipped: the probe can't
// resolve them without the cwd, and systemd services use absolute paths.
func javaJars(cmd string) []string {
	f := strings.Fields(cmd)
	var out []string
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "-jar" {
			if j := strings.Trim(f[i+1], `"'`); strings.HasPrefix(j, "/") && strings.HasSuffix(j, ".jar") {
				out = append(out, j)
			}
		}
	}
	return out
}

// springConfigRoots returns the filesystem directories named by
// -Dspring.config.location / -Dspring.config.additional-location, so the probe
// scans them for application.* (erel). A `file:` / `optional:` prefix is
// stripped; a value that is a file contributes its directory; classpath: and
// other non-filesystem values are dropped.
func springConfigRoots(cmd string) []string {
	var out []string
	for _, f := range strings.Fields(cmd) {
		for _, key := range []string{"-Dspring.config.location=", "-Dspring.config.additional-location="} {
			v := strings.TrimPrefix(f, key)
			if v == f || v == "" {
				continue
			}
			for _, e := range strings.Split(strings.Trim(v, `"'`), ",") {
				e = strings.TrimPrefix(strings.TrimPrefix(e, "optional:"), "file:")
				if !strings.HasPrefix(e, "/") {
					continue
				}
				if strings.HasSuffix(e, "/") {
					out = append(out, strings.TrimRight(e, "/"))
				} else {
					out = append(out, path.Dir(e))
				}
			}
		}
	}
	return out
}

// cronScriptPaths returns the absolute script paths a cron command runs: the
// interpreter's argument when there is one (sh /opt/x.sh), else the first
// absolute path token. A value already redacted ({{secret…}}) is skipped.
func cronScriptPaths(cmd string) []string {
	var out []string
	for _, w := range strings.Fields(cmd) {
		w = strings.Trim(w, `"'`)
		if strings.HasPrefix(w, "/") && !strings.Contains(w, "{{secret") {
			out = append(out, w)
			if len(out) >= 4 {
				break
			}
		}
	}
	return out
}

// sliceSet turns a path list into a lookup set for detectCreds.
func sliceSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, s := range list {
		m[s] = true
	}
	return m
}

// cronIdentityFiles returns the key paths passed with -i (ssh/scp/rsync -e) in
// a cron command line, so root_ssh_keys can read a key outside /root/.ssh.
func cronIdentityFiles(cmd string) []string {
	var out []string
	toks := strings.Fields(cmd)
	for i, w := range toks {
		w = strings.Trim(w, `"'`)
		switch {
		case w == "-i" && i+1 < len(toks):
			if p := strings.Trim(toks[i+1], `"'`); strings.HasPrefix(p, "/") {
				out = append(out, p)
			}
		case strings.HasPrefix(w, "-i/"):
			out = append(out, strings.TrimPrefix(w, "-i"))
		}
	}
	return out
}
