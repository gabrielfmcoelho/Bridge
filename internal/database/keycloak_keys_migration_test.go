package database

import (
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v98 on an install with SEAD-managed APIs and Redis keys: the APIs move to
// keycloak with their scope prefix, the master keys are wiped, and the Redis
// keys become revoked manual rows.
func TestMigrationV98_SEADToKeycloak(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var v98 string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "ADD COLUMN IF NOT EXISTS scope_prefix") {
			v98 = m
		}
	}
	if v98 == "" {
		t.Fatal("v98 not found")
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Rewind to the v97 shape.
	mustExec(`ALTER TABLE api_catalog DROP CONSTRAINT api_catalog_key_management_check`)
	mustExec(`ALTER TABLE api_catalog ADD CONSTRAINT api_catalog_key_management_check CHECK (key_management IN ('none','manual','sead'))`)
	mustExec(`ALTER TABLE api_keys DROP CONSTRAINT api_keys_source_check`)
	mustExec(`ALTER TABLE api_keys ADD CONSTRAINT api_keys_source_check CHECK (source IN ('manual','sead'))`)
	mustExec(`ALTER TABLE api_catalog DROP COLUMN scope_prefix`)
	mustExec(`ALTER TABLE api_catalog ADD COLUMN admin_key_cipher BYTEA, ADD COLUMN admin_key_nonce BYTEA, ADD COLUMN admin_api_key_cipher BYTEA, ADD COLUMN admin_api_key_nonce BYTEA`)

	mustExec(`INSERT INTO users (username, password_hash, role) VALUES ('ana', '', 'admin')`)
	var uid int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&uid)
	api := func(name, mode, base string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO api_catalog (scope, name, source_type, spec_json, owner_user_id, created_by,
				key_management, admin_base_url, admin_key_cipher, admin_key_nonce)
			VALUES ('avulso', ?, 'upload', '{}', ?, ?, ?, ?, '\x01', '\x02') RETURNING id`, name, uid, uid, mode, base).Scan(&id); err != nil {
			t.Fatalf("api %s: %v", name, err)
		}
		return id
	}
	folha := api("API Servidores", "sead", "http://10.0.122.91:8000")
	sei := api("Processos", "sead", "http://10.0.122.91:8001/sei")
	other := api("Outra", "manual", "")
	mustExec(`INSERT INTO api_keys (api_id, label, source, external_label, notes) VALUES (?, 'painel', 'sead', 'painel', ''), (?, 'bot', 'sead', 'bot', 'do RH'), (?, 'm', 'manual', NULL, '')`, folha, sei, other)

	mustExec(v98)
	mustExec(v98) // idempotent

	check := func(id int64, mode, prefix string) {
		t.Helper()
		var m, p string
		var hasKey bool
		d.SQL.QueryRow(`SELECT key_management, scope_prefix, admin_key_cipher IS NOT NULL FROM api_catalog WHERE id = ?`, id).Scan(&m, &p, &hasKey)
		if m != mode || p != prefix || hasKey {
			t.Errorf("api %d = %s %q key=%v, want %s %q no key", id, m, p, hasKey, mode, prefix)
		}
	}
	check(folha, "keycloak", "servidores")
	check(sei, "keycloak", "sei")
	check(other, "manual", "")

	rows, err := d.SQL.Query(`SELECT label, source, external_label IS NULL, revoked_at IS NOT NULL, notes FROM api_keys ORDER BY label`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var label, source, notes string
		var noExt, revoked bool
		rows.Scan(&label, &source, &noExt, &revoked, &notes)
		got[label] = source + "|" + map[bool]string{true: "revoked", false: "live"}[revoked] + "|" + notes
		if !noExt {
			t.Errorf("%s kept its Redis label", label)
		}
	}
	want := map[string]string{
		"painel": "manual|revoked|chave Redis legada",
		"bot":    "manual|revoked|chave Redis legada. do RH",
		"m":      "manual|live|",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %s = %q, want %q", k, got[k], v)
		}
	}
	// The new CHECKs hold.
	if _, err := d.SQL.Exec(`UPDATE api_catalog SET key_management = 'sead' WHERE id = ?`, other); err == nil {
		t.Error("key_management 'sead' still accepted")
	}
	if _, err := d.SQL.Exec(`INSERT INTO api_keys (api_id, label, source) VALUES (?, 'k', 'keycloak')`, folha); err != nil {
		t.Errorf("source keycloak refused: %v", err)
	}
}

// v99: duplicate prefixes keep only the lowest id, the index then refuses a
// duplicate, and the SEAD credential columns are gone.
func TestMigrationV99_UniqueScopePrefix(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	v99 := migrationsPostgres[len(migrationsPostgres)-1]
	if !strings.Contains(v99, "api_catalog_scope_prefix_uq") {
		t.Fatal("v99 is not the last migration")
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Rewind to the v98 shape.
	mustExec(`DROP INDEX api_catalog_scope_prefix_uq`)
	mustExec(`ALTER TABLE api_catalog ADD COLUMN admin_key_cipher BYTEA, ADD COLUMN admin_key_nonce BYTEA, ADD COLUMN admin_api_key_cipher BYTEA, ADD COLUMN admin_api_key_nonce BYTEA`)

	mustExec(`INSERT INTO users (username, password_hash, role) VALUES ('ana', '', 'admin')`)
	var uid int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&uid)
	api := func(name, prefix string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO api_catalog (scope, name, source_type, spec_json, owner_user_id, created_by, key_management, scope_prefix)
			VALUES ('avulso', ?, 'upload', '{}', ?, ?, 'keycloak', ?) RETURNING id`, name, uid, uid, prefix).Scan(&id); err != nil {
			t.Fatalf("api %s: %v", name, err)
		}
		return id
	}
	first, second, other := api("SEI", "sei"), api("SEI de novo", "sei"), api("Servidores", "servidores")

	mustExec(v99)
	mustExec(v99) // idempotent

	for id, want := range map[int64]string{first: "sei", second: "", other: "servidores"} {
		var p string
		d.SQL.QueryRow(`SELECT scope_prefix FROM api_catalog WHERE id = ?`, id).Scan(&p)
		if p != want {
			t.Errorf("api %d prefix = %q, want %q", id, p, want)
		}
	}
	if _, err := d.SQL.Exec(`UPDATE api_catalog SET scope_prefix = 'sei' WHERE id = ?`, second); err == nil {
		t.Error("duplicate scope_prefix accepted")
	}
	var n int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'api_catalog' AND column_name LIKE 'admin%key%'`).Scan(&n)
	if n != 0 {
		t.Errorf("%d SEAD credential column(s) left", n)
	}
}
