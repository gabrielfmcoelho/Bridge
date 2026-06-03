package vault_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/apicatalog"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// seedAPI inserts a small two-operation API catalog row for bundle tests.
func seedAPI(t *testing.T, env *secretTestEnv, owner int64) int64 {
	t.Helper()
	a := &models.APICatalog{
		Scope:       models.APICatalogScopeAvulso,
		Name:        "Things API",
		SourceType:  models.APICatalogSourceUpload,
		SpecVersion: "openapi-3.0.1",
		SpecJSON:    `{"openapi":"3.0.1","info":{"title":"Things"},"paths":{"/things":{"get":{"operationId":"listThings"}},"/things/{id}":{"delete":{}}}}`,
		SpecHash:    "hash",
		Title:       "Things",
		OwnerUserID: owner,
		CreatedBy:   owner,
	}
	ops := []models.APIOperation{
		{Method: "GET", Path: "/things", OperationID: "listThings", OpKey: "listThings"},
		{Method: "DELETE", Path: "/things/{id}", OpKey: "DELETE /things/{id}"},
	}
	if err := store.NewAPICatalogRepo(env.d.SQL).Create(context.Background(), a, ops); err != nil {
		t.Fatalf("seed api: %v", err)
	}
	return a.ID
}

func pathsOf(t *testing.T, spec json.RawMessage) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("spec json: %v", err)
	}
	p, _ := doc["paths"].(map[string]any)
	return p
}

func TestBundle_CreateRedeem_LiveResolveAndPartialFilter(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()

	secretID, err := env.repo.Create(ctx, env.bob, mkInput("avulso", "personal", "db-pass", 0, env.bob), "s3cr3t")
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	apiID := seedAPI(t, env, env.bob.UserID)

	tok, view, err := env.repo.CreateBundle(ctx, env.bob, []vault.BundleItemInput{
		{Type: vault.BundleItemSecret, RefID: secretID},
		{Type: vault.BundleItemAPIDoc, RefID: apiID, Selector: &apicatalog.Selector{Mode: apicatalog.SelectorOperations, OpKeys: []string{"listThings"}}},
	}, vault.CreateBundleOpts{Title: "for the vendor", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	if tok == "" || view.ID == 0 || len(view.Items) != 2 {
		t.Fatalf("unexpected view: %+v", view)
	}

	payload, err := env.repo.RedeemBundle(ctx, tok, "")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if len(payload.Secrets) != 1 || payload.Secrets[0].Payload != "s3cr3t" {
		t.Fatalf("secret not live-resolved: %+v", payload.Secrets)
	}
	if len(payload.APIDocs) != 1 {
		t.Fatalf("api docs = %d", len(payload.APIDocs))
	}
	paths := pathsOf(t, payload.APIDocs[0].Spec)
	if _, ok := paths["/things"]; !ok {
		t.Error("expected /things in filtered spec")
	}
	if _, ok := paths["/things/{id}"]; ok {
		t.Error("expected /things/{id} filtered OUT (not selected)")
	}

	// Live-resolve: mutate the secret, redeem again -> new value.
	newVal := "rotated!"
	if err := env.repo.Update(ctx, env.bob, secretID, vault.SecretPatch{Payload: &newVal}); err != nil {
		t.Fatalf("update secret: %v", err)
	}
	payload2, err := env.repo.RedeemBundle(ctx, tok, "")
	if err != nil {
		t.Fatalf("redeem 2: %v", err)
	}
	if payload2.Secrets[0].Payload != "rotated!" {
		t.Errorf("live-resolve did not reflect update: %q", payload2.Secrets[0].Payload)
	}

	// Delete the secret -> it drops out gracefully, API doc stays.
	if err := env.repo.SoftDelete(ctx, env.bob, secretID); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	payload3, err := env.repo.RedeemBundle(ctx, tok, "")
	if err != nil {
		t.Fatalf("redeem 3: %v", err)
	}
	if len(payload3.Secrets) != 0 {
		t.Errorf("deleted secret should be skipped, got %+v", payload3.Secrets)
	}
	if len(payload3.APIDocs) != 1 {
		t.Errorf("api doc should still resolve, got %d", len(payload3.APIDocs))
	}
}

func TestBundle_Gates(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	apiID := seedAPI(t, env, env.bob.UserID)

	// Passphrase gate.
	tok, _, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{TTL: time.Hour, Passphrase: "open-sesame"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, ""); err != vault.ErrShareLinkPassphraseBad {
		t.Errorf("expected passphrase error, got %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok, "open-sesame"); err != nil {
		t.Errorf("correct passphrase should redeem, got %v", err)
	}

	// Wrong token.
	if _, err := env.repo.RedeemBundle(ctx, "not-a-real-token", ""); err != vault.ErrShareLinkNotFound {
		t.Errorf("expected not-found, got %v", err)
	}

	// Revoke.
	tok2, view, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemAPIDoc, RefID: apiID}},
		vault.CreateBundleOpts{TTL: time.Hour})
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := env.repo.RevokeBundle(ctx, env.bob, view.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := env.repo.RedeemBundle(ctx, tok2, ""); err != vault.ErrShareLinkRevoked {
		t.Errorf("expected revoked, got %v", err)
	}
}

func TestBundle_AllowsSharedSecret(t *testing.T) {
	env := newSecretTestEnv(t)
	ctx := context.Background()
	// A shared secret CAN now be bundled by anyone who can reveal it
	// (policy relaxation). bob is an editor → canReveal a shared secret.
	sharedID, err := env.repo.Create(ctx, env.bob, mkInput("avulso", "shared", "team-key", 0, env.bob), "team-value")
	if err != nil {
		t.Fatalf("create shared: %v", err)
	}
	tok, _, err := env.repo.CreateBundle(ctx, env.bob,
		[]vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: sharedID}},
		vault.CreateBundleOpts{TTL: time.Hour})
	if err != nil {
		t.Fatalf("expected shared secret to be bundleable, got %v", err)
	}
	payload, err := env.repo.RedeemBundle(ctx, tok, "")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if len(payload.Secrets) != 1 || payload.Secrets[0].Payload != "team-value" {
		t.Errorf("shared secret not resolved in bundle: %+v", payload.Secrets)
	}

	// Empty bundle still rejected.
	if _, _, err := env.repo.CreateBundle(ctx, env.bob, nil, vault.CreateBundleOpts{}); err != vault.ErrBundleEmpty {
		t.Errorf("expected ErrBundleEmpty, got %v", err)
	}
}
