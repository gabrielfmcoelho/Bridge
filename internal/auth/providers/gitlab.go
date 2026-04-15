package providers

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"golang.org/x/oauth2"
)

// GitLabProvider authenticates users via GitLab OAuth 2.0.
type GitLabProvider struct {
	db  *sql.DB
	enc *database.Encryptor
}

// NewGitLabProvider creates a GitLab authentication provider.
func NewGitLabProvider(db *sql.DB, enc *database.Encryptor) *GitLabProvider {
	return &GitLabProvider{db: db, enc: enc}
}

func (p *GitLabProvider) Name() string             { return "gitlab" }
func (p *GitLabProvider) SupportsDirectLogin() bool { return false }

func (p *GitLabProvider) Enabled() bool {
	return models.GetAppSettingValue(p.db, "auth_active_provider") == "gitlab"
}

func (p *GitLabProvider) DisplayInfo() auth.ProviderDisplayInfo {
	return auth.ProviderDisplayInfo{
		Label: "GitLab",
		Icon:  "git-branch",
		Color: "#e24329",
	}
}

func (p *GitLabProvider) Authenticate(_ context.Context, _, _ string) (*auth.ExternalIdentity, error) {
	return nil, fmt.Errorf("gitlab does not support direct login")
}

type gitlabConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
}

func (p *GitLabProvider) loadConfig() (*gitlabConfig, error) {
	get := func(key string) string { return models.GetAppSettingValue(p.db, key) }

	clientSecret, err := p.decryptSetting("auth_gitlab_client_secret")
	if err != nil {
		return nil, fmt.Errorf("decrypt client secret: %w", err)
	}

	cfg := &gitlabConfig{
		BaseURL:      get("auth_gitlab_base_url"),
		ClientID:     get("auth_gitlab_client_id"),
		ClientSecret: clientSecret,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://gitlab.com"
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("gitlab is not fully configured")
	}
	return cfg, nil
}

func (p *GitLabProvider) decryptSetting(prefix string) (string, error) {
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

func (p *GitLabProvider) oauthConfig(cfg *gitlabConfig, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  callbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.BaseURL + "/oauth/authorize",
			TokenURL: cfg.BaseURL + "/oauth/token",
		},
		Scopes: []string{"read_user", "api"},
	}
}

func (p *GitLabProvider) AuthorizationURL(state, callbackURL string) (string, error) {
	cfg, err := p.loadConfig()
	if err != nil {
		return "", err
	}
	return p.oauthConfig(cfg, callbackURL).AuthCodeURL(state), nil
}

func (p *GitLabProvider) ExchangeCode(ctx context.Context, code, callbackURL string) (*auth.ExternalIdentity, error) {
	cfg, err := p.loadConfig()
	if err != nil {
		return nil, err
	}
	oauthCfg := p.oauthConfig(cfg, callbackURL)

	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// Fetch user info from GitLab API.
	client := oauthCfg.Client(ctx, token)
	client.Timeout = 10 * time.Second
	resp, err := client.Get(cfg.BaseURL + "/api/v4/user")
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read user info: %w", err)
	}

	var userInfo struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("parse user info: %w", err)
	}

	return &auth.ExternalIdentity{
		ProviderName: "gitlab",
		ExternalID:   fmt.Sprintf("%d", userInfo.ID),
		Username:     userInfo.Username,
		DisplayName:  userInfo.Name,
		Email:        userInfo.Email,
		RawClaims: map[string]string{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"gitlab_id":     fmt.Sprintf("%d", userInfo.ID),
		},
	}, nil
}

// AccessToken returns the stored access token from RawClaims (for use after login to store in user_gitlab_tokens).
func AccessTokenFromIdentity(identity *auth.ExternalIdentity) (accessToken, refreshToken string) {
	if identity.RawClaims != nil {
		accessToken = identity.RawClaims["access_token"]
		refreshToken = identity.RawClaims["refresh_token"]
	}
	return
}
