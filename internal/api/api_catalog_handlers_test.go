package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

const testOpenAPI30 = `openapi: 3.0.1
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://test.example.com
paths:
  /things:
    get:
      operationId: listThings
      tags: [things]
  /things/{id}:
    delete:
      summary: Delete a thing
      tags: [things]
`

type catalogEnv struct {
	t      *testing.T
	d      *database.DB
	server *httptest.Server
	user   *models.User
}

func newCatalogEnv(t *testing.T) *catalogEnv { return newCatalogEnvOn(t, nil, "editor", 0) }

// newCatalogEnvOn builds an env over d (opened fresh when nil) whose requests
// carry editor user `name` and, when entidade != 0, a scope limited to that
// entidade (primary = that entidade). entidade == 0 ⇒ unscoped.
func newCatalogEnvOn(t *testing.T, d *database.DB, name string, entidade int64) *catalogEnv {
	t.Helper()
	if d == nil {
		var err error
		d, err = dbtest.Open(t)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		t.Cleanup(func() { d.Close() })
	}

	u := &models.User{Username: name, DisplayName: name, Role: "editor", Email: name + "@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	mux := http.NewServeMux()
	h := &apiCatalogHandlers{db: d}
	// Identity wrap: ignore the required role (the outer middleware injects a
	// fixed editor user); production wires authenticated()/authedRole().
	h.register(mux, func(role string, next http.Handler) http.Handler { return next })
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithUser(r.Context(), u)
		if entidade != 0 {
			ctx = store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{entidade}, PrimaryEntidadeID: entidade})
		}
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	return &catalogEnv{t: t, d: d, server: srv, user: u}
}

func entidadeID(t *testing.T, d *database.DB, slug string) int64 {
	t.Helper()
	var id int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = ?`, slug).Scan(&id); err != nil {
		t.Fatalf("entidade %s: %v", slug, err)
	}
	return id
}

func (e *catalogEnv) do(method, path, body string) *http.Response {
	e.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, e.server.URL+path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("do %s %s: %v", method, path, err)
	}
	return resp
}

func (e *catalogEnv) uploadSpec(spec string) *http.Response {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "Test API")
	_ = mw.WriteField("scope", "avulso")
	fw, err := mw.CreateFormFile("spec", "spec.yaml")
	if err != nil {
		e.t.Fatalf("form file: %v", err)
	}
	io.Copy(fw, strings.NewReader(spec))
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, e.server.URL+"/api/api-catalog/import/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("upload: %v", err)
	}
	return resp
}

func TestCatalog_ImportListGetSpecFilter(t *testing.T) {
	e := newCatalogEnv(t)

	// Import via upload.
	resp := e.uploadSpec(testOpenAPI30)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, body=%s", resp.StatusCode, readBody(resp))
	}
	created := decodeObj(t, resp)
	id := int64(created["id"].(float64))
	if created["external_url"] != "https://test.example.com" {
		t.Errorf("external_url = %v", created["external_url"])
	}

	// List.
	resp = e.do(http.MethodGet, "/api/api-catalog", "")
	arr := decodeArr(t, resp)
	if len(arr) != 1 || arr[0]["operation_count"].(float64) != 2 {
		t.Fatalf("list unexpected: %v", arr)
	}

	// Get with operations.
	resp = e.do(http.MethodGet, "/api/api-catalog/"+itoa(id), "")
	got := decodeObj(t, resp)
	ops := got["operations"].([]any)
	if len(ops) != 2 {
		t.Fatalf("operations = %d", len(ops))
	}

	// Spec endpoint returns valid JSON.
	resp = e.do(http.MethodGet, "/api/api-catalog/"+itoa(id)+"/spec", "")
	var spec map[string]any
	if err := json.Unmarshal([]byte(readBody(resp)), &spec); err != nil {
		t.Fatalf("spec not json: %v", err)
	}

	// Filter to a single operation.
	resp = e.do(http.MethodPost, "/api/api-catalog/"+itoa(id)+"/spec/filter",
		`{"mode":"operations","op_keys":["listThings"]}`)
	var filtered map[string]any
	if err := json.Unmarshal([]byte(readBody(resp)), &filtered); err != nil {
		t.Fatalf("filtered not json: %v", err)
	}
	paths := filtered["paths"].(map[string]any)
	if _, ok := paths["/things"]; !ok {
		t.Error("expected /things kept")
	}
	if _, ok := paths["/things/{id}"]; ok {
		t.Error("expected /things/{id} dropped")
	}

	// Delete.
	resp = e.do(http.MethodDelete, "/api/api-catalog/"+itoa(id), "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	resp = e.do(http.MethodGet, "/api/api-catalog/"+itoa(id), "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", resp.StatusCode)
	}
}

func TestCatalog_ImportBadSpec(t *testing.T) {
	e := newCatalogEnv(t)
	resp := e.uploadSpec("not: a: valid: : spec")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad spec status = %d, want 400 (body=%s)", resp.StatusCode, readBody(resp))
	}
}

func TestCatalog_ImportURL_SSRFBlocked(t *testing.T) {
	e := newCatalogEnv(t)
	resp := e.do(http.MethodPost, "/api/api-catalog/import/url",
		`{"name":"x","scope":"avulso","source_url":"http://127.0.0.1:9/openapi.json"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ssrf status = %d, want 400 (body=%s)", resp.StatusCode, readBody(resp))
	}
}

// TestCatalog_EntidadeScope: a scoped sga user imports with no explicit grants
// ⇒ creator defaults to sga, the row is visible to sga and invisible to sgp.
func TestCatalog_EntidadeScope(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	sgaID, sgpID := entidadeID(t, d, "sga"), entidadeID(t, d, "sgp")
	sga := newCatalogEnvOn(t, d, "sga-user", sgaID)
	sgp := newCatalogEnvOn(t, d, "sgp-user", sgpID)

	resp := sga.uploadSpec(testOpenAPI30)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, body=%s", resp.StatusCode, readBody(resp))
	}
	created := decodeObj(t, resp)
	id := int64(created["id"].(float64))
	ents, _ := created["entidades"].(map[string]any)
	if ents == nil || ents["creator_entidade_id"] != float64(sgaID) {
		t.Errorf("entidades = %v, want creator %d", created["entidades"], sgaID)
	}

	if arr := decodeArr(t, sga.do(http.MethodGet, "/api/api-catalog", "")); len(arr) != 1 {
		t.Errorf("sga list = %d, want 1", len(arr))
	}
	if arr := decodeArr(t, sgp.do(http.MethodGet, "/api/api-catalog", "")); len(arr) != 0 {
		t.Errorf("sgp list = %d, want 0", len(arr))
	}
	if resp := sgp.do(http.MethodGet, "/api/api-catalog/"+itoa(id), ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("sgp get = %d, want 404", resp.StatusCode)
	}
	if resp := sga.do(http.MethodGet, "/api/api-catalog/"+itoa(id), ""); resp.StatusCode != http.StatusOK {
		t.Errorf("sga get = %d, want 200", resp.StatusCode)
	}
}

// --- helpers ---

func readBody(resp *http.Response) string {
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func decodeObj(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(readBody(resp)), &m); err != nil {
		t.Fatalf("decode obj: %v", err)
	}
	return m
}

// decodeArr unwraps the R4 list envelope {data:[...], meta:{...}}.
func decodeArr(t *testing.T, resp *http.Response) []map[string]any {
	t.Helper()
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(readBody(resp)), &env); err != nil {
		t.Fatalf("decode arr: %v", err)
	}
	return env.Data
}

func itoa(v int64) string { return strings.TrimSpace(strconv.FormatInt(v, 10)) }
