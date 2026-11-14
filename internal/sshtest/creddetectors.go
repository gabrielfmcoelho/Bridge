package sshtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// creddetectors.go — the per-detector parsers for the config-file secret probe.
// Pure, static (no app code runs), DB-free, so each has a unit test with
// synthetic fixtures. Each turns one credFile into zero or more CredHits; a hit
// with an empty Secret is an "indirect" reference (getenv/${…}/a constant):
// recorded for provenance, nothing stored.
//
// Phase 1 ships the priority detectors that cover hosts 35/50: ci3_database,
// dotenv, wp_config, mysql_debian_cnf, mysql_root_mycnf, mysql_rpm_temp_root,
// virtualmin_domain, postfix_sasl, cron_embedded, root_ssh_keys. The other
// PHP/DB-admin detectors (ci4, joomla, drupal, moodle, scriptcase, webmin,
// plesk, pgpass, redis, php literal connect) are Phase 1b; Java/Oracle are Phase 2.

// detectCreds dispatches every probed file to its detector. cronScripts and
// keyFiles name the arbitrary-path files whose detector can't be read from the
// path alone (a script cron calls, a key passed with -i).
func detectCreds(files []credFile, cronScripts, keyFiles map[string]bool) []CredHit {
	var hits []CredHit
	for _, f := range files {
		hits = append(hits, detectCredFile(f, cronScripts, keyFiles)...)
	}
	return hits
}

func detectCredFile(f credFile, cronScripts, keyFiles map[string]bool) []CredHit {
	base := path.Base(f.Path)
	dir := path.Dir(f.Path)
	switch {
	case keyFiles[f.Path], looksLikePrivateKey(f.Content):
		return rootSSHKeys(f)
	case cronScripts[f.Path]:
		return cronEmbedded(f)
	case f.Path == "/etc/mysql/debian.cnf":
		return mysqlCnf(f, "mysql_debian_cnf")
	case f.Path == "/root/.my.cnf", strings.HasSuffix(dir, "/my.cnf.d") && strings.HasSuffix(base, ".cnf"):
		return mysqlCnf(f, "mysql_root_mycnf")
	case f.Path == "/var/log/mysqld.log":
		return mysqlTempPassword(f)
	case f.Path == "/etc/postfix/sasl_passwd":
		return postfixSasl(f)
	case strings.Contains(f.Path, "/etc/webmin/virtual-server/domains/"):
		return virtualminDomain(f)
	case strings.HasSuffix(dir, "/.ssh") && strings.HasPrefix(base, "id_") && !strings.HasSuffix(base, ".pub"):
		return rootSSHKeys(f)
	case base == ".env":
		return dotenv(f)
	case base == "wp-config.php":
		return wpConfig(f)
	case base == "database.php" && strings.HasSuffix(dir, "/application/config"):
		return ci3Database(f)
	}
	return nil
}

// ---- helpers ----

var superusers = map[string]bool{"root": true, "sa": true, "postgres": true, "system": true, "admin": true, "mysql": true}

func isSuperuser(user string) bool { return superusers[strings.ToLower(strings.TrimSpace(user))] }

var kebabRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug lowercases and kebab-cases s, bounded, with a leading letter guaranteed.
func slug(s string) string {
	s = strings.Trim(kebabRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" || !(s[0] >= 'a' && s[0] <= 'z') {
		s = "x" + s
	}
	return s
}

func hash6(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:3])
}

// siteSlug is the group-label slug for a context path (an app dir, a config
// file, a fixed system dir): a readable basename plus a hash of the full path,
// so two apps with the same basename never share a group.
func siteSlug(ctxPath string) string {
	return slug(path.Base(ctxPath)) + "-" + hash6(ctxPath)
}

// credJSON builds the cred payload. Password (the secret) is always present;
// the rest are omitted when empty. Field order is fixed, so the same login
// keeps one value fingerprint.
func credJSON(user, pass, host, port, db string) string {
	p := struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password"`
		Host     string `json:"host,omitempty"`
		Port     string `json:"port,omitempty"`
		DB       string `json:"db,omitempty"`
	}{user, pass, host, port, db}
	b, _ := json.Marshal(p)
	return string(b)
}

// fields builds the non-secret field map shown in the tab and stored in the
// scan JSON (never a password). Empty values are dropped.
func fields(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			m[kv[i]] = kv[i+1]
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// dynamicValue reports a value Bridge must not resolve: a PHP/env reference
// (getenv/$var/${…}/constant) or empty. The detector records it as indirect.
func dynamicValue(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || strings.HasPrefix(v, "$") || strings.Contains(v, "${") ||
		strings.Contains(v, "getenv") || strings.Contains(v, "::")
}

func parseINI(content string) map[string]map[string]string {
	out := map[string]map[string]string{}
	section := ""
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			key := strings.ToLower(strings.TrimSpace(k))
			val := strings.Trim(strings.TrimSpace(v), `"'`)
			if out[section] == nil {
				out[section] = map[string]string{}
			}
			out[section][key] = val
		}
	}
	return out
}

func looksLikePrivateKey(content string) bool {
	return strings.Contains(content, "PRIVATE KEY-----")
}

// ---- DB-admin detectors (admin tier) ----

// mysqlCnf reads a MySQL client config ([client] user/password) — debian.cnf
// or /root/.my.cnf / my.cnf.d. Admin tier: it's a DB-admin login.
func mysqlCnf(f credFile, detector string) []CredHit {
	ini := parseINI(f.Content)
	cl := ini["client"]
	if cl == nil {
		return nil
	}
	user, pass := cl["user"], firstNonEmpty(cl["password"], cl["pass"])
	if pass == "" {
		return nil
	}
	host := firstNonEmpty(cl["host"], cl["socket"])
	return []CredHit{{
		Detector: detector, Path: f.Path, Locator: "client", Kind: "cred", Admin: true,
		Site:   siteSlug(f.Path),
		Fields: fields("engine", "mysql", "user", user, "host", host),
		Secret: credJSON(user, pass, cl["host"], cl["port"], ""),
	}}
}

var mysqlTempRe = regexp.MustCompile(`temporary password is generated for ([^\s:@]+)@\S+:\s*(\S+)`)

// mysqlTempPassword reads the MySQL 8 RPM temporary root password from
// mysqld.log. Admin tier. May be expired (flagged in fields).
func mysqlTempPassword(f credFile) []CredHit {
	m := mysqlTempRe.FindStringSubmatch(f.Content)
	if m == nil {
		return nil
	}
	user := m[1]
	return []CredHit{{
		Detector: "mysql_rpm_temp_root", Path: f.Path, Locator: "temp", Kind: "cred", Admin: true,
		Site:   siteSlug("/var/log/mysqld"),
		Fields: fields("engine", "mysql", "user", user, "note", "temporary"),
		Secret: credJSON(user, m[2], "", "", ""),
	}}
}

// virtualminDomain reads a Virtualmin domain file (plain-text password storage
// on): the owner login (pass=) and the domain MySQL user (mysql_pass=). Admin
// tier (panel-managed privileged creds).
func virtualminDomain(f credFile) []CredHit {
	kv := parseKeyValue(f.Content)
	dom, owner := kv["dom"], kv["user"]
	site := siteSlug(f.Path)
	var hits []CredHit
	if pw := kv["pass"]; pw != "" {
		hits = append(hits, CredHit{
			Detector: "virtualmin_domain", Path: f.Path, Locator: "owner", Kind: "cred", Admin: true, Site: site,
			Fields: fields("panel", "virtualmin", "domain", dom, "user", owner),
			Secret: credJSON(owner, pw, "", "", dom),
		})
	}
	if pw := kv["mysql_pass"]; pw != "" {
		myUser := firstNonEmpty(kv["mysql_user"], owner)
		hits = append(hits, CredHit{
			Detector: "virtualmin_domain", Path: f.Path, Locator: "mysql", Kind: "cred", Admin: true, Site: site,
			Fields: fields("engine", "mysql", "domain", dom, "user", myUser),
			Secret: credJSON(myUser, pw, "", "", kv["db"]),
		})
	}
	return hits
}

// ---- host-level detectors ----

// postfixSasl reads /etc/postfix/sasl_passwd: "<relay> <user>:<password>" per
// line. SMTP relay login — the normal reveal tier.
func postfixSasl(f credFile) []CredHit {
	var hits []CredHit
	site := siteSlug("/etc/postfix")
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		relay, rest, ok := strings.Cut(line, " ")
		if !ok {
			if relay, rest, ok = strings.Cut(line, "\t"); !ok {
				continue
			}
		}
		user, pass, ok := strings.Cut(strings.TrimSpace(rest), ":")
		if !ok || pass == "" {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "postfix_sasl", Path: f.Path, Locator: slug(relay), Kind: "cred", Site: site,
			Fields: fields("host", relay, "user", user, "protocol", "smtp"),
			Secret: credJSON(user, pass, relay, "", ""),
		})
	}
	return hits
}

// rootSSHKeys reads a private key (root's outbound key, or one cron passes with
// -i). Admin tier. A passphrase-protected key is stored anyway, flagged
// encrypted (the passphrase is unknown). The host's own keys are never read.
func rootSSHKeys(f credFile) []CredHit {
	if !looksLikePrivateKey(f.Content) {
		return nil // a .pub slipped in, or an unrelated file
	}
	encrypted, pub := "", ""
	if signer, err := gossh.ParsePrivateKey([]byte(f.Content)); err == nil {
		pub = strings.TrimSpace(string(gossh.MarshalAuthorizedKey(signer.PublicKey())))
	} else if pm := (&gossh.PassphraseMissingError{}); errors.As(err, &pm) {
		encrypted = "true"
	}
	payload, _ := json.Marshal(struct {
		Username      string `json:"username"`
		PrivateKeyPEM string `json:"private_key_pem"`
		PublicKey     string `json:"public_key"`
	}{"root", f.Content, pub})
	return []CredHit{{
		Detector: "root_ssh_keys", Path: f.Path, Locator: path.Base(f.Path), Kind: "sshkey", Admin: true,
		Site:   siteSlug(path.Dir(f.Path)),
		Fields: fields("user", "root", "encrypted", encrypted),
		Secret: string(payload),
	}}
}

var cronSecretRe = regexp.MustCompile(`(?i)\b(PGPASSWORD|MYSQL_PWD|RSYNC_PASSWORD)=(\S+)`)
var cronSSHPassRe = regexp.MustCompile(`\bsshpass\s+-p\s*(\S+)`)
var cronMysqlPwRe = regexp.MustCompile(`\bmysql(?:dump|admin)?\b[^\n]*?\s-p(\S+)`)

// cronEmbedded scans a script cron calls for passwords baked into it: the
// mysql/rsync/pg env vars, sshpass -p, and a mysql client's -p<pw>. (A lighter
// scan than the command-line pipeline's FindSecrets, enough for the common
// backup scripts; URL userinfo and the full flag set are Phase 1b.)
func cronEmbedded(f credFile) []CredHit {
	var hits []CredHit
	site := siteSlug(f.Path)
	seen := map[string]bool{}
	add := func(locator, pass string, admin bool) {
		pass = strings.Trim(pass, `"'`)
		if pass == "" || dynamicValue(pass) || seen[locator] {
			return
		}
		seen[locator] = true
		hits = append(hits, CredHit{
			Detector: "cron_embedded", Path: f.Path, Locator: locator, Kind: "env_var", Admin: admin,
			Site: site, Fields: fields("source", "cron-script"), Secret: pass,
		})
	}
	for _, m := range cronSecretRe.FindAllStringSubmatch(f.Content, -1) {
		add(strings.ToLower(m[1]), m[2], false)
	}
	for _, m := range cronSSHPassRe.FindAllStringSubmatch(f.Content, -1) {
		add("sshpass", m[1], false)
	}
	for _, m := range cronMysqlPwRe.FindAllStringSubmatch(f.Content, -1) {
		// A root dump is admin-tier.
		admin := strings.Contains(m[0], "-uroot") || strings.Contains(m[0], "--user=root") || strings.Contains(m[0], "-u root")
		add("mysql_password", m[1], admin)
	}
	return hits
}

// ---- application detectors ----

var dotenvKeyRe = regexp.MustCompile(`(?i)(PASSWORD|PASSWD|SECRET|TOKEN|_KEY$|^KEY$|API_?KEY|PRIVATE_KEY|CREDENTIAL|^DSN$|_DSN$|^DATABASE_URL$)`)

// dotenv reads a .env file, taking allowlisted sensitive keys as env_var
// secrets (DB_*, MAIL_*, *_PASSWORD, *_SECRET, *_KEY, …). Dynamic values
// (${VAR}) are recorded indirect.
func dotenv(f credFile) []CredHit {
	var hits []CredHit
	site := siteSlug(path.Dir(f.Path))
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "export ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !dotenvKeyRe.MatchString(key) {
			continue
		}
		val = dotenvValue(val)
		hit := CredHit{Detector: "dotenv", Path: f.Path, Locator: key, Kind: "env_var", Site: site}
		if !dynamicValue(val) {
			hit.Secret = val
		}
		hits = append(hits, hit)
	}
	return hits
}

// dotenvValue strips a quoted value and any trailing unquoted comment.
func dotenvValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if j := strings.IndexByte(v[1:], v[0]); j >= 0 {
			return v[1 : 1+j]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

var wpDefineRe = regexp.MustCompile(`(?i)define\(\s*['"](DB_USER|DB_PASSWORD|DB_HOST|DB_NAME)['"]\s*,\s*['"]([^'"]*)['"]`)

// wpConfig reads wp-config.php DB_* defines (salts ignored).
func wpConfig(f credFile) []CredHit {
	v := map[string]string{}
	for _, m := range wpDefineRe.FindAllStringSubmatch(f.Content, -1) {
		v[strings.ToUpper(m[1])] = m[2]
	}
	if len(v) == 0 {
		return nil
	}
	user, pass, host, db := v["DB_USER"], v["DB_PASSWORD"], v["DB_HOST"], v["DB_NAME"]
	hit := CredHit{
		Detector: "wp_config", Path: f.Path, Locator: "wp", Kind: "cred", Admin: isSuperuser(user),
		Site:   siteSlug(path.Dir(f.Path)),
		Fields: fields("engine", "mysql", "user", user, "host", host, "db", db),
	}
	if !dynamicValue(pass) {
		hit.Secret = credJSON(user, pass, host, "", db)
	}
	return []CredHit{hit}
}

var (
	ci3AssignRe    = regexp.MustCompile(`\$db\s*\[\s*['"]([A-Za-z0-9_]+)['"]\s*\]\s*\[\s*['"]([a-z_]+)['"]\s*\]\s*=\s*['"]([^'"]*)['"]`)
	ci3GroupArrRe  = regexp.MustCompile(`\$db\s*\[\s*['"]([A-Za-z0-9_]+)['"]\s*\]\s*=\s*array\s*\(`)
	ci3NestGroupRe = regexp.MustCompile(`['"]([A-Za-z0-9_]+)['"]\s*=>\s*array\s*\(`)
	ci3ArrowRe     = regexp.MustCompile(`['"]([a-z_]+)['"]\s*=>\s*['"]([^'"]*)['"]`)
)

var ci3Fields = map[string]bool{"hostname": true, "username": true, "password": true, "database": true, "dbdriver": true, "port": true}

// ci3Database reads CodeIgniter 3 application/config/database.php — both the
// `$db['g']['k'] = …;` and the `$db['g'] = array('k' => …)` / nested-array
// syntaxes, every group. One cred per group with a literal password; a group
// whose password is dynamic (getenv/const) is recorded indirect.
func ci3Database(f credFile) []CredHit {
	groups := map[string]map[string]string{}
	set := func(g, k, v string) {
		if !ci3Fields[k] {
			return
		}
		if groups[g] == nil {
			groups[g] = map[string]string{}
		}
		if _, ok := groups[g][k]; !ok {
			groups[g][k] = v
		}
	}
	// Syntax A: explicit $db['g']['k'] = 'v'.
	for _, m := range ci3AssignRe.FindAllStringSubmatch(f.Content, -1) {
		set(m[1], m[2], m[3])
	}
	// Syntax B: group => array( 'k' => 'v', … ). Balance parens per block.
	for _, loc := range append(ci3GroupArrRe.FindAllStringSubmatchIndex(f.Content, -1),
		ci3NestGroupRe.FindAllStringSubmatchIndex(f.Content, -1)...) {
		g := f.Content[loc[2]:loc[3]]
		block := balancedParen(f.Content, loc[1]-1)
		for _, a := range ci3ArrowRe.FindAllStringSubmatch(block, -1) {
			set(g, a[1], a[2])
		}
	}

	appDir := strings.TrimSuffix(path.Dir(f.Path), "/application/config")
	site := siteSlug(appDir)
	var hits []CredHit
	for _, g := range sortedKeys(groups) {
		fv := groups[g]
		if fv["username"] == "" && fv["password"] == "" && fv["hostname"] == "" {
			continue
		}
		hit := CredHit{
			Detector: "ci3_database", Path: f.Path, Locator: g, Kind: "cred", Admin: isSuperuser(fv["username"]),
			Site:   site,
			Fields: fields("engine", ci3Engine(fv["dbdriver"]), "host", fv["hostname"], "port", fv["port"], "db", fv["database"], "user", fv["username"]),
		}
		if p := fv["password"]; !dynamicValue(p) {
			hit.Secret = credJSON(fv["username"], p, fv["hostname"], fv["port"], fv["database"])
		}
		hits = append(hits, hit)
	}
	return hits
}

func ci3Engine(driver string) string {
	switch {
	case strings.Contains(driver, "mysql"):
		return "mysql"
	case strings.Contains(driver, "postgre"):
		return "postgresql"
	case strings.Contains(driver, "sqlsrv"), strings.Contains(driver, "mssql"):
		return "sqlserver"
	}
	return driver
}

// balancedParen returns the text from just after the '(' at openParen up to its
// matching ')', so a value with its own parens isn't cut short. Bounded.
func balancedParen(s string, openParen int) string {
	if openParen < 0 || openParen >= len(s) || s[openParen] != '(' {
		return ""
	}
	depth := 0
	for i := openParen; i < len(s) && i < openParen+8192; i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return s[openParen+1 : i]
			}
		}
	}
	return ""
}

// ---- small shared utilities ----

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseKeyValue reads "key=value" or "key value" lines (Webmin/Virtualmin).
func parseKeyValue(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var k, v string
		if i := strings.IndexByte(line, '='); i >= 0 {
			k, v = line[:i], line[i+1:]
		} else if i := strings.IndexAny(line, " \t"); i >= 0 {
			k, v = line[:i], line[i+1:]
		} else {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return out
}

func sortedKeys(m map[string]map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Small maps; insertion sort keeps it dependency-free and deterministic.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
