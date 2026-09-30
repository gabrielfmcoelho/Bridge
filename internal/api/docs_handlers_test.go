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

func TestDocs_ReDocAtDocs(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	router := NewRouter(d, "/tmp/sshcm-test-config")
	u := &models.User{Username: "ana", Role: "viewer", AuthProvider: "local"}
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("user: %v", err)
	}
	session, _, _ := auth.CreateSession(d.SQL, u.ID)

	get := func(path, cookie, accept string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := get("/docs", session, "text/html"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<redoc") {
		t.Fatalf("/docs = %d", rec.Code)
	}
	if rec := get("/docs", "", "text/html"); rec.Code != http.StatusFound || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Errorf("/docs without a session (browser) = %d %s, want a redirect to /login", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get("/docs/openapi.json", "", "application/json"); rec.Code != http.StatusUnauthorized {
		t.Errorf("spec without a session = %d, want 401", rec.Code)
	}
	if rec := get("/docs/redoc.standalone.js", "", ""); rec.Code != 200 || rec.Body.Len() < 100_000 {
		t.Errorf("redoc bundle = %d (%d bytes)", rec.Code, rec.Body.Len())
	}
	if rec := get("/api/docs/index.html", "", ""); rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/docs" {
		t.Errorf("/api/docs/index.html = %d %s, want 301 /docs", rec.Code, rec.Header().Get("Location"))
	}

	rec := get("/docs/openapi.json", session, "")
	var spec struct {
		Paths map[string]map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil || rec.Code != 200 {
		t.Fatalf("spec = %d: %v", rec.Code, err)
	}
	if got := spec.Paths["/api/hosts"]["post"]["x-bridge-scope"]; got != "hosts:write" {
		t.Errorf("POST /api/hosts x-bridge-scope = %v, want hosts:write", got)
	}
	if got := spec.Paths["/api/secrets/{id}/reveal"]["get"]["x-bridge-scope"]; got != "vault:reveal" {
		t.Errorf("reveal x-bridge-scope = %v, want vault:reveal", got)
	}
	login := spec.Paths["/api/auth/login"]["post"]
	if _, scoped := login["x-bridge-scope"]; scoped || !strings.Contains(login["description"].(string), "Public") {
		t.Errorf("login should be labelled public: %v", login["description"])
	}
}
