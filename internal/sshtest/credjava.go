package sshtest

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// credjava.go — Phase 2 config-file detectors: Java app servers (Spring Boot,
// Tomcat, WildFly, WebLogic, Hibernate/JDBC) and Oracle tnsnames. Like the
// Phase 1 detectors they are pure and static (no app code runs): each turns one
// credFile into zero or more CredHits, with the plaintext only ever in
// CredHit.Secret (transient). Passwords never go into Fields. DB-admin /
// superuser logins and Tomcat/WildFly/WebLogic admin logins carry Admin=true so
// the recorder files them under the admin-only reveal tier.

// ---- shared parsing helpers ----

var (
	// One XML start-tag attribute, double- or single-quoted.
	xmlAttrRe = regexp.MustCompile(`([A-Za-z_][\w.:-]*)\s*=\s*"([^"]*)"|([A-Za-z_][\w.:-]*)\s*=\s*'([^']*)'`)
	// <property name="K" value="V"/> and <property name="K">V</property>
	propInlineRe = regexp.MustCompile(`(?is)<property\s+[^>]*?name\s*=\s*["']([^"']+)["'][^>]*?\svalue\s*=\s*["']([^"']*)["']`)
	propTextRe   = regexp.MustCompile(`(?is)<property\s+[^>]*?name\s*=\s*["']([^"']+)["'][^>]*?>([^<]*)</property>`)
)

// xmlAttrs parses the attributes of a single XML start-tag fragment (lowercased
// names). Values are returned verbatim (no entity decoding — a password rarely
// needs it, and a mangled value is safer than a wrong one).
func xmlAttrs(tag string) map[string]string {
	m := map[string]string{}
	for _, a := range xmlAttrRe.FindAllStringSubmatch(tag, -1) {
		if a[1] != "" {
			m[strings.ToLower(a[1])] = a[2]
		} else {
			m[strings.ToLower(a[3])] = a[4]
		}
	}
	return m
}

// xmlElem returns the trimmed inner text of the first <name>…</name> in block.
func xmlElem(block, name string) string {
	re := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(name) + `\b[^>]*>(.*?)</` + regexp.QuoteMeta(name) + `>`)
	if m := re.FindStringSubmatch(block); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func reFirst(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// parseProperties reads a flat Java .properties file (key=value or key:value,
// `#`/`!` comments, `\` line continuation) into lowercased keys.
func parseProperties(content string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		for strings.HasSuffix(line, `\`) && i+1 < len(lines) {
			line = strings.TrimSuffix(line, `\`) + strings.TrimSpace(lines[i+1])
			i++
		}
		j := strings.IndexAny(line, "=:")
		if j < 0 {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(line[:j]))] = strings.TrimSpace(line[j+1:])
	}
	return out
}

// flattenYAML turns the simple nested maps Spring config uses into dotted,
// lowercased keys. It handles `key:` nesting and `key: value` leaves only — no
// lists, anchors, flow maps or block scalars (enough for datasource/mail/redis;
// anything fancier is skipped, never mis-read). shortcut: no YAML lists/anchors,
// upgrade if a real parser is ever needed.
func flattenYAML(content string) map[string]string {
	out := map[string]string{}
	type lvl struct {
		indent int
		key    string
	}
	var stack []lvl
	for _, raw := range strings.Split(content, "\n") {
		t := strings.TrimSpace(raw)
		if t == "" || strings.HasPrefix(t, "#") || t == "-" || strings.HasPrefix(t, "- ") {
			continue
		}
		key, val, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, lvl{indent, strings.TrimSpace(key)})
		val = stripYAMLComment(strings.TrimSpace(val))
		if val == "" {
			continue
		}
		parts := make([]string, len(stack))
		for i, s := range stack {
			parts[i] = s.key
		}
		out[strings.ToLower(strings.Join(parts, "."))] = unquote(val)
	}
	return out
}

// stripYAMLComment drops a trailing ` # comment`, respecting a quoted value.
func stripYAMLComment(v string) string {
	if v == "" {
		return v
	}
	if v[0] == '"' || v[0] == '\'' {
		if j := strings.IndexByte(v[1:], v[0]); j >= 0 {
			return v[:j+2]
		}
		return v
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// unquote strips a single pair of surrounding quotes.
func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

// pickBySuffix returns the value of the key whose last dot/underscore segment
// equals one of suffixes (sorted keys → deterministic on ties).
func pickBySuffix(props map[string]string, suffixes ...string) string {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, s := range suffixes {
		for _, k := range keys {
			seg := k
			if i := strings.LastIndexAny(k, "._"); i >= 0 {
				seg = k[i+1:]
			}
			if seg == s && props[k] != "" {
				return props[k]
			}
		}
	}
	return ""
}

// jdbcEngine names the DB engine from a JDBC URL or driver class.
func jdbcEngine(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "mysql"), strings.Contains(s, "mariadb"):
		return "mysql"
	case strings.Contains(s, "postgre"):
		return "postgresql"
	case strings.Contains(s, "oracle"):
		return "oracle"
	case strings.Contains(s, "sqlserver"), strings.Contains(s, "mssql"), strings.Contains(s, "jtds"):
		return "sqlserver"
	case strings.Contains(s, "h2:"):
		return "h2"
	}
	return ""
}

// indirectNote describes a password Bridge must not resolve: a WildFly vault
// expression or any ${…} placeholder. "" when the value is a real literal.
func indirectNote(pass string) string {
	switch {
	case strings.Contains(pass, "${VAULT::"):
		return "vault expression"
	case strings.Contains(pass, "${"):
		return "indirect"
	}
	return ""
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return ""
}

// dbHit builds one JDBC-style CredHit, recording an indirect note when the
// password is a reference rather than a literal.
func dbHit(detector, path, locator, site, user, pass, url string, admin bool, extra ...string) CredHit {
	fv := append([]string{"engine", jdbcEngine(url), "user", user, "url", url}, extra...)
	hit := CredHit{
		Detector: detector, Path: path, Locator: locator, Kind: "cred", Admin: admin, Site: site,
	}
	if pass != "" && !dynamicValue(pass) {
		hit.Secret = credJSON(user, pass, "", "", "")
	} else if n := indirectNote(pass); n != "" {
		fv = append(fv, "note", "password "+n)
	}
	hit.Fields = fields(fv...)
	return hit
}

// ---- Spring Boot ----

// springProps reads application.properties / .yml (incl. a Spring Boot fat
// jar's BOOT-INF/classes member, path "<jar>!/…") for the datasource, mail and
// redis logins, every profile variant under its own locator.
func springProps(f credFile) []CredHit {
	base := path.Base(f.Path)
	var flat map[string]string
	if strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml") {
		flat = flattenYAML(f.Content)
	} else {
		flat = parseProperties(f.Content)
	}
	site := siteSlug(springSite(f.Path))
	suffix := ""
	if prof := springProfile(base); prof != "" {
		suffix = "-" + slug(prof)
	}
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := flat[k]; v != "" {
				return v
			}
		}
		return ""
	}

	var hits []CredHit
	dsURL := get("spring.datasource.url", "spring.datasource.jdbc-url")
	dsUser := get("spring.datasource.username", "spring.datasource.user")
	dsPass := get("spring.datasource.password")
	if dsURL != "" || dsUser != "" || dsPass != "" {
		hits = append(hits, dbHit("spring_props", f.Path, "datasource"+suffix, site, dsUser, dsPass, dsURL, isSuperuser(dsUser)))
	}
	if mp := get("spring.mail.password"); mp != "" {
		mUser := get("spring.mail.username")
		hit := CredHit{Detector: "spring_props", Path: f.Path, Locator: "mail" + suffix, Kind: "cred", Site: site,
			Fields: fields("protocol", "smtp", "host", get("spring.mail.host"), "port", get("spring.mail.port"), "user", mUser)}
		if !dynamicValue(mp) {
			hit.Secret = credJSON(mUser, mp, get("spring.mail.host"), get("spring.mail.port"), "")
		}
		hits = append(hits, hit)
	}
	if rp := get("spring.redis.password", "spring.data.redis.password"); rp != "" {
		rHost := get("spring.redis.host", "spring.data.redis.host")
		rPort := get("spring.redis.port", "spring.data.redis.port")
		hit := CredHit{Detector: "spring_props", Path: f.Path, Locator: "redis" + suffix, Kind: "cred", Site: site,
			Fields: fields("engine", "redis", "host", rHost, "port", rPort)}
		if !dynamicValue(rp) {
			hit.Secret = credJSON("", rp, rHost, rPort, "")
		}
		hits = append(hits, hit)
	}
	return hits
}

// springProfile is the {profile} of application-{profile}.{ext}, else "".
func springProfile(base string) string {
	b := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, ".properties"), ".yaml"), ".yml")
	if p := strings.TrimPrefix(b, "application-"); p != b {
		return p
	}
	return ""
}

// springSite is the app identity for the group label: the fat jar itself, else
// the config dir minus a /config or /BOOT-INF/classes suffix.
func springSite(p string) string {
	if i := strings.Index(p, "!/"); i >= 0 {
		return p[:i]
	}
	d := path.Dir(p)
	d = strings.TrimSuffix(d, "/BOOT-INF/classes")
	d = strings.TrimSuffix(d, "/config")
	return d
}

// ---- Tomcat ----

var resourceTagRe = regexp.MustCompile(`(?is)<Resource\b([^>]*?)/?>`)

// tomcatResource reads <Resource … username password url> JDBC DataSources from
// context.xml / server.xml / conf/Catalina/*/*.xml. TLS <Connector> keystore
// passwords are not matched (only <Resource>).
func tomcatResource(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for i, m := range resourceTagRe.FindAllStringSubmatch(f.Content, -1) {
		at := xmlAttrs(m[1])
		user := firstNonEmpty(at["username"], at["user"])
		pass := at["password"]
		url := firstNonEmpty(at["url"], at["jdbcurl"])
		if pass == "" && url == "" && user == "" {
			continue
		}
		loc := slug(strings.TrimPrefix(firstNonEmpty(at["name"], ""), "jdbc/"))
		if loc == "x" || loc == "" {
			loc = "resource-" + strconv.Itoa(i+1)
		}
		extra := []string{}
		if d := at["driverclassname"]; d != "" && jdbcEngine(url) == "" {
			extra = append(extra, "engine", jdbcEngine(d))
		}
		hits = append(hits, dbHit("tomcat_resource", f.Path, loc, site, user, pass, url, isSuperuser(user), extra...))
	}
	return hits
}

var userTagRe = regexp.MustCompile(`(?is)<user\b([^>]*?)/?>`)

// tomcatUsers reads conf/tomcat-users.xml, capturing only users holding a
// manager-* / admin-* role (the privileged manager/host-manager logins). Admin
// tier — a Tomcat manager login deploys arbitrary code.
func tomcatUsers(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for _, m := range userTagRe.FindAllStringSubmatch(f.Content, -1) {
		at := xmlAttrs(m[1])
		user, pass, roles := at["username"], at["password"], at["roles"]
		if user == "" || pass == "" || dynamicValue(pass) || !tomcatPrivileged(roles) {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "tomcat_users", Path: f.Path, Locator: slug(user), Kind: "cred", Admin: true, Site: site,
			Fields: fields("panel", "tomcat", "user", user, "roles", roles),
			Secret: credJSON(user, pass, "", "", ""),
		})
	}
	return hits
}

func tomcatPrivileged(roles string) bool {
	for _, r := range strings.Split(roles, ",") {
		r = strings.TrimSpace(strings.ToLower(r))
		if strings.HasPrefix(r, "manager") || strings.HasPrefix(r, "admin") {
			return true
		}
	}
	return false
}

// ---- WildFly / JBoss EAP ----

var dsBlockRe = regexp.MustCompile(`(?is)<datasource\b([^>]*)>(.*?)</datasource>`)

// wildflyDS reads standalone*.xml <datasource> blocks (connection-url,
// user-name, password). A ${VAULT::…} or ${env} password is recorded indirect,
// never resolved.
func wildflyDS(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for i, m := range dsBlockRe.FindAllStringSubmatch(f.Content, -1) {
		at, block := xmlAttrs(m[1]), m[2]
		url := xmlElem(block, "connection-url")
		user := xmlElem(block, "user-name")
		pass := xmlElem(block, "password")
		if url == "" && user == "" && pass == "" {
			continue
		}
		loc := slug(firstNonEmpty(at["pool-name"], at["jndi-name"]))
		if loc == "x" || loc == "" {
			loc = "ds-" + strconv.Itoa(i+1)
		}
		hits = append(hits, dbHit("wildfly_ds", f.Path, loc, site, user, pass, url, isSuperuser(user)))
	}
	return hits
}

// ---- WebLogic (Oracle Linux) ----

var wlPropRe = regexp.MustCompile(`(?is)<property>\s*<name>([^<]*)</name>\s*<value>([^<]*)</value>\s*</property>`)

// weblogicJDBC reads config/jdbc/*-jdbc.xml and boot.properties. A WebLogic
// password is `{AES}…`, encrypted with the domain's own key: it is recorded as
// the secret with encrypted=true and NEVER decrypted (there is nothing here to
// decrypt it with, and that is the point).
func weblogicJDBC(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	if path.Base(f.Path) == "boot.properties" {
		kv := parseKeyValue(f.Content)
		pass := kv["password"]
		if pass == "" {
			return nil
		}
		return []CredHit{{
			Detector: "weblogic_jdbc", Path: f.Path, Locator: "boot", Kind: "cred", Admin: true, Site: site,
			Fields: fields("panel", "weblogic", "encrypted", boolStr(strings.HasPrefix(pass, "{AES}"))),
			Secret: credJSON("", pass, "", "", ""),
		}}
	}
	url := xmlElem(f.Content, "url")
	user := weblogicProp(f.Content, "user")
	pass := firstNonEmpty(xmlElem(f.Content, "password-encrypted"), xmlElem(f.Content, "password"))
	if url == "" && user == "" && pass == "" {
		return nil
	}
	loc := slug(xmlElem(f.Content, "name"))
	if loc == "x" || loc == "" {
		loc = "jdbc"
	}
	hit := CredHit{
		Detector: "weblogic_jdbc", Path: f.Path, Locator: loc, Kind: "cred", Admin: isSuperuser(user), Site: site,
		Fields: fields("engine", jdbcEngine(url), "user", user, "url", url, "encrypted", boolStr(strings.HasPrefix(pass, "{AES}"))),
	}
	if pass != "" {
		hit.Secret = credJSON(user, pass, "", "", "") // {AES}… stored verbatim, not decoded
	}
	return []CredHit{hit}
}

// weblogicProp returns the <value> of the <property> whose <name> is key.
func weblogicProp(content, key string) string {
	for _, m := range wlPropRe.FindAllStringSubmatch(content, -1) {
		if strings.EqualFold(strings.TrimSpace(m[1]), key) {
			return strings.TrimSpace(m[2])
		}
	}
	return ""
}

// ---- Hibernate / plain JDBC properties ----

// hibernateJDBC reads hibernate.cfg.xml, persistence.xml and jdbc/db.properties
// for the connection url / user / password, by key suffix (hibernate.connection.*,
// javax.persistence.jdbc.*, jdbc.*, db.*, or bare url/username/password).
func hibernateJDBC(f credFile) []CredHit {
	var props map[string]string
	if strings.HasSuffix(path.Base(f.Path), ".xml") {
		props = hibernateXMLProps(f.Content)
	} else {
		props = parseProperties(f.Content)
	}
	url := pickBySuffix(props, "url")
	user := pickBySuffix(props, "username", "user")
	pass := pickBySuffix(props, "password", "pass")
	if url == "" && user == "" && pass == "" {
		return nil
	}
	return []CredHit{dbHit("hibernate_jdbc_props", f.Path, "jdbc", siteSlug(path.Dir(f.Path)), user, pass, url, isSuperuser(user))}
}

// hibernateXMLProps reads <property name="K" value="V"/> and
// <property name="K">V</property> into lowercased keys (inline value wins).
func hibernateXMLProps(content string) map[string]string {
	m := map[string]string{}
	for _, x := range propInlineRe.FindAllStringSubmatch(content, -1) {
		m[strings.ToLower(x[1])] = x[2]
	}
	for _, x := range propTextRe.FindAllStringSubmatch(content, -1) {
		if k := strings.ToLower(x[1]); m[k] == "" {
			m[k] = strings.TrimSpace(x[2])
		}
	}
	return m
}

// ---- Oracle tnsnames ----

var (
	tnsAliasRe   = regexp.MustCompile(`(?im)^[ \t]*([A-Za-z0-9_.]+)[ \t]*=\s*(\()`)
	tnsHostRe    = regexp.MustCompile(`(?i)HOST\s*=\s*([^)\s]+)`)
	tnsPortRe    = regexp.MustCompile(`(?i)PORT\s*=\s*(\d+)`)
	tnsServiceRe = regexp.MustCompile(`(?i)(?:SERVICE_NAME|SID)\s*=\s*([^)\s]+)`)
)

// oracleTNS reads tnsnames.ora connection metadata (host / port / service per
// alias) — not a secret, so each hit has no Secret (recorded for Atlas linking).
func oracleTNS(f credFile) []CredHit {
	site := siteSlug(path.Dir(f.Path))
	var hits []CredHit
	for _, loc := range tnsAliasRe.FindAllStringSubmatchIndex(f.Content, -1) {
		alias := f.Content[loc[2]:loc[3]]
		block := balancedParen(f.Content, loc[4]) // loc[4] = index of the '('
		host := reFirst(tnsHostRe, block)
		if host == "" && reFirst(tnsServiceRe, block) == "" {
			continue
		}
		hits = append(hits, CredHit{
			Detector: "oracle_tns", Path: f.Path, Locator: slug(alias), Kind: "cred", Site: site,
			Fields: fields("engine", "oracle", "alias", alias, "host", host,
				"port", reFirst(tnsPortRe, block), "service", reFirst(tnsServiceRe, block)),
		})
		if len(hits) >= 50 {
			break
		}
	}
	return hits
}
