package kcadmin_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
)

func TestClientLifecycle(t *testing.T) {
	ctx := context.Background()
	fake, kcSrv, apiSrv := kcfake.New(t, "keycloak.example", "apis", []map[string]any{
		{"name": "servidores:cadastro", "kind": "route", "description": "Cadastro", "routes": []string{"/api/cadastro"}},
		{"name": "servidores:admin.uso", "kind": "route", "description": "Uso", "routes": []string{"/admin/uso"}},
	})
	cfg := kcadmin.Config{BaseURL: "http://keycloak.example", InternalURL: kcSrv.URL, HostHeader: "keycloak.example",
		Realm: "apis", ClientID: kcfake.AdminClient, ClientSecret: kcfake.AdminSecret}
	kc := kcadmin.New(cfg)

	cat, err := kcadmin.Catalogue(ctx, apiSrv.URL)
	if err != nil || len(cat) != 2 || cat[0].Name != "servidores:cadastro" {
		t.Fatalf("catalogue = %v %v", cat, err)
	}
	if n, err := kc.EnsureClientScopes(ctx, cat); err != nil || n != 2 {
		t.Fatalf("ensure scopes = %d %v, want 2 created", n, err)
	}
	if n, err := kc.EnsureClientScopes(ctx, cat); err != nil || n != 0 {
		t.Fatalf("ensure scopes again = %d %v, want 0 (idempotent)", n, err)
	}

	id, secret, err := kc.CreateClient(ctx, "servidores-painel", "rh", []string{"servidores:cadastro"}, 120)
	if err != nil || id == "" || secret == "" {
		t.Fatalf("create = %q %q %v", id, secret, err)
	}
	// A client the integration itself uses is never listed, even under a prefix.
	resCfg := cfg
	resCfg.UsageClientID = "servidores-painel"
	if l, err := kcadmin.New(resCfg).ListClients(ctx, "servidores"); err != nil || len(l) != 0 {
		t.Errorf("reserved client listed: %v %v", l, err)
	}
	list, err := kc.ListClients(ctx, "servidores")
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	ci := list[0]
	if got := ci.Scopes(); !slices.Equal(got, []string{"servidores:cadastro"}) {
		t.Errorf("scopes = %v (offline_access must be ignored)", got)
	}
	if rl := ci.RateLimit(); rl == nil || *rl != 120 {
		t.Errorf("rate limit = %v, want 120", rl)
	}
	// A scope the realm lacks is refused before the client is half-made.
	if _, _, err := kc.CreateClient(ctx, "servidores-x", "", []string{"servidores:nope"}, 0); err == nil {
		t.Error("create with an unknown scope succeeded")
	}
	fake.Mu.Lock()
	if fake.ByClientID("servidores-x") != nil {
		t.Error("failed create left the client behind")
	}
	fake.Mu.Unlock()
	if _, _, err := kc.CreateClient(ctx, "servidores-painel", "", nil, 0); !isStatus(err, http.StatusConflict) {
		t.Errorf("duplicate create = %v, want 409", err)
	}

	if err := kc.SetScopes(ctx, id, []string{"servidores:admin.uso"}); err != nil {
		t.Fatal(err)
	}
	if err := kc.SetRateLimit(ctx, id, 30); err != nil {
		t.Fatal(err)
	}
	got, _ := kc.GetClient(ctx, id)
	if !slices.Equal(got.Scopes(), []string{"servidores:admin.uso"}) || *got.RateLimit() != 30 {
		t.Errorf("after edit: scopes %v rate %v", got.Scopes(), *got.RateLimit())
	}
	if err := kc.SetRateLimit(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := kc.GetClient(ctx, id); got.RateLimit() != nil {
		t.Error("rate 0 should drop the claim")
	}

	if s2, err := kc.RegenerateSecret(ctx, id); err != nil || s2 == secret {
		t.Errorf("regenerate = %q %v", s2, err)
	}
	if err := kc.SetEnabled(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if found, _ := kc.FindClient(ctx, "servidores-painel"); found == nil || found.Enabled {
		t.Errorf("after disable = %+v", found)
	}
	if found, err := kc.FindClient(ctx, "servidores-none"); found != nil || err != nil {
		t.Errorf("find missing = %v %v", found, err)
	}

	fake.Mu.Lock()
	fake.Usage["servidores-painel"] = 7
	fake.Mu.Unlock()
	// Without the usage client there is no usage: the admin token never
	// goes to an API.
	if _, err := kc.Usage(ctx, apiSrv.URL, "servidores", "servidores-painel"); !errors.Is(err, kcadmin.ErrUsageNotConfigured) {
		t.Fatalf("usage without usage client = %v", err)
	}
	cfg.UsageClientID, cfg.UsageClientSecret = kcfake.UsageClient, kcfake.UsageSecret
	kc = kcadmin.New(cfg)
	u, err := kc.Usage(ctx, apiSrv.URL, "servidores", "servidores-painel")
	if err != nil || u.Lifetime != 7 || u.Daily["2026-09-30"] != 7 {
		t.Fatalf("usage = %+v %v", u, err)
	}

	fake.Mu.Lock()
	defer fake.Mu.Unlock()
	if len(fake.BadHost) > 0 {
		t.Errorf("requests without the Host header: %v", fake.BadHost)
	}
	if !slices.Contains(fake.TokenScopes, "servidores:admin.uso") {
		t.Errorf("usage token did not ask for admin.uso: %v", fake.TokenScopes)
	}
	for _, b := range fake.UsageBearers {
		if strings.Contains(b, kcfake.AdminClient) {
			t.Errorf("the admin token reached the API: %q", b)
		}
	}
	if len(fake.UsageBearers) != 1 {
		t.Errorf("usage calls = %v, want 1", fake.UsageBearers)
	}
}

func TestBadCredentials(t *testing.T) {
	_, kcSrv, _ := kcfake.New(t, "keycloak.example", "apis", nil)
	kc := kcadmin.New(kcadmin.Config{BaseURL: "http://keycloak.example", InternalURL: kcSrv.URL, HostHeader: "keycloak.example",
		Realm: "apis", ClientID: kcfake.AdminClient, ClientSecret: "wrong"})
	if _, err := kc.ListClients(context.Background(), "sei"); !isStatus(err, http.StatusUnauthorized) {
		t.Errorf("bad secret = %v, want 401", err)
	}
	if _, err := kcadmin.New(kcadmin.Config{}).ListClients(context.Background(), "sei"); err == nil {
		t.Error("unconfigured client should refuse")
	}
}

func isStatus(err error, status int) bool {
	var ke *kcadmin.Error
	return errors.As(err, &ke) && ke.Status == status
}
