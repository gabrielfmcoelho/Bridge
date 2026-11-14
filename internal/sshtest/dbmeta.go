package sshtest

import (
	"encoding/base64"
	"log"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// dbmeta.go — the A5 database-metadata probe. Root-only, CaptureJob-only.
// Running as root on the VM it connects to the local MySQL / MariaDB through
// the socket (or /etc/mysql/debian.cnf, /root/.my.cnf — the DB-admin creds
// Phase 1 captured) and reads information_schema + a few status variables.
// ONLY metadata leaves the VM — schema and table names, sizes, account
// identities (user@host), the empty-password flag, @@bind_address, binlog
// expiry and denied-login counts by source class. No row value, no password,
// no authentication_string, and no client IP ever crosses the boundary.
//
// service.RecordScanDatabases turns the schemas into Atlas database rows.
// Modelled on deepfacts.go: one root session, base64 markers so an identifier
// can never forge a frame, caps on rows kept.
//
// shortcut: MySQL / MariaDB only (what the legacy fleet runs). Postgres is a
// later probe; nothing here assumes a single engine so one can be added beside.

// DBMeta is the metadata of one MySQL/MariaDB server. nil on VMInfo = the probe
// didn't run (not root, or a synchronous scan), like DeepFacts.
type DBMeta struct {
	Engine   string      `json:"engine"`             // mysql | mariadb
	Version  string      `json:"version,omitempty"`  // server version string
	Schemas  []DBSchema  `json:"schemas,omitempty"`  // non-system schemas, with size + tables
	Accounts []DBAccount `json:"accounts,omitempty"` // user@host, empty-password flag
	// BindAddress is the effective @@bind_address; ConfigBind is what the config
	// file sets. Exposed = effective listens on every interface (0.0.0.0 / ::
	// / * / empty): the host-35 case where the config says 127.0.0.1 but MySQL
	// answers on *.
	BindAddress   string `json:"bind_address,omitempty"`
	ConfigBind    string `json:"config_bind,omitempty"`
	Exposed       bool   `json:"exposed,omitempty"`
	BinlogSeconds int    `json:"binlog_seconds,omitempty"` // binlog expiry; 0 = off / unknown
	DeniedPrivate int    `json:"denied_private,omitempty"` // failed logins from RFC1918 sources
	DeniedPublic  int    `json:"denied_public,omitempty"`  // failed logins from public sources
}

// DBSchema is one non-system schema: its size, table count and (capped) table
// names, plus whether a personal-data column was seen (feeds the sensitivity
// hint). No table holds a value — only names.
type DBSchema struct {
	Name   string   `json:"name"`
	SizeMB int      `json:"size_mb,omitempty"`
	Tables int      `json:"tables,omitempty"`
	Names  []string `json:"table_names,omitempty"`
	PII    bool     `json:"pii,omitempty"` // a cpf/rg/… column exists in this schema
}

// DBAccount is one grant identity (mysql.user row): user@host and whether it
// has an empty password. RootAny marks root reachable from any host (root@%).
type DBAccount struct {
	User          string `json:"user"`
	Host          string `json:"host"`
	EmptyPassword bool   `json:"empty_password,omitempty"`
	RootAny       bool   `json:"root_any,omitempty"`
}

const (
	dbMaxSchemas    = 100  // schemas kept
	dbMaxTables     = 2000 // table-name rows kept across all schemas
	dbMaxPerSchema  = 60   // table names kept per schema
	dbMaxAccounts   = 500  // mysql.user rows kept
	dbMaxIdentLen   = 128  // a schema/table/user/host longer than this is dropped
	dbConnectTmoSec = 20   // per-query timeout on the VM
)

// dbMetaScript connects as root and emits the metadata markers. Every
// identifier is base64 so it can never look like a marker; counts and status
// variables are plain. BRIDGE_ROOT prefixes the config-file reads (tests).
var dbMetaScript = `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
R="${BRIDGE_ROOT:-}"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout ` + strconv.Itoa(dbConnectTmoSec) + `"
command -v mysql >/dev/null 2>&1 || { echo "@@DBNONE"; exit 0; }
# Find a working root connection: socket auth, then the admin creds on disk.
MYARGS=""
for a in "--protocol=socket" "--defaults-file=$R/etc/mysql/debian.cnf" "--defaults-file=$R/root/.my.cnf"; do
  if echo 'SELECT 1' | $T mysql $a -N -B 2>/dev/null | grep -q '^1$'; then MYARGS="$a"; break; fi
done
[ -z "$MYARGS" ] && { echo "@@DBNONE"; exit 0; }
Q() { $T mysql $MYARGS -N -B -e "$1" 2>/dev/null; }
b64() { printf %s "$1" | base64 | tr -d '\n'; }
echo "@@DBBEGIN"
printf '@@ver %s\n' "$(Q 'SELECT VERSION()')"
Q "SELECT table_schema, ROUND(SUM(data_length+index_length)/1048576), COUNT(*) FROM information_schema.tables GROUP BY table_schema" | while IFS='	' read -r s sz n; do
  [ -n "$s" ] && printf '@@schema %s %s %s\n' "$(b64 "$s")" "$sz" "$n"
done
Q "SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('information_schema','performance_schema','mysql','sys') ORDER BY table_schema LIMIT ` + strconv.Itoa(dbMaxTables) + `" | while IFS='	' read -r s t; do
  [ -n "$t" ] && printf '@@table %s %s\n' "$(b64 "$s")" "$(b64 "$t")"
done
Q "SELECT DISTINCT table_schema FROM information_schema.columns WHERE LOWER(column_name) IN ('cpf','rg','nis','pis','pis_pasep','cns','titulo_eleitor','nome_mae','nis_pis') AND table_schema NOT IN ('information_schema','performance_schema','mysql','sys')" | while IFS='	' read -r s; do
  [ -n "$s" ] && printf '@@pii %s\n' "$(b64 "$s")"
done
Q "SELECT user, host, IF(authentication_string='' AND (plugin IS NULL OR plugin NOT IN ('auth_socket','unix_socket','mysql_old_password','mysql_native_password') OR (plugin='mysql_native_password' AND authentication_string='')),1,0) FROM mysql.user" | while IFS='	' read -r u h e; do
  printf '@@acct %s %s %s\n' "$(b64 "$u")" "$(b64 "$h")" "$e"
done
printf '@@bind %s\n' "$(Q 'SELECT @@bind_address')"
cb=$(grep -hsE '^[[:space:]]*bind-address' "$R"/etc/mysql/my.cnf "$R"/etc/mysql/mysql.conf.d/*.cnf "$R"/etc/mysql/mariadb.conf.d/*.cnf "$R"/etc/my.cnf "$R"/etc/my.cnf.d/*.cnf 2>/dev/null | head -n1 | sed 's/.*=[[:space:]]*//; s/[[:space:]]*$//')
printf '@@cfgbind %s\n' "$cb"
bs=$(Q 'SELECT @@binlog_expire_logs_seconds'); [ -z "$bs" ] && bs=$(Q 'SELECT @@expire_logs_days*86400')
printf '@@binlog %s\n' "$bs"
Q "SELECT ip, count_authentication_errors FROM performance_schema.host_cache WHERE count_authentication_errors > 0" | awk '
  { ip=$1; c=$2+0
    if (ip ~ /^10\.|^192\.168\.|^127\.|^172\.(1[6-9]|2[0-9]|3[01])\.|^::1|^fe80|^f[cd]|^169\.254\./) priv+=c; else if (ip!="" && ip!="-") pub+=c }
  END { printf "@@denied %d %d\n", priv+0, pub+0 }'
echo "@@DBEND"
`

// collectDBMeta runs the probe as root (it reads root-only admin creds and the
// MySQL socket). Without a stored password it doesn't run and returns nil —
// db_meta stays null, "didn't run", like deep facts. "@@DBNONE" (no mysql, or
// no working root connection) also returns nil.
func collectDBMeta(client *ssh.Client, sudoPassword string) *DBMeta {
	if sudoPassword == "" {
		return nil
	}
	out, err := runSudoCmd(client, sudoPassword, dbMetaScript)
	if !strings.Contains(out, "@@DBBEGIN") {
		if !strings.Contains(out, "@@DBNONE") {
			log.Printf("[sshscan] db-meta probe produced no output: %v", err)
		}
		return nil
	}
	return parseDBMeta(out)
}

// parseDBMeta reads the markers into a DBMeta. Identifiers are base64; counts
// are plain. Caps bound every list. Engine is read from the version string
// (MariaDB names itself in VERSION()).
func parseDBMeta(raw string) *DBMeta {
	m := &DBMeta{Engine: "mysql"}
	schemas := map[string]*DBSchema{}
	var order []string
	pii := map[string]bool{}
	tableRows := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "@@ver "):
			m.Version = strings.TrimSpace(line[6:])
			if strings.Contains(strings.ToLower(m.Version), "mariadb") {
				m.Engine = "mariadb"
			}
		case strings.HasPrefix(line, "@@schema "):
			f := strings.Fields(line[9:])
			if len(f) != 3 {
				continue
			}
			name := decodeIdent(f[0])
			if name == "" || isSystemSchema(name) {
				continue
			}
			if schemas[name] == nil && len(order) < dbMaxSchemas {
				sz, _ := strconv.Atoi(f[1])
				n, _ := strconv.Atoi(f[2])
				schemas[name] = &DBSchema{Name: name, SizeMB: sz, Tables: n}
				order = append(order, name)
			}
		case strings.HasPrefix(line, "@@table "):
			f := strings.Fields(line[8:])
			if len(f) != 2 || tableRows >= dbMaxTables {
				continue
			}
			tableRows++
			s, t := decodeIdent(f[0]), decodeIdent(f[1])
			if sc := schemas[s]; sc != nil && t != "" && len(sc.Names) < dbMaxPerSchema {
				sc.Names = append(sc.Names, t)
			}
		case strings.HasPrefix(line, "@@pii "):
			if name := decodeIdent(strings.TrimSpace(line[6:])); name != "" {
				pii[name] = true
			}
		case strings.HasPrefix(line, "@@acct "):
			f := strings.Fields(line[7:])
			if len(f) != 3 || len(m.Accounts) >= dbMaxAccounts {
				continue
			}
			u, h := decodeIdent(f[0]), decodeIdent(f[1])
			if h == "" && u == "" {
				continue
			}
			a := DBAccount{User: u, Host: h, EmptyPassword: f[2] == "1"}
			a.RootAny = u == "root" && (h == "%" || h == "")
			m.Accounts = append(m.Accounts, a)
		case strings.HasPrefix(line, "@@bind "):
			m.BindAddress = strings.TrimSpace(line[7:])
		case strings.HasPrefix(line, "@@cfgbind "):
			m.ConfigBind = strings.TrimSpace(line[10:])
		case strings.HasPrefix(line, "@@binlog "):
			m.BinlogSeconds = atoiClamped(strings.TrimSpace(line[9:]))
		case strings.HasPrefix(line, "@@denied "):
			f := strings.Fields(line[9:])
			if len(f) == 2 {
				m.DeniedPrivate, m.DeniedPublic = atoiClamped(f[0]), atoiClamped(f[1])
			}
		}
	}
	for _, name := range order {
		schemas[name].PII = pii[name]
		m.Schemas = append(m.Schemas, *schemas[name])
	}
	m.Exposed = bindExposed(m.BindAddress)
	return m
}

// bindExposed reports whether the effective bind address listens on every
// interface: 0.0.0.0, ::, * or empty (the MySQL default of all interfaces).
func bindExposed(bind string) bool {
	switch strings.TrimSpace(bind) {
	case "", "*", "0.0.0.0", "::":
		return true
	}
	return false
}

func isSystemSchema(s string) bool {
	switch s {
	case "information_schema", "performance_schema", "mysql", "sys":
		return true
	}
	return false
}

// decodeIdent base64-decodes a probe token into a bounded, printable identifier
// ("" when it decodes to nothing valid).
func decodeIdent(tok string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tok))
	if err != nil {
		return ""
	}
	s := string(raw)
	if len(s) > dbMaxIdentLen || strings.ContainsFunc(s, unicode.IsControl) {
		return ""
	}
	return s
}

// atoiClamped parses a non-negative int, 0 on anything else.
func atoiClamped(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
