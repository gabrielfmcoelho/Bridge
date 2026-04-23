package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth/providers"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	glpiclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/glpi"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// NewRouter creates the API mux with all routes and middleware.
func NewRouter(db *database.DB, configPath string) http.Handler {
	mux := http.NewServeMux()

	// Build the auth provider registry.
	registry := auth.NewProviderRegistry()
	registry.Register(providers.NewLocalProvider(db.SQL))
	registry.Register(providers.NewLDAPProvider(db.SQL, db.Encryptor))
	registry.Register(providers.NewKeycloakProvider(db.SQL, db.Encryptor))
	registry.Register(providers.NewGitLabProvider(db.SQL, db.Encryptor))

	ah := &authHandlers{db: db, registry: registry}
	hh := &hostHandlers{db: db}
	dh := &dnsHandlers{db: db}
	ph := &projectHandlers{db: db}
	sh := &serviceHandlers{db: db}
	oh := &orchestratorHandlers{db: db}
	ssh := &sshHandlers{db: db, configPath: configPath}
	gh := &graphHandlers{db: db}
	dash := &dashboardHandlers{db: db}
	eh := &enumHandlers{db: db}
	sth := &settingsHandlers{db: db}
	ih := &issueHandlers{db: db}
	rh := &releaseHandlers{db: db}
	th := &toolHandlers{db: db}
	ch := &contactHandlers{db: db}
	skh := &sshKeyHandlers{db: db}
	imh := &importHandlers{db: db}
	bkh := &backupHandlers{db: db}
	gih := &globalIssueHandlers{db: db}
	hah := &hostAlertHandlers{db: db}
	hch := &hostChamadoHandlers{db: db}
	ish := &integrationSettingsHandlers{db: db, registry: registry}
	oah := &oauthHandlers{db: db, registry: registry, ah: ah}
	glh := &gitlabHandlers{db: db}
	pglh := &projectGitLabHandlers{db: db}
	aih := &aiHandlers{db: db}
	clh := &coolifyHandlers{db: db}
	grh := &grafanaHandlers{db: db}
	gwh := &grafanaWebhookHandlers{db: db}
	olh := &outlineHandlers{db: db}
	glpiSessionCache := glpiclient.NewSessionCache(30 * time.Minute)
	glpih := &glpiHandlers{db: db, cache: glpiSessionCache}

	// Auth routes (no auth required)
	mux.HandleFunc("GET /api/auth/status", ah.handleStatus)
	mux.HandleFunc("POST /api/auth/setup", ah.handleSetup)
	mux.HandleFunc("POST /api/auth/login", ah.handleLogin)

	// OAuth routes (no auth — they ARE the auth)
	mux.HandleFunc("GET /api/auth/oauth/{provider}/authorize", oah.handleAuthorize)
	mux.HandleFunc("GET /api/auth/oauth/{provider}/callback", oah.handleCallback)

	// Auth routes (auth required)
	mux.Handle("POST /api/auth/logout", authenticated(db, http.HandlerFunc(ah.handleLogout)))
	mux.Handle("GET /api/auth/me", authenticated(db, http.HandlerFunc(ah.handleMe)))

	// User management (admin only)
	mux.Handle("GET /api/users", authedRole(db, "admin", http.HandlerFunc(ah.handleListUsers)))
	mux.Handle("POST /api/users", authedRole(db, "admin", http.HandlerFunc(ah.handleCreateUser)))
	mux.Handle("PUT /api/users/{id}", authedRole(db, "admin", http.HandlerFunc(ah.handleUpdateUser)))
	mux.Handle("DELETE /api/users/{id}", authedRole(db, "admin", http.HandlerFunc(ah.handleDeleteUser)))

	// Hosts
	mux.Handle("GET /api/hosts", authenticated(db, http.HandlerFunc(hh.handleList)))
	mux.Handle("POST /api/hosts", authedRole(db, "editor", http.HandlerFunc(hh.handleCreate)))
	mux.Handle("GET /api/hosts/{slug}", authenticated(db, http.HandlerFunc(hh.handleGet)))
	mux.Handle("PUT /api/hosts/{slug}", authedRole(db, "editor", http.HandlerFunc(hh.handleUpdate)))
	mux.Handle("DELETE /api/hosts/{slug}", authedRole(db, "admin", http.HandlerFunc(hh.handleDelete)))
	mux.Handle("GET /api/hosts/{slug}/password", authedRole(db, "admin", http.HandlerFunc(hh.handleGetPassword)))

	// Host alerts (manual)
	mux.Handle("GET /api/hosts/{slug}/alerts", authenticated(db, http.HandlerFunc(hah.handleList)))
	mux.Handle("POST /api/hosts/{slug}/alerts", authedRole(db, "editor", http.HandlerFunc(hah.handleCreate)))
	mux.Handle("PUT /api/hosts/{slug}/alerts/{alertId}", authedRole(db, "editor", http.HandlerFunc(hah.handleUpdate)))
	mux.Handle("POST /api/hosts/{slug}/alerts/{alertId}/conclude", authedRole(db, "editor", http.HandlerFunc(hah.handleConclude)))
	mux.Handle("DELETE /api/hosts/{slug}/alerts/{alertId}", authedRole(db, "admin", http.HandlerFunc(hah.handleDelete)))

	// Host chamados
	mux.Handle("GET /api/hosts/{slug}/chamados", authenticated(db, http.HandlerFunc(hch.handleList)))
	mux.Handle("POST /api/hosts/{slug}/chamados", authedRole(db, "editor", http.HandlerFunc(hch.handleCreate)))
	mux.Handle("PUT /api/hosts/{slug}/chamados/{chamadoId}", authedRole(db, "editor", http.HandlerFunc(hch.handleUpdate)))
	mux.Handle("DELETE /api/hosts/{slug}/chamados/{chamadoId}", authedRole(db, "admin", http.HandlerFunc(hch.handleDelete)))

	// DNS
	mux.Handle("GET /api/dns", authenticated(db, http.HandlerFunc(dh.handleList)))
	mux.Handle("POST /api/dns", authedRole(db, "editor", http.HandlerFunc(dh.handleCreate)))
	mux.Handle("GET /api/dns/{id}", authenticated(db, http.HandlerFunc(dh.handleGet)))
	mux.Handle("PUT /api/dns/{id}", authedRole(db, "editor", http.HandlerFunc(dh.handleUpdate)))
	mux.Handle("DELETE /api/dns/{id}", authedRole(db, "admin", http.HandlerFunc(dh.handleDelete)))

	// Projects
	mux.Handle("GET /api/projects", authenticated(db, http.HandlerFunc(ph.handleList)))
	mux.Handle("POST /api/projects", authedRole(db, "editor", http.HandlerFunc(ph.handleCreate)))
	mux.Handle("GET /api/projects/{id}", authenticated(db, http.HandlerFunc(ph.handleGet)))
	mux.Handle("PUT /api/projects/{id}", authedRole(db, "editor", http.HandlerFunc(ph.handleUpdate)))
	mux.Handle("DELETE /api/projects/{id}", authedRole(db, "admin", http.HandlerFunc(ph.handleDelete)))

	// Services
	mux.Handle("GET /api/services", authenticated(db, http.HandlerFunc(sh.handleList)))
	mux.Handle("POST /api/services", authedRole(db, "editor", http.HandlerFunc(sh.handleCreate)))
	mux.Handle("GET /api/services/{id}", authenticated(db, http.HandlerFunc(sh.handleGet)))
	mux.Handle("PUT /api/services/{id}", authedRole(db, "editor", http.HandlerFunc(sh.handleUpdate)))
	mux.Handle("DELETE /api/services/{id}", authedRole(db, "admin", http.HandlerFunc(sh.handleDelete)))
	mux.Handle("POST /api/services/{id}/fixate", authedRole(db, "editor", http.HandlerFunc(sh.handleFixate)))
	mux.Handle("PUT /api/services/{id}/container", authedRole(db, "editor", http.HandlerFunc(sh.handleUpdateContainer)))

	// Service credentials
	mux.Handle("GET /api/services/credentials/all", authenticated(db, http.HandlerFunc(sh.handleListAllCredentials)))
	mux.Handle("GET /api/services/{id}/credentials", authenticated(db, http.HandlerFunc(sh.handleListCredentials)))
	mux.Handle("POST /api/services/{id}/credentials", authedRole(db, "editor", http.HandlerFunc(sh.handleCreateCredential)))
	mux.Handle("GET /api/services/{id}/credentials/{credId}", authenticated(db, http.HandlerFunc(sh.handleGetCredential)))
	mux.Handle("DELETE /api/services/{id}/credentials/{credId}", authedRole(db, "admin", http.HandlerFunc(sh.handleDeleteCredential)))

	// Orchestrators
	mux.Handle("GET /api/orchestrators", authenticated(db, http.HandlerFunc(oh.handleList)))
	mux.Handle("POST /api/orchestrators", authedRole(db, "editor", http.HandlerFunc(oh.handleCreate)))
	mux.Handle("PUT /api/orchestrators/{id}", authedRole(db, "editor", http.HandlerFunc(oh.handleUpdate)))
	mux.Handle("DELETE /api/orchestrators/{id}", authedRole(db, "admin", http.HandlerFunc(oh.handleDelete)))

	// SSH operations
	mux.Handle("GET /api/ssh/preview-config", authenticated(db, http.HandlerFunc(ssh.handlePreviewConfig)))
	mux.Handle("POST /api/ssh/generate-config", authedRole(db, "editor", http.HandlerFunc(ssh.handleGenerateConfig)))
	mux.Handle("POST /api/ssh/test/{slug}", authedRole(db, "editor", http.HandlerFunc(ssh.handleTestConnection)))
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
	mux.Handle("POST /api/ssh/nginx-cleanup/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleNginxCleanup)))
	mux.Handle("POST /api/ssh/grafana-agent-setup/{slug}", authedRole(db, "admin", http.HandlerFunc(ssh.handleGrafanaAgentSetup)))
	mux.Handle("GET /api/ssh/host-config/{slug}", authenticated(db, http.HandlerFunc(ssh.handleHostSSHConfig)))

	// Graph & Dashboard
	mux.Handle("GET /api/graph", authenticated(db, http.HandlerFunc(gh.handleGraph)))
	mux.Handle("GET /api/dashboard", authenticated(db, http.HandlerFunc(dash.handleDashboard)))

	// Enums
	mux.Handle("GET /api/enums", authenticated(db, http.HandlerFunc(eh.handleListAll)))
	mux.Handle("GET /api/enums/{category}", authenticated(db, http.HandlerFunc(eh.handleList)))
	mux.Handle("POST /api/enums/{category}", authedRole(db, "admin", http.HandlerFunc(eh.handleCreate)))
	mux.Handle("PUT /api/enums/{category}/{value}", authedRole(db, "admin", http.HandlerFunc(eh.handleUpdate)))
	mux.Handle("DELETE /api/enums/{category}/{value}", authedRole(db, "admin", http.HandlerFunc(eh.handleDelete)))

	// Issues (nested under projects)
	mux.Handle("GET /api/projects/{id}/issues", authenticated(db, http.HandlerFunc(ih.handleList)))
	mux.Handle("POST /api/projects/{id}/issues", authedRole(db, "editor", http.HandlerFunc(ih.handleCreate)))
	mux.Handle("PUT /api/projects/{id}/issues/{issueId}", authedRole(db, "editor", http.HandlerFunc(ih.handleUpdate)))
	mux.Handle("PATCH /api/projects/{id}/issues/{issueId}/move", authedRole(db, "editor", http.HandlerFunc(ih.handleMove)))
	mux.Handle("DELETE /api/projects/{id}/issues/{issueId}", authedRole(db, "admin", http.HandlerFunc(ih.handleDelete)))

	// Issues by service
	mux.Handle("GET /api/services/{id}/issues", authenticated(db, http.HandlerFunc(ih.handleListByService)))

	// Global issues
	mux.Handle("GET /api/issues", authenticated(db, http.HandlerFunc(gih.handleList)))
	mux.Handle("POST /api/issues", authedRole(db, "editor", http.HandlerFunc(gih.handleCreate)))
	mux.Handle("PUT /api/issues/{id}", authedRole(db, "editor", http.HandlerFunc(gih.handleUpdate)))
	mux.Handle("PATCH /api/issues/{id}/move", authedRole(db, "editor", http.HandlerFunc(gih.handleMove)))
	mux.Handle("PATCH /api/issues/{id}/archive", authedRole(db, "editor", http.HandlerFunc(gih.handleArchive)))
	mux.Handle("DELETE /api/issues/{id}", authedRole(db, "admin", http.HandlerFunc(gih.handleDelete)))

	// Releases (public GET for timeline, auth for management)
	mux.HandleFunc("GET /api/releases", rh.handleList)
	mux.Handle("POST /api/releases", authedRole(db, "editor", http.HandlerFunc(rh.handleCreate)))
	mux.HandleFunc("GET /api/releases/{id}", rh.handleGet)
	mux.Handle("PUT /api/releases/{id}", authedRole(db, "editor", http.HandlerFunc(rh.handleUpdate)))
	mux.Handle("DELETE /api/releases/{id}", authedRole(db, "admin", http.HandlerFunc(rh.handleDelete)))

	// External tools
	mux.Handle("GET /api/tools", authenticated(db, http.HandlerFunc(th.handleList)))
	mux.Handle("POST /api/tools", authedRole(db, "admin", http.HandlerFunc(th.handleCreate)))
	mux.Handle("POST /api/tools/sync-service", authedRole(db, "admin", http.HandlerFunc(th.handleSyncFromService)))
	mux.Handle("DELETE /api/tools/sync-service/{id}", authedRole(db, "admin", http.HandlerFunc(th.handleUnsyncService)))
	mux.Handle("GET /api/tools/{id}", authenticated(db, http.HandlerFunc(th.handleGet)))
	mux.Handle("GET /api/tools/{id}/credentials", authenticated(db, http.HandlerFunc(th.handleListToolCredentials)))
	mux.Handle("GET /api/tools/{id}/credentials/{credId}", authenticated(db, http.HandlerFunc(th.handleGetToolCredential)))
	mux.Handle("PUT /api/tools/{id}", authedRole(db, "admin", http.HandlerFunc(th.handleUpdate)))
	mux.Handle("DELETE /api/tools/{id}", authedRole(db, "admin", http.HandlerFunc(th.handleDelete)))

	// App settings (appearance)
	// GET is public (no auth) so login/setup pages can load branding
	mux.HandleFunc("GET /api/settings/appearance", sth.handleGetAppearance)
	mux.Handle("PUT /api/settings/appearance", authedRole(db, "admin", http.HandlerFunc(sth.handleUpdateAppearance)))
	mux.Handle("POST /api/settings/appearance/logo", authedRole(db, "admin", http.HandlerFunc(sth.handleUploadLogo)))
	mux.Handle("DELETE /api/settings/appearance/logo", authedRole(db, "admin", http.HandlerFunc(sth.handleDeleteLogo)))
	mux.Handle("GET /api/settings/alerts", authenticated(db, http.HandlerFunc(sth.handleGetAlertThresholds)))
	mux.Handle("PUT /api/settings/alerts", authedRole(db, "admin", http.HandlerFunc(sth.handleUpdateAlertThresholds)))

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
	mux.Handle("GET /api/ai/status", authenticated(db, http.HandlerFunc(aih.handleStatus)))
	mux.Handle("POST /api/ai/assist/issue", authedPermission(db, "ai.use", http.HandlerFunc(aih.handleAssistIssue)))
	mux.Handle("POST /api/ai/assist/host-doc", authedPermission(db, "ai.use", http.HandlerFunc(aih.handleAssistHostDoc)))
	mux.Handle("POST /api/ai/chat", authedPermission(db, "ai.use", http.HandlerFunc(aih.handleChat)))
	mux.Handle("GET /api/projects/{id}/ai/analyze", authenticated(db, http.HandlerFunc(aih.handleGetProjectAnalysis)))
	mux.Handle("POST /api/projects/{id}/ai/analyze", authedPermission(db, "ai.use", http.HandlerFunc(aih.handleAnalyzeProject)))

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
	mux.Handle("GET /api/contacts", authenticated(db, http.HandlerFunc(ch.handleList)))
	mux.Handle("POST /api/contacts", authedRole(db, "editor", http.HandlerFunc(ch.handleCreate)))
	mux.Handle("PUT /api/contacts/{id}", authedRole(db, "editor", http.HandlerFunc(ch.handleUpdate)))
	mux.Handle("DELETE /api/contacts/{id}", authedRole(db, "admin", http.HandlerFunc(ch.handleDelete)))

	// SSH Keys (managed in DB)
	mux.Handle("GET /api/ssh-keys", authenticated(db, http.HandlerFunc(skh.handleList)))
	mux.Handle("POST /api/ssh-keys", authedRole(db, "editor", http.HandlerFunc(skh.handleCreate)))
	mux.Handle("GET /api/ssh-keys/{id}", authenticated(db, http.HandlerFunc(skh.handleGet)))
	mux.Handle("PUT /api/ssh-keys/{id}", authedRole(db, "editor", http.HandlerFunc(skh.handleUpdate)))
	mux.Handle("DELETE /api/ssh-keys/{id}", authedRole(db, "admin", http.HandlerFunc(skh.handleDelete)))

	// Bulk import (admin only)
	mux.Handle("POST /api/import", authedRole(db, "admin", http.HandlerFunc(imh.handleImport)))
	mux.Handle("POST /api/import/hosts", authedRole(db, "admin", http.HandlerFunc(imh.handleImportHosts)))
	mux.Handle("POST /api/import/dns", authedRole(db, "admin", http.HandlerFunc(imh.handleImportDNS)))

	// Database backup/restore (admin only)
	mux.Handle("GET /api/backup", authedRole(db, "admin", http.HandlerFunc(bkh.handleBackup)))
	mux.Handle("POST /api/restore", authedRole(db, "admin", http.HandlerFunc(bkh.handleRestore)))

	// Tags
	mux.Handle("GET /api/tags", authenticated(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entityType := r.URL.Query().Get("type")
		tags := []string{}
		var err error
		if entityType != "" {
			tags, err = models.GetDistinctTags(db.SQL, entityType)
		} else {
			tags, err = models.GetAllDistinctTags(db.SQL)
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to list tags")
			return
		}
		jsonOK(w, tags)
	})))

	return corsMiddleware(mux)
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
