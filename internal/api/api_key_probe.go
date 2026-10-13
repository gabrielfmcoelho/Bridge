package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Key test: Bridge asks Keycloak for a token of the key's client (secret read
// from Keycloak, token never stored nor returned), checks what the token
// carries, and calls the API through its base URL (normally the gateway).
//
// The default call goes to a path no API has: the gateway checks the token
// before routing, so a 404 from the API proves the gateway took it and no
// query runs. A real route (single key only) also exercises the API's own
// checks: the route's scope and clock drift.

// probePath is the default test path: it must not exist on any API.
const probePath = "/__bridge-teste-token"

// keyTestRequest optionally names a real route (GET, relative to the API's
// base URL) to call instead of the default path.
type keyTestRequest struct {
	Path string `json:"path" example:"/api/cadastro/ugs"`
}

// keyTestResult is one key's verdict. Body is the start of an error answer
// (never of a 2xx one, which may carry personal data).
type keyTestResult struct {
	KeyID      int64    `json:"key_id"`
	Label      string   `json:"label"`
	ClientID   string   `json:"client_id"`
	OK         bool     `json:"ok"`
	TokenOK    bool     `json:"token_ok"`
	TokenError string   `json:"token_error,omitempty"`
	Granted    []string `json:"granted"`
	Missing    []string `json:"missing"` // in Bridge's list, absent from the token
	RateLimit  *int     `json:"rate_limit_per_minute,omitempty"`
	ExpiresIn  int      `json:"expires_in,omitempty"` // seconds
	URL        string   `json:"url"`
	Status     int      `json:"status,omitempty"`
	AnsweredBy string   `json:"answered_by,omitempty"` // "gateway" or "api"
	Body       string   `json:"body,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type keyTestBatchResponse struct {
	Results []keyTestResult `json:"results"`
	Total   int             `json:"total"`
	OK      int             `json:"ok"`
}

// handleTestKey godoc
//
//	@Summary		Test a Keycloak key through the gateway
//	@Description	apis.keys.manage. Bridge reads the client's secret from Keycloak, asks for a token with the key's scopes and calls the API's base URL (normally the gateway) with it. The token and the secret are never returned. Default: a path no API has, so a 404 from the API proves the gateway took the token and nothing runs; "path" calls a real GET route instead (relative to the base URL), which also tests the route's scope. "missing" lists scopes Bridge has on the key but the token lacks; "answered_by" tells the gateway's refusal from the API's. Always 200 with the verdict in "ok".
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"API catalog ID"
//	@Param			keyId	path		int				true	"Key ID"
//	@Param			body	body		keyTestRequest	false	"Optional real route"
//	@Success		200		{object}	keyTestResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/{keyId}/test [post]
func (h *apiKeyHandlers) handleTestKey(w http.ResponseWriter, r *http.Request) {
	a, k, ok := h.loadKey(w, r)
	if !ok {
		return
	}
	var req keyTestRequest
	if r.ContentLength > 0 && !decodeBody(w, r, &req) {
		return
	}
	if k.Source != models.APIKeySourceKeycloak || k.ExternalLabel == nil {
		jsonError(w, http.StatusBadRequest, "only Keycloak keys can be tested")
		return
	}
	if h.reservedKey(w, r, k) {
		return
	}
	target, err := probeURL(a, req.Path)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	kc, ok := h.kcClient(w, r, a)
	if !ok {
		return
	}
	jsonOK(w, probeKey(r.Context(), kc, k, target))
}

// handleTestKeys godoc
//
//	@Summary		Test every key of an API through the gateway
//	@Description	apis.keys.manage. The default test (token + gateway, no real route: a batch must not fire one query per key) on every active or in-grace Keycloak key of the API, 4 at a time.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"API catalog ID"
//	@Success		200	{object}	keyTestBatchResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/api-catalog/{id}/keys/test [post]
func (h *apiKeyHandlers) handleTestKeys(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadAPI(w, r)
	if !ok {
		return
	}
	target, err := probeURL(a, "")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	kc, ok := h.kcClient(w, r, a)
	if !ok {
		return
	}
	cfg, err := h.kcConfig(r.Context())
	if err != nil {
		jsonServerError(w, r, "load keycloak settings", err)
		return
	}
	keys, err := store.NewAPIKeyRepo(h.db.SQL).List(r.Context(), a.ID, false)
	if err != nil {
		jsonServerError(w, r, "list api keys", err)
		return
	}
	live := []models.APIKey{}
	for _, k := range keys {
		if k.Source == models.APIKeySourceKeycloak && k.ExternalLabel != nil && !cfg.Reserved(*k.ExternalLabel) &&
			(k.Status == models.APIKeyStatusActive || k.Status == models.APIKeyStatusGrace) {
			live = append(live, k)
		}
	}
	out := keyTestBatchResponse{Results: make([]keyTestResult, len(live)), Total: len(live)}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := range live {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			out.Results[i] = probeKey(r.Context(), kc, &live[i], target)
		}()
	}
	wg.Wait()
	for _, res := range out.Results {
		if res.OK {
			out.OK++
		}
	}
	jsonOK(w, out)
}

// probeURL joins path (default probePath) to the API's base URL, refusing
// anything that could leave that host: the token must only go there.
func probeURL(a *models.APICatalog, path string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(a.AdminBaseURL), "/")
	}
	b, err := url.Parse(base)
	if err != nil || b.Host == "" || (b.Scheme != "http" && b.Scheme != "https") {
		return "", errors.New("set this API's base URL (normally its gateway address) first")
	}
	if path = strings.TrimSpace(path); path == "" {
		path = probePath
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "@\\") {
		return "", errors.New("path must be relative to the API's base URL and start with /")
	}
	u, err := url.Parse(base + path)
	if err != nil || u.Host != b.Host {
		return "", errors.New("invalid path")
	}
	return u.String(), nil
}

// probeKey runs the test of key k against target.
func probeKey(ctx context.Context, kc *kcadmin.Client, k *models.APIKey, target string) keyTestResult {
	res := keyTestResult{KeyID: k.ID, Label: k.Label, ClientID: *k.ExternalLabel, URL: target, Granted: []string{}, Missing: []string{}}
	ci, err := kc.FindClient(ctx, *k.ExternalLabel)
	if err == nil && ci == nil {
		err = errors.New("client not found in Keycloak")
	}
	var tok string
	if err == nil {
		var secret string
		if secret, err = kc.ClientSecret(ctx, ci.ID); err == nil {
			tok, err = kc.IssueToken(ctx, ci.ClientID, secret, strings.Join(k.Scopes, " "))
		}
	}
	if err != nil {
		res.TokenError = err.Error()
		return res
	}
	res.TokenOK = true

	claims, _ := jwtClaims(tok)
	scope, _ := claims["scope"].(string)
	for _, s := range strings.Fields(scope) {
		if strings.Contains(s, ":") { // API scopes only, not profile/email
			res.Granted = append(res.Granted, s)
		}
	}
	for _, s := range k.Scopes {
		if !slices.Contains(res.Granted, s) {
			res.Missing = append(res.Missing, s)
		}
	}
	if n, ok := claims[kcadmin.RateLimitClaim].(float64); ok {
		v := int(n)
		res.RateLimit = &v
	}
	if exp, ok := claims["exp"].(float64); ok {
		res.ExpiresIn = int(time.Until(time.Unix(int64(exp), 0)).Seconds())
	}

	res.Status, res.AnsweredBy, res.Body, err = callWithToken(ctx, target, tok)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.OK = len(res.Missing) == 0 && res.Status != http.StatusUnauthorized && res.Status != http.StatusForbidden && res.Status < 500
	return res
}

// probeHTTP never follows redirects: the token must not leave the API's host.
var probeHTTP = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}}

// callWithToken GETs target with the token and says who answered: the
// gateway (APISIX's own 401 with a realm, or its route-not-found) or the API.
func callWithToken(ctx context.Context, target, tok string) (status int, by, body string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := probeHTTP.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	text := string(raw)
	by = "api"
	if strings.Contains(resp.Header.Get("WWW-Authenticate"), "realm=") || strings.Contains(text, `"error_msg"`) {
		by = "gateway"
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if body = strings.TrimSpace(text); len(body) > 300 {
			body = body[:300] + "…"
		}
	}
	return resp.StatusCode, by, body, nil
}

// jwtClaims decodes a JWT's payload without checking its signature: Bridge
// only reads what Keycloak just handed it.
func jwtClaims(tok string) (map[string]any, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return nil, false
	}
	return claims, true
}
