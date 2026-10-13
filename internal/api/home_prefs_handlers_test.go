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

// TestHomePrefs: {} until set, PUT replaces the whole document, each user
// sees only their own, and anything but a small JSON object is a 400.
func TestHomePrefs(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	router := NewRouter(d, "/tmp/sshcm-test-config")

	session := func(username string) string {
		u := &models.User{Username: username, Role: "viewer", AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}
	ana, bia := session("ana"), session("bia")

	do := func(method, body, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/auth/me/home", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	get := func(cookie string) map[string]any {
		rec := do("GET", "", cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("get = %d %s", rec.Code, rec.Body)
		}
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("decode %s: %v", rec.Body, err)
		}
		return m
	}

	if m := get(ana); len(m) != 0 {
		t.Fatalf("fresh prefs = %v, want {}", m)
	}
	if rec := do("PUT", `{"v":1,"personas":["infra","atlas"],"widgets":["infraHealth"]}`, ana); rec.Code != http.StatusOK {
		t.Fatalf("put = %d %s", rec.Code, rec.Body)
	}
	if rec := do("PUT", `{"v":1,"personas":["chamados"]}`, ana); rec.Code != http.StatusOK {
		t.Fatalf("second put = %d %s", rec.Code, rec.Body)
	}
	m := get(ana)
	if _, kept := m["widgets"]; kept || len(m["personas"].([]any)) != 1 {
		t.Fatalf("prefs after replace = %v, want only the second document", m)
	}
	if m := get(bia); len(m) != 0 {
		t.Fatalf("other user's prefs = %v, want {}", m)
	}

	for name, body := range map[string]string{
		"array":     `["infra"]`,
		"null":      `null`,
		"not json":  `{"v":`,
		"too large": `{"x":"` + strings.Repeat("a", maxHomePrefsBytes) + `"}`,
	} {
		if rec := do("PUT", body, ana); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: put = %d, want 400", name, rec.Code)
		}
	}
	if len(get(ana)["personas"].([]any)) != 1 {
		t.Fatal("a rejected put changed the stored prefs")
	}
}
