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

// active is the package-wide dialect. The app is Postgres-only; DialectSQLite
// remains defined only so the portable backup/restore code (which still carries
// dialect-tagged branches) compiles — it is never selected at runtime.
var active DialectKind = DialectPostgres

// Active returns the currently-selected dialect.
func Active() DialectKind { return active }

// LikeOp returns the case-insensitive string-match operator (Postgres ILIKE).
func LikeOp() string { return "ILIKE" }

// Rebind rewrites "?" placeholders to Postgres positional form "$1","$2",...,
// respecting single-quoted string literals so a "?" inside a string is left
// alone. The pgx-rebind driver calls this on every statement, so model code
// stays placeholder-portable.
func Rebind(query string) string {
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

// resolveDriverConfig returns the Postgres driver config. The app is
// Postgres-only: SSHCM_DB_DSN must be a pgx-compatible DSN (SSHCM_DB_DRIVER is
// accepted for back-compat but only "postgres"/"pg"/"postgresql"/"" are valid).
func resolveDriverConfig() driverConfig {
	drv := strings.ToLower(strings.TrimSpace(os.Getenv("SSHCM_DB_DRIVER")))
	switch drv {
	case "", "postgres", "pg", "postgresql":
		return driverConfig{kind: DialectPostgres, driver: "pgx", dsn: os.Getenv("SSHCM_DB_DSN")}
	default:
		// Unknown driver (e.g. legacy "sqlite") — surface a clear error at Open.
		return driverConfig{kind: DialectPostgres, driver: "pgx", dsn: ""}
	}
}
