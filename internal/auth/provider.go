package auth

import "context"

// ExternalIdentity represents user info returned by an external auth provider.
type ExternalIdentity struct {
	ProviderName string
	ExternalID   string
	Username     string
	DisplayName  string
	Email        string
	Groups       []string
	RawClaims    map[string]string
}

// ProviderDisplayInfo holds UI metadata for the login page.
type ProviderDisplayInfo struct {
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// AuthProvider is the interface all authentication backends implement.
type AuthProvider interface {
	// Name returns the unique provider identifier ("local", "ldap", "keycloak", "gitlab").
	Name() string

	// Enabled returns whether this provider is currently configured and active.
	Enabled() bool

	// SupportsDirectLogin returns true if this provider handles username+password
	// (local, LDAP). Returns false for redirect-based flows (OAuth/OIDC).
	SupportsDirectLogin() bool

	// Authenticate validates username+password and returns an ExternalIdentity.
	// Only called if SupportsDirectLogin() is true.
	Authenticate(ctx context.Context, username, password string) (*ExternalIdentity, error)

	// AuthorizationURL returns the OAuth redirect URL for browser-based flows.
	// Only meaningful for OAuth/OIDC providers.
	// callbackURL is the full absolute callback URL (e.g. "http://host/api/auth/oauth/gitlab/callback").
	AuthorizationURL(state, callbackURL string) (string, error)

	// ExchangeCode exchanges an authorization code for an ExternalIdentity.
	// Only meaningful for OAuth/OIDC providers.
	// callbackURL must match the one used in AuthorizationURL.
	ExchangeCode(ctx context.Context, code, callbackURL string) (*ExternalIdentity, error)

	// DisplayInfo returns UI metadata (button label, icon, color) for the login page.
	DisplayInfo() ProviderDisplayInfo
}
