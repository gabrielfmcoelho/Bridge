package api

import (
	"context"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/signoz"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// bundleUsageTimeout bounds the SigNoz queries a public redeem makes.
const bundleUsageTimeout = 8 * time.Second

// shareKeyUsage is the last 30 days of one shared key's requests to its API —
// only that key's own traffic (its Keycloak client), never the API's whole.
type shareKeyUsage struct {
	Secret  string `json:"secret"` // the shared secret's name, as the page lists it
	APIName string `json:"api_name"`
	signoz.Result
}

// sharePayload is a redeemed bundle plus the shared keys' usage.
type sharePayload struct {
	*vault.BundlePayload
	Usage []shareKeyUsage `json:"usage,omitempty"`
}

// keyUsage reads SigNoz for every shared Keycloak key. Best-effort: SigNoz
// off, a key without client id or an API without gateway path, or a failed
// query just leaves that key out; the redeem never fails on it.
func (h *publicBundleHandlers) keyUsage(ctx context.Context, p *vault.BundlePayload) []shareKeyUsage {
	s, err := signoz.LoadSettings(h.db.SQL, h.db.Encryptor)
	client := signoz.NewServiceClient(s)
	if err != nil || client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, bundleUsageTimeout)
	defer cancel()
	var out []shareKeyUsage
	for _, sec := range p.Secrets {
		k := sec.Key
		if k == nil || k.ClientID == "" || gatewayPath(k.APIBaseURL) == "" {
			continue
		}
		res, err := client.Requests(ctx, signoz.Filter{PathPrefix: gatewayPath(k.APIBaseURL), Clients: []string{k.ClientID}}, "30d", "route")
		if err != nil {
			continue
		}
		out = append(out, shareKeyUsage{Secret: sec.Name, APIName: k.APIName, Result: *res})
	}
	return out
}
