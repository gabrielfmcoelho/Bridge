package api

import (
	"context"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// bundleScopesTimeout bounds each API's GET /escopos during a public redeem.
const bundleScopesTimeout = 5 * time.Second

// enrichKeys fills, for every shared API key in p, each scope's kind and
// description from its API's catalogue (one GET /escopos per distinct API)
// and the realm's public token URL from the keycloak_apis settings.
// Best-effort: any failure leaves the names alone and never fails the redeem.
func (h *publicBundleHandlers) enrichKeys(ctx context.Context, p *vault.BundlePayload) {
	var tokenURL string
	catalogues := map[int64]map[string]kcadmin.ScopeInfo{}
	for i := range p.Secrets {
		k := p.Secrets[i].Key
		if k == nil {
			continue
		}
		if tokenURL == "" {
			tokenURL = h.tokenURL(ctx)
		}
		k.TokenURL = tokenURL
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

// tokenURL is the realm's public token endpoint (issuer + OIDC path), or ""
// when the keycloak_apis integration has no base URL. Only plain settings
// are read — never the client secrets.
func (h *publicBundleHandlers) tokenURL(ctx context.Context) string {
	settings := store.NewAppSettingsRepo(h.db.SQL)
	cfg := kcadmin.FromSettings(func(key string) string {
		if key == kcadmin.SettingClientSecret || key == kcadmin.SettingUsageSecret {
			return ""
		}
		return settings.Value(ctx, key)
	})
	if cfg.BaseURL == "" {
		return ""
	}
	return cfg.Issuer() + "/protocol/openid-connect/token"
}
