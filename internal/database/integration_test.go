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

// TestSecretsSchemaShape exercises every spec'd constraint on the new
// secrets / secret_share_links / secret_audit_log tables introduced in
// migration v61. Sqlite-only; postgres parity is implicit because the same
// SQL semantics are required (see migrations_postgres.go).
//
// Spec ref: internal/spec/secrets-manager.md §4.1.
func TestSecretsSchemaShape(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	// Seed two users for the owner_user_id FK.
	r1, err := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
		"owner1", "h1", "admin")
	if err != nil {
		t.Fatalf("seed user1: %v", err)
	}
	owner1, _ := r1.LastInsertId()
	r2, err := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
		"owner2", "h2", "admin")
	if err != nil {
		t.Fatalf("seed user2: %v", err)
	}
	owner2, _ := r2.LastInsertId()

	ins := func(typ, scope, vis string, parentID any, owner int64, name string, group any) (int64, error) {
		res, err := d.SQL.Exec(
			`INSERT INTO secrets
			   (type, scope, visibility, parent_id, owner_user_id, name, group_label,
			    payload_ciphertext, payload_nonce, key_version, created_by)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			typ, scope, vis, parentID, owner, name, group,
			[]byte{0x01}, []byte{0x02}, 1, owner,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}

	// T1: valid scoped (service) shared secret
	if _, err := ins("cred", "service", "shared", int64(1), owner1, "db-root", nil); err != nil {
		t.Errorf("T1 valid scoped shared should succeed: %v", err)
	}

	// T2: valid avulso (parent_id NULL) personal secret
	if _, err := ins("password", "avulso", "personal", nil, owner1, "personal-note", nil); err != nil {
		t.Errorf("T2 valid avulso personal should succeed: %v", err)
	}

	// T3: avulso WITH parent_id → CHECK must reject
	if _, err := ins("password", "avulso", "personal", int64(7), owner1, "bad-avulso", nil); err == nil {
		t.Error("T3 CHECK should reject avulso+parent_id")
	}

	// T4: scoped WITHOUT parent_id → CHECK must reject
	if _, err := ins("cred", "service", "shared", nil, owner1, "bad-scoped", nil); err == nil {
		t.Error("T4 CHECK should reject scoped+null parent_id")
	}

	// T5: two personal secrets, same (scope, parent_id, name), different owners → both succeed
	if _, err := ins("sshkey", "host", "personal", int64(10), owner1, "ssh-key", nil); err != nil {
		t.Errorf("T5 first personal should succeed: %v", err)
	}
	if _, err := ins("sshkey", "host", "personal", int64(10), owner2, "ssh-key", nil); err != nil {
		t.Errorf("T5 second user's personal with same name on same host should succeed: %v", err)
	}

	// T6: two shared secrets, same (scope, parent_id, name) → second must fail
	if _, err := ins("sshkey", "host", "shared", int64(20), owner1, "team-key", nil); err != nil {
		t.Errorf("T6 first shared should succeed: %v", err)
	}
	if _, err := ins("sshkey", "host", "shared", int64(20), owner2, "team-key", nil); err == nil {
		t.Error("T6 partial-unique-on-shared should reject duplicate (scope, parent_id, name)")
	}

	// T6b: two shared AVULSO secrets, same name (parent_id NULL in both)
	// Bug guard: NULLs-distinct semantics would silently allow this without COALESCE.
	if _, err := ins("password", "avulso", "shared", nil, owner1, "team-note", nil); err != nil {
		t.Errorf("T6b first shared avulso should succeed: %v", err)
	}
	if _, err := ins("password", "avulso", "shared", nil, owner2, "team-note", nil); err == nil {
		t.Error("T6b two shared avulso with same name must be rejected (COALESCE(parent_id, 0) in index)")
	}

	// T7: env_var rows are unique per (parent, name, environment)
	if _, err := ins("env_var", "service", "shared", int64(30), owner1, "DB_URL", "prod"); err != nil {
		t.Errorf("T7 first env_var should succeed: %v", err)
	}
	if _, err := ins("env_var", "service", "shared", int64(30), owner2, "DB_URL", "prod"); err == nil {
		t.Error("T7 duplicate env_var (same name+env+parent) must be rejected on shared")
	}
	// But same var name in a different environment is fine.
	if _, err := ins("env_var", "service", "shared", int64(30), owner1, "DB_URL", "staging"); err != nil {
		t.Errorf("T7 same var in different env should succeed: %v", err)
	}

	// T8: sibling tables exist (smoke test — counts return 0 cleanly)
	for _, table := range []string{"secret_share_links", "secret_audit_log"} {
		var n int
		if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Errorf("table %s should exist: %v", table, err)
		}
	}

	// T9: secret_share_links FK CASCADE works on hard delete (sanity check
	// the eventual hard-purge path; soft delete is handled at the app layer).
	sid, err := ins("password", "avulso", "personal", nil, owner1, "to-share", nil)
	if err != nil {
		t.Fatalf("T9 seed secret: %v", err)
	}
	if _, err := d.SQL.Exec(
		`INSERT INTO secret_share_links (secret_id, token_hash, expires_at, created_by)
		 VALUES (?,?,?,?)`,
		sid, []byte{0xab, 0xcd}, "2099-01-01T00:00:00Z", owner1,
	); err != nil {
		t.Fatalf("T9 insert share link: %v", err)
	}
	if _, err := d.SQL.Exec(`DELETE FROM secrets WHERE id = ?`, sid); err != nil {
		t.Fatalf("T9 hard delete: %v", err)
	}
	var linkCount int
	if err := d.SQL.QueryRow(
		`SELECT COUNT(*) FROM secret_share_links WHERE secret_id = ?`, sid,
	).Scan(&linkCount); err != nil {
		t.Fatalf("T9 count links: %v", err)
	}
	if linkCount != 0 {
		t.Errorf("T9 ON DELETE CASCADE should have removed share link; got %d", linkCount)
	}

	// T10: audit log survives parent deletion (secret_id intentionally not FK)
	if _, err := d.SQL.Exec(
		`INSERT INTO secret_audit_log (secret_id, action, actor_user_id) VALUES (?,?,?)`,
		9999, "reveal", owner1,
	); err != nil {
		t.Errorf("T10 audit insert with non-existent secret_id should succeed (not a FK): %v", err)
	}
}
