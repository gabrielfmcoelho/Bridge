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
