package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// appDepsServer serves the app-deps routes as an editor; scope nil = unscoped.
func appDepsServer(t *testing.T, d *database.DB, scope *store.Scope) func(method, path, body string) (int, string) {
	t.Helper()
	u := &models.User{Username: fmt.Sprintf("ed%d", time.Now().UnixNano()), Role: "editor"}
	u.DisplayName, u.Email = u.Username, u.Username+"@example.com"
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	mux := newEcho()
	(&appDepsHandlers{db: d}).registerRoutes(routeRegistrar{e: mux})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithUser(r.Context(), u)
		if scope != nil {
			ctx = store.WithScope(ctx, *scope)
		}
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return func(method, path, body string) (int, string) {
		t.Helper()
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, srv.URL+path, rd)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
}

func TestAppDepLinkHandlers(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil || d == nil {
		t.Skip("no database")
	}
	c, _ := models.ParseAppDepCatalog([]byte("ci: shop-api\n"))
	if _, err := store.NewAppDepRepo(d.SQL).Apply(context.Background(),
		[]models.AppDepSource{{GitLabProjectID: 7, ProjectPath: "g/shop-api", Ref: "main", Catalog: c, SyncedAt: time.Now()}}, nil); err != nil {
		t.Fatal(err)
	}
	var host int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('vm','vm') RETURNING id`).Scan(&host); err != nil {
		t.Fatal(err)
	}
	do := appDepsServer(t, d, nil)
	link := func(ci, typ string, id int64) string {
		return fmt.Sprintf(`{"ci":%q,"entity_type":%q,"entity_id":%d}`, ci, typ, id)
	}

	for _, tc := range []struct {
		body string
		want int
	}{
		{link("shop-api", "contact", host), http.StatusBadRequest},
		{link("", "host", host), http.StatusBadRequest},
		{link("nope", "host", host), http.StatusNotFound},
		{link("shop-api", "host", host+999), http.StatusNotFound},
		{link("shop-api", "host", host), http.StatusCreated},
		{link("shop-api", "host", host), http.StatusCreated}, // again: no-op
	} {
		if code, body := do("POST", "/api/app-deps/links", tc.body); code != tc.want {
			t.Errorf("POST %s = %d %s, want %d", tc.body, code, body, tc.want)
		}
	}

	code, body := do("GET", fmt.Sprintf("/api/app-deps/links?entity_type=host&entity_id=%d", host), "")
	var env struct {
		Data []models.AppDepAssetLink `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &env)
	if code != http.StatusOK || len(env.Data) != 1 || env.Data[0].CI != "shop-api" || env.Data[0].CIID != 7 ||
		strings.Join(env.Data[0].Sources, ",") != "manual" {
		t.Fatalf("GET by host = %d %s", code, body)
	}
	if code, _ := do("GET", "/api/app-deps/links?entity_type=bogus&entity_id=1", ""); code != http.StatusBadRequest {
		t.Errorf("bad entity_type = %d", code)
	}

	// A caller who can't see the host: nothing listed, and it can't be linked or unlinked (404, not 403).
	blind := appDepsServer(t, d, &store.Scope{EntidadeIDs: []int64{}})
	if code, body := blind("GET", "/api/app-deps/links?ci=shop-api", ""); code != http.StatusOK || strings.Contains(body, "shop-api") {
		t.Errorf("scoped GET = %d %s", code, body)
	}
	if code, _ := blind("POST", "/api/app-deps/links", link("shop-api", "host", host)); code != http.StatusNotFound {
		t.Errorf("scoped POST = %d, want 404", code)
	}
	if code, _ := blind("DELETE", fmt.Sprintf("/api/app-deps/links/shop-api/host/%d", host), ""); code != http.StatusNotFound {
		t.Errorf("scoped DELETE = %d, want 404", code)
	}

	if code, body := do("DELETE", fmt.Sprintf("/api/app-deps/links/shop-api/host/%d", host), ""); code != http.StatusNoContent {
		t.Errorf("DELETE = %d %s", code, body)
	}
	if code, _ := do("DELETE", fmt.Sprintf("/api/app-deps/links/shop-api/host/%d", host), ""); code != http.StatusNotFound {
		t.Errorf("second DELETE = %d, want 404", code)
	}
}
