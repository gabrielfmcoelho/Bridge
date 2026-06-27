package vault_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestBundle_DescriptionRoundTrip verifies the new title/description metadata
// survives create → owner list view → guest redeem payload, and that reissue
// carries the description too.
func TestBundle_DescriptionRoundTrip(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "Vendor onboarding", Description: "Read-only access for the integration team.", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if view.Description != "Read-only access for the integration team." {
		t.Errorf("create view description = %q", view.Description)
	}

	// Owner list view exposes the description.
	got, err := env.repo.ListBundlesForItem(ctx, env.bob, vault.BundleItemAPIDoc, apiID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Description != "Read-only access for the integration team." {
		t.Fatalf("list view description not round-tripped: %+v", got)
	}

	// Guest redeem payload carries title + description.
	payload, err := env.repo.RedeemBundle(ctx, tok, "", vault.RedeemMeta{})
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if payload.Title != "Vendor onboarding" || payload.Description != "Read-only access for the integration team." {
		t.Errorf("redeem payload title/description = %q / %q", payload.Title, payload.Description)
	}

	// Reissue under a supplied token carries the description.
	rawToken := "Zt7Yx9bQ-reissue-desc-token-000000000000000"
	rv, err := env.repo.ReissueBundle(ctx, env.bob, rawToken,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "Revived", Description: "carried over", TTL: time.Hour})
	if err != nil {
		t.Fatalf("reissue: %v", err)
	}
	if rv.Description != "carried over" {
		t.Errorf("reissue description = %q", rv.Description)
	}
	rp, err := env.repo.RedeemBundle(ctx, rawToken, "", vault.RedeemMeta{})
	if err != nil {
		t.Fatalf("redeem reissued: %v", err)
	}
	if rp.Description != "carried over" {
		t.Errorf("reissued redeem description = %q", rp.Description)
	}
}

// TestBundle_AccessLogWrittenOnRedeem verifies a successful redeem records
// exactly one anonymous access-log row carrying the passed IP / user-agent and
// used_passphrase=false (no passphrase on this bundle).
func TestBundle_AccessLogWrittenOnRedeem(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "log-me", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	meta := vault.RedeemMeta{RemoteIP: "203.0.113.7", UserAgent: "Mozilla/5.0 (probe)"}
	if _, err := env.repo.RedeemBundle(ctx, tok, "", meta); err != nil {
		t.Fatalf("redeem: %v", err)
	}

	entries, err := env.repo.BundleAccessLog(ctx, env.bob, view.ID)
	if err != nil {
		t.Fatalf("access log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 access-log row, got %d", len(entries))
	}
	e := entries[0]
	if e.RemoteIP != "203.0.113.7" || e.UserAgent != "Mozilla/5.0 (probe)" {
		t.Errorf("metadata not recorded: %+v", e)
	}
	if e.UsedPassphrase {
		t.Errorf("used_passphrase should be false for an unprotected bundle")
	}
	if e.AccessedAt.IsZero() {
		t.Errorf("accessed_at should be populated")
	}
}

// TestBundle_AccessLogPassphraseGate verifies a gated (wrong-passphrase) redeem
// writes NO log row, while a successful passphrase redeem records
// used_passphrase=true.
func TestBundle_AccessLogPassphraseGate(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "gated", TTL: time.Hour, Passphrase: "open-sesame"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Wrong passphrase: gated out, no log row.
	if _, err := env.repo.RedeemBundle(ctx, tok, "nope", vault.RedeemMeta{RemoteIP: "10.0.0.1"}); err != vault.ErrShareLinkPassphraseBad {
		t.Fatalf("expected passphrase error, got %v", err)
	}
	entries, err := env.repo.BundleAccessLog(ctx, env.bob, view.ID)
	if err != nil {
		t.Fatalf("access log after gate: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("gated redeem must not log, got %d rows", len(entries))
	}

	// Correct passphrase: logged with used_passphrase=true.
	if _, err := env.repo.RedeemBundle(ctx, tok, "open-sesame", vault.RedeemMeta{RemoteIP: "10.0.0.2"}); err != nil {
		t.Fatalf("redeem with passphrase: %v", err)
	}
	entries, err = env.repo.BundleAccessLog(ctx, env.bob, view.ID)
	if err != nil {
		t.Fatalf("access log after success: %v", err)
	}
	if len(entries) != 1 || !entries[0].UsedPassphrase {
		t.Fatalf("expected 1 row with used_passphrase=true, got %+v", entries)
	}
}

// TestBundle_AccessLogOwnerScopedNewestFirst verifies BundleAccessLog is
// owner-scoped (ErrBundleNotFound for a foreign caller) and returns rows
// newest-first.
func TestBundle_AccessLogOwnerScopedNewestFirst(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{Title: "scoped", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := env.repo.RedeemBundle(ctx, tok, "", vault.RedeemMeta{RemoteIP: "first"}); err != nil {
		t.Fatalf("redeem 1: %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, "", vault.RedeemMeta{RemoteIP: "second"}); err != nil {
		t.Fatalf("redeem 2: %v", err)
	}

	entries, err := env.repo.BundleAccessLog(ctx, env.bob, view.ID)
	if err != nil {
		t.Fatalf("access log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(entries))
	}
	if entries[0].RemoteIP != "second" {
		t.Errorf("expected newest-first ordering, got %q first", entries[0].RemoteIP)
	}

	// A non-owner cannot read the log — collapses to not-found.
	if _, err := env.repo.BundleAccessLog(ctx, env.alice, view.ID); err != vault.ErrBundleNotFound {
		t.Errorf("expected ErrBundleNotFound for non-owner, got %v", err)
	}
}
