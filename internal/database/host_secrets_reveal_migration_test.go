package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v130: hosts.secrets.reveal is seeded for editors (admins always pass), and
// the migration runs again on top of itself.
func TestMigrationSeedsHostSecretsReveal(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var mig string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "hosts.secrets.reveal") {
			mig = m
		}
	}
	if mig == "" {
		t.Fatal("migration not found")
	}
	if _, err := d.SQL.Exec(mig); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	var roles []string
	rows, err := d.SQL.Query(`SELECT role FROM role_permissions WHERE permission = 'hosts.secrets.reveal' ORDER BY role`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		roles = append(roles, r)
	}
	if strings.Join(roles, ",") != "editor" {
		t.Fatalf("roles = %v, want [editor]", roles)
	}
	var cat string
	if err := d.SQL.QueryRow(`SELECT category FROM permissions WHERE code = 'hosts.secrets.reveal'`).Scan(&cat); err != nil || cat != "hosts" {
		t.Fatalf("permission category = %q, %v", cat, err)
	}
}
