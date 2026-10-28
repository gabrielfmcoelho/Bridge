package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestTestGestorConexao(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	router := NewRouter(d, "/tmp/sshcm-test-config")
	fake, kcSrv, _ := kcfake.New(t, "keycloak.example", "apis", nil)
	fake.Creds["gestor-bridge"] = "g3st0r"

	status := http.StatusOK
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"detail":"escopo gestor:projetos ausente"}`)
			return
		}
		fmt.Fprint(w, `{"projetos":{}}`)
	}))
	t.Cleanup(gw.Close)

	u := &models.User{Username: "ana", Role: "admin", AuthProvider: "local"}
	if err := store.NewUserRepo(d.SQL).Create(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := auth.CreateSession(d.SQL, u.ID)
	do := func(method, path, body string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: tok})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	for path, body := range map[string]string{
		"/api/settings/integrations/keycloak_apis": fmt.Sprintf(`{"kc_apis_base_url":"http://keycloak.example","kc_apis_internal_url":%q,"kc_apis_host_header":"keycloak.example"}`, kcSrv.URL),
		"/api/settings/integrations/gestor":        fmt.Sprintf(`{"gestor_base_url":%q,"gestor_client_id":"gestor-bridge","gestor_client_secret":"g3st0r"}`, gw.URL),
	} {
		if code, raw := do("PUT", path, body); code != 200 {
			t.Fatalf("PUT %s = %d %s", path, code, raw)
		}
	}
	code, raw := do("POST", "/api/settings/integrations/test/gestor", "")
	if code != 200 || !strings.Contains(raw, `"success":true`) {
		t.Errorf("ok case = %d %s", code, raw)
	}
	status = http.StatusUnauthorized
	code, raw = do("POST", "/api/settings/integrations/test/gestor", "")
	if code != 200 || !strings.Contains(raw, `"success":false`) || !strings.Contains(raw, "401") {
		t.Errorf("401 case = %d %s", code, raw)
	}
	if _, raw := do("GET", "/api/settings/integrations", ""); strings.Contains(raw, "g3st0r") {
		t.Error("secret leaked in settings listing")
	}
}

func TestScopesTemPrefixoGestor(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	_ = NewRouter(d, "/tmp/sshcm-test-config") // panics at start if a route lacks a scope prefix
	if g, ok := auth.ScopeFor("GET", "/api/gestor/x"); !ok || !strings.HasPrefix(g, "integrations") {
		t.Errorf("DomainFor = %q %v", g, ok)
	}
}
