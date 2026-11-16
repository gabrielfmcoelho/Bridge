package sshtest

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// credstores.go — config-file detectors for the common developer/ops credential
// stores: .netrc, docker/registry auth, git-credentials, npm/composer/maven/pip
// tokens, kubeconfig and systemd EnvironmentFile. Like the Phase 1/2 detectors
// they are pure and static (no app code runs): each turns one credFile into
// zero or more CredHits, the plaintext only ever in CredHit.Secret (transient).
// Passwords never go into Fields. Registry / cloud / infra credentials carry
// Admin=true so the recorder files them under the admin-only reveal tier.
//
// Out of scope by design: /etc/shadow and OS password hashes, TLS/SSH private
// keys (root's outbound keys are the Phase-1 root_ssh_keys detector), kubeconfig
// client-key-data (a private key — recorded as "not captured", never read),
// shell history and mailboxes.

// ---- generic helper ----

// sortedMapKeys returns a map's string keys sorted, for deterministic iteration.
func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---- .netrc ----

// netrc reads ~/.netrc / /root/.netrc: whitespace-tokenised machine/login/
// password entries (single- or multi-line), `default` as a catch-all, macdef
// macro bodies skipped. One cred per machine holding a password. Normal tier.
func netrc(f credFile) []CredHit {
	site := siteSlug(f.Path)
	var hits []CredHit
	var host, user, pass string
	skipMacro := false
	flush := func() {
		if pass != "" {
			hits = append(hits, CredHit{
				Detector: "netrc", Path: f.Path, Locator: slug(firstNonEmpty(host, "default")), Kind: "cred", Site: site,
				Fields: fields("host", host, "user", user),
				Secret: credJSON(user, pass, host, "", ""),
			})
		}
		host, user, pass = "", "", ""
	}
	for _, line := range strings.Split(f.Content, "\n") {
		toks := strings.Fields(line)
		if len(toks) == 0 {
			skipMacro = false // a blank line ends a macdef body
			continue
		}
		if skipMacro {
			continue
		}
		for i := 0; i < len(toks); i++ {
			switch toks[i] {
			case "machine":
				flush()
				if i+1 < len(toks) {
					host = toks[i+1]
					i++
				}
			case "default":
				flush() // host stays "" → locator "default"
			case "login", "user":
				if i+1 < len(toks) {
					user = toks[i+1]
					i++
				}
			case "password":
				if i+1 < len(toks) {
					pass = toks[i+1]
					i++
				}
			case "account":
				i++ // skip its value
			case "macdef":
				skipMacro = true
				i = len(toks)
			}
		}
	}
	flush()
	return hits
}

// ---- docker / registry auth ----

// dockerConfig reads ~/.docker/config.json: auths[<registry>].auth is base64
// "user:password" (decoded in Go), or explicit username/password. The registry
// host is context only. Admin tier — a registry push credential.
func dockerConfig(f credFile) []CredHit {
	var cfg struct {
		Auths map[string]struct {
			Auth     string
			Username string
			Password string
		}
	}
	if err := json.Unmarshal([]byte(f.Content), &cfg); err != nil {
		return nil
	}
	site := siteSlug(f.Path)
	var hits []CredHit
	for _, reg := range sortedMapKeys(cfg.Auths) {
		a := cfg.Auths[reg]
		user, pass := a.Username, a.Password
		if a.Auth != "" {
			if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(a.Auth)); err == nil {
				if u, p, ok := strings.Cut(string(dec), ":"); ok {
					user = firstNonEmpty(user, u)
					pass = firstNonEmpty(pass, p)
				}
			}
		}
		if pass == "" {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "docker_config", Path: f.Path, Locator: slug(reg), Kind: "cred", Admin: true, Site: site,
			Fields: fields("registry", reg, "user", user),
			Secret: credJSON(user, pass, reg, "", ""),
		})
	}
	return hits
}

// ---- git-credentials ----

// gitCredentials reads ~/.git-credentials: one `scheme://user:password@host`
// per line (git's credential-store). Normal tier.
func gitCredentials(f credFile) []CredHit {
	site := siteSlug(f.Path)
	var hits []CredHit
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.User == nil {
			continue
		}
		pass, ok := u.User.Password()
		if !ok || pass == "" {
			continue
		}
		user := u.User.Username()
		hits = append(hits, CredHit{
			Detector: "git_credentials", Path: f.Path, Locator: slug(firstNonEmpty(u.Host, user)), Kind: "cred", Site: site,
			Fields: fields("host", u.Host, "user", user, "scheme", u.Scheme),
			Secret: credJSON(user, pass, u.Host, "", ""),
		})
		if len(hits) >= 50 {
			break
		}
	}
	return hits
}

// ---- npm ----

// npmrc reads an .npmrc (home or project): _authToken / _auth / _password,
// which may be registry-scoped (`//registry/:_authToken=…`). The value is the
// token (an _auth blob is base64 user:password, stored verbatim). A ${NPM_TOKEN}
// placeholder is recorded indirect. Admin tier — a registry publish credential.
func npmrc(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		scope, field := "", strings.ToLower(key)
		if i := strings.LastIndex(key, ":"); i >= 0 {
			scope, field = key[:i], strings.ToLower(key[i+1:])
		}
		if field != "_authtoken" && field != "_auth" && field != "_password" {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if val == "" || dynamicValue(val) {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "npmrc", Path: f.Path, Locator: slug(firstNonEmpty(scope, field)), Kind: "env_var", Admin: true, Site: site,
			Fields: fields("registry", strings.TrimPrefix(scope, "//"), "field", field),
			Secret: val,
		})
	}
	return hits
}

// ---- composer ----

// composerAuth reads a Composer auth.json (home or project): http-basic
// (username/password per host) and the token maps github-oauth / gitlab-token /
// gitlab-oauth / bearer. Admin tier — a private-package registry credential.
func composerAuth(f credFile) []CredHit {
	var cfg struct {
		HTTPBasic map[string]struct {
			Username string
			Password string
		} `json:"http-basic"`
		GitHubOAuth map[string]string `json:"github-oauth"`
		GitLabToken map[string]string `json:"gitlab-token"`
		GitLabOAuth map[string]string `json:"gitlab-oauth"`
		Bearer      map[string]string `json:"bearer"`
	}
	if err := json.Unmarshal([]byte(f.Content), &cfg); err != nil {
		return nil
	}
	site := siteSlug(f.Path)
	var hits []CredHit
	for _, host := range sortedMapKeys(cfg.HTTPBasic) {
		b := cfg.HTTPBasic[host]
		if b.Password == "" {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "composer_auth", Path: f.Path, Locator: slug(host), Kind: "cred", Admin: true, Site: site,
			Fields: fields("host", host, "user", b.Username, "kind", "http-basic"),
			Secret: credJSON(b.Username, b.Password, host, "", ""),
		})
	}
	tokenMap := func(kind string, m map[string]string) {
		for _, host := range sortedMapKeys(m) {
			if m[host] == "" {
				continue
			}
			hits = append(hits, CredHit{
				Detector: "composer_auth", Path: f.Path, Locator: slug(kind + "-" + host), Kind: "env_var", Admin: true, Site: site,
				Fields: fields("host", host, "kind", kind),
				Secret: m[host],
			})
		}
	}
	tokenMap("github-oauth", cfg.GitHubOAuth)
	tokenMap("gitlab-token", cfg.GitLabToken)
	tokenMap("gitlab-oauth", cfg.GitLabOAuth)
	tokenMap("bearer", cfg.Bearer)
	return hits
}

// ---- maven ----

var mvnServerRe = regexp.MustCompile(`(?is)<server\b[^>]*>(.*?)</server>`)

// mavenSettings reads ~/.m2/settings.xml <servers><server> blocks (id, username,
// password). A {…} Maven-master-encrypted password is stored verbatim, flagged
// encrypted, never decoded. A ${env.*} placeholder is recorded indirect. Normal
// tier. shortcut: no settings-security.xml master-key decode — reveal gives the
// {…} blob; upgrade only if a consumer needs the cleartext.
func mavenSettings(f credFile) []CredHit {
	site := siteSlug(f.Path)
	var hits []CredHit
	for i, m := range mvnServerRe.FindAllStringSubmatch(f.Content, -1) {
		block := m[1]
		pass := xmlElem(block, "password")
		if pass == "" || dynamicValue(pass) {
			continue
		}
		id, user := xmlElem(block, "id"), xmlElem(block, "username")
		loc := slug(id)
		if loc == "" || loc == "x" {
			loc = "server-" + strconv.Itoa(i+1)
		}
		hits = append(hits, CredHit{
			Detector: "maven_settings", Path: f.Path, Locator: loc, Kind: "cred", Site: site,
			Fields: fields("id", id, "user", user, "encrypted", boolStr(strings.HasPrefix(pass, "{") && strings.HasSuffix(pass, "}"))),
			Secret: credJSON(user, pass, "", "", ""),
		})
		if len(hits) >= 50 {
			break
		}
	}
	return hits
}

// ---- pip ----

// pipConf reads /etc/pip.conf, ~/.pip/pip.conf, ~/.config/pip/pip.conf: the
// userinfo of an index-url / extra-index-url (https://user:token@host). Normal
// tier.
func pipConf(f credFile) []CredHit {
	site := siteSlug(f.Path)
	seen := map[string]bool{}
	var hits []CredHit
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		_, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		for _, tok := range strings.Fields(val) {
			u, err := url.Parse(tok)
			if err != nil || u.User == nil {
				continue
			}
			pass, ok := u.User.Password()
			if !ok || pass == "" || dynamicValue(pass) || seen[u.Host+"\x00"+pass] {
				continue
			}
			seen[u.Host+"\x00"+pass] = true
			user := u.User.Username()
			hits = append(hits, CredHit{
				Detector: "pip_conf", Path: f.Path, Locator: slug(firstNonEmpty(u.Host, user)), Kind: "cred", Site: site,
				Fields: fields("host", u.Host, "user", user, "scheme", u.Scheme),
				Secret: credJSON(user, pass, u.Host, "", ""),
			})
		}
	}
	return hits
}

// ---- kubeconfig ----

var (
	kubeTokenRe = regexp.MustCompile(`(?m)^\s*"?token"?\s*:\s*(\S.*)$`)
	kubePassRe  = regexp.MustCompile(`(?m)^\s*"?password"?\s*:\s*(\S.*)$`)
	kubeKeyRe   = regexp.MustCompile(`(?m)^\s*"?client-key-data"?\s*:`)
)

// kubeconfig reads ~/.kube/config, /root/.kube/config, /etc/kubernetes/admin.conf:
// users[*].user.{token,password} ONLY. client-key-data is a private key — it is
// recorded as "not captured" and NEVER read. Admin tier — cluster-admin creds.
// shortcut: field scan, not a YAML tree, so a token is grouped by its own value
// hash, not by its user; upgrade only if per-user grouping is ever needed.
func kubeconfig(f credFile) []CredHit {
	site := siteSlug(f.Path)
	var hits []CredHit
	add := func(kind, val string) {
		val = strings.Trim(strings.TrimSpace(val), `"',`)
		if val == "" || val == "|" || dynamicValue(val) {
			return
		}
		hits = append(hits, CredHit{
			Detector: "kubeconfig", Path: f.Path, Locator: kind + "-" + hash6(val), Kind: "env_var", Admin: true, Site: site,
			Fields: fields("note", "k8s "+kind),
			Secret: val,
		})
	}
	for _, m := range kubeTokenRe.FindAllStringSubmatch(f.Content, -1) {
		add("token", m[1])
	}
	for _, m := range kubePassRe.FindAllStringSubmatch(f.Content, -1) {
		add("password", m[1])
	}
	if kubeKeyRe.MatchString(f.Content) { // recorded for provenance, value never read
		hits = append(hits, CredHit{
			Detector: "kubeconfig", Path: f.Path, Locator: "client-key-data", Kind: "env_var", Admin: true, Site: site,
			Fields: fields("note", "client cert key (not captured)"),
		})
	}
	return hits
}

// ---- systemd EnvironmentFile ----

var systemdEnvFileRe = regexp.MustCompile(`(?m)^\s*EnvironmentFile\s*=\s*-?\s*(\S+)`)

// isSystemdUnit reports a streamed systemd *.service unit file (the index the
// probe streams so Go can find its EnvironmentFile= references).
func isSystemdUnit(p string) bool {
	return strings.HasSuffix(p, ".service") &&
		(strings.HasPrefix(p, "/etc/systemd/system/") ||
			strings.HasPrefix(p, "/lib/systemd/system/") ||
			strings.HasPrefix(p, "/usr/lib/systemd/system/"))
}

// systemdEnvSet returns the absolute paths every streamed unit names with
// EnvironmentFile=, so detectCreds can route those files to the systemd_env
// detector (they are arbitrary paths, like cron scripts and -i keys).
func systemdEnvSet(files []credFile) map[string]bool {
	set := map[string]bool{}
	for _, f := range files {
		if !isSystemdUnit(f.Path) {
			continue
		}
		for _, m := range systemdEnvFileRe.FindAllStringSubmatch(f.Content, -1) {
			if p := strings.Trim(m[1], `"'`); strings.HasPrefix(p, "/") {
				set[p] = true
			}
		}
	}
	return set
}

// systemdEnv parses a file a unit's EnvironmentFile= points at for *_PASSWORD /
// *_TOKEN / DB_* env secrets — the .env detector's allowlist, relabelled.
func systemdEnv(f credFile) []CredHit {
	hits := dotenv(f)
	for i := range hits {
		hits[i].Detector = "systemd_env"
	}
	return hits
}
