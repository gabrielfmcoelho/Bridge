package sshtest

import (
	"path"
	"regexp"
	"strings"
)

// credphp.go — Phase 1b config-file detectors: the PHP/CMS frameworks
// (CodeIgniter 4, CodeIgniter email, Joomla, Drupal, Moodle, ScriptCase, a
// literal PHP connect call) and the DB-admin / host-level stores (Webmin,
// Plesk, .pgpass, Redis). Like Phase 1/2 they are pure and static (no app code
// runs): each turns one credFile into zero or more CredHits, the plaintext only
// ever in CredHit.Secret (transient). Passwords never go into Fields. DB-admin
// logins (Webmin, Plesk, .pgpass) and superuser app logins carry Admin=true so
// the recorder files them under the admin-only reveal tier.

// ---- shared PHP parsing ----

var (
	// 'key' => value, where value is a single/double-quoted string, or a
	// bareword/number, or a function call (env(), getenv() — group 5 is its '(').
	phpArrowRe = regexp.MustCompile(`['"]([A-Za-z_][A-Za-z0-9_]*)['"]\s*=>\s*(?:'([^']*)'|"([^"]*)"|([A-Za-z0-9_.]+)\s*(\()?)`)
	// $config['key'] = value (CodeIgniter config arrays).
	ciConfigRe = regexp.MustCompile(`\$config\s*\[\s*['"]([a-z_]+)['"]\s*\]\s*=\s*(?:'([^']*)'|"([^"]*)"|([A-Za-z0-9_.]+)\s*(\()?)`)
	// $CFG->key = 'value' (Moodle).
	phpCfgRe = regexp.MustCompile(`\$CFG->(\w+)\s*=\s*(?:'([^']*)'|"([^"]*)")`)
	// [public] $key = 'value' (Joomla JConfig).
	joomlaRe = regexp.MustCompile(`(?:public\s+)?\$(\w+)\s*=\s*(?:'([^']*)'|"([^"]*)")`)
	// [public] [array] $default = [ … ] / array( … ) (CodeIgniter 4).
	ci4DefaultRe = regexp.MustCompile(`\$default\s*=\s*(\[|array\s*\()`)
	// $databases['g1']['g2'] = [ … ] / array( … ) (Drupal).
	drupalDBRe = regexp.MustCompile(`\$databases\s*\[\s*['"]([^'"]+)['"]\s*\]\s*\[\s*['"]([^'"]+)['"]\s*\]\s*=\s*(\[|array\s*\()`)
)

// arrowVal picks the value captured by phpArrowRe / ciConfigRe: the quoted
// string, else a function call → "$dynamic" (indirect, dynamicValue catches it),
// else the bareword/number.
func arrowVal(a []string) string {
	switch {
	case a[2] != "":
		return a[2]
	case a[3] != "":
		return a[3]
	case a[5] == "(": // env('…') / getenv('…') — the real value is elsewhere
		return "$dynamic"
	default:
		return a[4]
	}
}

// phpArray reads a PHP array block ('key' => value, …) into lowercased keys
// (first wins), function-call values recorded as indirect.
func phpArray(block string) map[string]string {
	m := map[string]string{}
	for _, a := range phpArrowRe.FindAllStringSubmatch(block, -1) {
		if k := strings.ToLower(a[1]); m[k] == "" {
			m[k] = arrowVal(a)
		}
	}
	return m
}

// dbEngine names a DB engine from a driver / dbtype / connection-type string.
func dbEngine(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "mysql"), strings.Contains(s, "maria"):
		return "mysql"
	case strings.Contains(s, "postgre"), strings.Contains(s, "pgsql"):
		return "postgresql"
	case strings.Contains(s, "oracle"), strings.Contains(s, "oci"):
		return "oracle"
	case strings.Contains(s, "sqlsrv"), strings.Contains(s, "mssql"), strings.Contains(s, "sqlserver"):
		return "sqlserver"
	case strings.Contains(s, "sqlite"):
		return "sqlite"
	}
	return ""
}

// ---- CodeIgniter 4 ----

// ci4Database reads app/Config/Database.php's $default connection array. The
// password often lives in .env (env('database.default.password')): that is
// recorded indirect here and captured as an env_var by the dotenv detector.
func ci4Database(f credFile) []CredHit {
	loc := ci4DefaultRe.FindStringSubmatchIndex(f.Content)
	if loc == nil {
		return nil
	}
	fv := phpArray(balancedParen(f.Content, loc[1]-1)) // loc[1]-1 = the '[' or '('
	user := firstNonEmpty(fv["username"], fv["user"])
	pass := firstNonEmpty(fv["password"], fv["pass"])
	host := firstNonEmpty(fv["hostname"], fv["host"])
	db := fv["database"]
	if user == "" && pass == "" && host == "" {
		return nil
	}
	appDir := strings.TrimSuffix(path.Dir(f.Path), "/app/Config")
	hit := CredHit{
		Detector: "ci4_database", Path: f.Path, Locator: "default", Kind: "cred", Admin: isSuperuser(user),
		Site:   siteSlug(appDir),
		Fields: fields("engine", dbEngine(fv["dbdriver"]), "host", host, "port", fv["port"], "db", db, "user", user),
	}
	if !dynamicValue(pass) {
		hit.Secret = credJSON(user, pass, host, fv["port"], db)
	}
	return []CredHit{hit}
}

// ---- CodeIgniter email (SMTP) ----

// ciEmail reads application/config/email.php ($config['smtp_*']). SMTP relay
// login — the normal reveal tier.
func ciEmail(f credFile) []CredHit {
	kv := map[string]string{}
	for _, m := range ciConfigRe.FindAllStringSubmatch(f.Content, -1) {
		if k := strings.ToLower(m[1]); kv[k] == "" {
			kv[k] = arrowVal(m)
		}
	}
	pass := kv["smtp_pass"]
	if pass == "" {
		return nil
	}
	user, host, port := kv["smtp_user"], kv["smtp_host"], kv["smtp_port"]
	appDir := strings.TrimSuffix(path.Dir(f.Path), "/application/config")
	hit := CredHit{
		Detector: "ci_email", Path: f.Path, Locator: "smtp", Kind: "cred", Site: siteSlug(appDir),
		Fields: fields("protocol", "smtp", "host", host, "port", port, "user", user),
	}
	if !dynamicValue(pass) {
		hit.Secret = credJSON(user, pass, host, port, "")
	}
	return []CredHit{hit}
}

// ---- Joomla ----

// joomlaConfig reads configuration.php (JConfig): the DB login ($host/$user/
// $password/$db) and, when present, the SMTP login ($smtphost/$smtpuser/
// $smtppass). DB login is admin tier only when its user is a DB superuser.
func joomlaConfig(f credFile) []CredHit {
	kv := map[string]string{}
	for _, m := range joomlaRe.FindAllStringSubmatch(f.Content, -1) {
		v := m[2]
		if v == "" {
			v = m[3]
		}
		if k := strings.ToLower(m[1]); kv[k] == "" {
			kv[k] = v
		}
	}
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	if kv["password"] != "" || kv["user"] != "" {
		user, pass, host, db := kv["user"], kv["password"], kv["host"], kv["db"]
		hit := CredHit{
			Detector: "joomla_config", Path: f.Path, Locator: "database", Kind: "cred", Admin: isSuperuser(user), Site: site,
			Fields: fields("engine", dbEngine(kv["dbtype"]), "host", host, "db", db, "user", user),
		}
		if !dynamicValue(pass) {
			hit.Secret = credJSON(user, pass, host, "", db)
		}
		hits = append(hits, hit)
	}
	if sp := kv["smtppass"]; sp != "" {
		host, user, port := kv["smtphost"], kv["smtpuser"], kv["smtpport"]
		hit := CredHit{
			Detector: "joomla_config", Path: f.Path, Locator: "smtp", Kind: "cred", Site: site,
			Fields: fields("protocol", "smtp", "host", host, "port", port, "user", user),
		}
		if !dynamicValue(sp) {
			hit.Secret = credJSON(user, sp, host, port, "")
		}
		hits = append(hits, hit)
	}
	return hits
}

// ---- Drupal ----

// drupalSettings reads sites/*/settings.php $databases['g1']['g2'] = [ … ]
// connection arrays (every connection). Admin tier only for a superuser login.
func drupalSettings(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for _, loc := range drupalDBRe.FindAllStringSubmatchIndex(f.Content, -1) {
		g1, g2 := f.Content[loc[2]:loc[3]], f.Content[loc[4]:loc[5]]
		fv := phpArray(balancedParen(f.Content, loc[1]-1))
		user, pass := fv["username"], fv["password"]
		if user == "" && pass == "" {
			continue
		}
		host, port, db := fv["host"], fv["port"], fv["database"]
		hit := CredHit{
			Detector: "drupal_settings", Path: f.Path, Locator: slug(g1 + "-" + g2), Kind: "cred", Admin: isSuperuser(user), Site: site,
			Fields: fields("engine", dbEngine(fv["driver"]), "host", host, "port", port, "db", db, "user", user),
		}
		if !dynamicValue(pass) {
			hit.Secret = credJSON(user, pass, host, port, db)
		}
		if hits = append(hits, hit); len(hits) >= 20 {
			break
		}
	}
	return hits
}

// ---- Moodle ----

// moodleConfig reads config.php $CFG->db* (dbhost/dbname/dbuser/dbpass/dbtype).
// Returns nil for any config.php that isn't a Moodle one (no $CFG db vars).
func moodleConfig(f credFile) []CredHit {
	kv := map[string]string{}
	for _, m := range phpCfgRe.FindAllStringSubmatch(f.Content, -1) {
		v := m[2]
		if v == "" {
			v = m[3]
		}
		if k := strings.ToLower(m[1]); kv[k] == "" {
			kv[k] = v
		}
	}
	user, pass := kv["dbuser"], kv["dbpass"]
	if user == "" && pass == "" {
		return nil
	}
	host, db := kv["dbhost"], kv["dbname"]
	hit := CredHit{
		Detector: "moodle_config", Path: f.Path, Locator: "db", Kind: "cred", Admin: isSuperuser(user),
		Site:   siteSlug(path.Dir(f.Path)),
		Fields: fields("engine", dbEngine(kv["dbtype"]), "host", host, "db", db, "user", user),
	}
	if !dynamicValue(pass) {
		hit.Secret = credJSON(user, pass, host, "", db)
	}
	return []CredHit{hit}
}

// ---- ScriptCase ----

// scriptcaseConn reads a ScriptCase conexao.ini connection file. ScriptCase
// scrambles the password with its own reversible encoding: v1 stores it
// VERBATIM and flags encoded=true — it is never decoded here (there is nothing
// to gain by half-decoding a value we store behind a reveal gate anyway).
func scriptcaseConn(f credFile) []CredHit {
	ini := parseINI(f.Content)
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for _, sec := range sortedKeys(ini) {
		kv := ini[sec]
		pass := firstNonEmpty(kv["pass"], kv["password"], kv["senha"])
		if pass == "" {
			continue
		}
		user := firstNonEmpty(kv["user"], kv["usuario"], kv["login"])
		host := firstNonEmpty(kv["host"], kv["server"], kv["servidor"])
		db := firstNonEmpty(kv["nome_base"], kv["dbname"], kv["database"], kv["base"])
		hits = append(hits, CredHit{
			Detector: "scriptcase_conn", Path: f.Path, Locator: slug(sec), Kind: "cred", Admin: isSuperuser(user), Site: site,
			Fields: fields("engine", dbEngine(firstNonEmpty(kv["tp_conexao"], kv["type"])), "host", host, "db", db, "user", user, "encoded", "true"),
			Secret: credJSON(user, pass, host, "", db), // encoded blob, stored as-is
		})
	}
	return hits
}

// ---- Webmin (DB admin) ----

// webminDB reads /etc/webmin/{mysql,postgresql}/config (login=, pass=). Admin
// tier — Webmin's stored DB-admin login.
func webminDB(f credFile, detector, engine string) []CredHit {
	kv := parseKeyValue(f.Content)
	pass := firstNonEmpty(kv["pass"], kv["password"])
	if pass == "" {
		return nil
	}
	user, host := firstNonEmpty(kv["login"], kv["user"]), kv["host"]
	return []CredHit{{
		Detector: detector, Path: f.Path, Locator: "login", Kind: "cred", Admin: true,
		Site:   siteSlug(f.Path),
		Fields: fields("engine", engine, "user", user, "host", host),
		Secret: credJSON(user, pass, host, kv["port"], ""),
	}}
}

// ---- Plesk (DB admin) ----

// pleskAdmin reads /etc/psa/.psa.shadow: the whole file is the admin password
// (user "admin"). Some Plesk builds store it as {AES}/$AES…; stored verbatim,
// flagged encrypted, never decoded. Admin tier.
func pleskAdmin(f credFile) []CredHit {
	pass := strings.TrimSpace(f.Content)
	if pass == "" {
		return nil
	}
	return []CredHit{{
		Detector: "plesk_admin", Path: f.Path, Locator: "admin", Kind: "cred", Admin: true,
		Site:   siteSlug(f.Path),
		Fields: fields("panel", "plesk", "user", "admin", "encrypted", boolStr(strings.HasPrefix(pass, "$AES") || strings.HasPrefix(pass, "{AES}"))),
		Secret: credJSON("admin", pass, "", "", ""),
	}}
}

// ---- .pgpass (DB admin) ----

// pgpass reads a PostgreSQL password file (host:port:db:user:password per line,
// '\' escapes a ':' or '\'). Admin tier — it is /root/.pgpass or ~postgres's.
func pgpass(f credFile) []CredHit {
	site := siteSlug(f.Path)
	var hits []CredHit
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fld := splitPgpass(line)
		if len(fld) != 5 {
			continue
		}
		host, port, db, user, pass := fld[0], fld[1], fld[2], fld[3], fld[4]
		if pass == "" {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "pgpass", Path: f.Path, Locator: slug(host + "-" + user), Kind: "cred", Admin: true, Site: site,
			Fields: fields("engine", "postgresql", "host", host, "port", port, "db", db, "user", user),
			Secret: credJSON(user, pass, host, port, db),
		})
		if len(hits) >= 50 {
			break
		}
	}
	return hits
}

// splitPgpass splits a .pgpass line on unescaped ':' (a '\' escapes the next
// character). A field is '*' for any, so only the password (last field) matters.
func splitPgpass(line string) []string {
	var out []string
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if i+1 < len(line) {
				b.WriteByte(line[i+1])
				i++
			}
		case ':':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(line[i])
		}
	}
	return append(out, b.String())
}

// ---- Redis ----

// redisConf reads redis.conf (requirepass / masterauth) or a Redis ACL file
// (>plaintext passwords; #hashes are skipped). A Redis AUTH password is a
// standalone value (env_var), the normal reveal tier.
func redisConf(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	if strings.HasSuffix(path.Base(f.Path), ".acl") {
		return redisACL(f, site)
	}
	var hits []CredHit
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, " ")
		if key = strings.ToLower(key); !ok || (key != "requirepass" && key != "masterauth") {
			continue
		}
		pass := strings.Trim(strings.TrimSpace(val), `"'`)
		if pass == "" || dynamicValue(pass) {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "redis_conf", Path: f.Path, Locator: key, Kind: "env_var", Site: site,
			Fields: fields("engine", "redis"), Secret: pass,
		})
	}
	return hits
}

// redisACL reads an aclfile (user <name> … >password): only a '>' token is a
// plaintext password; '#<sha256>' hashes are never captured.
func redisACL(f credFile, site string) []CredHit {
	var hits []CredHit
	for _, line := range strings.Split(f.Content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "user ") {
			continue
		}
		toks := strings.Fields(line)
		user := ""
		if len(toks) >= 2 {
			user = toks[1]
		}
		for _, tok := range toks {
			if strings.HasPrefix(tok, ">") && len(tok) > 1 {
				hits = append(hits, CredHit{
					Detector: "redis_conf", Path: f.Path, Locator: slug("acl-" + user), Kind: "env_var", Site: site,
					Fields: fields("engine", "redis", "user", user), Secret: tok[1:],
				})
			}
		}
	}
	return hits
}

// ---- literal PHP connect calls ----

var (
	mysqliConnectRe = regexp.MustCompile(`mysqli_connect\s*\(\s*'([^']*)'\s*,\s*'([^']*)'\s*,\s*'([^']*)'(?:\s*,\s*'([^']*)')?`)
	pdoConnectRe    = regexp.MustCompile(`new\s+PDO\s*\(\s*['"]([a-z]+):([^'"]*)['"]\s*,\s*'([^']*)'\s*,\s*'([^']*)'`)
)

// phpLiteralConnect reads mysqli_connect('h','u','p'[,'db']) and
// new PDO('mysql:host=…;dbname=…','u','p') calls with LITERAL string arguments
// (a variable argument is skipped — only literals are captured). Low confidence:
// the same file may hold several, and the args are positional.
func phpLiteralConnect(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	add := func(engine, host, db, user, pass string) {
		if pass == "" || user == "" {
			return
		}
		hits = append(hits, CredHit{
			Detector: "php_literal_connect", Path: f.Path, Locator: slug(user) + "-" + hash6(host+db+user), Kind: "cred", Admin: isSuperuser(user), Site: site,
			Fields: fields("engine", engine, "host", host, "db", db, "user", user, "confidence", "low"),
			Secret: credJSON(user, pass, host, "", db),
		})
	}
	for _, m := range mysqliConnectRe.FindAllStringSubmatch(f.Content, -1) {
		add("mysql", m[1], m[4], m[2], m[3])
	}
	for _, m := range pdoConnectRe.FindAllStringSubmatch(f.Content, -1) {
		add(dbEngine(m[1]), pdoDSNField(m[2], "host"), pdoDSNField(m[2], "dbname"), m[3], m[4])
		if len(hits) >= 50 {
			break
		}
	}
	return hits
}

// pdoDSNField reads host= / dbname= from a PDO DSN tail (host=h;dbname=d;…).
func pdoDSNField(dsn, key string) string {
	for _, part := range strings.Split(dsn, ";") {
		if k, v, ok := strings.Cut(part, "="); ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
