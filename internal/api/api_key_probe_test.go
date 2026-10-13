package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAPIKeys_Probe(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	router := NewRouter(d, "/tmp/sshcm-test-config")

	fake, kcSrv, apiSrv := kcfake.New(t, "keycloak.example", "apis", []map[string]any{
		{"name": "servidores:cadastro", "kind": "route", "description": "referência", "routes": []string{"/api/cadastro"}},
	})

	// The gateway: APISIX checks the token (401 + realm, HTML), the API
	// checks the route's scope (403 JSON) and 404s unknown paths.
	var gwReject atomic.Bool
	var gwTokens atomic.Int32
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "APISIX/3.13.0")
		claims, ok := jwtClaims(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if !ok || gwReject.Load() {
			w.Header().Set("WWW-Authenticate", `Bearer realm="apis"`)
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "<html><head><title>401 Authorization Required</title></head></html>")
			return
		}
		gwTokens.Add(1)
		switch r.URL.Path {
		case "/servidores/api/cadastro":
			if !slices.Contains(strings.Fields(fmt.Sprint(claims["scope"])), "servidores:cadastro") {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":"AUTH_SCOPE_DENIED","detail":"escopo servidores:cadastro ausente"}`)
				return
			}
			fmt.Fprint(w, `[{"cpf":"000.000.000-00"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"detail":"Not Found"}`)
		}
	}))
	t.Cleanup(gw.Close)

	session := func(name, role string) string {
		u := &models.User{Username: name, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatalf("user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}
	admin, editor := session("ana", "admin"), session("bia", "editor")
	do := func(cookie, method, path, body string) (int, map[string]any, string) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var obj map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &obj)
		return rec.Code, obj, rec.Body.String()
	}

	var adminID int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&adminID)
	a := &models.APICatalog{Name: "Servidores", SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
		BaseURL: gw.URL + "/servidores", OwnerUserID: adminID, CreatedBy: adminID}
	if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, a, nil); err != nil {
		t.Fatalf("api: %v", err)
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetAPICatalog, a.ID, models.AssetGrants{IsGlobal: true}); err != nil {
		t.Fatalf("grants: %v", err)
	}
	base := "/api/api-catalog/" + itoa(a.ID)
	for _, s := range []struct{ path, body string }{
		{base + "/key-management", `{"key_management":"keycloak","admin_base_url":"` + apiSrv.URL + `","scope_prefix":"servidores"}`},
		{"/api/settings/integrations/keycloak_apis", fmt.Sprintf(
			`{"kc_apis_base_url":"http://keycloak.example","kc_apis_internal_url":%q,"kc_apis_host_header":"keycloak.example","kc_apis_client_id":%q,"kc_apis_client_secret":%q}`,
			kcSrv.URL, kcfake.AdminClient, kcfake.AdminSecret)},
	} {
		if code, _, raw := do(admin, "PUT", s.path, s.body); code != http.StatusOK {
			t.Fatalf("PUT %s = %d %s", s.path, code, raw)
		}
	}
	if code, _, raw := do(admin, "POST", base+"/keys/sync-scopes", ""); code != http.StatusOK {
		t.Fatalf("sync scopes = %d %s", code, raw)
	}
	issue := func(label string) string {
		code, obj, raw := do(admin, "POST", base+"/keys", `{"label":"`+label+`","scopes":["servidores:cadastro"],"rate_limit_per_minute":60}`)
		if code != http.StatusCreated {
			t.Fatalf("issue %s = %d %s", label, code, raw)
		}
		return itoa(int64(obj["key"].(map[string]any)["id"].(float64)))
	}
	painel, drift, gone := issue("painel"), issue("drift"), issue("gone")
	// drift: Keycloak lost the scope Bridge still lists.
	fake.Mu.Lock()
	fake.ByClientID("servidores-drift").Optional = nil
	fake.Mu.Unlock()
	if code, _, raw := do(admin, "POST", base+"/keys/"+gone+"/revoke", ""); code != http.StatusOK {
		t.Fatalf("revoke = %d %s", code, raw)
	}

	probe := func(key, body string) map[string]any {
		t.Helper()
		code, obj, raw := do(admin, "POST", base+"/keys/"+key+"/test", body)
		if code != http.StatusOK {
			t.Fatalf("test key %s = %d %s", key, code, raw)
		}
		if strings.Contains(raw, "fake") || strings.Contains(raw, "secret-") {
			t.Fatalf("test result leaks the token or the secret: %s", raw)
		}
		return obj
	}

	if code, _, _ := do(editor, "POST", base+"/keys/"+painel+"/test", ""); code != http.StatusForbidden {
		t.Errorf("editor tests a key = %d, want 403", code)
	}

	// Default: token + gateway, on a path no API has (the API 404s, nothing runs).
	r := probe(painel, "")
	if r["ok"] != true || r["token_ok"] != true || r["status"].(float64) != 404 || r["answered_by"] != "api" ||
		fmt.Sprint(r["granted"]) != "[servidores:cadastro]" || len(r["missing"].([]any)) != 0 ||
		r["rate_limit_per_minute"].(float64) != 60 || r["client_id"] != "servidores-painel" {
		t.Errorf("default test = %v", r)
	}
	if !strings.HasPrefix(r["url"].(string), gw.URL+"/servidores/") {
		t.Errorf("default test url = %v, want under the API's base URL", r["url"])
	}

	// A real route: 200, and the data itself is never echoed back.
	r = probe(painel, `{"path":"/api/cadastro"}`)
	if r["ok"] != true || r["status"].(float64) != 200 || r["body"] != nil {
		t.Errorf("route test = %v", r)
	}
	for _, p := range []string{"https://evil.example/x", "//evil.example/x", "api/cadastro", "/x@evil"} {
		if code, _, _ := do(admin, "POST", base+"/keys/"+painel+"/test", `{"path":"`+p+`"}`); code != http.StatusBadRequest {
			t.Errorf("path %q = %d, want 400", p, code)
		}
	}

	// Drift: the token lacks a scope Bridge lists; the route then 403s at the API.
	r = probe(drift, `{"path":"/api/cadastro"}`)
	if r["ok"] != false || fmt.Sprint(r["missing"]) != "[servidores:cadastro]" || r["status"].(float64) != 403 ||
		r["answered_by"] != "api" || !strings.Contains(fmt.Sprint(r["body"]), "AUTH_SCOPE_DENIED") {
		t.Errorf("drift test = %v", r)
	}

	// Revoked: Keycloak refuses the token, the gateway is never called.
	before := gwTokens.Load()
	r = probe(gone, "")
	if r["ok"] != false || r["token_ok"] != false || r["token_error"] == "" || r["status"] != nil {
		t.Errorf("revoked test = %v", r)
	}
	if gwTokens.Load() != before {
		t.Errorf("a revoked key still reached the gateway")
	}

	// The gateway refusing the token is told apart from the API.
	gwReject.Store(true)
	r = probe(painel, "")
	if r["ok"] != false || r["status"].(float64) != 401 || r["answered_by"] != "gateway" {
		t.Errorf("gateway-rejected test = %v", r)
	}
	gwReject.Store(false)

	// Batch: every live key (revoked ones skipped), default test only.
	code, obj, raw := do(admin, "POST", base+"/keys/test", `{"path":"/api/cadastro"}`)
	if code != http.StatusOK {
		t.Fatalf("batch = %d %s", code, raw)
	}
	res := obj["results"].([]any)
	if len(res) != 2 || obj["total"].(float64) != 2 || obj["ok"].(float64) != 1 {
		t.Fatalf("batch = %s, want 2 keys, 1 ok", raw)
	}
	for _, x := range res {
		if x.(map[string]any)["status"].(float64) != 404 {
			t.Errorf("batch ran a real route: %v", x)
		}
	}
	if code, _, _ := do(editor, "POST", base+"/keys/test", ""); code != http.StatusForbidden {
		t.Errorf("editor batch = %d, want 403", code)
	}
}
