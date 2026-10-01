package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAPIKeys_KeycloakLifecycle(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	router := NewRouter(d, "/tmp/sshcm-test-config")

	fake, kcSrv, apiSrv := kcfake.New(t, "keycloak.example", "apis", []map[string]any{
		{"name": "servidores:cadastro", "kind": "route", "description": "referência", "routes": []string{"/api/cadastro"}},
		{"name": "servidores:demo", "kind": "modifier", "description": "anonimiza", "routes": []string{}},
	})

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

	// A global API, visible to the editor too.
	var adminID int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&adminID)
	a := &models.APICatalog{Name: "Servidores", SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
		OwnerUserID: adminID, CreatedBy: adminID}
	if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, a, nil); err != nil {
		t.Fatalf("api: %v", err)
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetAPICatalog, a.ID, models.AssetGrants{IsGlobal: true}); err != nil {
		t.Fatalf("grants: %v", err)
	}
	base := "/api/api-catalog/" + itoa(a.ID)

	// Mode: admin-only; keycloak needs a prefix and a base URL; "bridge" is reserved.
	cfg := `{"key_management":"keycloak","admin_base_url":"` + apiSrv.URL + `","scope_prefix":"servidores"}`
	if code, _, _ := do(editor, "PUT", base+"/key-management", cfg); code != http.StatusForbidden {
		t.Fatalf("editor sets key management = %d, want 403", code)
	}
	if code, _, _ := do(admin, "PUT", base+"/key-management", `{"key_management":"keycloak","admin_base_url":"`+apiSrv.URL+`"}`); code != http.StatusBadRequest {
		t.Errorf("keycloak without prefix = %d, want 400", code)
	}
	if code, _, _ := do(admin, "PUT", base+"/key-management", `{"key_management":"keycloak","admin_base_url":"`+apiSrv.URL+`","scope_prefix":"sei-x"}`); code != http.StatusBadRequest {
		t.Errorf("prefix with a dash = %d, want 400 (prefixes must not overlap)", code)
	}
	if code, _, _ := do(admin, "PUT", base+"/key-management", `{"key_management":"keycloak","scope_prefix":"bridge"}`); code != http.StatusBadRequest {
		t.Errorf("prefix bridge on another API = %d, want 400", code)
	}
	code, obj, raw := do(admin, "PUT", base+"/key-management", cfg)
	if code != http.StatusOK || obj["scope_prefix"] != "servidores" || obj["key_management"] != "keycloak" {
		t.Fatalf("set key management = %d %s", code, raw)
	}

	// Without the integration, the connection test says so (still 200).
	if _, obj, _ := do(admin, "POST", base+"/key-management/test", ""); obj["success"] != false {
		t.Errorf("test without integration = %v, want success false", obj)
	}
	// The integration settings (secret encrypted like the other groups).
	if code, _, raw := do(admin, "PUT", "/api/settings/integrations/keycloak_apis", fmt.Sprintf(
		`{"kc_apis_base_url":"http://keycloak.example","kc_apis_internal_url":%q,"kc_apis_host_header":"keycloak.example","kc_apis_client_id":%q,"kc_apis_client_secret":%q}`,
		kcSrv.URL, kcfake.AdminClient, kcfake.AdminSecret)); code != http.StatusOK {
		t.Fatalf("save integration = %d %s", code, raw)
	}
	if _, obj, raw := do(admin, "GET", "/api/settings/integrations", ""); strings.Contains(raw, "s3cret") || obj["keycloak_apis"] == nil {
		t.Fatalf("integration settings leak or miss the group: %s", raw)
	}
	if code, obj, _ := do(admin, "POST", base+"/key-management/test", ""); code != http.StatusOK || obj["success"] != true || obj["scopes"].(float64) != 2 {
		t.Fatalf("test connection = %d %v", code, obj)
	}

	// Scopes come from the API; sync-scopes creates them in Keycloak.
	if _, _, raw := do(editor, "GET", base+"/keys/scopes", ""); !strings.Contains(raw, "servidores:cadastro") {
		t.Fatalf("scopes = %s", raw)
	}
	if code, _, _ := do(editor, "POST", base+"/keys/sync-scopes", ""); code != http.StatusForbidden {
		t.Errorf("editor syncs scopes = %d, want 403", code)
	}
	if code, obj, _ := do(admin, "POST", base+"/keys/sync-scopes", ""); code != http.StatusOK || obj["created"].(float64) != 2 {
		t.Fatalf("sync scopes = %d %v", code, obj)
	}

	// Issue: a client servidores-painel with the scopes and the rate claim.
	if code, _, _ := do(editor, "POST", base+"/keys", `{"label":"painel","scopes":["servidores:cadastro"]}`); code != http.StatusForbidden {
		t.Fatalf("editor issues key = %d, want 403", code)
	}
	if code, _, _ := do(admin, "POST", base+"/keys", `{"label":"painel"}`); code != http.StatusBadRequest {
		t.Errorf("issue without scopes = %d, want 400", code)
	}
	// Another API's scopes are refused.
	if code, _, _ := do(admin, "POST", base+"/keys", `{"label":"x","scopes":["sei:lake.ler"]}`); code != http.StatusBadRequest {
		t.Errorf("issue with another API's scope = %d, want 400", code)
	}
	code, obj, raw = do(admin, "POST", base+"/keys", `{"label":"painel","owner":"rh@sead","scopes":["servidores:cadastro","servidores:demo"],"rate_limit_per_minute":60}`)
	if code != http.StatusCreated {
		t.Fatalf("issue = %d %s", code, raw)
	}
	secret := obj["plaintext"].(string)
	key := obj["key"].(map[string]any)
	keyID, secretID := itoa(int64(key["id"].(float64))), itoa(int64(key["secret_id"].(float64)))
	fake.Mu.Lock()
	remote := fake.ByClientID("servidores-painel")
	fake.Mu.Unlock()
	if remote == nil || remote.Secret != secret || key["external_label"] != "servidores-painel" || key["source"] != "keycloak" {
		t.Fatalf("issued key = %v (remote %+v)", key, remote)
	}
	if len(remote.Optional) != 2 || key["rate_limit_per_minute"].(float64) != 60 {
		t.Errorf("remote scopes %v, rate %v", remote.Optional, key["rate_limit_per_minute"])
	}

	// The vault copy: the secret and the client id, revealable by admin only.
	reveal := func(cookie string) (int, string) {
		code, obj, _ := do(cookie, "GET", "/api/secrets/"+secretID+"/reveal", "")
		p, _ := obj["payload"].(string)
		return code, p
	}
	if code, p := reveal(admin); code != http.StatusOK || !strings.Contains(p, secret) || !strings.Contains(p, "servidores-painel") {
		t.Fatalf("admin reveal = %d %q", code, p)
	}
	if code, _ := reveal(editor); code != http.StatusForbidden {
		t.Errorf("editor reveal = %d, want 403", code)
	}

	if code, _, _ := do(admin, "PUT", base+"/keys/"+keyID, `{"owner":"rh","scopes":["sei:lake.ler"]}`); code != http.StatusBadRequest {
		t.Errorf("edit to another API's scope = %d, want 400", code)
	}
	// Edit scopes and rate limit on the client.
	if code, obj, raw := do(admin, "PUT", base+"/keys/"+keyID, `{"owner":"rh","notes":"n","scopes":["servidores:cadastro"],"rate_limit_per_minute":0}`); code != http.StatusOK ||
		fmt.Sprint(obj["scopes"]) != "[servidores:cadastro]" || obj["rate_limit_per_minute"] != nil || obj["owner"] != "rh" {
		t.Fatalf("edit = %d %s", code, raw)
	}
	fake.Mu.Lock()
	if len(remote.Optional) != 1 || len(remote.Mappers) != 0 {
		t.Errorf("remote after edit: scopes %v mappers %v", remote.Optional, remote.Mappers)
	}
	fake.Mu.Unlock()

	// Rotate: a new secret on the client and in the vault.
	code, obj, raw = do(admin, "POST", base+"/keys/"+keyID+"/rotate", "")
	if code != http.StatusOK || obj["plaintext"] == secret {
		t.Fatalf("rotate = %d %s", code, raw)
	}
	if _, p := reveal(admin); !strings.Contains(p, obj["plaintext"].(string)) {
		t.Errorf("vault still holds the old secret after rotate: %q", p)
	}

	// A client created in Keycloak by someone else shows up on sync.
	fake.Mu.Lock()
	fake.Clients["ext"] = &kcfake.Client{ID: "ext", ClientID: "servidores-cli", Desc: "ops", Enabled: true, Optional: []string{"servidores:demo"}, Mappers: map[string]map[string]any{}}
	fake.Clients["other"] = &kcfake.Client{ID: "other", ClientID: "sei-x", Enabled: true, Mappers: map[string]map[string]any{}}
	fake.Mu.Unlock()
	if code, obj, _ := do(admin, "POST", base+"/keys/sync", ""); code != http.StatusOK || obj["created"].(float64) != 1 || obj["updated"].(float64) != 1 {
		t.Fatalf("sync = %d %v, want 1 created 1 updated", code, obj)
	}
	_, obj, _ = do(admin, "GET", base+"/keys", "")
	if n := len(obj["data"].([]any)); n != 2 {
		t.Errorf("keys after sync = %d, want 2", n)
	}

	// Usage comes straight from the API, with an admin.uso token.
	// Usage needs the separate usage client; the admin token never reaches the API.
	fake.Mu.Lock()
	fake.Usage["servidores-painel"] = 3
	fake.Mu.Unlock()
	if code, _, raw := do(editor, "GET", base+"/keys/"+keyID+"/usage", ""); code != http.StatusBadRequest || !strings.Contains(raw, "usage client not configured") {
		t.Errorf("usage without usage client = %d %s, want 400", code, raw)
	}
	if code, _, raw := do(admin, "PUT", "/api/settings/integrations/keycloak_apis", fmt.Sprintf(
		`{"kc_apis_usage_client_id":%q,"kc_apis_usage_client_secret":%q}`, kcfake.UsageClient, kcfake.UsageSecret)); code != http.StatusOK {
		t.Fatalf("save usage client = %d %s", code, raw)
	}
	if code, obj, _ := do(editor, "GET", base+"/keys/"+keyID+"/usage", ""); code != http.StatusOK || obj["lifetime"].(float64) != 3 {
		t.Errorf("usage = %d %v", code, obj)
	}
	fake.Mu.Lock()
	for _, b := range fake.UsageBearers {
		if strings.Contains(b, kcfake.AdminClient) {
			t.Errorf("admin token sent to the API: %q", b)
		}
	}
	fake.Mu.Unlock()

	// Revoke: the client is disabled, the row revoked; a sync keeps it so.
	if code, obj, _ := do(admin, "POST", base+"/keys/"+keyID+"/revoke", ""); code != http.StatusOK || obj["status"] != "revoked" {
		t.Fatalf("revoke = %d %v", code, obj)
	}
	fake.Mu.Lock()
	if remote.Enabled {
		t.Error("revoke did not disable the client")
	}
	fake.Mu.Unlock()
	do(admin, "POST", base+"/keys/sync", "")
	_, obj, _ = do(admin, "GET", base+"/keys", "")
	if n := len(obj["data"].([]any)); n != 1 {
		t.Errorf("live keys after revoke+sync = %d, want 1", n)
	}

	// If the vault can't take the secret, the client is deleted again: a
	// shared secret named "clash" already sits on this API.
	if _, err := d.SQL.Exec(`INSERT INTO secrets (type, scope, visibility, parent_id, owner_user_id, name, payload_ciphertext, payload_nonce, key_version, created_by)
		VALUES ('api_key','api_catalog','shared',?,?,'clash','\x00','\x00',1,?)`, a.ID, adminID, adminID); err != nil {
		t.Fatalf("seed clash: %v", err)
	}
	if code, _, _ := do(admin, "POST", base+"/keys", `{"label":"clash","scopes":["servidores:demo"]}`); code < 400 {
		t.Fatalf("issue into a vault clash = %d, want an error", code)
	}
	fake.Mu.Lock()
	if fake.ByClientID("servidores-clash") != nil {
		t.Error("client left in Keycloak after the vault write failed")
	}
	if len(fake.BadHost) > 0 {
		t.Errorf("Keycloak requests without the Host header: %v", fake.BadHost)
	}
	fake.Mu.Unlock()

	// Manual mode: register an existing key.
	do(admin, "PUT", base+"/key-management", `{"key_management":"manual"}`)
	if code, _, _ := do(admin, "POST", base+"/keys", `{"label":"legacy"}`); code != http.StatusBadRequest {
		t.Errorf("manual key without value = %d, want 400", code)
	}
	if code, obj, raw := do(admin, "POST", base+"/keys", `{"label":"legacy","value":"abc123"}`); code != http.StatusCreated || obj["key"].(map[string]any)["source"] != "manual" {
		t.Fatalf("manual key = %d %s", code, raw)
	}

	// Deleting the API trashes its key secrets with it.
	if code, _, _ := do(admin, "DELETE", base, ""); code != http.StatusNoContent {
		t.Fatalf("delete api = %d", code)
	}
	if code, _ := reveal(admin); code != http.StatusNotFound {
		t.Errorf("key secret after api delete = %d, want 404", code)
	}

	// A scope prefix is one API's (manual mode cleared the first one's).
	newAPI := func(name string) string {
		x := &models.APICatalog{Name: name, SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
			OwnerUserID: adminID, CreatedBy: adminID}
		if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, x, nil); err != nil {
			t.Fatalf("api %s: %v", name, err)
		}
		return "/api/api-catalog/" + itoa(x.ID) + "/key-management"
	}
	if code, _, raw := do(admin, "PUT", newAPI("Servidores v2"), cfg); code != http.StatusOK {
		t.Fatalf("free scope_prefix = %d %s", code, raw)
	}
	if code, _, raw := do(admin, "PUT", newAPI("Servidores v3"), cfg); code != http.StatusConflict {
		t.Errorf("duplicate scope_prefix = %d %s, want 409", code, raw)
	}

	// ── Bridge itself ──────────────────────────────────────────────────
	// Public catalogue with the bridge: prefix and no wildcard.
	req := httptest.NewRequest("GET", "/api/escopos", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var esc []map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &esc) != nil || len(esc) == 0 {
		t.Fatalf("/api/escopos = %d %s", rec.Code, rec.Body)
	}
	for _, s := range esc {
		if n := s["name"].(string); !strings.HasPrefix(n, "bridge:") || n == "bridge:*" {
			t.Errorf("escopo %q", n)
		}
	}

	// Seeding puts Bridge in its catalogue once (admin-only: no grants).
	if err := SeedBridgeCatalog(ctx, d); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := SeedBridgeCatalog(ctx, d); err != nil {
		t.Fatalf("seed again: %v", err)
	}
	var bridgeID int64
	var n int
	d.SQL.QueryRow(`SELECT MIN(id), COUNT(*) FROM api_catalog WHERE scope_prefix = 'bridge' AND key_management = 'keycloak'`).Scan(&bridgeID, &n)
	if n != 1 {
		t.Fatalf("bridge entries = %d, want 1", n)
	}
	// A start with a newer build refreshes the spec (stale hash → rewritten).
	var specHash string
	d.SQL.QueryRow(`SELECT spec_hash FROM api_catalog WHERE id = ?`, bridgeID).Scan(&specHash)
	d.SQL.Exec(`UPDATE api_catalog SET spec_hash = 'old', spec_json = '{}' WHERE id = ?`, bridgeID)
	if err := SeedBridgeCatalog(ctx, d); err != nil {
		t.Fatal(err)
	}
	var after string
	d.SQL.QueryRow(`SELECT spec_hash FROM api_catalog WHERE id = ?`, bridgeID).Scan(&after)
	if after != specHash {
		t.Errorf("bridge spec hash after reseed = %q, want %q", after, specHash)
	}
	bbase := "/api/api-catalog/" + itoa(bridgeID)
	if code, _, _ := do(editor, "GET", bbase, ""); code != http.StatusNotFound {
		t.Errorf("editor sees the bridge entry = %d, want 404 (admin-only)", code)
	}
	if code, obj, _ := do(admin, "POST", bbase+"/keys/sync-scopes", ""); code != http.StatusOK || obj["created"].(float64) == 0 {
		t.Fatalf("bridge sync scopes = %d %v", code, obj)
	}
	if code, _, _ := do(admin, "POST", bbase+"/keys", `{"label":"x","scopes":["servidores:demo"]}`); code != http.StatusBadRequest {
		t.Errorf("bridge key with a foreign scope = %d, want 400", code)
	}
	code, obj, raw = do(admin, "POST", bbase+"/keys", `{"label":"painel","scopes":["bridge:hosts:read","bridge:hosts:write"]}`)
	if code != http.StatusCreated {
		t.Fatalf("bridge key = %d %s", code, raw)
	}
	// Its service account: least role for the scopes, linked by client id.
	ident, _ := store.NewUserIdentityRepo(d.SQL).GetByProviderAndExternalID(ctx, auth.KeycloakAPIsProvider, "bridge-painel")
	if ident == nil {
		t.Fatal("no keycloak-apis identity for bridge-painel")
	}
	svc, _ := store.NewUserRepo(d.SQL).GetByID(ctx, ident.UserID)
	if svc == nil || svc.Kind != models.UserKindService || svc.Role != "editor" {
		t.Fatalf("service user = %+v, want an editor service account", svc)
	}
	if code, _, _ := do(admin, "GET", bbase+"/keys/"+itoa(int64(obj["key"].(map[string]any)["id"].(float64)))+"/usage", ""); code != http.StatusBadRequest {
		t.Errorf("bridge key usage = %d, want 400", code)
	}
	// Revoking a Bridge key drops its identity: tokens already issued stop.
	if code, _, raw := do(admin, "POST", bbase+"/keys/"+itoa(int64(obj["key"].(map[string]any)["id"].(float64)))+"/revoke", ""); code != http.StatusOK {
		t.Fatalf("revoke bridge key = %d %s", code, raw)
	}
	if ident, _ := store.NewUserIdentityRepo(d.SQL).GetByProviderAndExternalID(ctx, auth.KeycloakAPIsProvider, "bridge-painel"); ident != nil {
		t.Error("revoked bridge-painel still linked to its service user")
	}

	// A token caller is capped by its token's scopes, not only its role: an
	// admin's token holding only apis:keys can't mint hosts:write.
	plain, hash, prefix, _ := auth.GenerateAPIToken()
	if err := store.NewAPITokenRepo(d.SQL).Create(ctx, &models.APIToken{UserID: adminID, Name: "narrow", Prefix: prefix, Scopes: []string{"apis:keys"}}, hash); err != nil {
		t.Fatal(err)
	}
	treq := httptest.NewRequest("POST", bbase+"/keys", strings.NewReader(`{"label":"viatoken","scopes":["bridge:hosts:write"]}`))
	treq.Header.Set("Authorization", "Bearer "+plain)
	treq.Header.Set("Content-Type", "application/json")
	trec := httptest.NewRecorder()
	router.ServeHTTP(trec, treq)
	if trec.Code != http.StatusForbidden {
		t.Errorf("narrow token mints hosts:write = %d %s, want 403", trec.Code, trec.Body)
	}

	// The integration's own admin client, when its id falls under a prefix
	// (here "bridge-admin"), is never synced, issued, rotated or revoked.
	fake.Mu.Lock()
	fake.Creds["bridge-admin"] = "adm"
	fake.Admins["bridge-admin"] = true
	fake.Clients["adm"] = &kcfake.Client{ID: "adm", ClientID: "bridge-admin", Enabled: true, Mappers: map[string]map[string]any{}}
	fake.Mu.Unlock()
	do(admin, "PUT", "/api/settings/integrations/keycloak_apis", `{"kc_apis_client_id":"bridge-admin","kc_apis_client_secret":"adm"}`)
	if code, obj, _ := do(admin, "POST", bbase+"/keys/sync", ""); code != http.StatusOK || obj["created"].(float64) != 0 {
		t.Errorf("sync imported the admin client: %d %v", code, obj)
	}
	if code, _, _ := do(admin, "POST", bbase+"/keys", `{"label":"admin","scopes":["bridge:hosts:read"]}`); code != http.StatusBadRequest {
		t.Errorf("issue the admin client id = %d, want 400", code)
	}
	var admKeyID int64
	if err := d.SQL.QueryRow(`INSERT INTO api_keys (api_id, label, source, external_label) VALUES (?, 'admin', 'keycloak', 'bridge-admin') RETURNING id`, bridgeID).Scan(&admKeyID); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/rotate", ""}, {"POST", "/revoke", ""}, {"PUT", "", `{"owner":"x","scopes":["bridge:hosts:read"]}`},
	} {
		if code, _, _ := do(admin, c.method, bbase+"/keys/"+itoa(admKeyID)+c.path, c.body); code != http.StatusForbidden {
			t.Errorf("%s%s on the admin client = %d, want 403", c.method, c.path, code)
		}
	}
	fake.Mu.Lock()
	if c := fake.ByClientID("bridge-admin"); !c.Enabled || c.Secret != "" {
		t.Errorf("admin client touched: %+v", c)
	}
	fake.Mu.Unlock()
}
