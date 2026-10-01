package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// signJWT makes an RS256 token with key (kid "k1").
func signJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	head := enc(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(head))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return head + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestRequireAuth_KeycloakAPIsToken(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// JWKS served on the "internal" address; it must be asked for with the
	// public Host.
	var mu sync.Mutex
	var hosts []string
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hosts = append(hosts, r.Host)
		mu.Unlock()
		if r.URL.Path != "/realms/apis/protocol/openid-connect/certs" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwks.Close)

	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{
		"kc_apis_base_url": "http://keycloak.example", "kc_apis_internal_url": jwks.URL,
		"kc_apis_host_header": "keycloak.example", "kc_apis_realm": "apis",
	} {
		if err := settings.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	ResetAPIsVerifier()
	t.Cleanup(ResetAPIsVerifier)

	// A viewer service account behind client bridge-painel.
	u := &models.User{Username: "bridge-painel", Role: "viewer", Kind: models.UserKindService, AuthProvider: KeycloakAPIsProvider}
	if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := store.NewUserIdentityRepo(d.SQL).Create(ctx, &models.UserExternalIdentity{UserID: u.ID, ProviderName: KeycloakAPIsProvider, ExternalID: "bridge-painel"}); err != nil {
		t.Fatal(err)
	}

	var seenID int64
	var seenScopes []string
	h := RequireAuth(d.SQL, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenID = APITokenFromContext(r.Context())
		seenScopes, _ = TokenScopesFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	call := func(tok string) int {
		req := httptest.NewRequest("GET", "/api/hosts", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	claims := func(over map[string]any) map[string]any {
		c := map[string]any{"iss": "http://keycloak.example/realms/apis", "azp": "bridge-painel", "aud": "account",
			"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(),
			"scope": "profile bridge:hosts:read bridge:hosts:write servidores:cadastro", "rate_limit_per_minute": 2}
		for k, v := range over {
			c[k] = v
		}
		return c
	}

	if code := call(signJWT(t, key, claims(nil))); code != http.StatusNoContent {
		t.Fatalf("valid token = %d", code)
	}
	if seenID != -u.ID {
		t.Errorf("token id = %d, want %d (synthetic, negative)", seenID, -u.ID)
	}
	// hosts:write needs editor: capped away; servidores:… isn't Bridge's.
	if !slices.Equal(seenScopes, []string{"hosts:read"}) {
		t.Errorf("scopes = %v, want [hosts:read]", seenScopes)
	}
	mu.Lock()
	for _, host := range hosts {
		if host != "keycloak.example" {
			t.Errorf("JWKS fetched with Host %q", host)
		}
	}
	mu.Unlock()

	// Rate limit from the claim: 2 per minute, this is the 2nd and 3rd call.
	if code := call(signJWT(t, key, claims(nil))); code != http.StatusNoContent {
		t.Fatalf("2nd call = %d", code)
	}
	if code := call(signJWT(t, key, claims(nil))); code != http.StatusTooManyRequests {
		t.Errorf("3rd call = %d, want 429", code)
	}
	// The synthetic id is never queued for api_token_usage (no such row).
	meter.mu.Lock()
	for id := range meter.pending {
		if id <= 0 {
			t.Errorf("negative id %d queued for usage", id)
		}
	}
	meter.mu.Unlock()

	for name, tok := range map[string]string{
		"wrong issuer": signJWT(t, key, claims(map[string]any{"iss": "http://evil.example/realms/apis"})),
		"unknown azp":  signJWT(t, key, claims(map[string]any{"azp": "bridge-ghost"})),
		"expired":      signJWT(t, key, claims(map[string]any{"exp": time.Now().Add(-time.Minute).Unix()})),
		"bad sig":      signJWT(t, key, claims(nil))[:40] + strings.Repeat("A", 10) + ".x.y",
	} {
		if code := call(tok); code != http.StatusUnauthorized {
			t.Errorf("%s = %d, want 401", name, code)
		}
	}

	// An identity pointing at a person is never honoured.
	p := &models.User{Username: "maria", Role: "admin", Kind: models.UserKindPerson, AuthProvider: "local"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := store.NewUserIdentityRepo(d.SQL).Create(ctx, &models.UserExternalIdentity{UserID: p.ID, ProviderName: KeycloakAPIsProvider, ExternalID: "bridge-maria"}); err != nil {
		t.Fatal(err)
	}
	if code := call(signJWT(t, key, claims(map[string]any{"azp": "bridge-maria"}))); code != http.StatusUnauthorized {
		t.Errorf("identity of a person user = %d, want 401", code)
	}

	// Revocation drops the client's identity: its still-valid token stops at once.
	if err := store.NewUserIdentityRepo(d.SQL).DeleteByProviderAndExternalID(ctx, KeycloakAPIsProvider, "bridge-painel"); err != nil {
		t.Fatal(err)
	}
	if code := call(signJWT(t, key, claims(nil))); code != http.StatusUnauthorized {
		t.Errorf("unlinked client = %d, want 401", code)
	}

	// Not configured: a JWT is refused, a brg_ token still takes its own path.
	_ = settings.Set(ctx, "kc_apis_base_url", "")
	ResetAPIsVerifier()
	if code := call(signJWT(t, key, claims(nil))); code != http.StatusUnauthorized {
		t.Errorf("unconfigured = %d, want 401", code)
	}
	if code := call("brg_nope"); code != http.StatusUnauthorized {
		t.Errorf("unknown brg_ token = %d, want 401", code)
	}
}
