package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v88 on an install whose admin renamed the situação options before roles
// existed ("active" → "Ativa", …) while Coolify/Proxmox kept writing the bare
// "active": roles land on the renamed options and those rows are rewritten.
func TestMigrationV88_SituacaoRolesOnRenamedOptions(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	// Found by content, so later migrations don't shift it.
	var v88 string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "ALTER TABLE enum_options ADD COLUMN IF NOT EXISTS role") {
			v88 = m
		}
	}
	if v88 == "" {
		t.Fatalf("situação roles migration (v88) not found")
	}

	// Rewind to "before v88" with the real install's shape.
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`UPDATE enum_options SET role = '' WHERE category = 'situacao'`)
	mustExec(`UPDATE enum_options SET value = 'Ativa' WHERE category = 'situacao' AND value = 'active'`)
	mustExec(`UPDATE enum_options SET value = 'Desligada' WHERE category = 'situacao' AND value = 'inactive'`)
	mustExec(`UPDATE enum_options SET value = 'Em manutenção' WHERE category = 'situacao' AND value = 'maintenance'`)
	mustExec(`INSERT INTO enum_options (category, value, sort_order) VALUES ('situacao', 'Em quarentena', 9)`)
	mustExec(`INSERT INTO hosts (nickname, oficial_slug, situacao) VALUES ('a', 'a', 'active'), ('b', 'b', 'inactive'), ('c', 'c', 'Em quarentena')`)
	mustExec(`INSERT INTO dns_records (domain, situacao) VALUES ('x.gov', 'active')`)

	mustExec(v88)
	mustExec(v88) // idempotent

	roles := map[string]string{}
	rows, err := d.SQL.Query(`SELECT value, role FROM enum_options WHERE category = 'situacao'`)
	if err != nil {
		t.Fatalf("roles: %v", err)
	}
	for rows.Next() {
		var v, r string
		if err := rows.Scan(&v, &r); err != nil {
			t.Fatalf("scan: %v", err)
		}
		roles[v] = r
	}
	rows.Close()
	want := map[string]string{"Ativa": "active", "Desligada": "inactive", "Em manutenção": "maintenance", "Em quarentena": ""}
	for v, r := range want {
		if roles[v] != r {
			t.Fatalf("role of %q = %q, want %q (all: %v)", v, roles[v], r, roles)
		}
	}

	sit := func(q string) string {
		t.Helper()
		var s string
		if err := d.SQL.QueryRow(q).Scan(&s); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return s
	}
	if s := sit(`SELECT situacao FROM hosts WHERE oficial_slug = 'a'`); s != "Ativa" {
		t.Fatalf("host a = %q, want Ativa", s)
	}
	if s := sit(`SELECT situacao FROM hosts WHERE oficial_slug = 'b'`); s != "Desligada" {
		t.Fatalf("host b = %q, want Desligada", s)
	}
	if s := sit(`SELECT situacao FROM hosts WHERE oficial_slug = 'c'`); s != "Em quarentena" {
		t.Fatalf("host c = %q, want untouched", s)
	}
	if s := sit(`SELECT situacao FROM dns_records WHERE domain = 'x.gov'`); s != "Ativa" {
		t.Fatalf("dns = %q, want Ativa", s)
	}
}
