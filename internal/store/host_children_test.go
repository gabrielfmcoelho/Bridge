package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostRemoteUserRepo_UpsertGetDelete(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	repo := store.NewHostRemoteUserRepo(d.SQL)

	// Upsert with nil key.
	if err := repo.CreateOrUpdate(ctx, hostID, "coolify", nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetByUsername(ctx, hostID, "coolify")
	if err != nil || got == nil || got.Username != "coolify" || got.SSHKeyID != nil {
		t.Fatalf("get = %+v, %v", got, err)
	}

	// Upsert again with a key id -> single row, updated. ssh_key_id FKs
	// ssh_keys, so seed a key first.
	var keyID int64
	if err := d.SQL.QueryRow(`INSERT INTO ssh_keys (name) VALUES ('k1') RETURNING id`).Scan(&keyID); err != nil {
		t.Fatalf("seed ssh_key: %v", err)
	}
	if err := repo.CreateOrUpdate(ctx, hostID, "coolify", &keyID); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.GetByUsername(ctx, hostID, "coolify")
	if got == nil || got.SSHKeyID == nil || *got.SSHKeyID != keyID {
		t.Fatalf("after update = %+v, want ssh_key_id=%d", got, keyID)
	}

	if err := repo.Delete(ctx, hostID, "coolify"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.GetByUsername(ctx, hostID, "coolify"); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

