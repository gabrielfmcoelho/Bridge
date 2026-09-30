package api

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Coolify picks the key to upload: the caller's choice, then the key the
// remote user links to, then the host's own.
func TestCoolifySelectRegistrationKeyOrder(t *testing.T) {
	env := newSecretAPIEnv(t)
	ctx := context.Background()
	actor := vault.ActorContext{UserID: env.alice.ID, Role: "admin"}
	repo := vault.NewSecretRepo(env.d)
	mk := func(name, priv string) int64 {
		id, err := repo.Create(ctx, actor, &models.Secret{Type: models.SecretTypeSSHKey, Scope: models.SecretScopeAvulso,
			Visibility: models.SecretVisibilityShared, OwnerUserID: env.alice.ID, Name: name, KeyVersion: 1, CreatedBy: env.alice.ID},
			`{"private_key_pem":"`+priv+`"}`)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	picked, linked := mk("picked key", "PRIV-A"), mk("linked", "PRIV-B")
	var hostID int64
	env.d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, ssh_user) VALUES ('h','h-01','deploy') RETURNING id`).Scan(&hostID)
	host, _ := store.NewHostRepo(env.d.SQL).GetByID(ctx, hostID)
	if err := vault.HostSetSSHKey(ctx, env.d, hostID, env.alice.ID, vault.HostSSHKey{PrivateKeyPEM: "PRIV-OWN"}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewHostRemoteUserRepo(env.d.SQL).CreateOrUpdate(ctx, hostID, "coolify", &linked); err != nil {
		t.Fatal(err)
	}
	h := &coolifyHandlers{db: env.d}

	if priv, name, _, err := h.selectRegistrationKey(host, picked, "coolify"); err != nil || priv != "PRIV-A" || name != "sshcm-key-picked_key" {
		t.Errorf("explicit = %q %q %v", priv, name, err)
	}
	if priv, name, _, err := h.selectRegistrationKey(host, 0, "coolify"); err != nil || priv != "PRIV-B" || name != "sshcm-key-linked" {
		t.Errorf("linked = %q %q %v", priv, name, err)
	}
	if priv, name, _, err := h.selectRegistrationKey(host, 0, ""); err != nil || priv != "PRIV-OWN" || name != "sshcm-h-01" {
		t.Errorf("own = %q %q %v", priv, name, err)
	}
}
