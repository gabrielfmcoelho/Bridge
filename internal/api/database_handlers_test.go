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

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// dbHandlerEnv serves databaseHandlers as one fixed user (role), scoped to
// entidade when != 0.
func dbHandlerEnv(t *testing.T, d *database.DB, role string, entidade int64) func(method, path, body string) (int, map[string]any) {
	t.Helper()
	u := &models.User{Username: "db-" + role + fmt.Sprint(entidade), DisplayName: role, Role: role, Email: role + fmt.Sprint(entidade) + "@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	mux := newEcho()
	(&databaseHandlers{db: d}).registerRoutes(routeRegistrar{e: mux})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithUser(r.Context(), u)
		if entidade != 0 {
			ctx = store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{entidade}, PrimaryEntidadeID: entidade})
		}
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return func(method, path, body string) (int, map[string]any) {
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
		out := map[string]any{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
}

func TestDatabaseHandlers_CreatePartialUpdateAndScope(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	mkSvc := func(nick string) int64 {
		s := &models.Service{Nickname: nick, ServiceKind: "database"}
		if err := store.NewServiceRepo(d.SQL).Create(ctx, s); err != nil {
			t.Fatalf("service: %v", err)
		}
		return s.ID
	}
	pg, app := mkSvc("pg"), mkSvc("app")

	editor := dbHandlerEnv(t, d, "editor", 0)
	admin := dbHandlerEnv(t, d, "admin", 0)

	code, created := editor("POST", "/api/databases", fmt.Sprintf(
		`{"name":"SEI","db_name":"sei_prod","engine":"postgres","instance_service_id":%d,"consumer_service_ids":[%d],"sensitivity":"sensivel","is_global":true}`, pg, app))
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, created)
	}
	id := int64(created["id"].(float64))
	path := fmt.Sprintf("/api/databases/%d", id)

	// Partial: only description changes; links, instance and the rest stay.
	code, got := editor("PUT", path, `{"description":"main db"}`)
	if code != http.StatusOK || got["name"] != "SEI" || got["description"] != "main db" || got["sensitivity"] != "sensivel" ||
		fmt.Sprint(got["consumer_service_ids"]) != fmt.Sprintf("[%d]", app) || got["instance_service_id"] == nil {
		t.Fatalf("partial update = %d %v", code, got)
	}
	// null clears a nullable field: the database becomes external.
	if code, got = editor("PUT", path, `{"instance_service_id":null,"external_provider":"ATI"}`); code != http.StatusOK || got["instance_service_id"] != nil {
		t.Fatalf("clear instance = %d %v", code, got)
	}
	// [] clears links.
	if code, got = editor("PUT", path, `{"consumer_service_ids":[]}`); code != http.StatusOK || fmt.Sprint(got["consumer_service_ids"]) != "[]" {
		t.Fatalf("clear consumers = %d %v", code, got)
	}
	// A free host is where Bridge will connect: admin only.
	if code, _ = editor("PUT", path, `{"host":"10.0.0.5"}`); code != http.StatusForbidden {
		t.Fatalf("editor set host = %d; want 403", code)
	}
	if code, got = admin("PUT", path, `{"host":"10.0.0.5","port":5432}`); code != http.StatusOK || got["host"] != "10.0.0.5" {
		t.Fatalf("admin set host = %d %v", code, got)
	}
	// Re-sending the same host is not a change.
	if code, _ = editor("PUT", path, `{"host":"10.0.0.5","name":"SEI prod"}`); code != http.StatusOK {
		t.Fatalf("editor resend host = %d; want 200", code)
	}
	if code, _ = editor("PUT", path, `{"engine":"db2"}`); code != http.StatusBadRequest {
		t.Fatalf("bad engine = %d; want 400", code)
	}
	if code, _ = editor("POST", "/api/databases", fmt.Sprintf(`{"name":"dup","db_name":"x","engine":"postgres","instance_service_id":%d,"is_global":true}`, pg)); code != http.StatusCreated {
		t.Fatalf("second db on instance = %d", code)
	}
	if code, _ = editor("POST", "/api/databases", fmt.Sprintf(`{"name":"dup2","db_name":"x","engine":"postgres","instance_service_id":%d,"is_global":true}`, pg)); code != http.StatusConflict {
		t.Fatalf("duplicate db_name on instance = %d; want 409", code)
	}

	// A scoped editor: the global database is visible, a granted-elsewhere
	// one is 404, and linking a service it can't see is 404.
	sga := entidadeID(t, d, "sga")
	scoped := dbHandlerEnv(t, d, "editor", sga)
	if code, _ = scoped("GET", path, ""); code != http.StatusOK {
		t.Fatalf("scoped get global = %d", code)
	}
	_, hidden := admin("POST", "/api/databases", `{"name":"hidden","engine":"oracle","external_provider":"X"}`)
	if code, _ = scoped("GET", fmt.Sprintf("/api/databases/%d", int64(hidden["id"].(float64))), ""); code != http.StatusNotFound {
		t.Fatalf("scoped get hidden = %d; want 404", code)
	}
	if code, _ = scoped("POST", "/api/databases", fmt.Sprintf(`{"name":"z","engine":"postgres","instance_service_id":%d}`, pg)); code != http.StatusNotFound {
		t.Fatalf("scoped create on invisible instance = %d; want 404", code)
	}

	if code, _ = admin("DELETE", path, ""); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if code, _ = editor("GET", path, ""); code != http.StatusNotFound {
		t.Fatalf("get deleted = %d", code)
	}
	if code, _ = admin("POST", path+"/restore", ""); code != http.StatusOK {
		t.Fatalf("restore = %d", code)
	}
}
