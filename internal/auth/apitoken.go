package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// APITokenPrefix marks Bridge personal API tokens, so a leaked one is
// recognisable (and greppable by secret scanners).
const APITokenPrefix = "brg_"

// apiTokenDisplayLen is how much of the token the UI keeps to tell tokens apart.
const apiTokenDisplayLen = len(APITokenPrefix) + 6

// GenerateAPIToken returns a fresh token (shown to the user once), its
// SHA-256 hash (what the DB stores) and its display prefix.
func GenerateAPIToken() (token string, hash []byte, prefix string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, "", fmt.Errorf("generate api token: %w", err)
	}
	token = APITokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return token, HashAPIToken(token), token[:apiTokenDisplayLen], nil
}

// HashAPIToken is the lookup key for a presented token.
func HashAPIToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// bearerAPIToken returns the Bridge API token from "Authorization: Bearer …",
// or "" when the header is absent or carries something else.
func bearerAPIToken(r *http.Request) string {
	scheme, tok, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	tok = strings.TrimSpace(tok)
	if !strings.HasPrefix(tok, APITokenPrefix) {
		return ""
	}
	return tok
}

const apiTokenContextKey contextKey = "api_token"

// tokenCtx is what a token-authenticated request carries.
type tokenCtx struct {
	id     int64
	scopes []string
}

// APITokenFromContext returns the id of the API token that authenticated the
// request, or 0 when it came in on a browser session.
func APITokenFromContext(ctx context.Context) int64 {
	t, _ := ctx.Value(apiTokenContextKey).(tokenCtx)
	return t.id
}

// TokenScopesFromContext returns the scopes of the token that authenticated
// the request; ok is false for a browser session (no scope limits).
func TokenScopesFromContext(ctx context.Context) (scopes []string, ok bool) {
	t, ok := ctx.Value(apiTokenContextKey).(tokenCtx)
	return t.scopes, ok
}

// WithAPIToken marks ctx as authenticated by API token id holding scopes
// (tests use it too).
func WithAPIToken(ctx context.Context, id int64, scopes []string) context.Context {
	return context.WithValue(ctx, apiTokenContextKey, tokenCtx{id: id, scopes: scopes})
}

// RequireScope rejects token requests whose token lacks scope. Session
// requests pass: scopes only ever narrow what a token can do.
func RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if scopes, isToken := TokenScopesFromContext(r.Context()); isToken && !ScopeAllowed(scopes, scope) {
			auditAuthFailure(r, "token lacks scope "+scope)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"insufficient token scope","required":"` + scope + `"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
