package vault_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// The list labels every bundle's items in one pass; each bundle must still get
// exactly its own items, in order, and a deleted secret reads "(deleted)".
func TestListBundles_LabelsEachBundleItsOwnItems(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	s1, err := env.repo.Create(ctx, env.bob, mkInput("avulso", "personal", "db-pass", 0, env.bob), "x")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := env.repo.Create(ctx, env.bob, mkInput("avulso", "personal", "api-key", 0, env.bob), "y")
	if err != nil {
		t.Fatal(err)
	}
	apiID := seedAPI(t, env, env.bob.UserID)
	opts := vault.CreateBundleOpts{TTL: time.Hour}
	if _, _, err := env.repo.CreateBundle(ctx, env.bob, []vault.BundleItemInput{
		{Type: vault.BundleItemSecret, RefID: s1}, {Type: vault.BundleItemAPIDoc, RefID: apiID},
	}, opts); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.repo.CreateBundle(ctx, env.bob, []vault.BundleItemInput{
		{Type: vault.BundleItemAPIDoc, RefID: apiID}, {Type: vault.BundleItemSecret, RefID: s2},
	}, opts); err != nil {
		t.Fatal(err)
	}
	if err := env.repo.SoftDelete(ctx, env.bob, s2); err != nil {
		t.Fatal(err)
	}

	list, err := env.repo.ListBundles(ctx, env.bob)
	if err != nil {
		t.Fatal(err)
	}
	labels := func(b vault.BundleView) []string {
		var out []string
		for _, it := range b.Items {
			out = append(out, it.Label)
		}
		return out
	}
	if len(list) != 2 {
		t.Fatalf("bundles = %d", len(list))
	}
	// Newest first.
	if got := labels(list[0]); len(got) != 2 || got[0] != "Things API" || got[1] != "(deleted)" {
		t.Errorf("second bundle labels = %v", got)
	}
	if got := labels(list[1]); len(got) != 2 || got[0] != "db-pass" || got[1] != "Things API" {
		t.Errorf("first bundle labels = %v", got)
	}
}
