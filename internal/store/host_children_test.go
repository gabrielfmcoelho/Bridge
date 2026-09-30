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
	if err != nil || got == nil || got.Username != "coolify" || got.KeySecretID != nil {
		t.Fatalf("get = %+v, %v", got, err)
	}

	// Upsert again with a vault key -> single row, updated.
	var uid, keyID int64
	d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin') RETURNING id`).Scan(&uid)
	if err := d.SQL.QueryRow(`INSERT INTO secrets (type, scope, visibility, owner_user_id, name, payload_ciphertext, payload_nonce, key_version, created_by)
		VALUES ('sshkey','avulso','shared',?, 'k1', ?, ?, 1, ?) RETURNING id`, uid, []byte("c"), []byte("n"), uid).Scan(&keyID); err != nil {
		t.Fatalf("seed key secret: %v", err)
	}
	if err := repo.CreateOrUpdate(ctx, hostID, "coolify", &keyID); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.GetByUsername(ctx, hostID, "coolify")
	if got == nil || got.KeySecretID == nil || *got.KeySecretID != keyID {
		t.Fatalf("after update = %+v, want key_secret_id=%d", got, keyID)
	}

	if err := repo.Delete(ctx, hostID, "coolify"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.GetByUsername(ctx, hostID, "coolify"); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}
