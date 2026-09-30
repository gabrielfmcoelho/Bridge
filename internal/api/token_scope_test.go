package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestScopeFor(t *testing.T) {
	for _, c := range []struct{ method, pattern, want string }{
		{"GET", "/api/hosts", "hosts:read"},
		{"POST", "/api/hosts", "hosts:write"},
		{"GET", "/api/hosts/{slug}/password", "vault:reveal"},
		{"GET", "/api/secrets/{id}/reveal", "vault:reveal"},
		{"GET", "/api/secrets", "vault:read"},
		{"POST", "/api/ssh/test/{slug}", "ssh:operate"},
		{"GET", "/api/ssh/keys", "ssh:read"},
		{"GET", "/api/backup", "admin:backup"},
		{"POST", "/api/api-catalog/{id}/keys", "apis:keys"},
		{"GET", "/api/api-catalog/{id}/keys", "apis:read"},
		{"PUT", "/api/api-catalog/{id}/key-management", "apis:keys"},
		{"GET", "/api/services/{id}/issues", "services:read"},
		{"DELETE", "/api/settings/role-mappings/{id}", "settings:write"},
	} {
		if got, ok := auth.ScopeFor(c.method, c.pattern); !ok || got != c.want {
			t.Errorf("ScopeFor(%s %s) = %q, %v; want %q", c.method, c.pattern, got, ok, c.want)
		}
	}
	if _, ok := auth.ScopeFor("GET", "/api/nowhere"); ok {
		t.Error("an unknown route must have no scope")
	}
}

// TestEveryRouteHasAScope: NewRouter refuses to build when a route has no
// scope, so building it is the coverage check.
func TestEveryRouteHasAScope(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRouter: %v", r)
		}
	}()
	NewRouter(d, "/tmp/sshcm-test-config")
}

// TestTokenScopes drives the real router: a token only reaches what its scopes
// name, sessions are unaffected, scopes can't exceed the owner, the rate limit
// bites, usage is counted, and service accounts get tokens but never sessions.
func TestTokenScopes(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	router := NewRouter(d, "/tmp/sshcm-test-config")

	newUser := func(name, role string) (*models.User, string) {
		u := &models.User{Username: name, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatalf("user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return u, tok
	}
	_, admin := newUser("ana", "admin")
	_, editor := newUser("bia", "editor")
	_, viewer := newUser("caio", "viewer")

	do := func(method, path, body, cookie, bearer string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	issue := func(cookie, body string) (string, int64, *httptest.ResponseRecorder) {
		t.Helper()
		rec := do("POST", "/api/auth/tokens", body, cookie, "")
		var out apiTokenCreateResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out.Token, out.APIToken.ID, rec
	}

	// The catalogue.
	if rec := do("GET", "/api/auth/tokens/scopes", "", viewer, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"hosts:write"`) {
		t.Fatalf("scopes = %d %s", rec.Code, rec.Body)
	}

	// Scopes are required and can't exceed the owner.
	if _, _, rec := issue(editor, `{"name":"x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("token without scopes = %d, want 400", rec.Code)
	}
	if _, _, rec := issue(viewer, `{"name":"x","scopes":["hosts:write"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("viewer with hosts:write = %d, want 400", rec.Code)
	}
	if _, _, rec := issue(editor, `{"name":"x","scopes":["admin:read"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("editor with admin:read = %d, want 400", rec.Code)
	}
	if _, _, rec := issue(editor, `{"name":"x","scopes":["nope"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown scope = %d, want 400", rec.Code)
	}

	// A hosts:read token reads hosts, but can't write them or reveal secrets.
	readOnly, readID, rec := issue(editor, `{"name":"ro","scopes":["hosts:read"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue = %d %s", rec.Code, rec.Body)
	}
	if rec := do("GET", "/api/hosts", "", "", readOnly); rec.Code != http.StatusOK {
		t.Errorf("GET /api/hosts with hosts:read = %d, want 200", rec.Code)
	}
	for _, c := range []struct{ method, path, required string }{
		{"POST", "/api/hosts", "hosts:write"},
		{"GET", "/api/secrets/1/reveal", "vault:reveal"},
		{"GET", "/api/dns", "dns:read"},
	} {
		rec := do(c.method, c.path, `{}`, "", readOnly)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), c.required) {
			t.Errorf("%s %s with hosts:read = %d %s, want 403 naming %s", c.method, c.path, rec.Code, rec.Body, c.required)
		}
	}
	// A session is not scope-limited; "*" isn't either.
	if rec := do("POST", "/api/hosts", `{}`, editor, ""); rec.Code == http.StatusForbidden {
		t.Errorf("session POST /api/hosts = 403: sessions must not be scope-limited")
	}
	all, _, _ := issue(editor, `{"name":"all","scopes":["*"]}`)
	if rec := do("POST", "/api/hosts", `{}`, "", all); rec.Code == http.StatusForbidden {
		t.Errorf("* token POST /api/hosts = 403")
	}

	// Rate limit.
	limited, _, _ := issue(editor, `{"name":"lim","scopes":["auth:read"],"rate_limit_per_minute":2}`)
	for i, want := range []int{200, 200, 429} {
		if rec := do("GET", "/api/auth/me", "", "", limited); rec.Code != want {
			t.Errorf("request %d = %d, want %d", i+1, rec.Code, want)
		} else if want == 429 && rec.Header().Get("Retry-After") == "" {
			t.Error("429 without Retry-After")
		}
	}

	// Usage: the requests above are counted once flushed.
	auth.FlushTokenUsage(ctx, d.SQL)
	rec = do("GET", "/api/auth/tokens/"+itoa(readID)+"/usage", "", editor, "")
	var usage apiTokenUsageResponse
	if json.Unmarshal(rec.Body.Bytes(), &usage); rec.Code != 200 || usage.Lifetime < 4 {
		t.Errorf("usage = %d %s, want lifetime >= 4", rec.Code, rec.Body)
	}

	// Service accounts: created by admins, never sign in, get tokens from admins only.
	rec = do("POST", "/api/users", `{"kind":"service","username":"svc-iapep","role":"viewer"}`, admin, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create service account = %d %s", rec.Code, rec.Body)
	}
	var svc models.User
	json.Unmarshal(rec.Body.Bytes(), &svc)
	if _, _, err := auth.CreateSession(d.SQL, svc.ID); err != auth.ErrServiceAccount {
		t.Errorf("service account session err = %v, want ErrServiceAccount", err)
	}
	if rec := do("POST", "/api/auth/login", `{"username":"svc-iapep","password":""}`, "", ""); rec.Code == http.StatusOK {
		t.Error("service account signed in")
	}
	body := `{"name":"iapep","scopes":["hosts:read"],"user_id":` + itoa(svc.ID) + `}`
	if _, _, rec := issue(editor, body); rec.Code != http.StatusForbidden {
		t.Errorf("editor issuing for a service account = %d, want 403", rec.Code)
	}
	svcToken, _, rec := issue(admin, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin issuing for a service account = %d %s", rec.Code, rec.Body)
	}
	if rec := do("GET", "/api/hosts", "", "", svcToken); rec.Code != http.StatusOK {
		t.Errorf("service token GET /api/hosts = %d", rec.Code)
	}
	var ana models.User
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'caio'`).Scan(&ana.ID)
	if _, _, rec := issue(admin, `{"name":"p","scopes":["hosts:read"],"user_id":`+itoa(ana.ID)+`}`); rec.Code != http.StatusBadRequest {
		t.Errorf("admin issuing for a person via user_id = %d, want 400", rec.Code)
	}
}
