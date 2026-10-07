package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestCanvasAndEntidadeIssues: a canvas and the issues it sends to the
// backlog are visible by entidade; invisible is 404, a stale save is 409.
func TestCanvasAndEntidadeIssues(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if d == nil {
		return
	}
	var sga, etipi, uid int64
	_ = d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga)
	_ = d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'etipi'`).Scan(&etipi)
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('u', 'x', 'editor') RETURNING id`).Scan(&uid); err != nil {
		t.Fatalf("user: %v", err)
	}
	repo := store.NewCanvasRepo(d.SQL)
	hidden := &models.Canvas{EntidadeID: etipi, Title: "etipi"}
	if err := repo.Create(context.Background(), hidden); err != nil {
		t.Fatal(err)
	}

	ch := &canvasHandlers{canvases: repo}
	ih := &globalIssueHandlers{db: d}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/canvases", ch.handleCreate)
	mux.HandleFunc("GET /api/canvases/{id}", ch.handleGet)
	mux.HandleFunc("PUT /api/canvases/{id}", ch.handleUpdate)
	mux.HandleFunc("POST /api/issues", ih.handleCreate)
	user := &models.User{ID: uid, Username: "u", Role: "editor"}
	do := func(scope store.Scope, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(store.WithScope(auth.WithUser(req.Context(), user), scope))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	sgaScope := store.Scope{EntidadeIDs: []int64{sga}}
	admin := store.Scope{Admin: true}

	cases := []struct {
		name         string
		scope        store.Scope
		method, path string
		body         string
		want         int
	}{
		{"create in own entidade", sgaScope, "POST", "/api/canvases", `{"entidade_id":` + itoa(sga) + `,"title":"Q4"}`, 201},
		{"create in other entidade", sgaScope, "POST", "/api/canvases", `{"entidade_id":` + itoa(etipi) + `,"title":"x"}`, 404},
		{"get invisible", sgaScope, "GET", "/api/canvases/" + itoa(hidden.ID), "", 404},
		{"update invisible", sgaScope, "PUT", "/api/canvases/" + itoa(hidden.ID), `{"version":1,"title":"y"}`, 404},
		{"update", admin, "PUT", "/api/canvases/" + itoa(hidden.ID), `{"version":1,"content":{"nodes":[],"edges":[]}}`, 200},
		{"stale update", admin, "PUT", "/api/canvases/" + itoa(hidden.ID), `{"version":1,"title":"z"}`, 409},
		{"content not an object", admin, "PUT", "/api/canvases/" + itoa(hidden.ID), `{"version":2,"content":[1]}`, 400},
		{"issue on own entidade", sgaScope, "POST", "/api/issues", `{"title":"ideia","entity_type":"entidade","entity_id":` + itoa(sga) + `,"source":"canvas"}`, 201},
		{"issue on other entidade", sgaScope, "POST", "/api/issues", `{"title":"ideia","entity_type":"entidade","entity_id":` + itoa(etipi) + `}`, 404},
		{"admin issue on missing entidade", admin, "POST", "/api/issues", `{"title":"ideia","entity_type":"entidade","entity_id":999999}`, 404},
	}
	for _, c := range cases {
		if rec := do(c.scope, c.method, c.path, c.body); rec.Code != c.want {
			t.Fatalf("%s: code=%d want %d body=%s", c.name, rec.Code, c.want, rec.Body.String())
		}
	}
}
