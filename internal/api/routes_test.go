package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
)

// TestSelfRegisteredRoutes_Wired verifies the migrated handler groups' routes
// are actually registered with the expected auth policy through NewRouter:
// protected routes must reject an unauthenticated request (401, not 404), and
// public routes must not 401. A 404 would mean the route was dropped during the
// self-registration migration.
func TestSelfRegisteredRoutes_Wired(t *testing.T) {
	d, err := dbtest.Open(t)
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
		{"GET", "/api/hosts/trash"},
		{"POST", "/api/hosts/1/restore"},
		{"GET", "/api/services/trash"},
		{"POST", "/api/services/1/restore"},
		{"GET", "/api/projects/trash"},
		{"POST", "/api/projects/1/restore"},
		{"GET", "/api/ssh-keys"},
		{"POST", "/api/import"},
		{"GET", "/api/backup"},
		{"GET", "/api/ai/status"},
		{"POST", "/api/ai/chat"},

		// SSH operations
		{"GET", "/api/ssh/preview-config"},
		{"POST", "/api/ssh/generate-config"},
		{"POST", "/api/ssh/test/x"},
		{"POST", "/api/ssh/network-test/x"},
		{"POST", "/api/ssh/setup-key/x"},
		{"POST", "/api/ssh/fix-dev-null/x"},
		{"POST", "/api/ssh/setup-sudo-nopasswd/x"},
		{"POST", "/api/ssh/create-remote-user/x"},
		{"POST", "/api/ssh/delete-remote-user/x"},
		{"GET", "/api/ssh/keys"},
		{"GET", "/api/ssh/download-config"},
		{"GET", "/api/ssh/server-info"},
		{"GET", "/api/ssh/operation-logs/x"},
		{"POST", "/api/ssh/list-remote-keys/x"},
		{"POST", "/api/ssh/docker-setup/x"},
		{"POST", "/api/ssh/docker-logs/x"},
		{"POST", "/api/ssh/docker-logs-rotation/x"},
		{"POST", "/api/ssh/nginx-cleanup/x"},
		{"POST", "/api/ssh/grafana-agent-setup/x"},
		{"GET", "/api/ssh/host-config/x"},

		// Integration settings, permissions, role mappings (admin only)
		{"GET", "/api/settings/integrations"},
		{"PUT", "/api/settings/integrations/x"},
		{"POST", "/api/settings/integrations/test/ldap"},
		{"POST", "/api/settings/integrations/test/gitlab-code"},
		{"POST", "/api/settings/integrations/test/llm"},
		{"POST", "/api/settings/integrations/test/grafana"},
		{"POST", "/api/settings/integrations/test/outline"},
		{"DELETE", "/api/settings/integrations/x/secret/y"},
		{"GET", "/api/settings/permissions"},
		{"PUT", "/api/settings/permissions"},
		{"GET", "/api/settings/role-mappings"},
		{"POST", "/api/settings/role-mappings"},
		{"DELETE", "/api/settings/role-mappings/1"},

		// GitLab — per-user token
		{"GET", "/api/gitlab/status"},
		{"POST", "/api/gitlab/token"},
		{"DELETE", "/api/gitlab/token"},
		{"GET", "/api/gitlab/projects/1/commits"},
		{"GET", "/api/gitlab/projects/1/issues"},
		{"POST", "/api/gitlab/projects/1/link"},

		// GitLab Code Management — per-project
		{"GET", "/api/projects/1/gitlab/links"},
		{"POST", "/api/projects/1/gitlab/links"},
		{"DELETE", "/api/projects/1/gitlab/links/1"},
		{"GET", "/api/projects/1/gitlab/commits"},

		// Grafana integration
		{"GET", "/api/grafana/embed-url"},
		{"GET", "/api/hosts/x/metrics/live"},
		{"POST", "/api/hosts/x/grafana/provision"},
		{"POST", "/api/services/1/grafana/provision"},

		// GLPI integration
		{"GET", "/api/settings/integrations/glpi/tokens"},
		{"POST", "/api/settings/integrations/glpi/tokens"},
		{"PUT", "/api/settings/integrations/glpi/tokens/1"},
		{"DELETE", "/api/settings/integrations/glpi/tokens/1"},
		{"POST", "/api/settings/integrations/glpi/tokens/1/test"},
		{"GET", "/api/settings/integrations/glpi/dropdowns"},
		{"GET", "/api/settings/integrations/glpi/dropdowns/x"},
		{"PUT", "/api/settings/integrations/glpi/dropdowns/x"},
		{"DELETE", "/api/settings/integrations/glpi/dropdowns/x"},
		{"POST", "/api/glpi/tickets"},
		{"GET", "/api/glpi/tickets/1"},
		{"GET", "/api/glpi/tickets/1/details"},
		{"GET", "/api/glpi/documents/1"},
		{"GET", "/api/glpi/forms"},
		{"GET", "/api/glpi/forms/1"},
		{"POST", "/api/glpi/forms/1/submit"},
		{"POST", "/api/glpi/forms/uploads"},
		{"GET", "/api/glpi/dropdowns/x/search"},
		{"GET", "/api/glpi/users/search"},
		{"GET", "/api/glpi/formcreator/tags/search"},
		{"GET", "/api/projects/1/glpi/tickets"},
		{"GET", "/api/glpi/profiles/1/tickets"},
		{"GET", "/api/hosts/x/glpi/tickets"},
		{"POST", "/api/hosts/x/chamados/1/glpi/refresh"},

		// Outline (wiki) integration
		{"GET", "/api/projects/1/wiki"},
		{"POST", "/api/projects/1/wiki/documents"},
		{"GET", "/api/projects/1/wiki/search"},
		{"GET", "/api/wiki/documents"},
		{"POST", "/api/wiki/documents"},
		{"GET", "/api/wiki/search"},
		{"GET", "/api/wiki/collections"},
		{"GET", "/api/wiki/tree"},
		{"GET", "/api/wiki/documents/1"},

		// Coolify integration
		{"GET", "/api/coolify/status"},
		{"POST", "/api/coolify/test"},
		{"GET", "/api/coolify/server-status/x"},
		{"POST", "/api/coolify/check/x"},
		{"POST", "/api/coolify/register/x"},
		{"POST", "/api/coolify/validate/x"},
		{"POST", "/api/coolify/sync/x"},
		{"POST", "/api/coolify/server/x/key"},
		{"DELETE", "/api/coolify/server/x"},
		{"GET", "/api/coolify/keys/1/check"},
		{"POST", "/api/coolify/keys/1/sync"},
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
		{"POST", "/api/webhooks/grafana/alerts"},
	}
	for _, rt := range public {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(rt.method, rt.path, nil))
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s %s -> %d, want a non-401/404 (public route)", rt.method, rt.path, rec.Code)
		}
	}
}
