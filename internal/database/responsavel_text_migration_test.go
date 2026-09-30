package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v93: the old free-text responsável on DNS records and projects becomes a
// linked main contact — matched by name when the contact exists, created
// otherwise — only where nothing is linked yet; running it twice is harmless.
func TestMigrationV93_ResponsavelTextToContacts(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var v93 string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "ALTER TABLE contacts ADD COLUMN IF NOT EXISTS email") {
			v93 = m
		}
	}
	if v93 == "" {
		t.Fatal("v93 not found")
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var ana, other int64
	d.SQL.QueryRow(`INSERT INTO contacts (name, phone) VALUES ('Ana Souza', '869') RETURNING id`).Scan(&ana)
	d.SQL.QueryRow(`INSERT INTO contacts (name, phone) VALUES ('Outro', '') RETURNING id`).Scan(&other)
	var dnsA, dnsNew, dnsLinked, proj int64
	d.SQL.QueryRow(`INSERT INTO dns_records (domain, responsavel) VALUES ('a.gov', 'ana souza') RETURNING id`).Scan(&dnsA)
	d.SQL.QueryRow(`INSERT INTO dns_records (domain, responsavel) VALUES ('b.gov', 'Beto') RETURNING id`).Scan(&dnsNew)
	d.SQL.QueryRow(`INSERT INTO dns_records (domain, responsavel) VALUES ('c.gov', 'Beto') RETURNING id`).Scan(&dnsLinked)
	exec(`INSERT INTO responsaveis (entity_type, entity_id, contact_id, is_main) VALUES ('dns', ?, ?, TRUE)`, dnsLinked, other)
	d.SQL.QueryRow(`INSERT INTO projects (name, responsavel) VALUES ('p', 'Beto') RETURNING id`).Scan(&proj)

	exec(v93)
	exec(v93)

	main := func(typ string, id int64) (name string, n int) {
		d.SQL.QueryRow(`SELECT COUNT(*) FROM responsaveis WHERE entity_type = ? AND entity_id = ?`, typ, id).Scan(&n)
		d.SQL.QueryRow(`SELECT c.name FROM responsaveis r JOIN contacts c ON c.id = r.contact_id
			WHERE r.entity_type = ? AND r.entity_id = ? AND r.is_main`, typ, id).Scan(&name)
		return
	}
	if name, n := main("dns", dnsA); name != "Ana Souza" || n != 1 {
		t.Errorf("a.gov → %q (%d); want the existing Ana Souza", name, n)
	}
	if name, n := main("dns", dnsNew); name != "Beto" || n != 1 {
		t.Errorf("b.gov → %q (%d); want a new Beto contact", name, n)
	}
	if name, n := main("dns", dnsLinked); name != "Outro" || n != 1 {
		t.Errorf("c.gov → %q (%d); already linked, want untouched", name, n)
	}
	if name, _ := main("project", proj); name != "Beto" {
		t.Errorf("project → %q; want Beto", name)
	}
	var betos int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM contacts WHERE name = 'Beto'`).Scan(&betos)
	if betos != 1 {
		t.Errorf("Beto contacts = %d, want 1", betos)
	}
}
