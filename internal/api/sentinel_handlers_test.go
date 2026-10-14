package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestSentinelHandlers(t *testing.T) {
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
	call := func(cookie, method, path string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var obj map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &obj)
		return rec.Code, obj
	}

	var svcID int64
	if _, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('web', 'web')`); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO services (nickname) VALUES ('api') RETURNING id`).Scan(&svcID); err != nil {
		t.Fatal(err)
	}
	svc := "/api/services/" + itoa(svcID)

	// Integration off: a state of the host, answered 200 with the reason.
	code, body := call(admin, "GET", "/api/hosts/web/sentinel/metrics")
	if code != 200 || body["available"] != false || body["reason"] != "disabled" {
		t.Fatalf("metrics with integration off = %d %v", code, body)
	}
	if err := store.NewAppSettingsRepo(d.SQL).Set(ctx, "sentinel_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	code, body = call(admin, "GET", "/api/hosts/web/sentinel")
	st, _ := body["status"].(map[string]any)
	if code != 200 || st["reason"] != "not_linked" {
		t.Fatalf("status of an unlinked host = %d %v", code, body)
	}
	if code, body = call(admin, "GET", svc+"/sentinel/traffic"); code != 200 || body["reason"] != "not_linked" {
		t.Fatalf("traffic of a service on no Coolify host = %d %v", code, body)
	}

	// Per-host sync: editor+; an unlinked host records why instead of failing.
	if code, _ := call(viewer, "POST", "/api/hosts/web/sentinel/sync"); code != 403 {
		t.Errorf("viewer host sync = %d, want 403", code)
	}
	if code, body = call(admin, "POST", "/api/hosts/web/sentinel/sync"); code != 200 || body["state"] != "not_linked" {
		t.Fatalf("host sync of an unlinked host = %d %v", code, body)
	}
	if code, body = call(admin, "GET", "/api/hosts/web/sentinel/containers?fresh=1"); code != 200 || body["reason"] != "not_linked" {
		t.Fatalf("fresh containers = %d %v", code, body)
	}

	// Bad queries are the caller's fault.
	for _, p := range []string{"/api/hosts/web/sentinel/metrics?range=1y", svc + "/sentinel/traffic?range=1h", svc + "/sentinel/traffic/breakdown/ip"} {
		if code, _ := call(admin, "GET", p); code != 400 {
			t.Errorf("%s = %d, want 400", p, code)
		}
	}
	// Invisible / missing assets are 404 (a viewer sees no grant-less host).
	if code, _ := call(viewer, "GET", "/api/hosts/web/sentinel"); code != 404 {
		t.Errorf("viewer on a grant-less host = %d, want 404", code)
	}
	if code, _ := call(admin, "GET", "/api/services/999999/sentinel/resources"); code != 404 {
		t.Errorf("missing service = %d, want 404", code)
	}

	// Sync and the host list are admin-only.
	if code, _ := call(viewer, "POST", "/api/sentinel/sync"); code != 403 {
		t.Errorf("viewer sync = %d, want 403", code)
	}
	code, body = call(admin, "POST", "/api/sentinel/sync")
	if code != 200 || body["ok"] != float64(0) {
		t.Fatalf("sync with no linked host = %d %v", code, body)
	}
	if code, body = call(admin, "GET", "/api/sentinel/hosts"); code != 200 || !strings.Contains(toJSON(body), `"data":[]`) {
		t.Fatalf("hosts = %d %v", code, body)
	}

	// Alert thresholds: partial update keeps the omitted fields.
	req := httptest.NewRequest("PUT", "/api/settings/alerts", strings.NewReader(`{"traffic_5xx_critical":10}`))
	req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: admin})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("put thresholds = %d %s", rec.Code, rec.Body)
	}
	got, _ := store.NewAlertSettingsRepo(d.SQL).GetThresholds(ctx)
	if got.Traffic5xxCritical != 10 || got.ResourceCritical != 80 || got.LatencyP95Warning != 1000 {
		t.Fatalf("thresholds after partial update = %+v", got)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
