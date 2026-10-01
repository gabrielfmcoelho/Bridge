// Package kcadmin manages the Keycloak clients that hold access to SEAD's
// derived APIs (realm "apis"), and reads the two endpoints every such API
// exposes for Bridge: GET /escopos (its scope catalogue) and GET /admin/uso
// (per-client request counts).
//
// One Keycloak client per (consumer, API): clientId "<scope_prefix>-<label>",
// a confidential client with only the service-account flow, the API's scopes
// as optional client scopes and the per-minute rate limit as a hardcoded
// rate_limit_per_minute claim the API reads.
//
// Bridge reaches Keycloak on its internal address (an IP: servers can't
// resolve the public sslip.io name) and sends the public name as the Host
// header, so Keycloak answers as the issuer tokens are checked against.
package kcadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitClaim is the token claim the APIs read the per-minute limit from.
const RateLimitClaim = "rate_limit_per_minute"

// Config is the keycloak_apis integration settings group.
type Config struct {
	BaseURL      string // public root, e.g. http://keycloak.10.0.122.89.sslip.io (the issuer's host)
	InternalURL  string // where Bridge reaches Keycloak; "" = BaseURL
	HostHeader   string // Host sent to InternalURL; "" sends the URL's own
	Realm        string
	ClientID     string // the admin service account (realm-management roles), e.g. kc-bridge-admin
	ClientSecret string
	// UsageClientID/Secret: a separate client with no realm-management roles
	// that holds the "<prefix>:admin.uso" scopes. Its token is the only one
	// Bridge ever sends to an API, so an API never sees the admin token.
	UsageClientID     string
	UsageClientSecret string
}

func trim(u string) string { return strings.TrimRight(strings.TrimSpace(u), "/") }

// Issuer is the iss every token of the realm carries (public URL).
func (c Config) Issuer() string { return trim(c.BaseURL) + "/realms/" + c.Realm }

// internalRealm is the realm root on the internal address.
func (c Config) internalRealm() string {
	base := trim(c.InternalURL)
	if base == "" {
		base = trim(c.BaseURL)
	}
	return base + "/realms/" + c.Realm
}

// JWKSURL is where the realm's signing keys are fetched (internal address).
func (c Config) JWKSURL() string { return c.internalRealm() + "/protocol/openid-connect/certs" }

func (c Config) tokenURL() string { return c.internalRealm() + "/protocol/openid-connect/token" }

func (c Config) adminURL() string {
	base := trim(c.InternalURL)
	if base == "" {
		base = trim(c.BaseURL)
	}
	return base + "/admin/realms/" + c.Realm
}

// hostTransport sends every request with a fixed Host header.
type hostTransport struct {
	host string
	base http.RoundTripper
}

func (t hostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Host = t.host
	return t.base.RoundTrip(r)
}

// HTTPClient returns a client that sends host as the Host header (plain
// client when host is empty).
func HTTPClient(host string) *http.Client {
	c := &http.Client{Timeout: 10 * time.Second}
	if host = strings.TrimSpace(host); host != "" {
		c.Transport = hostTransport{host: host, base: http.DefaultTransport}
	}
	return c
}

// Error is a non-2xx answer from Keycloak or an API.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("keycloak: %d %s", e.Status, e.Message) }

// Client is the Keycloak admin client, authenticated as cfg's service account.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	toks map[string]cachedToken // by client id + "|" + requested scope
}

type cachedToken struct {
	value string
	until time.Time
}

// New returns an admin client for cfg.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: HTTPClient(cfg.HostHeader), toks: map[string]cachedToken{}}
}

// Configured reports whether the settings needed for admin calls are set.
func (c Config) Configured() bool {
	return trim(c.BaseURL) != "" && c.Realm != "" && c.ClientID != "" && c.ClientSecret != ""
}

// Reserved reports whether clientID is one of the integration's own clients
// (admin or usage): never imported as a key, never rotated or revoked
// through Atlas.
func (c Config) Reserved(clientID string) bool {
	return clientID != "" && (clientID == c.ClientID || clientID == c.UsageClientID)
}

// ErrUsageNotConfigured: the usage client (kc_apis_usage_client_*) isn't set.
var ErrUsageNotConfigured = errors.New("keycloak: the usage client (kc_apis_usage_client_id/secret) is not configured")

// token returns a client_credentials access token of clientID, optionally
// asking for scope, cached until 30s before it expires.
func (c *Client) token(ctx context.Context, clientID, secret, scope string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := clientID + "|" + scope
	if t, ok := c.toks[key]; ok && time.Now().Before(t.until) {
		return t.value, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}, "client_secret": {secret}}
	if scope != "" {
		form.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if _, err := send(c.http, req, &out); err != nil {
		return "", err
	}
	c.toks[key] = cachedToken{out.AccessToken, time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - 30*time.Second)}
	return out.AccessToken, nil
}

// admin calls the admin REST API at path (relative to the realm).
func (c *Client) admin(ctx context.Context, method, p string, body, out any) (http.Header, error) {
	if !c.cfg.Configured() {
		return nil, fmt.Errorf("keycloak: the keycloak_apis integration is not configured")
	}
	tok, err := c.token(ctx, c.cfg.ClientID, c.cfg.ClientSecret, "")
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.adminURL()+p, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return send(c.http, req, out)
}

func send(hc *http.Client, req *http.Request, out any) (http.Header, error) {
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.Header, &Error{Status: resp.StatusCode, Message: errorMessage(raw)}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.Header, fmt.Errorf("keycloak: unexpected response: %w", err)
		}
	}
	return resp.Header, nil
}

// errorMessage reads Keycloak's {"errorMessage"} / {"error","error_description"}
// and the APIs' {"detail"} / {"error":{"code","message"}}, else the raw body.
func errorMessage(raw []byte) string {
	var e map[string]any
	if json.Unmarshal(raw, &e) == nil {
		for _, k := range []string{"errorMessage", "error_description", "detail", "message"} {
			if s, ok := e[k].(string); ok && s != "" {
				return s
			}
		}
		switch v := e["error"].(type) {
		case string:
			return v
		case map[string]any:
			if s, ok := v["message"].(string); ok {
				return s
			}
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// ProtocolMapper is a client's protocol mapper.
type ProtocolMapper struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	ProtocolMapper string            `json:"protocolMapper"`
	Config         map[string]string `json:"config"`
}

// ClientInfo is the part of a Keycloak client representation Bridge reads.
type ClientInfo struct {
	ID                   string           `json:"id"`
	ClientID             string           `json:"clientId"`
	Description          string           `json:"description"`
	Enabled              bool             `json:"enabled"`
	OptionalClientScopes []string         `json:"optionalClientScopes"`
	ProtocolMappers      []ProtocolMapper `json:"protocolMappers"`
}

// RateLimit is the client's rate_limit_per_minute claim, nil when unset.
func (ci ClientInfo) RateLimit() *int {
	for _, m := range ci.ProtocolMappers {
		if m.Name == RateLimitClaim {
			if n, err := strconv.Atoi(m.Config["claim.value"]); err == nil {
				return &n
			}
		}
	}
	return nil
}

// Scopes are the client's API scopes: its optional client scopes that carry
// a "<prefix>:" (the realm's built-ins such as offline_access don't).
func (ci ClientInfo) Scopes() []string {
	out := []string{}
	for _, s := range ci.OptionalClientScopes {
		if strings.Contains(s, ":") {
			out = append(out, s)
		}
	}
	return out
}

// ListClients returns the realm's clients whose clientId starts with
// "<prefix>-", minus the integration's own (Reserved) clients.
func (c *Client) ListClients(ctx context.Context, prefix string) ([]ClientInfo, error) {
	var all []ClientInfo
	q := url.Values{"clientId": {prefix + "-"}, "search": {"true"}, "max": {"1000"}}
	if _, err := c.admin(ctx, http.MethodGet, "/clients?"+q.Encode(), nil, &all); err != nil {
		return nil, err
	}
	out := []ClientInfo{}
	for _, ci := range all {
		if strings.HasPrefix(ci.ClientID, prefix+"-") && !c.cfg.Reserved(ci.ClientID) {
			out = append(out, ci)
		}
	}
	return out, nil
}

// GetClient returns one client by its internal id.
func (c *Client) GetClient(ctx context.Context, id string) (*ClientInfo, error) {
	var ci ClientInfo
	if _, err := c.admin(ctx, http.MethodGet, "/clients/"+url.PathEscape(id), nil, &ci); err != nil {
		return nil, err
	}
	return &ci, nil
}

// FindClient returns the client with exactly clientID, or nil when none.
func (c *Client) FindClient(ctx context.Context, clientID string) (*ClientInfo, error) {
	var list []ClientInfo
	if _, err := c.admin(ctx, http.MethodGet, "/clients?clientId="+url.QueryEscape(clientID), nil, &list); err != nil {
		return nil, err
	}
	for _, ci := range list {
		if ci.ClientID == clientID {
			return &ci, nil
		}
	}
	return nil, nil
}

// DeleteClient removes a client for good.
func (c *Client) DeleteClient(ctx context.Context, id string) error {
	_, err := c.admin(ctx, http.MethodDelete, "/clients/"+url.PathEscape(id), nil, nil)
	return err
}

func rateMapper(rate int) ProtocolMapper {
	return ProtocolMapper{
		Name: RateLimitClaim, Protocol: "openid-connect", ProtocolMapper: "oidc-hardcoded-claim-mapper",
		Config: map[string]string{
			"claim.name": RateLimitClaim, "claim.value": strconv.Itoa(rate), "jsonType.label": "int",
			"access.token.claim": "true", "id.token.claim": "false", "userinfo.token.claim": "false",
		},
	}
}

// CreateClient creates a confidential, service-account-only client with
// scopes as optional client scopes and, when rate > 0, the rate-limit claim.
// It returns the client's internal id and secret. On a later failure the
// half-made client is deleted again.
func (c *Client) CreateClient(ctx context.Context, clientID, description string, scopes []string, rate int) (id, secret string, err error) {
	rep := map[string]any{
		"clientId": clientID, "description": description, "enabled": true, "protocol": "openid-connect",
		"publicClient": false, "clientAuthenticatorType": "client-secret", "serviceAccountsEnabled": true,
		"standardFlowEnabled": false, "directAccessGrantsEnabled": false, "implicitFlowEnabled": false,
	}
	if rate > 0 {
		rep["protocolMappers"] = []ProtocolMapper{rateMapper(rate)}
	}
	hdr, err := c.admin(ctx, http.MethodPost, "/clients", rep, nil)
	if err != nil {
		return "", "", err
	}
	id = path.Base(hdr.Get("Location"))
	if id == "" || id == "." || id == "/" {
		return "", "", fmt.Errorf("keycloak: client created without a Location")
	}
	created := id // the named results are blanked by a failing return
	defer func() {
		if err != nil {
			_ = c.DeleteClient(context.WithoutCancel(ctx), created)
		}
	}()
	if err = c.SetScopes(ctx, id, scopes); err != nil {
		return "", "", err
	}
	var cred struct {
		Value string `json:"value"`
	}
	if _, err = c.admin(ctx, http.MethodGet, "/clients/"+url.PathEscape(id)+"/client-secret", nil, &cred); err != nil {
		return "", "", err
	}
	return id, cred.Value, nil
}

// scopeIDs maps the realm's client-scope names to ids.
func (c *Client) scopeIDs(ctx context.Context) (map[string]string, error) {
	var list []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if _, err := c.admin(ctx, http.MethodGet, "/client-scopes", nil, &list); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list))
	for _, s := range list {
		out[s.Name] = s.ID
	}
	return out, nil
}

// SetScopes makes scopes the client's API scopes: missing ones are added as
// optional client scopes, API scopes not in the list removed. A scope the
// realm doesn't have is an error (run EnsureClientScopes first).
//
// ponytail: "API scope" = a name with ":"; the realm's built-ins have none.
func (c *Client) SetScopes(ctx context.Context, id string, scopes []string) error {
	ci, err := c.GetClient(ctx, id)
	if err != nil {
		return err
	}
	ids, err := c.scopeIDs(ctx)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, s := range scopes {
		if ids[s] == "" {
			return &Error{Status: http.StatusBadRequest, Message: fmt.Sprintf("client scope %q does not exist in the realm (sync the scopes first)", s)}
		}
		want[s] = true
	}
	have := map[string]bool{}
	for _, s := range ci.Scopes() {
		have[s] = true
		if !want[s] && ids[s] != "" {
			if _, err := c.admin(ctx, http.MethodDelete, "/clients/"+url.PathEscape(id)+"/optional-client-scopes/"+ids[s], nil, nil); err != nil {
				return err
			}
		}
	}
	for s := range want {
		if !have[s] {
			if _, err := c.admin(ctx, http.MethodPut, "/clients/"+url.PathEscape(id)+"/optional-client-scopes/"+ids[s], nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetRateLimit sets the client's rate-limit claim; rate <= 0 removes it (the
// API's default applies).
func (c *Client) SetRateLimit(ctx context.Context, id string, rate int) error {
	base := "/clients/" + url.PathEscape(id) + "/protocol-mappers/models"
	var mappers []ProtocolMapper
	if _, err := c.admin(ctx, http.MethodGet, base, nil, &mappers); err != nil {
		return err
	}
	for _, m := range mappers {
		if m.Name != RateLimitClaim {
			continue
		}
		if rate <= 0 {
			_, err := c.admin(ctx, http.MethodDelete, base+"/"+url.PathEscape(m.ID), nil, nil)
			return err
		}
		upd := rateMapper(rate)
		upd.ID = m.ID
		_, err := c.admin(ctx, http.MethodPut, base+"/"+url.PathEscape(m.ID), upd, nil)
		return err
	}
	if rate <= 0 {
		return nil
	}
	_, err := c.admin(ctx, http.MethodPost, base, rateMapper(rate), nil)
	return err
}

// RegenerateSecret issues a new client secret; the old one stops working.
func (c *Client) RegenerateSecret(ctx context.Context, id string) (string, error) {
	var cred struct {
		Value string `json:"value"`
	}
	if _, err := c.admin(ctx, http.MethodPost, "/clients/"+url.PathEscape(id)+"/client-secret", nil, &cred); err != nil {
		return "", err
	}
	return cred.Value, nil
}

// SetEnabled turns a client on or off (off = revoked: no new tokens).
func (c *Client) SetEnabled(ctx context.Context, id string, enabled bool) error {
	// Round-trip the whole representation so nothing else is reset.
	var rep map[string]any
	p := "/clients/" + url.PathEscape(id)
	if _, err := c.admin(ctx, http.MethodGet, p, nil, &rep); err != nil {
		return err
	}
	rep["enabled"] = enabled
	_, err := c.admin(ctx, http.MethodPut, p, rep, nil)
	return err
}

// ScopeInfo is one entry of an API's scope catalogue (GET /escopos); names
// already carry the API's "<prefix>:".
type ScopeInfo struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // "route" or "modifier"
	Description string   `json:"description"`
	Routes      []string `json:"routes"`
}

// EnsureClientScopes creates, as OIDC client scopes included in the token's
// scope claim, every catalogue entry the realm doesn't have yet. Existing
// ones are left as they are. It returns how many were created.
func (c *Client) EnsureClientScopes(ctx context.Context, scopes []ScopeInfo) (int, error) {
	ids, err := c.scopeIDs(ctx)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, s := range scopes {
		if s.Name == "" || ids[s.Name] != "" {
			continue
		}
		desc := s.Description
		if r := []rune(desc); len(r) > 255 {
			desc = string(r[:255])
		}
		rep := map[string]any{
			"name": s.Name, "description": desc, "protocol": "openid-connect",
			"attributes": map[string]string{"include.in.token.scope": "true", "display.on.consent.screen": "false"},
		}
		if _, err := c.admin(ctx, http.MethodPost, "/client-scopes", rep, nil); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// Catalogue reads an API's public scope catalogue at baseURL/escopos.
func Catalogue(ctx context.Context, baseURL string) ([]ScopeInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trim(baseURL)+"/escopos", nil)
	if err != nil {
		return nil, err
	}
	var out []ScopeInfo
	_, err = send(HTTPClient(""), req, &out)
	return out, err
}

// Usage is GET /admin/uso?cliente=…: daily is "YYYY-MM-DD" → requests.
type Usage struct {
	Cliente    string           `json:"cliente"`
	Lifetime   int64            `json:"lifetime"`
	LastUsedAt *string          `json:"last_used_at"`
	Daily      map[string]int64 `json:"daily"`
}

// Usage reads one client's request counts from the API at baseURL, with a
// token of the usage client (never the admin one) asking for
// "<prefix>:admin.uso".
func (c *Client) Usage(ctx context.Context, baseURL, prefix, clientID string) (*Usage, error) {
	if c.cfg.UsageClientID == "" || c.cfg.UsageClientSecret == "" {
		return nil, ErrUsageNotConfigured
	}
	tok, err := c.token(ctx, c.cfg.UsageClientID, c.cfg.UsageClientSecret, prefix+":admin.uso")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trim(baseURL)+"/admin/uso?cliente="+url.QueryEscape(clientID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	var out Usage
	if _, err := send(HTTPClient(""), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Time parses an ISO-8601 timestamp from an API (with or without offset;
// naive ones are UTC). nil for nil, empty or unparsable.
func Time(s *string) *time.Time {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999"} {
		if t, err := time.Parse(layout, strings.TrimSpace(*s)); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

// Setting keys of the keycloak_apis integration group.
const (
	SettingBaseURL      = "kc_apis_base_url"
	SettingInternalURL  = "kc_apis_internal_url"
	SettingHostHeader   = "kc_apis_host_header"
	SettingRealm        = "kc_apis_realm"
	SettingClientID     = "kc_apis_client_id"
	SettingClientSecret = "kc_apis_client_secret"
	SettingUsageID      = "kc_apis_usage_client_id"
	SettingUsageSecret  = "kc_apis_usage_client_secret"
)

// FromSettings builds a Config from the settings group; get answers a key's
// value ("" when unset, the decrypted value for the secret). Realm defaults
// to "apis".
func FromSettings(get func(key string) string) Config {
	c := Config{
		BaseURL:           strings.TrimSpace(get(SettingBaseURL)),
		InternalURL:       strings.TrimSpace(get(SettingInternalURL)),
		HostHeader:        strings.TrimSpace(get(SettingHostHeader)),
		Realm:             strings.TrimSpace(get(SettingRealm)),
		ClientID:          strings.TrimSpace(get(SettingClientID)),
		ClientSecret:      get(SettingClientSecret),
		UsageClientID:     strings.TrimSpace(get(SettingUsageID)),
		UsageClientSecret: get(SettingUsageSecret),
	}
	if c.Realm == "" {
		c.Realm = "apis"
	}
	return c
}
