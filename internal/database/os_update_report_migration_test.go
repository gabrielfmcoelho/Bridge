package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v129: a result's report is optional JSONB, and the migration runs again
// on top of itself.
func TestMigracaoV129(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var mig string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "os_update_results ADD COLUMN IF NOT EXISTS report JSONB") {
			mig = m
		}
	}
	if mig == "" {
		t.Fatal("migration not found")
	}
	if _, err := d.SQL.Exec(mig); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	var typ, nullable string
	if err := d.SQL.QueryRow(`SELECT data_type, is_nullable FROM information_schema.columns
		WHERE table_name = 'os_update_results' AND column_name = 'report'`).Scan(&typ, &nullable); err != nil {
		t.Fatal(err)
	}
	if typ != "jsonb" || nullable != "YES" {
		t.Fatalf("report = %s nullable=%s", typ, nullable)
	}
}
