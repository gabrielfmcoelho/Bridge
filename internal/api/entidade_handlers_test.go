package api

import (
	"context"
	"encoding/json"
	"fmt"
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

// entidadeEnv serves the entidade routes with a pluggable caller: each request
// runs with the given user AND the scope auth.RequireAuth would have loaded
// for them (admin bypass, or ScopeForUser). This is the first HTTP test that
// injects a store.Scope, so it doubles as the contract for how handlers see
// scope in production.
type entidadeEnv struct {
	t    *testing.T
	d    *database.DB
	mux  *http.ServeMux
	h    *entidadeHandlers
	user *models.User
}

func newEntidadeEnv(t *testing.T) *entidadeEnv {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	h := newEntidadeHandlers(d)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/entidades", h.handleList)
	mux.HandleFunc("POST /api/entidades", h.handleCreate)
	mux.HandleFunc("GET /api/entidades/unassigned", h.handleUnassigned)
	mux.HandleFunc("POST /api/entidades/bulk-assign", h.handleBulkAssign)
	mux.HandleFunc("GET /api/entidades/{id}", h.handleGet)
	mux.HandleFunc("PUT /api/entidades/{id}", h.handleUpdate)
	mux.HandleFunc("DELETE /api/entidades/{id}", h.handleDelete)
	mux.HandleFunc("GET /api/assets/{type}/{id}/entidades", h.handleGetAssetGrants)
	mux.HandleFunc("PUT /api/assets/{type}/{id}/entidades", h.handlePutAssetGrants)
	return &entidadeEnv{t: t, d: d, mux: mux, h: h}
}

func (e *entidadeEnv) as(username, role string, entidades ...int64) {
	e.t.Helper()
	u := &models.User{Username: username, DisplayName: username, Role: role}
	if err := store.NewUserRepo(e.d.SQL).Create(context.Background(), u); err != nil {
		e.t.Fatalf("create user: %v", err)
	}
	if len(entidades) > 0 {
		if err := store.NewUserEntidadeRepo(e.d.SQL).Replace(context.Background(), u.ID, entidades, entidades[0]); err != nil {
			e.t.Fatalf("memberships: %v", err)
		}
	}
	e.user = u
}

func (e *entidadeEnv) do(method, path, body string) (int, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := auth.WithUser(req.Context(), e.user)
	scope := store.Scope{Admin: e.user.Role == "admin"}
	if !scope.Admin {
		sc, err := store.NewEntidadeRepo(e.d.SQL).ScopeForUser(ctx, e.user.ID)
		if err != nil {
			e.t.Fatalf("scope: %v", err)
		}
		scope = sc
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req.WithContext(store.WithScope(ctx, scope)))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (e *entidadeEnv) entidadeID(slug string) int64 {
	e.t.Helper()
	var id int64
	if err := e.d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = ?`, slug).Scan(&id); err != nil {
		e.t.Fatalf("entidade %s: %v", slug, err)
	}
	return id
}

func (e *entidadeEnv) host(slug string, grants *models.AssetGrants) int64 {
	e.t.Helper()
	var id int64
	if err := e.d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES (?, ?) RETURNING id`, slug, slug).Scan(&id); err != nil {
		e.t.Fatalf("seed host: %v", err)
	}
	if grants != nil {
		if err := e.h.grants.Replace(context.Background(), e.d.SQL, store.AssetHost, id, *grants); err != nil {
			e.t.Fatalf("grants: %v", err)
		}
	}
	return id
}

func TestEntidadeHandlers_AdminCRUD(t *testing.T) {
	e := newEntidadeEnv(t)
	e.as("admin", "admin")
	etipi := e.entidadeID("etipi")

	code, body := e.do("POST", "/api/entidades", fmt.Sprintf(`{"name":"Núcleo Redes","parent_id":%d}`, etipi))
	if code != http.StatusCreated || body["slug"] != "n-cleo-redes" {
		t.Fatalf("create = %d %v", code, body)
	}
	childID := int64(body["id"].(float64))

	code, body = e.do("GET", "/api/entidades", "")
	if code != http.StatusOK || len(body["data"].([]any)) != 12 {
		t.Fatalf("list = %d, len=%d (seed 11 + 1)", code, len(body["data"].([]any)))
	}

	// Re-parenting ETIPI under its own new child is a cycle → 409.
	code, _ = e.do("PUT", fmt.Sprintf("/api/entidades/%d", etipi), fmt.Sprintf(`{"name":"ETIPI","slug":"etipi","parent_id":%d}`, childID))
	if code != http.StatusConflict {
		t.Fatalf("cycle update = %d, want 409", code)
	}
	// ETIPI now has a child → delete refused; child itself deletes fine.
	if code, _ = e.do("DELETE", fmt.Sprintf("/api/entidades/%d", etipi), ""); code != http.StatusConflict {
		t.Fatalf("delete parent = %d, want 409", code)
	}
	if code, _ = e.do("DELETE", fmt.Sprintf("/api/entidades/%d", childID), ""); code != http.StatusOK {
		t.Fatalf("delete child = %d, want 200", code)
	}
}

func TestEntidadeHandlers_AssetGrantsScoped(t *testing.T) {
	e := newEntidadeEnv(t)
	etipi, sga := e.entidadeID("etipi"), e.entidadeID("sga")
	hEtipi := e.host("h-etipi", &models.AssetGrants{CreatorEntidadeID: &etipi})
	hSga := e.host("h-sga", &models.AssetGrants{ResponsibleEntidadeIDs: []int64{sga}})

	e.as("sga-editor", "editor", sga)
	if code, _ := e.do("GET", fmt.Sprintf("/api/assets/host/%d/entidades", hEtipi), ""); code != http.StatusNotFound {
		t.Fatalf("invisible asset = %d, want 404", code)
	}
	code, body := e.do("GET", fmt.Sprintf("/api/assets/host/%d/entidades", hSga), "")
	if code != http.StatusOK || body["creator_entidade_id"] != nil {
		t.Fatalf("visible asset = %d %v", code, body)
	}
	// Creator outside the caller's scope → 403; inside → 200 and persisted.
	if code, _ = e.do("PUT", fmt.Sprintf("/api/assets/host/%d/entidades", hSga), fmt.Sprintf(`{"creator_entidade_id":%d}`, etipi)); code != http.StatusForbidden {
		t.Fatalf("foreign creator = %d, want 403", code)
	}
	code, body = e.do("PUT", fmt.Sprintf("/api/assets/host/%d/entidades", hSga), fmt.Sprintf(`{"creator_entidade_id":%d,"responsible_entidade_ids":[%d],"is_global":true}`, sga, etipi))
	if code != http.StatusOK || body["is_global"] != true {
		t.Fatalf("own creator = %d %v", code, body)
	}
	// Unknown asset type → 400.
	if code, _ = e.do("GET", "/api/assets/banana/1/entidades", ""); code != http.StatusBadRequest {
		t.Fatalf("unknown type = %d, want 400", code)
	}
}

func TestEntidadeHandlers_UnassignedTriage(t *testing.T) {
	e := newEntidadeEnv(t)
	e.as("admin", "admin")
	sga := e.entidadeID("sga")
	hNone := e.host("h-none", nil)
	e.host("h-sga", &models.AssetGrants{ResponsibleEntidadeIDs: []int64{sga}})

	code, body := e.do("GET", "/api/entidades/unassigned?asset_type=host", "")
	data := body["data"].([]any)
	if code != http.StatusOK || len(data) != 1 || data[0].(map[string]any)["name"] != "h-none" {
		t.Fatalf("unassigned = %d %v", code, body)
	}
	code, _ = e.do("POST", "/api/entidades/bulk-assign", fmt.Sprintf(`{"asset_type":"host","asset_ids":[%d],"creator_entidade_id":%d}`, hNone, sga))
	if code != http.StatusOK {
		t.Fatalf("bulk-assign = %d", code)
	}
	code, body = e.do("GET", "/api/entidades/unassigned?asset_type=host", "")
	if code != http.StatusOK || len(body["data"].([]any)) != 0 {
		t.Fatalf("after assign = %d %v", code, body)
	}
	// Non-admin scope check on bulk: an editor with no entidade and no creator
	// can't produce an invisible asset → 400.
	e.as("editor", "editor")
	if code, _ = e.do("POST", "/api/entidades/bulk-assign", fmt.Sprintf(`{"asset_type":"host","asset_ids":[%d]}`, hNone)); code != http.StatusBadRequest {
		t.Fatalf("no-scope bulk = %d, want 400", code)
	}
}
