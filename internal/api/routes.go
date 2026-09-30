package api

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// routeRegistrar gives handler groups a uniform vocabulary for self-registering
// their routes with the right auth policy, so each group's route+role table
// lives next to its handler (registerRoutes) instead of in one giant NewRouter
// block. The four policies mirror the middleware helpers in router.go.
//
// Patterns keep the stdlib ServeMux spelling ("GET /api/hosts/{slug}") and are
// translated to Echo's (":slug") at registration; handlers stay plain
// http.HandlerFuncs and read params with r.PathValue (see adapt).
type routeRegistrar struct {
	e  *echo.Echo
	db *database.DB // nil skips the auth middleware: handler tests inject identity themselves
}

// public registers a route with no auth (the caller's capability is the URL, or
// the endpoint is the auth itself).
func (rr routeRegistrar) public(pattern string, h http.HandlerFunc) {
	rr.add(pattern, h, nil)
}

// auth registers a route requiring an authenticated session (any role).
func (rr routeRegistrar) auth(pattern string, h http.HandlerFunc) {
	rr.add(pattern, h, func(next http.Handler) http.Handler { return authenticated(rr.db, next) })
}

// role registers a route requiring an authenticated session with at least the
// given role.
func (rr routeRegistrar) role(role, pattern string, h http.HandlerFunc) {
	rr.add(pattern, h, func(next http.Handler) http.Handler { return authedRole(rr.db, role, next) })
}

// perm registers a route requiring an authenticated session holding the given
// permission.
func (rr routeRegistrar) perm(permission, pattern string, h http.HandlerFunc) {
	rr.add(pattern, h, func(next http.Handler) http.Handler { return authedPermission(rr.db, permission, next) })
}

var pathParam = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// add translates "METHOD /path/{name}" into Echo's "/path/:name" and mounts h
// behind mw (the auth policy), unless the registrar has no db (tests).
func (rr routeRegistrar) add(pattern string, h http.HandlerFunc, mw func(http.Handler) http.Handler) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		panic("route pattern needs a method: " + pattern)
	}
	if mw == nil {
		markPublic(method, path) // the docs label these "Public"
	}
	var mws []echo.MiddlewareFunc
	if mw != nil && rr.db != nil {
		// Every authenticated route needs a token scope, resolved once here:
		// a route no rule covers stops the server from starting, so nothing
		// ships unscoped. The scope check runs after auth (it reads the token).
		scope, ok := auth.ScopeFor(method, path)
		if !ok {
			panic("route has no token scope (add a rule in internal/auth/scopes.go): " + pattern)
		}
		authMW := mw
		mws = append(mws, echo.WrapMiddleware(func(next http.Handler) http.Handler {
			return authMW(auth.RequireScope(scope, next))
		}))
	}
	path = pathParam.ReplaceAllString(path, ":$1")
	rr.e.Add(method, path, adapt(h), mws...)
}

// adapt runs a net/http handler under Echo. Echo keeps route params on its own
// Context, not on the request, so without copying them over every
// r.PathValue("id") in the handlers would silently read "". When the URL has
// escapes, Echo matches on RawPath and leaves params encoded; ServeMux handed
// handlers decoded values, so decode here to keep that contract.
func adapt(h http.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		for i, name := range c.ParamNames() {
			v := c.ParamValues()[i]
			if r.URL.RawPath != "" {
				if dec, err := url.PathUnescape(v); err == nil {
					v = dec
				}
			}
			r.SetPathValue(name, v)
		}
		h(c.Response(), r)
		return nil
	}
}
