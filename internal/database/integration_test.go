package database

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// TestSQLiteMigrationsApplyCleanly runs every migration against a fresh
// on-disk sqlite database and verifies the migration counter ends at the
// highest version in the slice. Regression guard for anyone editing a
// migration without realising it breaks a previous one.
func TestSQLiteMigrationsApplyCleanly(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if d.Dialect != DialectSQLite {
		t.Fatalf("expected sqlite dialect, got %v", d.Dialect)
	}

	var version int
	if err := d.SQL.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if want := len(migrationsSQLite) - 1; version != want {
		t.Fatalf("version mismatch: got %d, want %d", version, want)
	}
}

// TestPortableBackupRoundTrip exercises the portable backup format against
// sqlite only (it always runs in CI). Creates a host, backs up, wipes, and
// restores, then confirms the host is back.
func TestPortableBackupRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if _, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug, hostname, situacao) VALUES (?,?,?,?)`,
		"test", "test", "1.2.3.4", "active"); err != nil {
		t.Fatalf("insert host: %v", err)
	}

	var buf bytes.Buffer
	if err := d.WritePortableBackup(&buf); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty backup")
	}

	backup, err := ReadPortableBackup(&buf)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if backup.SourceDialect != "sqlite" {
		t.Errorf("source dialect: got %q, want sqlite", backup.SourceDialect)
	}
	hosts := backup.Tables["hosts"]
	if len(hosts) != 1 {
		t.Fatalf("hosts in backup: got %d, want 1", len(hosts))
	}
	if hosts[0]["nickname"] != "test" {
		t.Errorf("host nickname: got %v, want test", hosts[0]["nickname"])
	}

	// Wipe the existing row and restore.
	if _, err := d.SQL.Exec(`DELETE FROM hosts`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := d.RestorePortable(backup); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var count int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM hosts WHERE nickname = ?`, "test").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("host count after restore: got %d, want 1", count)
	}
}

// TestCrossDialectRestore optionally runs a sqlite→postgres restore if the
// SSHCM_TEST_PG_DSN env var is set. Local dev runs this manually; CI wires
// it up in the test matrix via a postgres service container.
func TestCrossDialectRestore(t *testing.T) {
	dsn := os.Getenv("SSHCM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SSHCM_TEST_PG_DSN not set")
	}

	// Source: sqlite with a row.
	srcDir := t.TempDir()
	src, err := Open(srcDir)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer src.Close()
	if _, err := src.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug, hostname, situacao, precisa_manutencao) VALUES (?,?,?,?,?)`,
		"crosstest", "crosstest", "9.9.9.9", "active", true); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	var buf bytes.Buffer
	if err := src.WritePortableBackup(&buf); err != nil {
		t.Fatalf("backup: %v", err)
	}
	backup, err := ReadPortableBackup(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Target: pg, schema already applied by the test harness.
	sql.Register("pgx-test", &stdlib.Driver{})
	rawPG, err := sql.Open("pgx-test", dsn)
	if err != nil {
		t.Fatalf("open pg: %v", err)
	}
	defer rawPG.Close()

	// Apply migrations via a second Open() call against a temp sqlite so
	// we have an *Encryptor, then swap in the pg connection. Or simpler:
	// apply schema directly by running Open with SSHCM_DB_* env vars.
	os.Setenv("SSHCM_DB_DRIVER", "postgres")
	os.Setenv("SSHCM_DB_DSN", dsn)
	defer os.Unsetenv("SSHCM_DB_DRIVER")
	defer os.Unsetenv("SSHCM_DB_DSN")

	pgDir := filepath.Join(t.TempDir(), "pgcfg")
	tgt, err := Open(pgDir)
	if err != nil {
		t.Fatalf("open pg DB: %v", err)
	}
	defer tgt.Close()
	if tgt.Dialect != DialectPostgres {
		t.Fatalf("expected postgres, got %v", tgt.Dialect)
	}

	if err := tgt.RestorePortable(backup); err != nil {
		t.Fatalf("restore into pg: %v", err)
	}
	var got string
	if err := tgt.SQL.QueryRowContext(context.Background(),
		`SELECT nickname FROM hosts WHERE oficial_slug = $1`, "crosstest").Scan(&got); err != nil {
		t.Fatalf("read pg after restore: %v", err)
	}
	if got != "crosstest" {
		t.Errorf("got %q, want crosstest", got)
	}
}
