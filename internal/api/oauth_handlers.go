package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type oauthHandlers struct {
	db       *database.DB
	registry *auth.ProviderRegistry
	ah       *authHandlers // for resolveOrProvisionUser
}

// callbackURL builds the full absolute OAuth callback URL.
// Priority: APP_URL env var > X-Forwarded-Host/Proto headers > r.Host.
func callbackURL(r *http.Request, providerName string) string {
	if base := os.Getenv("APP_URL"); base != "" {
		return fmt.Sprintf("%s/api/auth/oauth/%s/callback", strings.TrimRight(base, "/"), providerName)
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s/api/auth/oauth/%s/callback", scheme, host, providerName)
}

// handleAuthorize initiates the OAuth flow by redirecting to the external provider.
// GET /api/auth/oauth/{provider}/authorize
func (h *oauthHandlers) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")

	provider, ok := h.registry.Get(providerName)
	if !ok || !provider.Enabled() || provider.SupportsDirectLogin() {
		http.Redirect(w, r, "/login?auth=error&message=unsupported+provider", http.StatusFound)
		return
	}

	// Create an OAuth state for CSRF protection.
	state, err := models.CreateOAuthState(h.db.SQL, providerName)
	if err != nil {
		http.Redirect(w, r, "/login?auth=error&message=internal+error", http.StatusFound)
		return
	}

	authURL, err := provider.AuthorizationURL(state.State, callbackURL(r, providerName))
	if err != nil {
		http.Redirect(w, r, "/login?auth=error&message=provider+configuration+error", http.StatusFound)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback handles the OAuth callback from the external provider.
// GET /api/auth/oauth/{provider}/callback
func (h *oauthHandlers) handleCallback(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")

	// Check for error from the provider.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		if desc == "" {
			desc = errParam
		}
		http.Redirect(w, r, "/login?auth=error&message="+desc, http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	if code == "" || stateParam == "" {
		http.Redirect(w, r, "/login?auth=error&message=missing+code+or+state", http.StatusFound)
		return
	}

	// Validate the OAuth state (CSRF protection).
	oauthState, err := models.ValidateOAuthState(h.db.SQL, stateParam)
	if err != nil || oauthState == nil {
		http.Redirect(w, r, "/login?auth=error&message=invalid+or+expired+state", http.StatusFound)
		return
	}
	if oauthState.Provider != providerName {
		http.Redirect(w, r, "/login?auth=error&message=state+provider+mismatch", http.StatusFound)
		return
	}

	provider, ok := h.registry.Get(providerName)
	if !ok || !provider.Enabled() {
		http.Redirect(w, r, "/login?auth=error&message=provider+not+available", http.StatusFound)
		return
	}

	// Exchange the authorization code for user identity.
	identity, err := provider.ExchangeCode(r.Context(), code, callbackURL(r, providerName))
	if err != nil {
		http.Redirect(w, r, "/login?auth=error&message=authentication+failed", http.StatusFound)
		return
	}

	// Resolve or auto-provision the local user.
	user, err := h.ah.resolveOrProvisionUser(identity)
	if err != nil {
		http.Redirect(w, r, "/login?auth=error&message=user+provisioning+failed", http.StatusFound)
		return
	}

	// Create a local session.
	token, expiresAt, err := auth.CreateSession(h.db.SQL, user.ID)
	if err != nil {
		http.Redirect(w, r, "/login?auth=error&message=session+creation+failed", http.StatusFound)
		return
	}

	auth.SetSessionCookie(w, token, expiresAt)
	http.Redirect(w, r, "/login?auth=success", http.StatusFound)
}
