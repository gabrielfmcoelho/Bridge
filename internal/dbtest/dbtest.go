// Package dbtest gives tests an isolated, fully-migrated *database.DB backed by
// an ephemeral Postgres (via internal/pgtest + testcontainers). It replaces the
// old per-test ephemeral SQLite file. Requires a reachable Docker daemon
// (present locally + in CI).
//
// The database package's own white-box tests cannot import this (import cycle);
// they use internal/pgtest.SchemaDSN + database.OpenDSN directly.
package dbtest

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// Open returns a *database.DB on a fresh, isolated Postgres schema with all
// migrations applied — a drop-in replacement for database.Open(t.TempDir()) in
// tests (same (*DB, error) signature). Schema + connection are dropped on
// cleanup. Skips the test if Docker/Postgres can't start.
func Open(t *testing.T) (*database.DB, error) {
	t.Helper()
	dsn := pgtest.SchemaDSN(t)
	if dsn == "" { // test was skipped inside SchemaDSN
		return nil, nil
	}
	d, err := database.OpenDSN(dsn, t.TempDir())
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { d.Close() })
	return d, nil
}

// New is the Fatal-on-error variant of Open.
func New(t *testing.T) *database.DB {
	t.Helper()
	d, err := Open(t)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	return d
}
