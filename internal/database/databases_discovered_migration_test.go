package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v133: a scan-born Atlas database carries a "descoberto" badge
// (databases.discovered). The migration is found by content, re-runs on top of
// itself (idempotent), and the column defaults FALSE for existing rows.
func TestMigracaoV133Discovered(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	var mig string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "ADD COLUMN IF NOT EXISTS discovered") && strings.Contains(m, "ALTER TABLE databases") {
			mig = m
		}
	}
	if mig == "" {
		t.Fatal("v133 migration (databases.discovered) not found by content")
	}
	if _, err := d.SQL.Exec(mig); err != nil {
		t.Fatalf("re-run: %v", err)
	}

	// A plain insert defaults discovered to FALSE; an explicit TRUE sticks.
	var def bool
	if err := d.SQL.QueryRow(
		`INSERT INTO databases (name, db_name, engine, connection_mode) VALUES ('d', 'd', 'mysql', 'none') RETURNING discovered`).Scan(&def); err != nil {
		t.Fatalf("insert default: %v", err)
	}
	if def {
		t.Error("discovered should default FALSE for operator-made rows")
	}
	var disc bool
	if err := d.SQL.QueryRow(
		`INSERT INTO databases (name, db_name, engine, connection_mode, discovered) VALUES ('e', 'e', 'mysql', 'none', TRUE) RETURNING discovered`).Scan(&disc); err != nil {
		t.Fatalf("insert discovered: %v", err)
	}
	if !disc {
		t.Error("an explicit discovered=TRUE must persist")
	}
}
