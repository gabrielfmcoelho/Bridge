package vault_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// The list carries the context the vault page shows — parent name, owner,
// hosts using a shared credential, repeated values — and a filter for host
// credentials, without leaking another user's personal secret into a group.
func TestListCarriesContextAndDuplicates(t *testing.T) {
	d, hid, uid := newHostSecretFixture(t)
	ctx := context.Background()
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	vault.HostSetPassword(ctx, d, hid, uid, "same")
	shared := createSecret(t, repo, admin, "password", "avulso", "shared", nil, "deploy", `{"value":"same"}`)
	vault.LinkHostCredential(ctx, d.SQL, hid, shared, "password")
	var bob int64
	d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('bob','x','editor') RETURNING id`).Scan(&bob)
	bobActor := vault.ActorContext{UserID: bob, Role: "editor"}
	createSecret(t, repo, bobActor, "password", "avulso", "personal", nil, "mine", "same")
	env := "prod"
	if _, err := repo.Create(ctx, admin, &models.Secret{Type: "env_var", Scope: "avulso", Visibility: "shared",
		OwnerUserID: uid, Name: "A", GroupLabel: &env, KeyVersion: 1, CreatedBy: uid}, `{"value":"same"}`); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, admin, vault.SecretFilter{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]vault.SecretView{}
	for _, v := range list {
		byName[v.Name] = v
	}
	if p := byName["password"].ParentName; p == nil || *p != "h1" {
		t.Fatalf("parent name = %v, want h1", p)
	}
	if o := byName["deploy"].OwnerName; o == nil || *o != "alice" {
		t.Fatalf("owner name = %v", o)
	}
	if n := byName["deploy"].LinkedHosts; n != 1 {
		t.Fatalf("linked hosts = %d, want 1", n)
	}
	// Host copy + shared credential repeat; bob's personal one isn't counted
	// for the admin, the env var never is.
	if byName["deploy"].DupCount != 2 || byName["password"].DupGroup != byName["deploy"].DupGroup {
		t.Fatalf("dup = %d/%q vs %q", byName["deploy"].DupCount, byName["deploy"].DupGroup, byName["password"].DupGroup)
	}
	if byName["mine"].DupCount != 0 || byName["A"].DupCount != 0 {
		t.Fatalf("personal/env grouped: %+v %+v", byName["mine"], byName["A"])
	}

	hostCreds, _ := repo.List(ctx, admin, vault.SecretFilter{Kind: "host_cred"})
	for _, v := range hostCreds {
		if v.Type != "password" && v.Type != "sshkey" {
			t.Fatalf("host_cred kind returned %s", v.Type)
		}
	}
	if q, _ := repo.List(ctx, admin, vault.SecretFilter{Query: "h1"}); len(q) != 1 || q[0].Name != "password" {
		t.Fatalf("q=h1 → %+v, want the host's password", q)
	}
}
