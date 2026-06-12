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

func newCatalogEnv(t *testing.T) *catalogEnv {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	u := &models.User{Username: "editor", DisplayName: "Editor", Role: "editor", Email: "e@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	mux := http.NewServeMux()
	h := &apiCatalogHandlers{db: d}
	// Identity wrap: ignore the required role (the outer middleware injects a
	// fixed editor user); production wires authenticated()/authedRole().
	h.register(mux, func(role string, next http.Handler) http.Handler { return next })
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	})
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	return &catalogEnv{t: t, d: d, server: srv, user: u}
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
