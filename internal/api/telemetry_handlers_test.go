package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestTelemetryRequests(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	router := NewRouter(d, "/tmp/sshcm-test-config")

	session := func(name, role string) string {
		u := &models.User{Username: name, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatalf("user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}
	admin, viewer := session("ana", "admin"), session("vic", "viewer")
	get := func(cookie, path string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var obj map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &obj)
		return rec.Code, obj
	}

	// An API without grants: admin-only.
	var adminID int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&adminID)
	a := &models.APICatalog{Name: "Servidores", SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
		BaseURL: "https://gateway.example/datalakehouse/servidores/", OwnerUserID: adminID, CreatedBy: adminID}
	if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, a, nil); err != nil {
		t.Fatalf("api: %v", err)
	}
	ext := "servidores-painel"
	if err := store.NewAPIKeyRepo(d.SQL).Create(ctx, &models.APIKey{APIID: a.ID, Label: "Painel", Source: models.APIKeySourceKeycloak, ExternalLabel: &ext}); err != nil {
		t.Fatalf("key: %v", err)
	}
	path := "/api/telemetry/requests?kind=api&id=" + itoa(a.ID)

	if code, _ := get(viewer, path); code != http.StatusNotFound {
		t.Errorf("invisible api = %d, want 404", code)
	}
	if code, _ := get(admin, "/api/telemetry/requests?kind=service&id=1&group_by=key"); code != http.StatusBadRequest {
		t.Errorf("group_by key on a service = %d, want 400", code)
	}
	if code, obj := get(admin, path); code != http.StatusOK || obj["available"] != false || obj["reason"] != "not_configured" {
		t.Fatalf("unconfigured = %d %v", code, obj)
	}

	// Configured: the fake ClickHouse sees the API's gateway path and answers one key.
	var prefix string
	ch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix = r.URL.Query().Get("param_prefix")
		b, _ := io.ReadAll(r.Body)
		row := `"count":3,"errors":0,"p50":1,"p95":2,"p99":3`
		if strings.Contains(string(b), "GROUP BY key") {
			io.WriteString(w, `{"data":[{"key":"servidores-painel",`+row+`},{"key":"desconhecido",`+row+`}]}`)
			return
		}
		io.WriteString(w, `{"data":[{`+row+`}]}`)
	}))
	defer ch.Close()
	settings := store.NewAppSettingsRepo(d.SQL)
	settings.Set(ctx, "signoz_enabled", "true")
	settings.Set(ctx, "signoz_ch_url", ch.URL)

	code, obj := get(admin, path+"&group_by=key&range=7d")
	if code != http.StatusOK || obj["available"] != true {
		t.Fatalf("configured = %d %v", code, obj)
	}
	if prefix != "/datalakehouse/servidores" {
		t.Errorf("path prefix = %q", prefix)
	}
	top := obj["top"].([]any)
	if l := top[0].(map[string]any)["label"]; l != "Painel" {
		t.Errorf("known key label = %v, want Painel", l)
	}
	if _, has := top[1].(map[string]any)["label"]; has {
		t.Errorf("unknown key got a label: %v", top[1])
	}
}

// A trace opens only through an asset the viewer can see and that the trace
// touches; query values reach admins only, credentials reach nobody.
func TestTelemetryTrace(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	router := NewRouter(d, "/tmp/sshcm-test-config")
	session := func(name, role string) string {
		u := &models.User{Username: name, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatalf("user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}
	admin, viewer := session("ana", "admin"), session("vic", "viewer")
	get := func(cookie, path string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var obj map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &obj)
		return rec.Code, obj
	}
	var adminID int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&adminID)
	mk := func(name string, global bool) int64 {
		a := &models.APICatalog{Name: name, SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
			BaseURL: "https://gw.example/" + name, OwnerUserID: adminID, CreatedBy: adminID}
		if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, a, nil); err != nil {
			t.Fatalf("api: %v", err)
		}
		if global {
			if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetAPICatalog, a.ID, models.AssetGrants{IsGlobal: true}); err != nil {
				t.Fatalf("grants: %v", err)
			}
		}
		return a.ID
	}
	public, private := mk("publica", true), mk("privada", false)

	const id = "0123456789abcdef0123456789abcdef"
	ch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		q := string(b)
		switch {
		case strings.Contains(q, "count() AS n"):
			// The trace touches only the public API's gateway path.
			n := 0
			if r.URL.Query().Get("param_prefix") == "/publica" {
				n = 1
			}
			io.WriteString(w, `{"data":[{"n":`+itoa(int64(n))+`}]}`)
		default:
			io.WriteString(w, `{"data":[{"span_id":"a","parent_id":"","service":"apisix-gateway","name":"GET","kind":"Server",
				"start_ns":1000,"duration_ms":5,"status":"200","error":false,
				"attrs":{"url.query":"cpf=12345678900&pagina=2","http.url":"http://x/publica/p?cpf=12345678900","http.request.header.authorization":"Bearer s3cr3t"},"nums":{}}]}`)
		}
	}))
	defer ch.Close()
	settings := store.NewAppSettingsRepo(d.SQL)
	settings.Set(ctx, "signoz_enabled", "true")
	settings.Set(ctx, "signoz_ch_url", ch.URL)

	trace := func(api int64) string { return "/api/telemetry/traces/" + id + "?kind=api&id=" + itoa(api) }
	if code, _ := get(viewer, trace(private)); code != http.StatusNotFound {
		t.Errorf("trace via an invisible API = %d, want 404", code)
	}
	if code, _ := get(admin, trace(private)); code != http.StatusNotFound {
		t.Errorf("trace via an API it doesn't touch = %d, want 404", code)
	}
	if code, _ := get(admin, "/api/telemetry/traces/not-a-trace?kind=api&id="+itoa(public)); code != http.StatusNotFound {
		t.Errorf("malformed trace id = %d, want 404", code)
	}

	attrs := func(obj map[string]any) map[string]any {
		return obj["spans"].([]any)[0].(map[string]any)["attributes"].(map[string]any)
	}
	code, obj := get(viewer, trace(public))
	if code != http.StatusOK || obj["redacted"] != true {
		t.Fatalf("viewer trace = %d %v", code, obj)
	}
	va := attrs(obj)
	if va["url.query"] != "cpf, pagina" || va["http.url"] != "http://x/publica/p?cpf" {
		t.Errorf("viewer sees values: %v", va)
	}
	if _, has := va["http.request.header.authorization"]; has {
		t.Error("viewer sees the authorization header")
	}
	code, obj = get(admin, trace(public))
	if code != http.StatusOK || obj["redacted"] == true {
		t.Fatalf("admin trace = %d %v", code, obj)
	}
	aa := attrs(obj)
	if aa["url.query"] != "cpf=12345678900&pagina=2" {
		t.Errorf("admin url.query = %v", aa["url.query"])
	}
	if _, has := aa["http.request.header.authorization"]; has {
		t.Error("admin sees the authorization header")
	}
}

func TestRedactSpan(t *testing.T) {
	a := map[string]string{"http.target": "/p?a=1&b=2&a=3", "url.full": "https://x/p", "Cookie": "c", "url.query": "%63pf=1"}
	redactSpan(a, true)
	if a["http.target"] != "/p?a&b" || a["url.full"] != "https://x/p" || a["url.query"] != "cpf" {
		t.Errorf("redacted = %v", a)
	}
	if _, has := a["Cookie"]; has {
		t.Error("cookie kept")
	}
}

func TestCleanDomain(t *testing.T) {
	for in, want := range map[string]string{
		"Bridge.10.0.122.91.sslip.io":    "bridge.10.0.122.91.sslip.io",
		"https://App.Example:8443/x?y=1": "app.example",
		" gateway.sead.pi.gov.br ":       "gateway.sead.pi.gov.br",
		"a,b.example":                    "",
		"":                               "",
	} {
		if got := cleanDomain(in); got != want {
			t.Errorf("cleanDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsCollectorDomain(t *testing.T) {
	for d, want := range map[string]bool{
		"otel-collector-m0cwcowwg0gsk8kscggwc4w0": true,
		"otelcollectorhttp.10.0.122.91.sslip.io":  true,
		"api-bridge.10.0.122.91.sslip.io":         false,
		"gerenciador.sead.pi.gov.br":              false,
	} {
		if got := isCollectorDomain(d); got != want {
			t.Errorf("isCollectorDomain(%q) = %v", d, got)
		}
	}
}
