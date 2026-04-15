package database

import (
	"compress/gzip"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Portable backup/restore format
//
// A portable backup is a gzipped JSON document that survives round-trips
// between SQLite and Postgres. The goal is a single on-disk representation
// that can be produced by one dialect and consumed by the other without
// any external tooling (no pg_dump, no sqlite CLI).
//
// The format is deliberately simple: a manifest block followed by a map of
// table name → array of rows, where each row is an ordered map of
// column → value. Values are serialized in a dialect-neutral way:
//
//   - numbers stay as JSON numbers
//   - booleans stay as JSON booleans (coerced to 0/1 when loading SQLite)
//   - TEXT stays as strings
//   - BYTEA / BLOB columns are base64-encoded and prefixed with "\x00b64:"
//     so they can be round-tripped without losing bytes
//   - NULLs stay as null
//
// The format version is bumped if the shape changes. Restores that see an
// unfamiliar version refuse to proceed.

const (
	portableFormatVersion = 1
	portableMagic         = "sshcm-backup"
	blobPrefix            = "\x00b64:" // sentinel for base64-encoded []byte
)

// PortableBackup is the top-level structure written to disk. Tables are
// emitted in TableCopyOrder; the restore path honors that ordering too.
type PortableBackup struct {
	Magic         string                      `json:"magic"`
	FormatVersion int                         `json:"format_version"`
	CreatedAt     time.Time                   `json:"created_at"`
	SourceDialect string                      `json:"source_dialect"`
	SchemaVersion int                         `json:"schema_version"`
	Tables        map[string][]map[string]any `json:"tables"`
}

// WritePortableBackup streams a full portable backup of d to w, gzipping
// the JSON so big sqlite dumps stay tractable.
func (d *DB) WritePortableBackup(w io.Writer) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()

	backup := PortableBackup{
		Magic:         portableMagic,
		FormatVersion: portableFormatVersion,
		CreatedAt:     time.Now().UTC(),
		SourceDialect: dialectName(d.Dialect),
		Tables:        make(map[string][]map[string]any, len(TableCopyOrder)),
	}
	if err := d.SQL.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&backup.SchemaVersion); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, table := range TableCopyOrder {
		rows, err := d.dumpTable(table)
		if err != nil {
			return fmt.Errorf("dump %s: %w", table, err)
		}
		backup.Tables[table] = rows
	}

	enc := json.NewEncoder(gz)
	enc.SetEscapeHTML(false)
	return enc.Encode(&backup)
}

// dumpTable reads every row of one table into a slice of column maps,
// applying the column-rename map so sources at an older schema version
// (e.g. pre-v48 sqlite) still deserialize into the current column names.
func (d *DB) dumpTable(table string) ([]map[string]any, error) {
	cols, err := d.columnNames(table)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, nil
	}
	renames := ColumnRenames[table]

	selectCols := make([]string, len(cols))
	outNames := make([]string, len(cols))
	for i, c := range cols {
		selectCols[i] = `"` + c + `"`
		if r, ok := renames[c]; ok {
			outNames[i] = r
		} else {
			outNames[i] = c
		}
	}

	rows, err := d.SQL.Query(`SELECT ` + strings.Join(selectCols, ",") + ` FROM "` + table + `"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bools := BoolColumns[table]
	blobs := BlobColumns[table]
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range outNames {
			row[c] = normalizeForJSON(vals[i], bools[c], blobs[c])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// columnNames returns the column list for a table, dialect-agnostically.
func (d *DB) columnNames(table string) ([]string, error) {
	switch d.Dialect {
	case DialectPostgres:
		rows, err := d.SQL.Query(`SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1 ORDER BY ordinal_position`, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
			cols = append(cols, n)
		}
		return cols, rows.Err()
	default:
		rows, err := d.SQL.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
			cols = append(cols, n)
		}
		return cols, rows.Err()
	}
}

// normalizeForJSON prepares a scanned value for JSON serialization. The
// tricky cases:
//
//   - []byte in a declared blob column: always base64-prefixed, regardless
//     of content. A content heuristic misclassifies empty / short-printable
//     payloads as text and fails to restore into Postgres BYTEA.
//   - []byte in a non-blob column: may be a sqlite TEXT that the driver
//     returned as bytes; emit as a plain string.
//   - bool vs int64 for boolean-ish columns: unified to Go bool.
//   - time.Time: JSON encoder already handles this as RFC3339.
func normalizeForJSON(v any, isBool, isBlob bool) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []byte:
		if isBlob {
			return blobPrefix + base64.StdEncoding.EncodeToString(x)
		}
		return string(x)
	case int64:
		if isBool {
			return x != 0
		}
		return x
	case bool:
		return x
	default:
		return v
	}
}

// ReadPortableBackup decodes a portable backup from r. It does not touch
// the database; call RestorePortable on the result to apply it.
func ReadPortableBackup(r io.Reader) (*PortableBackup, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	var backup PortableBackup
	dec := json.NewDecoder(gz)
	dec.UseNumber()
	if err := dec.Decode(&backup); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if backup.Magic != portableMagic {
		return nil, fmt.Errorf("not an sshcm portable backup (magic=%q)", backup.Magic)
	}
	if backup.FormatVersion != portableFormatVersion {
		return nil, fmt.Errorf("unsupported backup format version %d (expected %d)", backup.FormatVersion, portableFormatVersion)
	}
	return &backup, nil
}

// RestorePortable wipes the target's user-data tables and reloads them
// from the decoded backup. FK checks are deferred during the load so
// table order doesn't need to match ColumnOrder (it does, but we don't
// rely on it). Runs inside a transaction: either everything imports or
// nothing does.
func (d *DB) RestorePortable(backup *PortableBackup) error {
	// Defer FK enforcement for both dialects during the bulk load.
	if d.Dialect == DialectPostgres {
		if _, err := d.SQL.Exec(`SET session_replication_role = 'replica'`); err != nil {
			return fmt.Errorf("defer fk checks: %w", err)
		}
		defer d.SQL.Exec(`SET session_replication_role = 'origin'`)
	} else {
		if _, err := d.SQL.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
			return fmt.Errorf("defer fk checks: %w", err)
		}
		defer d.SQL.Exec(`PRAGMA foreign_keys=ON`)
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Truncate first so we start from a clean slate. Iterate in reverse
	// so children go before parents under sqlite (which doesn't have
	// TRUNCATE CASCADE).
	for i := len(TableCopyOrder) - 1; i >= 0; i-- {
		t := TableCopyOrder[i]
		if _, err := tx.Exec(`DELETE FROM "` + t + `"`); err != nil {
			return fmt.Errorf("clear %s: %w", t, err)
		}
	}

	for _, table := range TableCopyOrder {
		rows := backup.Tables[table]
		if len(rows) == 0 {
			continue
		}
		if err := insertRows(tx, d.Dialect, table, rows); err != nil {
			return fmt.Errorf("load %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// On Postgres, bump each BIGSERIAL sequence past the max(id) that
	// just landed so future INSERTs don't collide.
	if d.Dialect == DialectPostgres {
		for _, t := range SerialTables {
			if _, err := d.SQL.Exec(fmt.Sprintf(
				`SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)`,
				t, t,
			)); err != nil {
				return fmt.Errorf("reset sequence %s: %w", t, err)
			}
		}
	}
	return nil
}

// insertRows inlines one row at a time. For dialect-specific coercion we
// look up the target column type via the live schema so stray strings in
// int columns (a known sqlite weakly-typed source data problem) are
// coerced to 0 rather than failing the whole restore.
func insertRows(tx *sql.Tx, dialect DialectKind, table string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}

	// Collect the union of column names across all rows so a single
	// prepared statement covers them all. In practice every row comes
	// from the same table so the columns are already uniform.
	colSet := make(map[string]struct{})
	for _, r := range rows {
		for c := range r {
			colSet[c] = struct{}{}
		}
	}
	cols := make([]string, 0, len(colSet))
	for c := range colSet {
		cols = append(cols, c)
	}

	// Dialect-aware column-type probe for coercion decisions.
	colTypes, err := queryColumnTypes(tx, dialect, table)
	if err != nil {
		return err
	}

	bools := BoolColumns[table]
	blobs := BlobColumns[table]

	placeholders := make([]string, len(cols))
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = `"` + c + `"`
		if dialect == DialectPostgres {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		} else {
			placeholders[i] = "?"
		}
	}
	stmt, err := tx.Prepare(fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s)`,
		table, strings.Join(quoted, ","), strings.Join(placeholders, ","),
	))
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for ri, row := range rows {
		args := make([]any, len(cols))
		for i, c := range cols {
			v, ok := row[c]
			if !ok {
				args[i] = nil
				continue
			}
			args[i] = coerceForDialect(v, dialect, colTypes[c], bools[c], blobs[c])
		}
		if _, err := stmt.Exec(args...); err != nil {
			return fmt.Errorf("row %d: %w", ri+1, err)
		}
	}
	return nil
}

// coerceForDialect massages a JSON-decoded value into a form the target
// dialect will accept:
//
//   - json.Number → int64 or float64 depending on the target column
//   - "" + int target → 0 (sqlite weak-typing artifact)
//   - non-numeric string + int target → 0 (with no error)
//   - blobPrefix string → decoded []byte
//   - bool ↔ int depending on target's bool/int expectation
//   - isBlob target + bare string (legacy backup where the dumper's text
//     heuristic mislabelled a blob) → raw bytes of that string, with ""
//     mapped to an empty []byte so Postgres BYTEA accepts it.
func coerceForDialect(v any, dialect DialectKind, colType string, isBool, isBlob bool) any {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok && strings.HasPrefix(s, blobPrefix) {
		if decoded, err := base64.StdEncoding.DecodeString(s[len(blobPrefix):]); err == nil {
			return decoded
		}
	}
	if isBlob {
		switch s := v.(type) {
		case string:
			return []byte(s)
		case []byte:
			return s
		}
	}

	// json.Number handling first so we have a concrete type downstream.
	if jn, ok := v.(json.Number); ok {
		if i, err := jn.Int64(); err == nil {
			v = i
		} else if f, err := jn.Float64(); err == nil {
			v = f
		} else {
			v = jn.String()
		}
	}

	typeIsInt := isIntType(colType)
	typeIsBool := isBoolType(colType) || (isBool && dialect == DialectPostgres)

	switch x := v.(type) {
	case string:
		if typeIsInt {
			if x == "" {
				return int64(0)
			}
			if n, err := strconv.ParseInt(x, 10, 64); err == nil {
				return n
			}
			// Weakly-typed sqlite source had a string where a bigint
			// was expected; default to 0 so the restore doesn't fail.
			return int64(0)
		}
		if typeIsBool {
			return x == "1" || strings.EqualFold(x, "true") || strings.EqualFold(x, "t")
		}
		return x
	case bool:
		if dialect == DialectSQLite && !typeIsBool {
			if x {
				return int64(1)
			}
			return int64(0)
		}
		return x
	case int64:
		if typeIsBool {
			return x != 0
		}
		return x
	case float64:
		if typeIsInt {
			return int64(x)
		}
		return x
	}
	return v
}

func queryColumnTypes(tx *sql.Tx, dialect DialectKind, table string) (map[string]string, error) {
	types := make(map[string]string)
	switch dialect {
	case DialectPostgres:
		rows, err := tx.Query(`SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1`, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var n, t string
			if err := rows.Scan(&n, &t); err != nil {
				return nil, err
			}
			types[n] = t
		}
		return types, rows.Err()
	default:
		rows, err := tx.Query(`SELECT name, type FROM pragma_table_info(?)`, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var n, t string
			if err := rows.Scan(&n, &t); err != nil {
				return nil, err
			}
			types[n] = t
		}
		return types, rows.Err()
	}
}

func isIntType(t string) bool {
	t = strings.ToLower(t)
	return strings.Contains(t, "int") || t == "bigserial" || t == "serial"
}

func isBoolType(t string) bool {
	t = strings.ToLower(t)
	return t == "boolean" || t == "bool"
}

func dialectName(k DialectKind) string {
	if k == DialectPostgres {
		return "postgres"
	}
	return "sqlite"
}
