package database

import "testing"

// TestMigrationArraysSameLength guards the lockstep invariant: every SQLite
// migration must have a Postgres counterpart at the same index (and vice
// versa). A drift here means one dialect would silently miss a schema change.
func TestMigrationArraysSameLength(t *testing.T) {
	if len(migrationsSQLite) != len(migrationsPostgres) {
		t.Fatalf("migration arrays out of lockstep: sqlite=%d postgres=%d",
			len(migrationsSQLite), len(migrationsPostgres))
	}
}
