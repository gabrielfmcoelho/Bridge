package service_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// fakeCatalogGitLab serves groups "g" and "h" (h repeats repo 1) and, per
// project id, a catalog file (base64), a 404 (no entry) or a 500 ("err").
func fakeCatalogGitLab(t *testing.T, files map[int]string) *gitlab.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.EscapedPath()
		switch {
		case p == "/api/v4/groups/g/projects":
			fmt.Fprint(w, `[{"id":1,"path_with_namespace":"g/api","web_url":"https://gl/g/api","default_branch":"main"},
				{"id":2,"path_with_namespace":"g/web","web_url":"https://gl/g/web","default_branch":"master"},
				{"id":3,"path_with_namespace":"g/flaky","default_branch":"main"},
				{"id":4,"path_with_namespace":"g/broken","default_branch":"main"},
				{"id":5,"path_with_namespace":"g/empty","default_branch":""}]`)
		case p == "/api/v4/groups/h/projects":
			fmt.Fprint(w, `[{"id":1,"path_with_namespace":"g/api","web_url":"https://gl/g/api","default_branch":"main"}]`)
		case strings.HasSuffix(p, "/repository/files/catalog%2Fdependencies.yml"):
			var id int
			fmt.Sscanf(strings.TrimPrefix(p, "/api/v4/projects/"), "%d", &id)
			content, ok := files[id]
			switch {
			case !ok:
				http.Error(w, `{"message":"404 File Not Found"}`, http.StatusNotFound)
			case content == "err":
				http.Error(w, "boom", http.StatusInternalServerError)
			default:
				fmt.Fprintf(w, `{"encoding":"base64","content":%q,"ref":%q,"commit_id":"head","last_commit_id":"c-%d"}`,
					base64.StdEncoding.EncodeToString([]byte(content)), r.URL.Query().Get("ref"), id)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return gitlab.NewClient(srv.URL, "tok")
}

func TestAppDepsSync(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	repo := store.NewAppDepRepo(d.SQL)
	old := time.Now().Add(-time.Hour).UTC()
	// Rows from an earlier sync: 2 (file now gone), 3 (GitLab errors this
	// time), 9 (repository no longer listed).
	if _, err := repo.Apply(ctx, []models.AppDepSource{
		{GitLabProjectID: 2, ProjectPath: "g/web", ParseError: "x", SyncedAt: old},
		{GitLabProjectID: 3, ProjectPath: "g/flaky", ParseError: "kept", SyncedAt: old},
		{GitLabProjectID: 9, ProjectPath: "g/moved", ParseError: "x", SyncedAt: old},
	}, nil); err != nil {
		t.Fatal(err)
	}

	gl := fakeCatalogGitLab(t, map[int]string{
		1: "ci: shop-api\ncomponents:\n  - {name: api, category: application, technology: rest-api}\n",
		3: "err",
		4: "- not: a mapping\n",
	})
	res, err := service.NewAppDepsServiceWith(d.SQL, gl, []string{"g", "h"}).Sync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Repos != 5 || res.Found != 2 || res.Parsed != 1 || res.Failed != 1 || res.Removed != 2 || len(res.Errors) != 1 ||
		!strings.HasPrefix(res.Errors[0], "g/flaky:") {
		t.Fatalf("result = %+v", res)
	}

	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int]models.AppDepSource{}
	for _, r := range rows {
		byID[r.GitLabProjectID] = r
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 1, 3 and 4", rows)
	}
	if a := byID[1]; a.Catalog == nil || a.Catalog.CI != "shop-api" || a.Ref != "main" || a.CommitID != "c-1" ||
		a.FileURL != "https://gl/g/api/-/blob/main/catalog/dependencies.yml" {
		t.Errorf("parsed row = %+v", a)
	}
	if f := byID[3]; f.ParseError != "kept" || f.SyncedAt.After(time.Now().Add(-30*time.Minute)) {
		t.Errorf("a read error must keep the old row, got %+v", f)
	}
	if b := byID[4]; b.Catalog != nil || b.ParseError == "" {
		t.Errorf("parse failure must be stored with its error, got %+v", b)
	}
}

func TestAppDepsSyncFailedGroupKeepsUnlistedRows(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	repo := store.NewAppDepRepo(d.SQL)
	if _, err := repo.Apply(ctx, []models.AppDepSource{{GitLabProjectID: 9, ProjectPath: "x/moved", ParseError: "x", SyncedAt: time.Now()}}, nil); err != nil {
		t.Fatal(err)
	}
	gl := fakeCatalogGitLab(t, map[int]string{1: "ci: a\n"})
	res, err := service.NewAppDepsServiceWith(d.SQL, gl, []string{"g", "missing"}).Sync(ctx)
	if err != nil || res.Found != 1 || res.Removed != 0 || len(res.Errors) != 1 || !strings.HasPrefix(res.Errors[0], "grupo missing:") {
		t.Fatalf("result = %+v, err = %v", res, err)
	}
	ids, _ := repo.IDs(ctx)
	if len(ids) != 2 {
		t.Errorf("an incomplete listing must not prune unlisted rows: ids = %v, want 1 and 9", ids)
	}

	if _, err := service.NewAppDepsServiceWith(d.SQL, gl, []string{"missing"}).Sync(ctx); err == nil {
		t.Error("no group listed: want an error")
	}
	if err := service.NewAppDepsServiceWith(d.SQL, nil, nil).Ready(); !errors.Is(err, service.ErrGitLabNaoConfigurado) {
		t.Errorf("Ready without a client = %v", err)
	}
}
