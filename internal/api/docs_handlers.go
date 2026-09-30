package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/api/docs"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// The API reference is ReDoc at /docs, reading /docs/openapi.json: the swag
// spec (make swagger) enriched at serve time with each operation's token scope
// from the same rules the router enforces, so the docs never drift from them.
// The ReDoc bundle is vendored (make redoc) so the page works on the intranet
// without a CDN.

//go:embed redoc/redoc.standalone.js
var redocJS []byte

const redocPage = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Bridge API</title>
<style>body{margin:0}</style>
</head>
<body>
<redoc spec-url="/docs/openapi.json" hide-hostname="true" path-in-middle-panel="true" required-props-first="true" sort-props-alphabetically="false"></redoc>
<script src="/docs/redoc.standalone.js"></script>
</body>
</html>`

var (
	publicMu     sync.Mutex
	publicRoutes = map[string]bool{} // "METHOD /pattern" registered without auth
)

func markPublic(method, pattern string) {
	publicMu.Lock()
	defer publicMu.Unlock()
	publicRoutes[method+" "+pattern] = true
}

func isPublic(method, pattern string) bool {
	publicMu.Lock()
	defer publicMu.Unlock()
	return publicRoutes[method+" "+pattern]
}

// registerDocs mounts the reference. The page and spec need a session or a
// token like the rest of the API (the spec maps admin and secret endpoints);
// a browser without a session is sent to the login page instead of a 401.
func registerDocs(e *echo.Echo, db *database.DB) {
	var once sync.Once
	var spec []byte
	enriched := func() []byte {
		once.Do(func() { spec = enrichSpec(docs.SwaggerInfo.ReadDoc()) })
		return spec
	}
	guard := func(h http.HandlerFunc) echo.HandlerFunc {
		return echo.WrapHandler(loginRedirect(authenticated(db, h)))
	}
	page := guard(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(redocPage))
	})
	e.GET("/docs", page)
	e.GET("/docs/", page)
	e.GET("/docs/openapi.json", guard(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(enriched())
	}))
	// The bundle is the public ReDoc library, not Bridge data: no auth, cached.
	e.GET("/docs/redoc.standalone.js", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=86400")
		return c.Blob(http.StatusOK, "application/javascript", redocJS)
	})
	// Old Swagger UI links.
	e.GET("/api/docs", redirectDocs)
	e.GET("/api/docs/*", redirectDocs)
}

func redirectDocs(c echo.Context) error {
	return c.Redirect(http.StatusMovedPermanently, "/docs")
}

// loginRedirect turns the auth middleware's 401 into a redirect to the login
// page when a browser asks for HTML; API clients still get the 401.
func loginRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "text/html") {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(&redirectOn401{ResponseWriter: w, r: r}, r)
	})
}

type redirectOn401 struct {
	http.ResponseWriter
	r          *http.Request
	redirected bool
}

func (rw *redirectOn401) WriteHeader(code int) {
	if code == http.StatusUnauthorized {
		rw.redirected = true
		http.Redirect(rw.ResponseWriter, rw.r, "/login?next=/docs", http.StatusFound)
		return
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *redirectOn401) Write(b []byte) (int, error) {
	if rw.redirected {
		return len(b), nil // drop the JSON error body
	}
	return rw.ResponseWriter.Write(b)
}

// enrichSpec adds, to every operation, how it is authenticated: public routes
// say so; the rest get x-bridge-scope, a line naming the token scope, and the
// security schemes. Anything unparsable is served as generated.
func enrichSpec(raw string) []byte {
	var spec map[string]any
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return []byte(raw)
	}
	spec["securityDefinitions"] = map[string]any{
		"BearerAuth": map[string]any{
			"type": "apiKey", "in": "header", "name": "Authorization",
			"description": `A personal or service-account token: "Bearer brg_…" (Configurações → Tokens de API). It acts as its owner, narrowed to its scopes.`,
		},
		"SessionCookie": map[string]any{
			"type": "apiKey", "in": "cookie", "name": "sshcm_session",
			"description": "The browser session set by POST /api/auth/login. Not scope-limited.",
		},
	}
	paths, _ := spec["paths"].(map[string]any)
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		for method, v := range ops {
			op, ok := v.(map[string]any)
			if !ok {
				continue
			}
			m := strings.ToUpper(method)
			desc, _ := op["description"].(string)
			if isPublic(m, path) {
				op["description"] = strings.TrimSpace(desc + "\n\nPublic: no authentication.")
				op["security"] = []any{}
				continue
			}
			if scope, ok := auth.ScopeFor(m, path); ok {
				op["x-bridge-scope"] = scope
				op["description"] = strings.TrimSpace(desc + "\n\nToken scope: `" + scope + "`.")
			}
			op["security"] = []any{map[string]any{"BearerAuth": []any{}}, map[string]any{"SessionCookie": []any{}}}
		}
	}
	out, err := json.Marshal(spec)
	if err != nil {
		return []byte(raw)
	}
	return out
}
