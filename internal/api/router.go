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
	mux.Handle("POST /api/share-bundles/reissue", authenticated(db, http.HandlerFunc(bundleH.handleReissue)))
	mux.Handle("GET /api/share-bundles", authenticated(db, http.HandlerFunc(bundleH.handleList)))
	mux.Handle("PATCH /api/share-bundles/{id}", authenticated(db, http.HandlerFunc(bundleH.handleRenew)))
	mux.Handle("PUT /api/share-bundles/{id}/items", authenticated(db, http.HandlerFunc(bundleH.handleUpdateItems)))
	mux.Handle("DELETE /api/share-bundles/{id}", authenticated(db, http.HandlerFunc(bundleH.handleRevoke)))
	mux.Handle("GET /api/share-bundles/{id}/access-log", authenticated(db, http.HandlerFunc(bundleH.handleAccessLog)))
	publicBundleH := &publicBundleHandlers{repo: secretRepo}
	mux.HandleFunc("GET /api/share-bundle/{token}", publicBundleH.handleRedeem)

	// Orchestrators
	oh.registerRoutes(rr) // /api/orchestrators/*

	// SSH operations
	ssh.registerRoutes(rr) // /api/ssh/*

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

	// Integration settings, permissions, role mappings (admin only)
	ish.registerRoutes(rr) // /api/settings/integrations/*, /api/settings/permissions, /api/settings/role-mappings*

	// GitLab — per-user token (profile-level, optional)
	glh.registerRoutes(rr) // /api/gitlab/*

	// GitLab Code Management — per-project link management + aggregated commits (uses shared service PAT)
	pglh.registerRoutes(rr) // /api/projects/{id}/gitlab/*

	// AI / LLM integration
	aih.registerRoutes(rr) // /api/ai/*, /api/projects/{id}/ai/analyze

	// Grafana integration
	grh.registerRoutes(rr) // /api/grafana/*, /api/hosts/{slug}/metrics/live, host/service grafana provision

	// Public webhook — no auth middleware; HMAC-signed by Grafana and verified in the handler.
	gwh.registerRoutes(rr) // POST /api/webhooks/grafana/alerts

	// GLPI integration
	glpih.registerRoutes(rr) // /api/settings/integrations/glpi/*, /api/glpi/*, project/host glpi tickets

	// Outline (wiki) integration
	olh.registerRoutes(rr) // /api/projects/{id}/wiki/*, /api/wiki/*

	// Coolify integration
	clh.registerRoutes(rr) // /api/coolify/*

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
