package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
)

// callAsAdmin calls a handler directly with path values and an admin actor.
func callAsAdmin(env *secretAPIEnv, h http.HandlerFunc, method, body string, path map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range path {
		req.SetPathValue(k, v)
	}
	req = req.WithContext(auth.WithUser(req.Context(), env.alice))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// An embed only takes http(s) URLs — the iframe must never load javascript:
// or data: — and a release can't be created outside a project.
func TestProjectEmbedAndRelease_Validation(t *testing.T) {
	env := newSecretAPIEnv(t)
	var projectID int64
	if err := env.d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p') RETURNING id`).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	pid := map[string]string{"id": itoa(projectID)}
	eh := &projectEmbedHandlers{db: env.d}

	for _, bad := range []string{`javascript:alert(1)`, `data:text/html,x`, `bi.example`, ``} {
		if rec := callAsAdmin(env, eh.handleCreate, "POST", `{"title":"BI","url":"`+bad+`"}`, pid); rec.Code != 400 {
			t.Errorf("url %q: %d, want 400", bad, rec.Code)
		}
	}
	if rec := callAsAdmin(env, eh.handleCreate, "POST", `{"title":"BI","url":"https://bi.example/d"}`, pid); rec.Code != 201 {
		t.Fatalf("valid embed: %d %s", rec.Code, rec.Body)
	}
	if rec := callAsAdmin(env, eh.handleList, "GET", "", map[string]string{"id": "999999"}); rec.Code != 404 {
		t.Errorf("embeds of missing project: %d, want 404", rec.Code)
	}

	rh := &releaseHandlers{db: env.d}
	if rec := callAsAdmin(env, rh.handleCreate, "POST", `{"title":"v1"}`, nil); rec.Code != 400 {
		t.Errorf("release without project: %d, want 400", rec.Code)
	}
	if rec := callAsAdmin(env, rh.handleCreate, "POST", `{"title":"v1","project_id":`+itoa(projectID)+`}`, nil); rec.Code != 201 {
		t.Fatalf("release in project: %d %s", rec.Code, rec.Body)
	}
}
