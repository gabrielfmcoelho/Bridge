package api

import (
	"context"
	"net/url"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// bundleScopesTimeout bounds each API's GET /escopos during a public redeem.
const bundleScopesTimeout = 5 * time.Second

// enrichKeys fills, for every shared API key in p, each scope's kind and
// description from its API's catalogue (one GET /escopos per distinct API)
// and where to ask for a token (see tokenURL).
// Best-effort: any failure leaves the names alone and never fails the redeem.
func (h *publicBundleHandlers) enrichKeys(ctx context.Context, p *vault.BundlePayload) {
	var cfg *kcadmin.Config
	catalogues := map[int64]map[string]kcadmin.ScopeInfo{}
	for i := range p.Secrets {
		k := p.Secrets[i].Key
		if k == nil {
			continue
		}
		if cfg == nil {
			c := h.kcConfig(ctx)
			cfg = &c
		}
		k.TokenURL = tokenURL(*cfg, k.APIBaseURL)
		byName, seen := catalogues[k.APIID]
		if !seen {
			byName = h.scopeCatalogue(ctx, k.APIID)
			catalogues[k.APIID] = byName
		}
		for j := range k.Scopes {
			if s, ok := byName[k.Scopes[j].Name]; ok {
				k.Scopes[j].Kind, k.Scopes[j].Description = s.Kind, s.Description
			}
		}
	}
}

// scopeCatalogue is the API's scopes by name, or nil when unreadable.
func (h *publicBundleHandlers) scopeCatalogue(ctx context.Context, apiID int64) map[string]kcadmin.ScopeInfo {
	a, err := store.NewAPICatalogRepo(h.db.SQL).Get(ctx, apiID)
	if err != nil || a == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, bundleScopesTimeout)
	defer cancel()
	scopes, err := apiScopes(ctx, a)
	if err != nil {
		return nil
	}
	out := make(map[string]kcadmin.ScopeInfo, len(scopes))
	for _, s := range scopes {
		out[s.Name] = s
	}
	return out
}

// kcConfig reads the keycloak_apis integration's plain settings — never the
// client secrets.
func (h *publicBundleHandlers) kcConfig(ctx context.Context) kcadmin.Config {
	settings := store.NewAppSettingsRepo(h.db.SQL)
	return kcadmin.FromSettings(func(key string) string {
		if key == kcadmin.SettingClientSecret || key == kcadmin.SettingUsageSecret {
			return ""
		}
		return settings.Value(ctx, key)
	})
}

// tokenURL is where the key's consumer asks for a token: the gateway in front
// of its API (same origin as the API's base URL), which exposes the realm's
// token endpoint publicly. Falls back to the configured issuer — Bridge's own
// address for Keycloak, often internal — when the API has no absolute base
// URL; "" when neither is known.
func tokenURL(cfg kcadmin.Config, apiBaseURL string) string {
	if u, err := url.Parse(apiBaseURL); err == nil && u.Scheme != "" && u.Host != "" && cfg.Realm != "" {
		return u.Scheme + "://" + u.Host + "/realms/" + cfg.Realm + "/protocol/openid-connect/token"
	}
	if cfg.BaseURL == "" {
		return ""
	}
	return cfg.Issuer() + "/protocol/openid-connect/token"
}
