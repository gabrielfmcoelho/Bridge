package auth

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

// RequireAuth is middleware that rejects unauthenticated requests with 401.
func RequireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetSessionToken(r)
		if token == "" {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		userID, err := ValidateSession(db, token)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired session"}`, http.StatusUnauthorized)
			return
		}

		user, err := models.GetUserByID(db, userID)
		if err != nil || user == nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole is middleware that rejects requests from users without the required role.
// Role hierarchy: admin > editor > viewer.
func RequireRole(minRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		if !hasMinRole(user.Role, minRole) {
			http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
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
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		if !models.HasPermission(db, user.Role, permission) {
			http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
