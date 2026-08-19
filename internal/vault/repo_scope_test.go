package vault_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestSecretRepo_EntidadeScope: personal rows stay owner-only (decideAccess);
// shared avulso rows use their own grants; shared parented rows inherit the
// parent's grants; unscoped ctx sees everything.
func TestSecretRepo_EntidadeScope(t *testing.T) {
	env := newSecretTestEnv(t)
	bg := context.Background()
	actor := env.bob // editor

	var sga int64
	if err := env.d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga); err != nil {
		t.Fatalf("entidade sga: %v", err)
	}
	grants := store.NewAssetEntidadeRepo(env.d.SQL)
	const hostGranted, hostUnassigned = int64(901), int64(902)
	if err := grants.Replace(bg, env.d.SQL, store.AssetHost, hostGranted, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant host: %v", err)
	}

	mk := func(scope, vis, name string, parent int64) int64 {
		t.Helper()
		id, err := env.repo.Create(bg, actor, mkInput(scope, vis, name, parent, actor), "pw")
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return id
	}
	a := mk("avulso", "shared", "a-avulso-granted", 0)
	if err := grants.Replace(bg, env.d.SQL, store.AssetSecret, a, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant secret: %v", err)
	}
	b := mk("avulso", "shared", "b-avulso-unassigned", 0)
	c := mk("avulso", "personal", "c-personal", 0)
	d := mk("host", "shared", "d-host-granted", hostGranted)
	e := mk("host", "shared", "e-host-unassigned", hostUnassigned)

	ids := func(views []vault.SecretView) map[int64]bool {
		out := map[int64]bool{}
		for _, v := range views {
			out[v.ID] = true
		}
		return out
	}

	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})
	views, err := env.repo.List(scoped, actor, vault.SecretFilter{})
	if err != nil {
		t.Fatalf("scoped List: %v", err)
	}
	got := ids(views)
	for _, want := range []int64{a, c, d} {
		if !got[want] {
			t.Errorf("scoped List missing id %d (got %v)", want, got)
		}
	}
	for _, no := range []int64{b, e} {
		if got[no] {
			t.Errorf("scoped List leaked id %d", no)
		}
	}
	for _, no := range []int64{b, e} {
		if _, err := env.repo.GetMetadata(scoped, actor, no); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Errorf("scoped GetMetadata(%d) err = %v, want ErrSecretNotFound", no, err)
		}
		if _, err := env.repo.Reveal(scoped, actor, no); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Errorf("scoped Reveal(%d) err = %v, want ErrSecretNotFound", no, err)
		}
		n := "renamed"
		if err := env.repo.Update(scoped, actor, no, vault.SecretPatch{Name: &n}); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Errorf("scoped Update(%d) err = %v, want ErrSecretNotFound", no, err)
		}
	}
	if _, err := env.repo.GetMetadata(scoped, actor, a); err != nil {
		t.Errorf("scoped GetMetadata(a) err = %v", err)
	}

	views, err = env.repo.List(bg, actor, vault.SecretFilter{})
	if err != nil {
		t.Fatalf("unscoped List: %v", err)
	}
	got = ids(views)
	for _, want := range []int64{a, b, c, d, e} {
		if !got[want] {
			t.Errorf("unscoped List missing id %d", want)
		}
	}
}
