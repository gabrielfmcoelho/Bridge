package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestGitLabRepoInventory(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	gl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "tok" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		switch r.URL.EscapedPath() {
		case "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":1,"name":"sead","full_path":"sead"},{"id":2,"name":"dl","full_path":"sead/dl","parent_id":1}]`))
		case "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":7,"name":"app","path_with_namespace":"sead/app","namespace":{"id":1,"full_path":"sead"}},
				{"id":8,"name":"lake","path_with_namespace":"sead/dl/lake","namespace":{"id":2,"full_path":"sead/dl"}}]`))
		case "/api/v4/projects/sead%2Fapp":
			_, _ = w.Write([]byte(`{"id":7,"name":"app","path_with_namespace":"sead/app"}`))
		default:
			http.Error(w, `{"message":"404 Project Not Found"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(gl.Close)
	glHost := mustHost(t, gl.URL)

	u := &models.User{Username: "admin", DisplayName: "admin", Role: "admin", Email: "admin@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	mux := newEcho()
	(&projectGitLabHandlers{db: d}).registerRoutes(routeRegistrar{e: mux})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	}))
	t.Cleanup(srv.Close)
	env := &catalogEnv{t: t, d: d, server: srv, user: u}

	if resp := env.do("GET", "/api/settings/gitlab/repos", ""); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unconfigured = %d, want 400", resp.StatusCode)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"gitlab_integration_enabled": "true", "auth_gitlab_base_url": gl.URL} {
		if err := settings.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "gitlab_code_service_token", "tok"); err != nil {
		t.Fatal(err)
	}

	// Projeto "Catálogo" → repo 7; projeto "Lake" → group 2.
	id := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := d.SQL.QueryRow(q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	catalogo := id(`INSERT INTO projects (name) VALUES ('Catálogo') RETURNING id`)
	lake := id(`INSERT INTO projects (name) VALUES ('Lake') RETURNING id`)
	id(`INSERT INTO project_gitlab_links (project_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind) VALUES (?, 7, ?, 'sead/app', 'project') RETURNING id`, catalogo, gl.URL)
	id(`INSERT INTO project_gitlab_links (project_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind) VALUES (?, 2, ?, 'sead/dl', 'group') RETURNING id`, lake, gl.URL)
	// Service "web": repository_url → sead/app. Service "old": Coolify repo
	// the token does not list → unlisted.
	web := id(`INSERT INTO services (nickname, repository_url) VALUES ('web', ?) RETURNING id`, gl.URL+"/sead/app/-/tree/main")
	id(`INSERT INTO services (nickname, git_repository) VALUES ('old', ?) RETURNING id`, "git@"+glHost+":sead/gone.git")
	dnsID := id(`INSERT INTO dns_records (domain) VALUES ('app.example') RETURNING id`)
	hostID := id(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('vm1', 'vm1') RETURNING id`)

	link := func(typ string, eid int64, path string) *http.Response {
		b, _ := json.Marshal(map[string]any{"entity_type": typ, "entity_id": eid, "path": path})
		return env.do("POST", "/api/settings/gitlab/repo-links", string(b))
	}
	if resp := link("dns", dnsID, "sead/app"); resp.StatusCode != http.StatusCreated {
		t.Fatalf("link dns = %d %s", resp.StatusCode, readBody(resp))
	}
	for _, c := range []struct {
		typ  string
		id   int64
		path string
		want int
	}{
		{"dns", dnsID, "sead/app", http.StatusConflict},
		{"host", hostID, "sead/missing", http.StatusNotFound},
		{"host", 9999, "sead/app", http.StatusNotFound},
		{"service", web, "sead/app", http.StatusBadRequest},
	} {
		if resp := link(c.typ, c.id, c.path); resp.StatusCode != c.want {
			t.Fatalf("link %s %d %s = %d, want %d", c.typ, c.id, c.path, resp.StatusCode, c.want)
		}
	}

	var inv gitlabInventory
	resp := env.do("GET", "/api/settings/gitlab/repos", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inventory = %d %s", resp.StatusCode, readBody(resp))
	}
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		t.Fatal(err)
	}
	if len(inv.Groups) != 2 || inv.Groups[1].ParentID == nil || len(inv.Groups[1].Projetos) != 1 || inv.Groups[1].Projetos[0].Name != "Lake" {
		t.Fatalf("groups = %+v", inv.Groups)
	}
	app := inv.Repos[0]
	if app.PathWithNamespace != "sead/app" || app.NamespaceID != 1 ||
		len(app.Projetos) != 1 || app.Projetos[0].Name != "Catálogo" ||
		len(app.Services) != 1 || app.Services[0].Name != "web" || app.Services[0].Field != "repository_url" ||
		len(app.DNS) != 1 || app.DNS[0].Name != "app.example" || app.DNS[0].LinkID == 0 || len(app.Hosts) != 0 {
		t.Fatalf("sead/app = %+v", app)
	}
	if lakeRepo := inv.Repos[1]; len(lakeRepo.Projetos)+len(lakeRepo.Services)+len(lakeRepo.DNS)+len(lakeRepo.Hosts) != 0 {
		t.Fatalf("sead/dl/lake has attachments: %+v", lakeRepo)
	}
	if len(inv.Unlisted) != 1 || inv.Unlisted[0].PathWithNamespace != "sead/gone" || inv.Unlisted[0].Services[0].Field != "git_repository" {
		t.Fatalf("unlisted = %+v", inv.Unlisted)
	}

	path := "/api/settings/gitlab/repo-links/" + itoa(app.DNS[0].LinkID)
	if resp := env.do("DELETE", path, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if resp := env.do("DELETE", path, ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", resp.StatusCode)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Hostname()
}
