package dbtest_test

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
)

// TestHarness proves the testcontainers Postgres harness works end-to-end:
// a fresh schema is created, all migrations apply, and two tests get isolated
// schemas (no cross-talk).
func TestHarness(t *testing.T) {
	d := dbtest.New(t)
	// Migrations created the users table; seed + read back.
	if _, err := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('alice','x','admin')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("users count = %d, want 1", n)
	}
}

func TestHarnessIsolation(t *testing.T) {
	d := dbtest.New(t)
	// A separate test's row must NOT be visible here (separate schema).
	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected isolated empty users table, got %d rows", n)
	}
}
