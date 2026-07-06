package vault_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestBundle_UpdateItems_SameTokenReplacesItems verifies the in-place edit path:
// UpdateBundleItems swaps a live bundle's item set without changing the token, so
// the same /share/{token} URL redeems the new contents. Also covers owner-scoping
// and the non-empty rule.
func TestBundle_UpdateItems_SameTokenReplacesItems(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	secretID, err := env.repo.Create(ctx, env.bob, mkInput("avulso", "personal", "db-pass", 0, env.bob), "s3cr3t")
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	apiID := seedAPI(t, env, env.bob.UserID)

	// Start with an API-only link.
	tok, view, err := env.repo.CreateBundle(ctx, env.bob, []vault.BundleItemInput{
		{Type: vault.BundleItemAPIDoc, RefID: apiID},
	}, vault.CreateBundleOpts{Title: "vendor link", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	if p, err := env.repo.RedeemBundle(ctx, tok, "", vault.RedeemMeta{}); err != nil {
		t.Fatalf("redeem: %v", err)
	} else if len(p.Secrets) != 0 || len(p.APIDocs) != 1 {
		t.Fatalf("initial payload: secrets=%d apis=%d", len(p.Secrets), len(p.APIDocs))
	}

	// Foreign actor cannot edit — must collapse to not-found.
	if _, err := env.repo.UpdateBundleItems(ctx, env.alice, view.ID, []vault.BundleItemInput{
		{Type: vault.BundleItemAPIDoc, RefID: apiID},
	}); !errors.Is(err, vault.ErrBundleNotFound) {
		t.Fatalf("foreign edit: want ErrBundleNotFound, got %v", err)
	}

	// Empty item set is rejected.
	if _, err := env.repo.UpdateBundleItems(ctx, env.bob, view.ID, nil); !errors.Is(err, vault.ErrBundleEmpty) {
		t.Fatalf("empty edit: want ErrBundleEmpty, got %v", err)
	}

	// Owner adds the secret alongside the API — items replaced in place.
	updated, err := env.repo.UpdateBundleItems(ctx, env.bob, view.ID, []vault.BundleItemInput{
		{Type: vault.BundleItemAPIDoc, RefID: apiID},
		{Type: vault.BundleItemSecret, RefID: secretID},
	})
	if err != nil {
		t.Fatalf("update items: %v", err)
	}
	if updated.ID != view.ID || len(updated.Items) != 2 {
		t.Fatalf("updated view: id=%d items=%d", updated.ID, len(updated.Items))
	}

	// SAME token now redeems the new contents (proves the URL is unchanged).
	p, err := env.repo.RedeemBundle(ctx, tok, "", vault.RedeemMeta{})
	if err != nil {
		t.Fatalf("redeem after edit: %v", err)
	}
	if len(p.APIDocs) != 1 {
		t.Fatalf("expected 1 api doc after edit, got %d", len(p.APIDocs))
	}
	if len(p.Secrets) != 1 || p.Secrets[0].Payload != "s3cr3t" {
		t.Fatalf("expected the added secret after edit, got %+v", p.Secrets)
	}
}
