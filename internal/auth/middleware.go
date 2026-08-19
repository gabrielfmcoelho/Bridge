package auth

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

// WithUser returns a context carrying u as the authenticated user. This is
// the inverse of UserFromContext and exists to let tests fabricate an
// authenticated request without going through the full session lookup. The
// production auth flow constructs the same context inside RequireAuth.
func WithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// RequireAuth is middleware that rejects unauthenticated requests with 401.
func RequireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetSessionToken(r)
		if token == "" {
			auditAuthFailure(r, "no session token")
			httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		userID, err := ValidateSession(db, token)
		if err != nil {
			auditAuthFailure(r, "invalid or expired session")
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}

		user, err := store.NewUserRepo(db).GetByID(r.Context(), userID)
		if err != nil || user == nil {
			auditAuthFailure(r, "user not found")
			httpx.WriteError(w, http.StatusUnauthorized, "user not found")
			return
		}

		// Record who authenticated so the request-logging middleware can report
		// the actor (it installed the sink on r's context before us).
		recordActor(r.Context(), user.Username)
		ctx := context.WithValue(r.Context(), userContextKey, user)
		// Entidade visibility scope, loaded once per request. Admin bypasses;
		// everyone else gets their visible set (own entidades + descendants).
		// On lookup failure fall back to an empty scope (sees only global)
		// rather than failing the request or silently widening access.
		scope := store.Scope{Admin: user.Role == "admin"}
		if !scope.Admin {
			if sc, err := store.NewEntidadeRepo(db).ScopeForUser(ctx, user.ID); err == nil {
				scope = sc
			} else {
				log.Printf("[auth] scope lookup for user %d failed: %v", user.ID, err)
			}
		}
		ctx = store.WithScope(ctx, scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole is middleware that rejects requests from users without the required role.
// Role hierarchy: admin > editor > viewer.
func RequireRole(minRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			auditAuthFailure(r, "role check: no authenticated user")
			httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if !hasMinRole(user.Role, minRole) {
			auditAuthFailure(r, "insufficient role: have "+user.Role+" need "+minRole)
			httpx.WriteError(w, http.StatusForbidden, "insufficient permissions")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func hasMinRole(userRole, minRole string) bool {
	levels := map[string]int{
		"viewer": 0,
		"editor": 1,
		"admin":  2,
	}
	return levels[userRole] >= levels[minRole]
}

// RequirePermission is middleware that rejects requests from users without the specified permission.
// Admin role always passes. For other roles, it checks the role_permissions table.
func RequirePermission(db *sql.DB, permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			auditAuthFailure(r, "permission check: no authenticated user")
			httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if !store.NewPermissionRepo(db).Has(r.Context(), user.Role, permission) {
			auditAuthFailure(r, "missing permission: "+permission)
			httpx.WriteError(w, http.StatusForbidden, "insufficient permissions")
			return
		}

		next.ServeHTTP(w, r)
	})
}
