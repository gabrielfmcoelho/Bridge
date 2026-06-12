package api

import (
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// NewRouter creates the API mux with all routes and middleware.
func NewRouter(db *database.DB, configPath string) http.Handler {
	mux := http.NewServeMux()

	// Wire the whole graph once (db → registry → deps → handlers), then bind the
	// short names the route table below uses to the container's handler fields.
	app := newApp(db, configPath)
	deps := app.deps
	ah := app.auth
	hh := app.host
	dh := app.dns
	ph := app.project
	sh := app.service
	oh := app.orchestrator
	ssh := app.ssh
	gh := app.graph
	dash := app.dashboard
	eh := app.enum
	sth := app.settings
	ih := app.issue
	rh := app.release
	th := app.tool
	ch := app.contact
	skh := app.sshKey
	imh := app.importH
	bkh := app.backup
	gih := app.globalIssue
	hah := app.hostAlert
	hch := app.hostChamado
	ish := app.integrationSettings
	oah := app.oauth
	glh := app.gitlab
	pglh := app.projectGitlab
	aih := app.ai
	clh := app.coolify
	grh := app.grafana
	gwh := app.grafanaWebhook
	olh := app.outline
	glpih := app.glpi

	// Self-registering handler groups own their route+role tables next to their
	// handler (R2). The integration/misc groups further below are not yet
	// migrated and remain inline.
	rr := routeRegistrar{mux: mux, db: db}
	ah.registerRoutes(rr)  // /api/auth/*, /api/users/*
	oah.registerRoutes(rr) // /api/auth/oauth/*
	hh.registerRoutes(rr)  // /api/hosts/*
	hah.registerRoutes(rr) // /api/hosts/{slug}/alerts/*
	hch.registerRoutes(rr) // /api/hosts/{slug}/chamados/*
	dh.registerRoutes(rr)  // /api/dns/*
	ph.registerRoutes(rr)  // /api/projects/*
	sh.registerRoutes(rr)  // /api/services/*

	// (Legacy /api/services/{id}/credentials and /api/services/credentials/all
	// were removed in Phase 1 cutover — callers use /api/secrets directly.)

	// Unified secrets (Phase 1 — spec §6). Auth at the perimeter only;
	// per-row ACL (RBAC for shared, ownership for personal) lives in vault.
	secretRepo := deps.Secrets
	secretH := &secretHandlers{db: db, repo: secretRepo}
	secretH.register(mux, func(next http.Handler) http.Handler { return authenticated(db, next) })

	// (Per-secret /api/share/{token} retired in R3 — single-secret shares are
	// now bundles; redemption is GET /api/share-bundle/{token} below.)

	// Atlas REST API catalog (Phase A/B). Browsing is authenticated;
	// import/mutate is editor; delete is admin — applied per-route inside
	// register.
	apiCatalogH := &apiCatalogHandlers{db: db, allowPrivateFetch: atlasAllowPrivateFetch()}
	apiCatalogH.register(mux, func(role string, next http.Handler) http.Handler {
		if role == "" {
			return authenticated(db, next)
		}
		return authedRole(db, role, next)
	})

	// Share bundles (Phase D). Authenticated owner routes + a public,
	// unwrapped redemption sibling to /api/share/{token}. Bundles reuse the
	// secret repo for crypto/ACL/reveal.
	bundleH := &bundleHandlers{repo: secretRepo}
	mux.Handle("POST /api/share-bundles", authenticated(db, http.HandlerFunc(bundleH.handleCreate)))
	mux.Handle("GET /api/share-bundles", authenticated(db, http.HandlerFunc(bundleH.handleList)))
	mux.Handle("DELETE /api/share-bundles/{id}", authenticated(db, http.HandlerFunc(bundleH.handleRevoke)))
	publicBundleH := &publicBundleHandlers{repo: secretRepo}
	mux.HandleFunc("GET /api/share-bundle/{token}", publicBundleH.handleRedeem)

	// Orchestrators
	oh.registerRoutes(rr) // /api/orchestrators/*

	// SSH operations
	mux.Handle("GET /api/ssh/preview-config", authenticated(db, http.HandlerFunc(ssh.handlePreviewConfig)))
	mux.Handle("POST /api/ssh/generate-config", authedRole(db, "editor", http.HandlerFunc(ssh.handleGenerateConfig)))
	mux.Handle("POST /api/ssh/test/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleTestConnection)))
	mux.Handle("POST /api/ssh/network-test/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleNetworkTest)))
	mux.Handle("POST /api/ssh/setup-key/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleSetupKey)))
	mux.Handle("POST /api/ssh/fix-dev-null/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleFixDevNull)))
	mux.Handle("POST /api/ssh/setup-sudo-nopasswd/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleSetupSudoNopasswd)))
	mux.Handle("POST /api/ssh/create-remote-user/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleCreateRemoteUser)))
	mux.Handle("POST /api/ssh/delete-remote-user/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleDeleteRemoteUser)))
	mux.Handle("GET /api/ssh/keys", authenticated(db, http.HandlerFunc(ssh.handleListKeys)))
	mux.Handle("GET /api/ssh/download-config", authenticated(db, http.HandlerFunc(ssh.handleDownloadConfig)))
	mux.Handle("GET /api/ssh/server-info", authenticated(db, http.HandlerFunc(ssh.handleServerInfo)))
	mux.Handle("GET /api/ssh/operation-logs/{slug}", authenticated(db, http.HandlerFunc(ssh.handleOperationLogs)))
	mux.Handle("POST /api/ssh/list-remote-keys/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleListRemoteKeys)))
	mux.Handle("POST /api/ssh/docker-setup/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleDockerSetup)))
	mux.Handle("POST /api/ssh/docker-logs/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleDockerLogsInspect)))
	mux.Handle("POST /api/ssh/docker-logs-rotation/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleDockerLogsApplyRotation)))
	mux.Handle("POST /api/ssh/nginx-cleanup/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleNginxCleanup)))
	mux.Handle("POST /api/ssh/grafana-agent-setup/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleGrafanaAgentSetup)))
	mux.Handle("GET /api/ssh/host-config/{slug}", authenticated(db, http.HandlerFunc(ssh.handleHostSSHConfig)))

	// Graph & Dashboard
	gh.registerRoutes(rr)   // /api/graph
	dash.registerRoutes(rr) // /api/dashboard

	// Enums
	eh.registerRoutes(rr) // /api/enums/*

	// Issues (project board + by-service)
	ih.registerRoutes(rr) // /api/projects/{id}/issues/*, /api/services/{id}/issues

	// Global issues
	gih.registerRoutes(rr) // /api/issues/*

	// Releases (public GET for timeline, auth for management)
	rh.registerRoutes(rr) // /api/releases/*

	// External tools (legacy /api/tools/{id}/credentials* removed in Phase 1 —
	// callers query /api/secrets?scope=service&parent_id=<tool.service_id>).
	th.registerRoutes(rr) // /api/tools/*

	// App settings (appearance + alert thresholds)
	sth.registerRoutes(rr) // /api/settings/appearance*, /api/settings/alerts

	// Integration settings (admin only)
	mux.Handle("GET /api/settings/integrations", authedRole(db, "admin", http.HandlerFunc(ish.handleGetIntegrations)))
	mux.Handle("PUT /api/settings/integrations/{group}", authedRole(db, "admin", http.HandlerFunc(ish.handleUpdateIntegrationGroup)))
	mux.Handle("POST /api/settings/integrations/test/ldap", authedRole(db, "admin", http.HandlerFunc(ish.handleTestLDAP)))
	mux.Handle("POST /api/settings/integrations/test/gitlab-code", authedRole(db, "admin", http.HandlerFunc(ish.handleTestGitLabCode)))
	mux.Handle("POST /api/settings/integrations/test/llm", authedRole(db, "admin", http.HandlerFunc(ish.handleTestLLM)))
	mux.Handle("POST /api/settings/integrations/test/grafana", authedRole(db, "admin", http.HandlerFunc(ish.handleTestGrafana)))
	mux.Handle("POST /api/settings/integrations/test/outline", authedRole(db, "admin", http.HandlerFunc(ish.handleTestOutline)))
	mux.Handle("DELETE /api/settings/integrations/{group}/secret/{key}", authedRole(db, "admin", http.HandlerFunc(ish.handleClearIntegrationSecret)))

	// Permissions management (admin only)
	mux.Handle("GET /api/settings/permissions", authedRole(db, "admin", http.HandlerFunc(ish.handleGetPermissions)))
	mux.Handle("PUT /api/settings/permissions", authedRole(db, "admin", http.HandlerFunc(ish.handleUpdatePermissions)))

	// Role mappings (admin only)
	mux.Handle("GET /api/settings/role-mappings", authedRole(db, "admin", http.HandlerFunc(ish.handleGetRoleMappings)))
	mux.Handle("POST /api/settings/role-mappings", authedRole(db, "admin", http.HandlerFunc(ish.handleCreateRoleMapping)))
	mux.Handle("DELETE /api/settings/role-mappings/{id}", authedRole(db, "admin", http.HandlerFunc(ish.handleDeleteRoleMapping)))

	// GitLab — per-user token (profile-level, optional)
	mux.Handle("GET /api/gitlab/status", authenticated(db, http.HandlerFunc(glh.handleStatus)))
	mux.Handle("POST /api/gitlab/token", authenticated(db, http.HandlerFunc(glh.handleSaveToken)))
	mux.Handle("DELETE /api/gitlab/token", authenticated(db, http.HandlerFunc(glh.handleDeleteToken)))
	mux.Handle("GET /api/gitlab/projects/{id}/commits", authenticated(db, http.HandlerFunc(glh.handleListCommits)))
	mux.Handle("GET /api/gitlab/projects/{id}/issues", authenticated(db, http.HandlerFunc(glh.handleListIssues)))
	mux.Handle("POST /api/gitlab/projects/{id}/link", authedRole(db, "editor", http.HandlerFunc(glh.handleLinkProject)))

	// GitLab Code Management — per-project link management + aggregated commits (uses shared service PAT)
	mux.Handle("GET /api/projects/{id}/gitlab/links", authenticated(db, http.HandlerFunc(pglh.handleListLinks)))
	mux.Handle("POST /api/projects/{id}/gitlab/links", authedRole(db, "editor", http.HandlerFunc(pglh.handleCreateLink)))
	mux.Handle("DELETE /api/projects/{id}/gitlab/links/{linkId}", authedRole(db, "editor", http.HandlerFunc(pglh.handleDeleteLink)))
	mux.Handle("GET /api/projects/{id}/gitlab/commits", authenticated(db, http.HandlerFunc(pglh.handleListCommits)))

	// AI / LLM integration
	aih.registerRoutes(rr) // /api/ai/*, /api/projects/{id}/ai/analyze

	// Grafana integration
	mux.Handle("GET /api/grafana/embed-url", authenticated(db, http.HandlerFunc(grh.handleEmbedURL)))
	mux.Handle("GET /api/hosts/{slug}/metrics/live", authenticated(db, http.HandlerFunc(grh.handleHostLiveMetrics)))
	mux.Handle("POST /api/hosts/{slug}/grafana/provision", authedRole(db, "admin", http.HandlerFunc(grh.handleProvisionHostDashboard)))
	mux.Handle("POST /api/services/{id}/grafana/provision", authedRole(db, "admin", http.HandlerFunc(grh.handleProvisionServiceDashboard)))

	// Public webhook — no auth middleware; HMAC-signed by Grafana and verified in the handler.
	mux.HandleFunc("POST /api/webhooks/grafana/alerts", gwh.handleAlertWebhook)

	// GLPI integration
	mux.Handle("GET /api/settings/integrations/glpi/tokens", authedRole(db, "admin", http.HandlerFunc(glpih.handleListTokenProfiles)))
	mux.Handle("POST /api/settings/integrations/glpi/tokens", authedRole(db, "admin", http.HandlerFunc(glpih.handleCreateTokenProfile)))
	mux.Handle("PUT /api/settings/integrations/glpi/tokens/{id}", authedRole(db, "admin", http.HandlerFunc(glpih.handleUpdateTokenProfile)))
	mux.Handle("DELETE /api/settings/integrations/glpi/tokens/{id}", authedRole(db, "admin", http.HandlerFunc(glpih.handleDeleteTokenProfile)))
	mux.Handle("POST /api/settings/integrations/glpi/tokens/{id}/test", authedRole(db, "admin", http.HandlerFunc(glpih.handleTestTokenProfile)))
	mux.Handle("GET /api/settings/integrations/glpi/dropdowns", authedRole(db, "admin", http.HandlerFunc(glpih.handleListDropdownCatalogues)))
	mux.Handle("GET /api/settings/integrations/glpi/dropdowns/{itemtype}", authedRole(db, "admin", http.HandlerFunc(glpih.handleGetDropdownCatalogue)))
	mux.Handle("PUT /api/settings/integrations/glpi/dropdowns/{itemtype}", authedRole(db, "admin", http.HandlerFunc(glpih.handleUpsertDropdownCatalogue)))
	mux.Handle("DELETE /api/settings/integrations/glpi/dropdowns/{itemtype}", authedRole(db, "admin", http.HandlerFunc(glpih.handleDeleteDropdownCatalogue)))
	mux.Handle("POST /api/glpi/tickets", authedRole(db, "editor", http.HandlerFunc(glpih.handleCreateTicket)))
	mux.Handle("GET /api/glpi/tickets/{id}", authenticated(db, http.HandlerFunc(glpih.handleGetTicket)))
	mux.Handle("GET /api/glpi/tickets/{id}/details", authenticated(db, http.HandlerFunc(glpih.handleGetTicketDetails)))
	mux.Handle("GET /api/glpi/documents/{id}", authenticated(db, http.HandlerFunc(glpih.handleGetGlpiDocument)))
	mux.Handle("GET /api/glpi/forms", authenticated(db, http.HandlerFunc(glpih.handleListForms)))
	mux.Handle("GET /api/glpi/forms/{id}", authenticated(db, http.HandlerFunc(glpih.handleGetFormBundle)))
	mux.Handle("POST /api/glpi/forms/{id}/submit", authedRole(db, "editor", http.HandlerFunc(glpih.handleSubmitForm)))
	mux.Handle("POST /api/glpi/forms/uploads", authedRole(db, "editor", http.HandlerFunc(glpih.handleUploadFormDocument)))
	mux.Handle("GET /api/glpi/dropdowns/{itemtype}/search", authenticated(db, http.HandlerFunc(glpih.handleSearchDropdown)))
	mux.Handle("GET /api/glpi/users/search", authenticated(db, http.HandlerFunc(glpih.handleSearchUsers)))
	mux.Handle("GET /api/glpi/formcreator/tags/search", authenticated(db, http.HandlerFunc(glpih.handleSearchFormcreatorTags)))
	mux.Handle("GET /api/projects/{id}/glpi/tickets", authenticated(db, http.HandlerFunc(glpih.handleListProjectTickets)))
	mux.Handle("GET /api/glpi/profiles/{id}/tickets", authenticated(db, http.HandlerFunc(glpih.handleListProfileTickets)))
	mux.Handle("GET /api/hosts/{slug}/glpi/tickets", authenticated(db, http.HandlerFunc(glpih.handleListHostTickets)))
	mux.Handle("POST /api/hosts/{slug}/chamados/{chamadoId}/glpi/refresh", authedRole(db, "editor", http.HandlerFunc(glpih.handleRefreshChamadoCache)))

	// Outline (wiki) integration
	mux.Handle("GET /api/projects/{id}/wiki", authenticated(db, http.HandlerFunc(olh.handleListProjectWiki)))
	mux.Handle("POST /api/projects/{id}/wiki/documents", authedRole(db, "editor", http.HandlerFunc(olh.handleCreateProjectDocument)))
	mux.Handle("GET /api/projects/{id}/wiki/search", authenticated(db, http.HandlerFunc(olh.handleSearchProjectWiki)))
	mux.Handle("GET /api/wiki/documents", authenticated(db, http.HandlerFunc(olh.handleListCommonWiki)))
	mux.Handle("POST /api/wiki/documents", authedRole(db, "editor", http.HandlerFunc(olh.handleCreateCommonDocument)))
	mux.Handle("GET /api/wiki/search", authenticated(db, http.HandlerFunc(olh.handleSearchCommonWiki)))
	mux.Handle("GET /api/wiki/collections", authedRole(db, "admin", http.HandlerFunc(olh.handleListWorkspaceCollections)))
	mux.Handle("GET /api/wiki/tree", authenticated(db, http.HandlerFunc(olh.handleCommonWikiTree)))
	mux.Handle("GET /api/wiki/documents/{id}", authenticated(db, http.HandlerFunc(olh.handleGetWikiDocument)))

	// Coolify integration
	mux.Handle("GET /api/coolify/status", authenticated(db, http.HandlerFunc(clh.handleStatus)))
	mux.Handle("POST /api/coolify/test", authedRole(db, "admin", http.HandlerFunc(clh.handleTestConnection)))
	mux.Handle("GET /api/coolify/server-status/{slug}", authedRole(db, "editor", http.HandlerFunc(clh.handleGetServerStatus)))
	mux.Handle("POST /api/coolify/check/{slug}", authedRole(db, "editor", http.HandlerFunc(clh.handleCheckHost)))
	mux.Handle("POST /api/coolify/register/{slug}", authedRole(db, "admin", http.HandlerFunc(clh.handleRegisterHost)))
	mux.Handle("POST /api/coolify/validate/{slug}", authedRole(db, "admin", http.HandlerFunc(clh.handleValidateHost)))
	mux.Handle("POST /api/coolify/sync/{slug}", authedRole(db, "admin", http.HandlerFunc(clh.handleSyncHost)))
	mux.Handle("POST /api/coolify/server/{slug}/key", authedRole(db, "admin", http.HandlerFunc(clh.handleUpdateServerKey)))
	mux.Handle("DELETE /api/coolify/server/{slug}", authedRole(db, "admin", http.HandlerFunc(clh.handleDeleteHost)))
	mux.Handle("GET /api/coolify/keys/{id}/check", authedRole(db, "editor", http.HandlerFunc(clh.handleCheckKey)))
	mux.Handle("POST /api/coolify/keys/{id}/sync", authedRole(db, "admin", http.HandlerFunc(clh.handleSyncKey)))

	// Contacts
	ch.registerRoutes(rr) // /api/contacts/*

	// SSH Keys (managed in DB)
	skh.registerRoutes(rr) // /api/ssh-keys/*

	// Bulk import (admin only)
	imh.registerRoutes(rr) // /api/import/*

	// Database backup/restore (admin only)
	bkh.registerRoutes(rr) // /api/backup, /api/restore

	// Tags
	mux.Handle("GET /api/tags", authenticated(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entityType := r.URL.Query().Get("type")
		tags := []string{}
		var err error
		if entityType != "" {
			tags, err = store.NewTagRepo(db.SQL).Distinct(r.Context(), entityType)
		} else {
			tags, err = store.NewTagRepo(db.SQL).AllDistinct(r.Context())
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to list tags")
			return
		}
		jsonOK(w, tags)
	})))

	// loggingMiddleware is outermost: it installs the actor sink before auth runs
	// and times the full request (incl. CORS handling) through to its final status.
	return loggingMiddleware(corsMiddleware(mux))
}

// authenticated wraps a handler with RequireAuth middleware.
func authenticated(db *database.DB, next http.Handler) http.Handler {
	return auth.RequireAuth(db.SQL, next)
}

// authedRole wraps a handler with RequireAuth + RequireRole middleware.
func authedRole(db *database.DB, role string, next http.Handler) http.Handler {
	return auth.RequireAuth(db.SQL, auth.RequireRole(role, next))
}

// authedPermission wraps a handler with RequireAuth + RequirePermission middleware.
func authedPermission(db *database.DB, permission string, next http.Handler) http.Handler {
	return auth.RequireAuth(db.SQL, auth.RequirePermission(db.SQL, permission, next))
}

// pathInt64 extracts an int64 path parameter.
func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
