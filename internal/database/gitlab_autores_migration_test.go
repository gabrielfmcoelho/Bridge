package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

func TestMigracaoV121(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var mig string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "gitlab_autores") {
			mig = m
		}
	}
	if mig == "" {
		t.Fatal("migration not found")
	}
	if _, err := d.SQL.Exec(mig); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO gitlab_autores (email) VALUES ('a@x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO gitlab_autores (email) VALUES ('a@x')`); err == nil {
		t.Error("UNIQUE(email) not enforced")
	}
}
