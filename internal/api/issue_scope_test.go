package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestIssueScope: issues inherit visibility from their parent (entity_type,
// entity_id). Two projects, one granted to sga; scoped caller sees only that
// project's issue; unscoped sees both.
func TestIssueScope(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if d == nil {
		return // skipped inside dbtest
	}
	ctx := context.Background()

	var sga, uid, visProj, hidProj int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga); err != nil {
		t.Fatalf("sga: %v", err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('u', 'x', 'editor') RETURNING id`).Scan(&uid); err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('visible') RETURNING id`).Scan(&visProj); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('hidden') RETURNING id`).Scan(&hidProj); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetProject, visProj, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	var visIssue, hidIssue models.Issue
	for _, c := range []struct {
		i   *models.Issue
		pid int64
	}{{&visIssue, visProj}, {&hidIssue, hidProj}} {
		pid := c.pid
		*c.i = models.Issue{ProjectID: &pid, Title: "t", Status: "backlog", Priority: "medium", CreatedBy: uid}
		if err := models.CreateIssue(d.SQL, c.i); err != nil {
			t.Fatalf("create issue: %v", err)
		}
	}

	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}})

	// Model level: filter predicate + visible count.
	f := models.IssueFilter{}
	f.VisibleSQL, f.VisibleArgs = store.VisibleExprDyn(scoped, "entity_type", "entity_id")
	got, err := models.ListIssues(d.SQL, f)
	if err != nil || len(got) != 1 || got[0].ID != visIssue.ID {
		t.Fatalf("scoped list: got %+v err %v, want only issue %d", got, err, visIssue.ID)
	}
	if n, err := models.OpenIssueCountVisible(d.SQL, f.VisibleSQL, f.VisibleArgs); err != nil || n != 1 {
		t.Fatalf("scoped open count = %d err %v, want 1", n, err)
	}
	if all, err := models.ListIssues(d.SQL, models.IssueFilter{}); err != nil || len(all) != 2 {
		t.Fatalf("unscoped list: %d err %v, want 2", len(all), err)
	}
	if n, err := models.OpenIssueCountVisible(d.SQL, "", nil); err != nil || n != 2 {
		t.Fatalf("unscoped open count = %d err %v, want 2", n, err)
	}

	// Handler level: global list + mutators on the hidden issue answer 404.
	h := &globalIssueHandlers{db: d}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/issues", h.handleList)
	mux.HandleFunc("PATCH /api/issues/{id}/archive", h.handleArchive)
	mux.HandleFunc("DELETE /api/issues/{id}", h.handleDelete)
	user := &models.User{ID: uid, Username: "u", Role: "editor"}
	do := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req = req.WithContext(store.WithScope(auth.WithUser(req.Context(), user), store.Scope{EntidadeIDs: []int64{sga}}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	rec := do("GET", "/api/issues")
	var body struct {
		Data []models.Issue `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if rec.Code != 200 || len(body.Data) != 1 || body.Data[0].ID != visIssue.ID {
		t.Fatalf("GET /api/issues scoped: code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, c := range []struct{ method, path string }{
		{"PATCH", "/api/issues/" + itoa(hidIssue.ID) + "/archive"},
		{"DELETE", "/api/issues/" + itoa(hidIssue.ID)},
	} {
		if rec := do(c.method, c.path); rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s on hidden issue: code=%d, want 404", c.method, c.path, rec.Code)
		}
	}
	if rec := do("PATCH", "/api/issues/"+itoa(visIssue.ID)+"/archive"); rec.Code != 200 {
		t.Fatalf("archive visible issue: code=%d body=%s", rec.Code, rec.Body.String())
	}
}
