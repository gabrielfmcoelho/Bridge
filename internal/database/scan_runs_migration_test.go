package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v132: scans are runs too (kind 'scan', with the auth method the caller
// picked), the migration runs again on top of itself, and the kind is still
// checked.
func TestMigracaoV132(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var mig string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "'backup','scan'") {
			mig = m
		}
	}
	if mig == "" {
		t.Fatal("migration not found")
	}
	if _, err := d.SQL.Exec(mig); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO os_update_runs (kind, auth_method) VALUES ('scan', 'key')`); err != nil {
		t.Fatalf("scan run: %v", err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO os_update_runs (kind) VALUES ('bogus')`); err == nil {
		t.Fatal("a bogus kind must be refused")
	}
}
