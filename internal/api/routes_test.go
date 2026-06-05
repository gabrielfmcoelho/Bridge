package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// TestSelfRegisteredRoutes_Wired verifies the migrated handler groups' routes
// are actually registered with the expected auth policy through NewRouter:
// protected routes must reject an unauthenticated request (401, not 404), and
// public routes must not 401. A 404 would mean the route was dropped during the
// self-registration migration.
func TestSelfRegisteredRoutes_Wired(t *testing.T) {
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	router := NewRouter(d, "/tmp/sshcm-test-config")

	protected := []struct{ method, path string }{
		{"GET", "/api/hosts"},
		{"POST", "/api/hosts"},
		{"GET", "/api/hosts/x"},
		{"DELETE", "/api/hosts/x"},
		{"GET", "/api/hosts/x/alerts"},
		{"GET", "/api/hosts/x/chamados"},
		{"GET", "/api/dns"},
		{"POST", "/api/dns"},
		{"GET", "/api/projects"},
		{"GET", "/api/services"},
		{"POST", "/api/services/1/fixate"},
		{"GET", "/api/orchestrators"},
		{"GET", "/api/users"},
		{"GET", "/api/auth/me"},
		{"GET", "/api/graph"},
		{"GET", "/api/dashboard"},
		{"GET", "/api/enums"},
		{"GET", "/api/projects/1/issues"},
		{"GET", "/api/services/1/issues"},
		{"GET", "/api/issues"},
		{"POST", "/api/releases"},
		{"GET", "/api/tools"},
		{"GET", "/api/settings/alerts"},
		{"GET", "/api/contacts"},
		{"GET", "/api/ssh-keys"},
		{"POST", "/api/import"},
		{"GET", "/api/backup"},
		{"GET", "/api/ai/status"},
		{"POST", "/api/ai/chat"},
	}
	for _, rt := range protected {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(rt.method, rt.path, nil))
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s %s -> 404 (route not registered)", rt.method, rt.path)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s -> %d, want 401 (auth middleware)", rt.method, rt.path, rec.Code)
		}
	}

	// Public routes must be reachable without auth (not 401, not 404).
	public := []struct{ method, path string }{
		{"GET", "/api/auth/status"},
		{"GET", "/api/releases"},
		{"GET", "/api/settings/appearance"},
	}
	for _, rt := range public {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(rt.method, rt.path, nil))
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s %s -> %d, want a non-401/404 (public route)", rt.method, rt.path, rec.Code)
		}
	}
}
