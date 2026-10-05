package database

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/pgtest"
)

// v106 on an install with the single-server Proxmox config: it becomes
// proxmox_servers row #1 with the same secret, linked hosts point at it, the
// old keys go and proxmox_enabled stays.
func TestMigrationV106_ProxmoxServers(t *testing.T) {
	d, err := OpenDSN(pgtest.SchemaDSN(t), t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var v106 string
	for _, m := range migrationsPostgres {
		if strings.Contains(m, "CREATE TABLE IF NOT EXISTS proxmox_servers") {
			v106 = m
		}
	}
	if v106 == "" {
		t.Fatal("v106 not found")
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.SQL.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Rewind to the v105 shape.
	mustExec(`ALTER TABLE hosts DROP COLUMN proxmox_server_id`)
	mustExec(`DROP TABLE proxmox_servers`)
	mustExec(`CREATE UNIQUE INDEX idx_hosts_proxmox_id ON hosts(proxmox_id) WHERE proxmox_id IS NOT NULL AND deleted_at IS NULL`)
	mustExec(`INSERT INTO app_settings (key, value) VALUES ('proxmox_enabled', 'true'), ('proxmox_base_url', 'https://pve:8006/'),
		('proxmox_token_id', 'bridge@pve!sync'), ('proxmox_skip_verify', 'true')
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`)
	cipher, nonce, err := d.Encryptor.Encrypt("s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(`INSERT INTO app_secrets (key, cipher, nonce) VALUES ('proxmox_token_secret', ?, ?)`, hex.EncodeToString(cipher), hex.EncodeToString(nonce))
	var linked, manual int64
	d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, proxmox_id) VALUES ('vm', 'vm', 'qemu/101') RETURNING id`).Scan(&linked)
	d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('m', 'm') RETURNING id`).Scan(&manual)

	mustExec(v106)
	mustExec(v106) // idempotent

	var n int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM proxmox_servers`).Scan(&n)
	if n != 1 {
		t.Fatalf("%d servers, want 1", n)
	}
	var id int64
	var url, tokenID string
	var c, nc []byte
	var skip, enabled bool
	if err := d.SQL.QueryRow(`SELECT id, base_url, token_id, token_cipher, token_nonce, skip_verify, enabled FROM proxmox_servers`).
		Scan(&id, &url, &tokenID, &c, &nc, &skip, &enabled); err != nil {
		t.Fatal(err)
	}
	if url != "https://pve:8006" || tokenID != "bridge@pve!sync" || !skip || !enabled {
		t.Fatalf("server = %q %q skip=%v enabled=%v", url, tokenID, skip, enabled)
	}
	if plain, err := d.Encryptor.Decrypt(c, nc); err != nil || plain != "s3cr3t" {
		t.Fatalf("secret = %q, %v", plain, err)
	}

	var sid *int64
	d.SQL.QueryRow(`SELECT proxmox_server_id FROM hosts WHERE id = ?`, linked).Scan(&sid)
	if sid == nil || *sid != id {
		t.Fatalf("linked host server = %v, want %d", sid, id)
	}
	d.SQL.QueryRow(`SELECT proxmox_server_id FROM hosts WHERE id = ?`, manual).Scan(&sid)
	if sid != nil {
		t.Fatalf("unlinked host got server %d", *sid)
	}

	var keys []string
	rows, err := d.SQL.Query(`SELECT key FROM app_settings WHERE key LIKE 'proxmox%' UNION ALL SELECT key FROM app_secrets WHERE key LIKE 'proxmox%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		rows.Scan(&k)
		keys = append(keys, k)
	}
	if strings.Join(keys, ",") != "proxmox_enabled" {
		t.Fatalf("proxmox keys left = %v, want only proxmox_enabled", keys)
	}

	var idx int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'idx_hosts_proxmox_id'`).Scan(&idx)
	if idx != 0 {
		t.Fatal("idx_hosts_proxmox_id still there")
	}
	// The same guest id on a second server is fine; twice on one is not.
	var other int64
	d.SQL.QueryRow(`INSERT INTO proxmox_servers (name) VALUES ('B') RETURNING id`).Scan(&other)
	mustExec(`INSERT INTO hosts (nickname, oficial_slug, proxmox_id, proxmox_server_id) VALUES ('vm-b', 'vm-b', 'qemu/101', ?)`, other)
	if _, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug, proxmox_id, proxmox_server_id) VALUES ('vm-a', 'vm-a', 'qemu/101', ?)`, id); err == nil {
		t.Fatal("duplicate (server, proxmox_id) accepted")
	}
	// Deleting a server orphans its hosts.
	mustExec(`DELETE FROM proxmox_servers WHERE id = ?`, id)
	d.SQL.QueryRow(`SELECT proxmox_server_id FROM hosts WHERE id = ?`, linked).Scan(&sid)
	if sid != nil {
		t.Fatalf("host still points at deleted server %d", *sid)
	}
}
