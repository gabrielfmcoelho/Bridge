package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestHostRemoteUserRepo_UpsertGetDelete(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	res, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h')`)
	hostID, _ := res.LastInsertId()
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
	kres, err := d.SQL.Exec(`INSERT INTO ssh_keys (name) VALUES ('k1')`)
	if err != nil {
		t.Fatalf("seed ssh_key: %v", err)
	}
	keyID, _ := kres.LastInsertId()
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

func TestHostEntidadeRepo_SyncEnforcesOneMain(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	res, _ := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h')`)
	hostID, _ := res.LastInsertId()
	repo := store.NewHostEntidadeRepo(d.SQL)

	// No main flagged -> alphabetically-first promoted.
	if err := repo.Sync(ctx, hostID, []models.HostEntidadeInput{
		{Entidade: "zeta"}, {Entidade: "alpha"},
	}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	list, err := repo.List(ctx, hostID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if !list[0].IsMain || list[0].Entidade != "alpha" {
		t.Fatalf("main = %+v, want alpha promoted", list[0])
	}
	main, _ := repo.MainBulk(ctx)
	if main[hostID] != "alpha" {
		t.Fatalf("mainbulk = %v, want alpha", main)
	}

	// Explicit main honored; second main flag ignored.
	if err := repo.Sync(ctx, hostID, []models.HostEntidadeInput{
		{Entidade: "x", IsMain: true}, {Entidade: "y", IsMain: true},
	}); err != nil {
		t.Fatalf("sync2: %v", err)
	}
	list, _ = repo.List(ctx, hostID)
	mains := 0
	for _, e := range list {
		if e.IsMain {
			mains++
		}
	}
	if mains != 1 {
		t.Fatalf("main count = %d, want exactly 1", mains)
	}

	// Empty entidade rejected.
	if err := repo.Sync(ctx, hostID, []models.HostEntidadeInput{{Entidade: ""}}); err == nil {
		t.Fatalf("expected error for empty entidade")
	}
}
