package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// routeRegistrar gives handler groups a uniform vocabulary for self-registering
// their routes with the right auth policy, so each group's route+role table
// lives next to its handler (registerRoutes) instead of in one giant NewRouter
// block. The four policies mirror the middleware helpers in router.go.
type routeRegistrar struct {
	mux *http.ServeMux
	db  *database.DB
}

// public registers a route with no auth (the caller's capability is the URL, or
// the endpoint is the auth itself).
func (rr routeRegistrar) public(pattern string, h http.HandlerFunc) {
	rr.mux.Handle(pattern, h)
}

// auth registers a route requiring an authenticated session (any role).
func (rr routeRegistrar) auth(pattern string, h http.HandlerFunc) {
	rr.mux.Handle(pattern, authenticated(rr.db, h))
}

// role registers a route requiring an authenticated session with at least the
// given role.
func (rr routeRegistrar) role(role, pattern string, h http.HandlerFunc) {
	rr.mux.Handle(pattern, authedRole(rr.db, role, h))
}

// perm registers a route requiring an authenticated session holding the given
// permission.
func (rr routeRegistrar) perm(permission, pattern string, h http.HandlerFunc) {
	rr.mux.Handle(pattern, authedPermission(rr.db, permission, h))
}
