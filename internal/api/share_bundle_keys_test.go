package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// TestPublicBundle_KeyScopes: a shared API key secret carries its client id,
// scopes (kind + description from the API's /escopos) and the realm's token
// URL; a plain secret carries no key; with /escopos down the redeem is still
// 200 with the scope names.
func TestPublicBundle_KeyScopes(t *testing.T) {
	f := newBundleHandlerFixture(t)
	ctx := context.Background()

	escopos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]kcadmin.ScopeInfo{
			{Name: "servidores:cadastro", Kind: "route", Description: "referência", Routes: []string{"/api/cadastro"}},
			{Name: "servidores:demo", Kind: "modifier", Description: "anonimiza"},
		})
	}))

	a := &models.APICatalog{Name: "Servidores", SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
		BaseURL: "https://gateway.example/servidores", OwnerUserID: f.owner.ID, CreatedBy: f.owner.ID}
	apis := store.NewAPICatalogRepo(f.d.SQL)
	if err := apis.Create(ctx, a, nil); err != nil {
		t.Fatalf("api: %v", err)
	}
	if _, err := apis.SetKeyManagement(ctx, a.ID, store.KeyManagementUpdate{
		Mode: models.APIKeyManagementKeycloak, AdminBaseURL: escopos.URL, ScopePrefix: "servidores"}); err != nil {
		t.Fatalf("key management: %v", err)
	}
	if err := store.NewAppSettingsRepo(f.d.SQL).Set(ctx, kcadmin.SettingBaseURL, "https://gateway.example/"); err != nil {
		t.Fatalf("settings: %v", err)
	}

	keySecret, err := f.repo.Create(ctx, f.actor, &models.Secret{
		Type: models.SecretTypeAPIKey, Scope: models.SecretScopeAvulso, Visibility: models.SecretVisibilityPersonal,
		OwnerUserID: f.owner.ID, Name: "servidores-painel", KeyVersion: 1, CreatedBy: f.owner.ID,
	}, `{"value":"s3cr3t","client_id":"servidores-painel"}`)
	if err != nil {
		t.Fatalf("key secret: %v", err)
	}
	client, rate := "servidores-painel", 60
	if err := store.NewAPIKeyRepo(f.d.SQL).Create(ctx, &models.APIKey{APIID: a.ID, Label: "painel",
		Source: models.APIKeySourceKeycloak, ExternalLabel: &client, SecretID: &keySecret,
		Scopes: []string{"servidores:cadastro", "servidores:demo"}, RateLimitPerMinute: &rate}); err != nil {
		t.Fatalf("key: %v", err)
	}

	tok, _, err := f.repo.CreateBundle(ctx, f.actor, []vault.BundleItemInput{
		{Type: vault.BundleItemSecret, RefID: keySecret},
		{Type: vault.BundleItemSecret, RefID: f.secretID},
	}, vault.CreateBundleOpts{Title: "chave", TTL: time.Hour})
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}

	h := &publicBundleHandlers{repo: f.repo, db: f.d}
	redeem := func() map[string]vault.BundleSecretItem {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/share-bundle/"+tok, nil)
		req.SetPathValue("token", tok)
		rec := httptest.NewRecorder()
		h.handleRedeem(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("redeem = %d %s", rec.Code, rec.Body.String())
		}
		var p vault.BundlePayload
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out := map[string]vault.BundleSecretItem{}
		for _, s := range p.Secrets {
			out[s.Name] = s
		}
		return out
	}

	got := redeem()
	if got["db-pass"].Key != nil {
		t.Errorf("plain secret carries a key: %+v", got["db-pass"].Key)
	}
	k := got["servidores-painel"].Key
	if k == nil || k.ClientID != client || k.APIName != "Servidores" || k.APIBaseURL != a.BaseURL ||
		k.RateLimitPerMinute == nil || *k.RateLimitPerMinute != 60 ||
		k.TokenURL != "https://gateway.example/realms/apis/protocol/openid-connect/token" {
		t.Fatalf("key info = %+v", k)
	}
	want := []vault.BundleScopeInfo{
		{Name: "servidores:cadastro", Kind: "route", Description: "referência"},
		{Name: "servidores:demo", Kind: "modifier", Description: "anonimiza"},
	}
	if len(k.Scopes) != 2 || k.Scopes[0] != want[0] || k.Scopes[1] != want[1] {
		t.Fatalf("scopes = %+v", k.Scopes)
	}

	// The API's /escopos is unreachable: still 200, names only.
	escopos.Close()
	k = redeem()["servidores-painel"].Key
	if k == nil || len(k.Scopes) != 2 || k.Scopes[0] != (vault.BundleScopeInfo{Name: "servidores:cadastro"}) {
		t.Fatalf("scopes without /escopos = %+v", k)
	}
}
