package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestExternalToolRepo_CRUDAndSyncLookup(t *testing.T) {
	ctx := context.Background()
	repo := store.NewExternalToolRepo(openDB(t).SQL)

	tool := &models.ExternalTool{Name: "grafana", URL: "https://g", EmbedEnabled: true}
	if err := repo.Create(ctx, tool); err != nil {
		t.Fatalf("create: %v", err)
	}
	if tool.ID == 0 || tool.Source != "manual" { // Source defaults to manual
		t.Fatalf("create = %+v, want ID set + source=manual", tool)
	}

	got, err := repo.Get(ctx, tool.ID)
	if err != nil || got == nil || got.Name != "grafana" || got.HasCredentials {
		t.Fatalf("get = %+v, %v (HasCredentials should be false w/o service)", got, err)
	}

	list, _ := repo.List(ctx)
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	tool.Name = "grafana2"
	if err := repo.Update(ctx, tool); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.Get(ctx, tool.ID)
	if got.Name != "grafana2" {
		t.Fatalf("after update name = %q", got.Name)
	}

	// GetByServiceAndDNS with no matching synced tool -> nil.
	if g, err := repo.GetByServiceAndDNS(ctx, 1, 1); g != nil || err != nil {
		t.Fatalf("sync-lookup = %+v, %v; want nil,nil", g, err)
	}

	if err := repo.Delete(ctx, tool.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, tool.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestSSHKeyRepo_CRUDWithEncryptedPayload(t *testing.T) {
	ctx := context.Background()
	repo := store.NewSSHKeyRepo(openDB(t).SQL)

	k := &models.SSHKey{
		Name:              "deploy",
		Username:          "git",
		PrivKeyCiphertext: []byte{1, 2, 3},
		PrivKeyNonce:      []byte{4, 5},
		Fingerprint:       "ab:cd",
	}
	if err := repo.Create(ctx, k); err != nil {
		t.Fatalf("create: %v", err)
	}
	if k.ID == 0 || k.CredentialType != "key" { // defaults to "key"
		t.Fatalf("create = %+v, want ID + credential_type=key", k)
	}

	got, err := repo.Get(ctx, k.ID)
	if err != nil || got == nil || got.Name != "deploy" || string(got.PrivKeyCiphertext) != string([]byte{1, 2, 3}) {
		t.Fatalf("get = %+v, %v (ciphertext should round-trip)", got, err)
	}

	got.Description = "prod deploy key"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	if list, _ := repo.List(ctx); len(list) != 1 || list[0].Description != "prod deploy key" {
		t.Fatalf("list = %+v", list)
	}

	if err := repo.Delete(ctx, k.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, k.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}
