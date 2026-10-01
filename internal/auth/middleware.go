package auth

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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
// A request authenticates with the session cookie or, for scripts, with
// "Authorization: Bearer brg_…" — a personal API token that acts as its owner
// — or with an access token of the keycloak_apis realm (oidcapis.go), which
// acts as the service account its client maps to.
func RequireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		var token *store.AuthenticatedToken
		var kc *apisToken // set for a Keycloak token
		bearer := bearerToken(r)
		if bearer != "" && !strings.HasPrefix(bearer, APITokenPrefix) && looksLikeJWT(bearer) {
			kt, err := authenticateAPIsToken(r.Context(), db, bearer)
			if err != nil {
				auditAuthFailure(r, "keycloak token: "+err.Error())
				httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired keycloak token")
				return
			}
			userID, kc = kt.userID, &kt // rate-limited below, once its user checks out
		} else if strings.HasPrefix(bearer, APITokenPrefix) {
			tok, ok, err := store.NewAPITokenRepo(db).Authenticate(r.Context(), HashAPIToken(bearer))
			if err != nil || !ok {
				auditAuthFailure(r, "invalid, expired or revoked api token")
				httpx.WriteError(w, http.StatusUnauthorized, "invalid, expired or revoked api token")
				return
			}
			limit := DefaultTokenRateLimit
			if tok.RateLimitPerMinute != nil {
				limit = *tok.RateLimitPerMinute
			}
			if allowed, retry := meter.allow(tok.ID, limit, time.Now()); !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				httpx.WriteError(w, http.StatusTooManyRequests, "api token rate limit exceeded")
				return
			}
			userID, token = tok.UserID, &tok
		} else {
			token := GetSessionToken(r)
			if token == "" {
				auditAuthFailure(r, "no session token")
				httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			var err error
			userID, err = ValidateSession(db, token)
			if err != nil {
				auditAuthFailure(r, "invalid or expired session")
				httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
		}

		user, err := store.NewUserRepo(db).GetByID(r.Context(), userID)
		if err != nil || user == nil {
			auditAuthFailure(r, "user not found")
			httpx.WriteError(w, http.StatusUnauthorized, "user not found")
			return
		}
		// A Keycloak client only ever acts as a service account: an identity
		// pointing at a person must not let a client impersonate them.
		if kc != nil {
			if user.Kind != models.UserKindService {
				auditAuthFailure(r, "keycloak token mapped to a non-service user")
				httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired keycloak token")
				return
			}
			if allowed, retry := meter.allow(-user.ID, kc.rateLimit, time.Now()); !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				httpx.WriteError(w, http.StatusTooManyRequests, "api token rate limit exceeded")
				return
			}
		}

		// Record who authenticated so the request-logging middleware can report
		// the actor (it installed the sink on r's context before us).
		recordActor(r.Context(), user.Username)
		ctx := context.WithValue(r.Context(), userContextKey, user)
		if token != nil {
			ctx = WithAPIToken(ctx, token.ID, token.Scopes)
		}
		if kc != nil {
			ctx = WithAPIToken(ctx, -user.ID, capScopes(ctx, db, user.Role, kc.scopes))
		}
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
