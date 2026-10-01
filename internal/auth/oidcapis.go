package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Bridge's own API also accepts access tokens of the Keycloak realm that
// governs SEAD's APIs (the keycloak_apis settings group). A token's azp — the
// Keycloak client — maps to a service-account user through an external
// identity with provider KeycloakAPIsProvider; its "bridge:"-prefixed scopes
// become the request's token scopes, capped by that user's role. brg_ tokens
// stay as they are: they are the manual/master path.

// KeycloakAPIsProvider is the user_external_identities provider that links a
// Keycloak client (external_id = clientId) to its service-account user.
const KeycloakAPIsProvider = "keycloak-apis"

// BridgeScopePrefix prefixes Bridge's scopes in Keycloak ("bridge:hosts:read").
const BridgeScopePrefix = "bridge"

var errAPIsNotConfigured = errors.New("keycloak apis integration not configured")

// apisVerifier caches the token verifier built from the settings; nil v with
// loaded set means "not configured".
var apisVerifier struct {
	sync.Mutex
	v      *oidc.IDTokenVerifier
	loaded bool
}

// ResetAPIsVerifier drops the cached verifier so the next token rebuilds it
// from the keycloak_apis settings (called when they change).
func ResetAPIsVerifier() {
	apisVerifier.Lock()
	defer apisVerifier.Unlock()
	apisVerifier.v, apisVerifier.loaded = nil, false
}

func loadAPIsVerifier(ctx context.Context, db *sql.DB) *oidc.IDTokenVerifier {
	apisVerifier.Lock()
	defer apisVerifier.Unlock()
	if apisVerifier.loaded {
		return apisVerifier.v
	}
	settings := store.NewAppSettingsRepo(db)
	cfg := kcadmin.FromSettings(func(k string) string {
		if k == kcadmin.SettingClientSecret {
			return "" // not needed to verify
		}
		return settings.Value(ctx, k)
	})
	apisVerifier.loaded = true
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil
	}
	// The issuer is the public URL; keys come from the internal address with
	// the public Host. The key set keeps this context for later refreshes.
	keyCtx := oidc.ClientContext(context.Background(), kcadmin.HTTPClient(cfg.HostHeader))
	ks := oidc.NewRemoteKeySet(keyCtx, cfg.JWKSURL())
	apisVerifier.v = oidc.NewVerifier(cfg.Issuer(), ks, &oidc.Config{SkipClientIDCheck: true})
	return apisVerifier.v
}

// apisToken is what a verified Keycloak token resolves to.
type apisToken struct {
	userID    int64
	scopes    []string // "bridge:" stripped, not yet capped by role
	rateLimit int
}

// authenticateAPIsToken verifies raw (signature, issuer, expiry), maps its
// azp to a service-account user and reads its Bridge scopes and rate limit.
func authenticateAPIsToken(ctx context.Context, db *sql.DB, raw string) (apisToken, error) {
	v := loadAPIsVerifier(ctx, db)
	if v == nil {
		return apisToken{}, errAPIsNotConfigured
	}
	tok, err := v.Verify(ctx, raw)
	if err != nil {
		return apisToken{}, err
	}
	var claims struct {
		Azp       string `json:"azp"`
		Scope     string `json:"scope"`
		RateLimit *int   `json:"rate_limit_per_minute"`
	}
	if err := tok.Claims(&claims); err != nil {
		return apisToken{}, err
	}
	if claims.Azp == "" {
		return apisToken{}, errors.New("token has no azp")
	}
	ident, err := store.NewUserIdentityRepo(db).GetByProviderAndExternalID(ctx, KeycloakAPIsProvider, claims.Azp)
	if err != nil {
		return apisToken{}, err
	}
	if ident == nil {
		return apisToken{}, errors.New("unknown keycloak client " + claims.Azp)
	}
	out := apisToken{userID: ident.UserID, scopes: []string{}, rateLimit: DefaultTokenRateLimit}
	for _, s := range strings.Fields(claims.Scope) {
		if rest, ok := strings.CutPrefix(s, BridgeScopePrefix+":"); ok {
			out.scopes = append(out.scopes, rest)
		}
	}
	if claims.RateLimit != nil {
		out.rateLimit = *claims.RateLimit
	}
	return out, nil
}

// capScopes keeps the scopes a user with role (and its permissions) can use.
func capScopes(ctx context.Context, db *sql.DB, role string, scopes []string) []string {
	perms := store.NewPermissionRepo(db)
	hasPerm := func(code string) bool { return perms.Has(ctx, role, code) }
	out := []string{}
	for _, s := range scopes {
		if ScopeUsable(role, hasPerm, s) {
			out = append(out, s)
		}
	}
	return out
}
