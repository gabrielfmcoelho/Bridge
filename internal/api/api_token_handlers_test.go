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

// TestAPIToken_Lifecycle drives the real router: a session mints a token, the
// token authenticates as its owner, can't mint another, and stops working
// once revoked. Other users' tokens are invisible (404).
func TestAPIToken_Lifecycle(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	router := NewRouter(d, "/tmp/sshcm-test-config")
	ctx := context.Background()

	session := func(username, role string) string {
		u := &models.User{Username: username, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}
	ana, bia := session("ana", "editor"), session("bia", "viewer")

	do := func(method, path, body, cookie, bearer string) *httptest.ResponseRecorder {
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

	rec := do("POST", "/api/auth/tokens", `{"name":"ci","expires_in_days":90,"scopes":["auth:read"]}`, ana, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	var created apiTokenCreateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(created.Token, auth.APITokenPrefix) || created.APIToken.ExpiresAt == nil ||
		!strings.HasPrefix(created.Token, created.APIToken.Prefix) {
		t.Fatalf("created = %+v", created)
	}

	// The token acts as ana.
	rec = do("GET", "/api/auth/me", "", "", created.Token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"ana"`) {
		t.Fatalf("me with token = %d %s", rec.Code, rec.Body)
	}
	// ...but cannot manage tokens.
	if rec = do("POST", "/api/auth/tokens", `{"name":"x"}`, "", created.Token); rec.Code != http.StatusForbidden {
		t.Fatalf("create with token = %d, want 403", rec.Code)
	}
	// A viewer neither sees nor revokes ana's token, nor lists everyone's.
	if rec = do("DELETE", "/api/auth/tokens/"+itoa(created.APIToken.ID), "", bia, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("revoke by other user = %d, want 404", rec.Code)
	}
	if rec = do("GET", "/api/auth/tokens?all=true", "", bia, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("list all as viewer = %d, want 403", rec.Code)
	}
	if rec = do("GET", "/api/auth/tokens", "", bia, ""); !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("bia's list = %s, want empty", rec.Body)
	}

	if rec = do("DELETE", "/api/auth/tokens/"+itoa(created.APIToken.ID), "", ana, ""); rec.Code != http.StatusOK {
		t.Fatalf("revoke = %d %s", rec.Code, rec.Body)
	}
	if rec = do("GET", "/api/auth/me", "", "", created.Token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after revoke = %d, want 401", rec.Code)
	}
	if rec = do("GET", "/api/auth/me", "", "", "brg_not-a-real-token"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me with junk token = %d, want 401", rec.Code)
	}

	// An expired token is refused too.
	rec = do("POST", "/api/auth/tokens", `{"name":"old","expires_in_days":1,"scopes":["auth:read"]}`, ana, "")
	var old apiTokenCreateResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &old)
	if _, err := d.SQL.Exec(`UPDATE api_tokens SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = ?`, old.APIToken.ID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if rec = do("GET", "/api/auth/me", "", "", old.Token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me with expired token = %d, want 401", rec.Code)
	}
}
