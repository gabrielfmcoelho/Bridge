package database

import (
	"os"
	"strings"
)

// DialectKind identifies the active SQL dialect.
type DialectKind int

const (
	DialectSQLite DialectKind = iota
	DialectPostgres
)

// active is the package-wide dialect set by Open(). Model code that needs
// dialect-specific SQL fragments (e.g. LIKE vs ILIKE) reads this via helpers.
var active DialectKind = DialectSQLite

// Active returns the currently-selected dialect.
func Active() DialectKind { return active }

// LikeOp returns the case-insensitive string-match operator for the active
// dialect. SQLite's LIKE is already case-insensitive for ASCII; Postgres
// requires ILIKE to match that behavior.
func LikeOp() string {
	if active == DialectPostgres {
		return "ILIKE"
	}
	return "LIKE"
}

// Rebind rewrites "?" placeholders in a SQL string to the positional form
// expected by the active driver. For SQLite it is a no-op. For Postgres it
// converts each "?" to "$1", "$2", ... respecting single-quoted string
// literals so "?" inside a string is left alone.
func Rebind(query string) string {
	if active != DialectPostgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	inString := false
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		if c == '\'' {
			// Handle doubled '' escape inside a string literal.
			if inString && i+1 < len(query) && query[i+1] == '\'' {
				b.WriteByte(c)
				b.WriteByte(c)
				i++
				continue
			}
			inString = !inString
			b.WriteByte(c)
			continue
		}
		if c == '?' && !inString {
			n++
			b.WriteByte('$')
			b.WriteString(itoa(n))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// driverConfig encapsulates how to open the backing database.
type driverConfig struct {
	kind   DialectKind
	driver string // database/sql driver name
	dsn    string // data source name
}

// resolveDriverConfig inspects environment variables and returns the driver
// config plus the dialect. Defaults to SQLite at configDir/sshcm.db.
//
// Environment variables:
//
//	SSHCM_DB_DRIVER  "sqlite" (default) or "postgres"
//	SSHCM_DB_DSN     driver-specific DSN; required for postgres
func resolveDriverConfig(configDir string) driverConfig {
	drv := strings.ToLower(strings.TrimSpace(os.Getenv("SSHCM_DB_DRIVER")))
	dsn := os.Getenv("SSHCM_DB_DSN")
	switch drv {
	case "postgres", "pg", "postgresql":
		return driverConfig{kind: DialectPostgres, driver: "pgx", dsn: dsn}
	default:
		return driverConfig{
			kind:   DialectSQLite,
			driver: "sqlite",
			dsn:    configDir + "/sshcm.db",
		}
	}
}
