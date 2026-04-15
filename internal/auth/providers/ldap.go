package providers

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/go-ldap/ldap/v3"
)

const ldapConnectTimeout = 5 * time.Second

// LDAPProvider authenticates users against an LDAP directory.
type LDAPProvider struct {
	db  *sql.DB
	enc *database.Encryptor
}

// NewLDAPProvider creates an LDAP authentication provider.
func NewLDAPProvider(db *sql.DB, enc *database.Encryptor) *LDAPProvider {
	return &LDAPProvider{db: db, enc: enc}
}

func (p *LDAPProvider) Name() string             { return "ldap" }
func (p *LDAPProvider) SupportsDirectLogin() bool { return true }

func (p *LDAPProvider) Enabled() bool {
	return models.GetAppSettingValue(p.db, "auth_active_provider") == "ldap"
}

func (p *LDAPProvider) DisplayInfo() auth.ProviderDisplayInfo {
	return auth.ProviderDisplayInfo{
		Label: "LDAP",
		Icon:  "shield",
		Color: "#3b82f6",
	}
}

func (p *LDAPProvider) AuthorizationURL(_, _ string) (string, error) {
	return "", fmt.Errorf("LDAP does not support OAuth")
}

func (p *LDAPProvider) ExchangeCode(_ context.Context, _, _ string) (*auth.ExternalIdentity, error) {
	return nil, fmt.Errorf("LDAP does not support OAuth")
}

// ldapConfig holds the resolved LDAP configuration values.
type ldapConfig struct {
	Host             string
	Port             string
	UseTLS           bool
	SkipVerify       bool
	BaseDN           string
	BindDN           string
	BindPassword     string
	UserFilter       string
	UsernameAttr     string
	DisplayNameAttr  string
	EmailAttr        string
}

func (p *LDAPProvider) loadConfig() (*ldapConfig, error) {
	get := func(key string) string { return models.GetAppSettingValue(p.db, key) }

	bindPassword, err := p.decryptSetting("auth_ldap_bind_password")
	if err != nil {
		return nil, fmt.Errorf("decrypt bind password: %w", err)
	}

	cfg := &ldapConfig{
		Host:            get("auth_ldap_host"),
		Port:            get("auth_ldap_port"),
		UseTLS:          get("auth_ldap_use_tls") != "false",
		SkipVerify:      get("auth_ldap_skip_verify") == "true",
		BaseDN:          get("auth_ldap_base_dn"),
		BindDN:          get("auth_ldap_bind_dn"),
		BindPassword:    bindPassword,
		UserFilter:      get("auth_ldap_user_filter"),
		UsernameAttr:    get("auth_ldap_username_attr"),
		DisplayNameAttr: get("auth_ldap_display_name_attr"),
		EmailAttr:       get("auth_ldap_email_attr"),
	}

	if cfg.Host == "" || cfg.BaseDN == "" || cfg.BindDN == "" {
		return nil, fmt.Errorf("LDAP is not fully configured")
	}
	if cfg.Port == "" {
		cfg.Port = "636"
	}
	if cfg.UserFilter == "" {
		cfg.UserFilter = "(mail=%s)"
	}
	if cfg.UsernameAttr == "" {
		cfg.UsernameAttr = "uid"
	}
	if cfg.DisplayNameAttr == "" {
		cfg.DisplayNameAttr = "cn"
	}
	if cfg.EmailAttr == "" {
		cfg.EmailAttr = "mail"
	}

	return cfg, nil
}

// decryptSetting decrypts a setting stored as hex-encoded _cipher/_nonce pair in app_settings.
func (p *LDAPProvider) decryptSetting(prefix string) (string, error) {
	cipherHex := models.GetAppSettingValue(p.db, prefix+"_cipher")
	nonceHex := models.GetAppSettingValue(p.db, prefix+"_nonce")
	if cipherHex == "" || nonceHex == "" {
		return "", nil // not configured
	}
	cipher, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", fmt.Errorf("decode cipher hex: %w", err)
	}
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return "", fmt.Errorf("decode nonce hex: %w", err)
	}
	return p.enc.Decrypt(cipher, nonce)
}

func (p *LDAPProvider) Authenticate(_ context.Context, username, password string) (*auth.ExternalIdentity, error) {
	cfg, err := p.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("ldap config: %w", err)
	}

	// Connect to LDAP server.
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	conn, err := p.dial(addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	// Stage 1: Bind with service account.
	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return nil, fmt.Errorf("ldap service bind: %w", err)
	}

	// Stage 2: Search for the user.
	filter := strings.ReplaceAll(cfg.UserFilter, "%s", ldap.EscapeFilter(username))
	searchReq := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, int(ldapConnectTimeout.Seconds()), false,
		filter,
		[]string{"dn", cfg.UsernameAttr, cfg.DisplayNameAttr, cfg.EmailAttr},
		nil,
	)
	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("invalid credentials")
	}

	entry := result.Entries[0]

	// Stage 3: Bind as the user to validate their password.
	if err := conn.Bind(entry.DN, password); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	uid := entry.GetAttributeValue(cfg.UsernameAttr)
	displayName := entry.GetAttributeValue(cfg.DisplayNameAttr)
	email := entry.GetAttributeValue(cfg.EmailAttr)

	if uid == "" {
		uid = username
	}

	return &auth.ExternalIdentity{
		ProviderName: "ldap",
		ExternalID:   entry.DN,
		Username:     uid,
		DisplayName:  displayName,
		Email:        email,
	}, nil
}

func (p *LDAPProvider) dial(addr string, cfg *ldapConfig) (*ldap.Conn, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.SkipVerify,
	}

	scheme := "ldap"
	if cfg.UseTLS {
		scheme = "ldaps"
	}
	url := fmt.Sprintf("%s://%s", scheme, addr)
	return ldap.DialURL(url, ldap.DialWithTLSConfig(tlsConfig))
}

// TestConnection validates the LDAP configuration by binding with the service account.
func (p *LDAPProvider) TestConnection() error {
	cfg, err := p.loadConfig()
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	conn, err := p.dial(addr, cfg)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return fmt.Errorf("bind failed: %w", err)
	}

	return nil
}
