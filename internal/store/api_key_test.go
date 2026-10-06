package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAPIKeyRepo_BySecret(t *testing.T) {
	ctx := context.Background()
	d, owner := newCatalogDB(t)
	a, ops := sampleCatalog(owner)
	if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, a, ops); err != nil {
		t.Fatalf("api: %v", err)
	}
	secret := func(name string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO secrets (type, scope, visibility, owner_user_id, name, payload_ciphertext, payload_nonce, key_version, created_by)
			VALUES ('api_key','avulso','shared',?, ?, ?, ?, 1, ?) RETURNING id`, owner, name, []byte("c"), []byte("n"), owner).Scan(&id); err != nil {
			t.Fatalf("secret: %v", err)
		}
		return id
	}
	live, revoked, plain := secret("live"), secret("revoked"), secret("plain")
	keys := store.NewAPIKeyRepo(d.SQL)
	client := "petstore-painel"
	if err := keys.Create(ctx, &models.APIKey{APIID: a.ID, Label: "painel", Source: models.APIKeySourceKeycloak,
		ExternalLabel: &client, SecretID: &live, Scopes: []string{"pets:ler"}, CreatedBy: &owner}); err != nil {
		t.Fatalf("key: %v", err)
	}
	past := time.Now().Add(-time.Hour)
	if err := keys.Create(ctx, &models.APIKey{APIID: a.ID, Label: "old", Source: models.APIKeySourceManual,
		SecretID: &revoked, RevokedAt: &past, CreatedBy: &owner}); err != nil {
		t.Fatalf("revoked key: %v", err)
	}

	k, err := keys.BySecret(ctx, live)
	if err != nil || k == nil || k.ExternalLabel == nil || *k.ExternalLabel != client || len(k.Scopes) != 1 {
		t.Fatalf("BySecret(live) = %+v, %v", k, err)
	}
	for name, id := range map[string]int64{"revoked": revoked, "plain": plain} {
		if k, err := keys.BySecret(ctx, id); err != nil || k != nil {
			t.Errorf("BySecret(%s) = %+v, %v; want nil", name, k, err)
		}
	}
}
