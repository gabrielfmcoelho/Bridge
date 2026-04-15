package providers

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"golang.org/x/oauth2"
)

// KeycloakProvider authenticates users via Keycloak OpenID Connect.
type KeycloakProvider struct {
	db  *sql.DB
	enc *database.Encryptor
}

// NewKeycloakProvider creates a Keycloak authentication provider.
func NewKeycloakProvider(db *sql.DB, enc *database.Encryptor) *KeycloakProvider {
	return &KeycloakProvider{db: db, enc: enc}
}

func (p *KeycloakProvider) Name() string             { return "keycloak" }
func (p *KeycloakProvider) SupportsDirectLogin() bool { return false }

func (p *KeycloakProvider) Enabled() bool {
	return models.GetAppSettingValue(p.db, "auth_active_provider") == "keycloak"
}

func (p *KeycloakProvider) DisplayInfo() auth.ProviderDisplayInfo {
	return auth.ProviderDisplayInfo{
		Label: "PI Login (SSO)",
		Icon:  "shield-check",
		Color: "#22c55e",
	}
}

func (p *KeycloakProvider) Authenticate(_ context.Context, _, _ string) (*auth.ExternalIdentity, error) {
	return nil, fmt.Errorf("keycloak does not support direct login")
}

type keycloakConfig struct {
	BaseURL      string
	Realm        string
	ClientID     string
	ClientSecret string
}

func (p *KeycloakProvider) loadConfig() (*keycloakConfig, error) {
	get := func(key string) string { return models.GetAppSettingValue(p.db, key) }

	clientSecret, err := p.decryptSetting("auth_keycloak_client_secret")
	if err != nil {
		return nil, fmt.Errorf("decrypt client secret: %w", err)
	}

	cfg := &keycloakConfig{
		BaseURL:      get("auth_keycloak_base_url"),
		Realm:        get("auth_keycloak_realm"),
		ClientID:     get("auth_keycloak_client_id"),
		ClientSecret: clientSecret,
	}

	if cfg.BaseURL == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("keycloak is not fully configured")
	}
	if cfg.Realm == "" {
		cfg.Realm = "pi"
	}

	return cfg, nil
}

func (p *KeycloakProvider) decryptSetting(prefix string) (string, error) {
	cipherHex := models.GetAppSettingValue(p.db, prefix+"_cipher")
	nonceHex := models.GetAppSettingValue(p.db, prefix+"_nonce")
	if cipherHex == "" || nonceHex == "" {
		return "", nil
	}
	cipher, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return "", err
	}
	return p.enc.Decrypt(cipher, nonce)
}

func (p *KeycloakProvider) issuerURL(cfg *keycloakConfig) string {
	return fmt.Sprintf("%s/realms/%s", cfg.BaseURL, cfg.Realm)
}

func (p *KeycloakProvider) oauthConfig(cfg *keycloakConfig, issuer, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  callbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  issuer + "/protocol/openid-connect/auth",
			TokenURL: issuer + "/protocol/openid-connect/token",
		},
		Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}
}

func (p *KeycloakProvider) AuthorizationURL(state, callbackURL string) (string, error) {
	cfg, err := p.loadConfig()
	if err != nil {
		return "", err
	}
	issuer := p.issuerURL(cfg)
	oauthCfg := p.oauthConfig(cfg, issuer, callbackURL)
	return oauthCfg.AuthCodeURL(state), nil
}

func (p *KeycloakProvider) ExchangeCode(ctx context.Context, code, callbackURL string) (*auth.ExternalIdentity, error) {
	cfg, err := p.loadConfig()
	if err != nil {
		return nil, err
	}
	issuer := p.issuerURL(cfg)
	oauthCfg := p.oauthConfig(cfg, issuer, callbackURL)

	// Exchange authorization code for tokens.
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// Verify the ID token.
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	// Extract claims.
	var claims struct {
		Sub               string   `json:"sub"`
		PreferredUsername  string   `json:"preferred_username"`
		Name              string   `json:"name"`
		Email             string   `json:"email"`
		RealmAccess       struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Merge roles and groups for role mapping.
	groups := append(claims.RealmAccess.Roles, claims.Groups...)

	// Store all claims as raw JSON for reference.
	var rawClaims map[string]any
	idToken.Claims(&rawClaims)
	rawClaimsMap := make(map[string]string)
	for k, v := range rawClaims {
		b, _ := json.Marshal(v)
		rawClaimsMap[k] = string(b)
	}

	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		username = claims.Sub
	}

	return &auth.ExternalIdentity{
		ProviderName: "keycloak",
		ExternalID:   claims.Sub,
		Username:     username,
		DisplayName:  claims.Name,
		Email:        claims.Email,
		Groups:       groups,
		RawClaims:    rawClaimsMap,
	}, nil
}
