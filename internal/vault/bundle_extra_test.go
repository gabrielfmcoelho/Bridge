package vault_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestBundle_ListForItem verifies the server-side "which bundles expose this
// item" filter — the generalized form of the handler's ?secret_id= filter,
// used by the API-share modal to list links already emitted for a given API.
func TestBundle_ListForItem(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	apiID := seedAPI(t, env, env.bob.UserID)
	secretID, err := env.repo.Create(ctx, env.bob, mkInput("avulso", "personal", "db-pass", 0, env.bob), "s3cr3t")
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	// Bundle A: contains the API doc (+ the secret).
	_, viewA, err := env.repo.CreateBundle(ctx, env.bob, []vault.BundleItemInput{
		{Type: vault.BundleItemAPIDoc, RefID: apiID},
		{Type: vault.BundleItemSecret, RefID: secretID},
	}, vault.CreateBundleOpts{Title: "A", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create bundle A: %v", err)
	}

	// Bundle B: ONLY the secret — does not reference the API doc.
	_, viewB, err := env.repo.CreateBundle(ctx, env.bob, []vault.BundleItemInput{
		{Type: vault.BundleItemSecret, RefID: secretID},
	}, vault.CreateBundleOpts{Title: "B", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create bundle B: %v", err)
	}

	// Filtering by the API doc returns ONLY bundle A, with its items attached.
	gotAPI, err := env.repo.ListBundlesForItem(ctx, env.bob, vault.BundleItemAPIDoc, apiID)
	if err != nil {
		t.Fatalf("list for api: %v", err)
	}
	if len(gotAPI) != 1 || gotAPI[0].ID != viewA.ID {
		t.Fatalf("expected only bundle A (id=%d), got %+v", viewA.ID, gotAPI)
	}
	if len(gotAPI[0].Items) != 2 {
		t.Errorf("expected 2 items attached to bundle A, got %d", len(gotAPI[0].Items))
	}

	// Filtering by the secret returns BOTH A and B (each contains the secret).
	gotSecret, err := env.repo.ListBundlesForItem(ctx, env.bob, vault.BundleItemSecret, secretID)
	if err != nil {
		t.Fatalf("list for secret: %v", err)
	}
	if len(gotSecret) != 2 {
		t.Fatalf("expected 2 bundles containing the secret, got %d (%+v)", len(gotSecret), gotSecret)
	}

	// Owner-scoped: a different user sees none of bob's bundles.
	gotAlice, err := env.repo.ListBundlesForItem(ctx, env.alice, vault.BundleItemAPIDoc, apiID)
	if err != nil {
		t.Fatalf("list as alice: %v", err)
	}
	if len(gotAlice) != 0 {
		t.Errorf("owner-scoping broken: alice sees %d of bob's bundles", len(gotAlice))
	}
	_ = viewB
}

// TestBundle_Renew verifies a share can be renewed/extended WITHOUT changing
// its token: the same URL keeps redeeming. Renew also reactivates a revoked
// bundle and can adjust max_views. It is owner-only.
func TestBundle_Renew(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	// Create a bundle that expires almost immediately.
	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "renew-me", TTL: time.Millisecond})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	time.Sleep(15 * time.Millisecond)

	// It is expired now.
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != vault.ErrShareLinkExpired {
		t.Fatalf("expected expired, got %v", err)
	}

	// Renew extends the window — SAME token must redeem.
	updated, err := env.repo.RenewBundle(ctx, env.bob, view.ID, vault.RenewBundleOpts{TTL: time.Hour})
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.After(time.Now()) {
		t.Errorf("expected future expiry after renew, got %v", updated.ExpiresAt)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != nil {
		t.Errorf("renewed link should redeem with the SAME token, got %v", err)
	}

	// Renew reactivates a revoked bundle.
	if err := env.repo.RevokeBundle(ctx, env.bob, view.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != vault.ErrShareLinkRevoked {
		t.Fatalf("expected revoked, got %v", err)
	}
	if _, err := env.repo.RenewBundle(ctx, env.bob, view.ID, vault.RenewBundleOpts{TTL: time.Hour}); err != nil {
		t.Fatalf("renew after revoke: %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != nil {
		t.Errorf("renewed (reactivated) link should redeem, got %v", err)
	}

	// Renew can raise the view cap.
	mv := 5
	updated2, err := env.repo.RenewBundle(ctx, env.bob, view.ID, vault.RenewBundleOpts{TTL: time.Hour, MaxViews: &mv})
	if err != nil {
		t.Fatalf("renew with max_views: %v", err)
	}
	if updated2.MaxViews == nil || *updated2.MaxViews != 5 {
		t.Errorf("expected max_views=5 after renew, got %v", updated2.MaxViews)
	}

	// Owner-only: a different user cannot renew bob's bundle.
	if _, err := env.repo.RenewBundle(ctx, env.alice, view.ID, vault.RenewBundleOpts{TTL: time.Hour}); err != vault.ErrBundleNotFound {
		t.Errorf("expected ErrBundleNotFound for non-owner renew, got %v", err)
	}
}

// TestBundle_ArchiveRedeemRenew verifies the soft-delete lifecycle: an archived
// (deleted_at) bundle stops redeeming but stays visible to its owner, and Renew
// revives it under the SAME token.
func TestBundle_ArchiveRedeemRenew(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "archive-me", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Simulate the janitor archiving it (soft-delete).
	if _, err := env.d.SQL.Exec(`UPDATE share_bundles SET deleted_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), view.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// An archived link must not redeem (collapses to not-found publicly).
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != vault.ErrShareLinkNotFound {
		t.Fatalf("archived link should not redeem, got %v", err)
	}

	// The owner STILL sees it (flagged DeletedAt) so it can be revived.
	got, err := env.repo.ListBundlesForItem(ctx, env.bob, vault.BundleItemAPIDoc, apiID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].DeletedAt == nil {
		t.Fatalf("archived bundle should be listed with DeletedAt set, got %+v", got)
	}

	// Renew revives it under the SAME token and clears the archived flag.
	if _, err := env.repo.RenewBundle(ctx, env.bob, view.ID, vault.RenewBundleOpts{TTL: time.Hour}); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != nil {
		t.Errorf("revived link should redeem with the same token, got %v", err)
	}
	got2, _ := env.repo.ListBundlesForItem(ctx, env.bob, vault.BundleItemAPIDoc, apiID)
	if len(got2) != 1 || got2[0].DeletedAt != nil {
		t.Errorf("revived bundle should have DeletedAt cleared, got %+v", got2)
	}
}

// TestBundle_Reissue verifies a link whose row no longer exists can be rebuilt
// under the SAME raw token (the URL the holder still has), and that reissuing a
// token already in use is rejected.
func TestBundle_Reissue(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	rawToken := "AOD2ud7zvCDBq65zmM-1ovtxyr4ZYsfu6wQ6uj0N8Pg"
	view, err := env.repo.ReissueBundle(ctx, env.bob, rawToken,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "revived", TTL: time.Hour})
	if err != nil {
		t.Fatalf("reissue: %v", err)
	}
	if view.ID == 0 || len(view.Items) != 1 {
		t.Fatalf("unexpected reissued view: %+v", view)
	}

	// The SAME raw token now redeems.
	payload, err := env.repo.RedeemBundle(ctx, rawToken, "")
	if err != nil {
		t.Fatalf("redeem reissued: %v", err)
	}
	if len(payload.APIDocs) != 1 {
		t.Errorf("expected the api doc to resolve, got %+v", payload)
	}

	// Reissuing a token already in use is rejected — the owner should Renew.
	if _, err := env.repo.ReissueBundle(ctx, env.bob, rawToken,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{TTL: time.Hour}); err != vault.ErrBundleTokenInUse {
		t.Errorf("expected ErrBundleTokenInUse for live token, got %v", err)
	}
}

// TestBundle_NeverExpires verifies a bundle can be created with no expiry
// (NULL expires_at), redeems without an expiry gate, and that Renew can toggle
// between a finite window and never.
func TestBundle_NeverExpires(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "forever", NoExpiry: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if view.ExpiresAt != nil {
		t.Errorf("never-expiring bundle should have nil ExpiresAt, got %v", *view.ExpiresAt)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != nil {
		t.Errorf("never-expiring link should redeem, got %v", err)
	}
	got, err := env.repo.ListBundlesForItem(ctx, env.bob, vault.BundleItemAPIDoc, apiID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ExpiresAt != nil {
		t.Errorf("listed never-expiry bundle should have nil ExpiresAt, got %+v", got)
	}

	// Renew to a finite window, then back to never.
	upd, err := env.repo.RenewBundle(ctx, env.bob, view.ID, vault.RenewBundleOpts{TTL: time.Hour})
	if err != nil {
		t.Fatalf("renew finite: %v", err)
	}
	if upd.ExpiresAt == nil {
		t.Errorf("finite renew should set an expiry")
	}
	upd2, err := env.repo.RenewBundle(ctx, env.bob, view.ID, vault.RenewBundleOpts{NoExpiry: true})
	if err != nil {
		t.Fatalf("renew never: %v", err)
	}
	if upd2.ExpiresAt != nil {
		t.Errorf("never renew should clear the expiry, got %v", *upd2.ExpiresAt)
	}
}
