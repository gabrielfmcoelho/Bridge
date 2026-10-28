package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

func TestMigracaoV120(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var mig string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "project_gestor_links") {
			mig = m
		}
	}
	if mig == "" {
		t.Fatal("migration not found")
	}
	if _, err := d.SQL.Exec(mig); err != nil { // idempotent: runs again on top of the applied one
		t.Fatalf("re-run: %v", err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO projects (name) VALUES ('a'), ('b')`); err != nil {
		t.Fatal(err)
	}
	ins := `INSERT INTO project_gestor_links (project_id, gestor_projeto_id) SELECT id, 5 FROM projects WHERE name = $1`
	if _, err := d.SQL.Exec(ins, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(ins, "b"); err == nil {
		t.Error("UNIQUE(gestor_projeto_id) not enforced")
	}
}
