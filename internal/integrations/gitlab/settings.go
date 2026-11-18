package gitlab

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Default GitLab base URL used when no admin setting is configured.
const DefaultBaseURL = "https://gitlab.com"

// Settings holds resolved GitLab Code Management configuration.
type Settings struct {
	Enabled      bool
	BaseURL      string
	ServiceToken string
	DefaultRef   string
	// CatalogGroups are the groups (full paths) whose repositories the app
	// dependencies sync reads; empty = every project the token is a member of.
	CatalogGroups []string
}

// LoadSettings resolves the shared GitLab Code Management configuration from app_settings,
// decrypting the service PAT. Empty ServiceToken means no token configured — callers
// should treat that as "not configured" rather than an error.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	s := Settings{
		Enabled:    store.NewAppSettingsRepo(db).Value(context.Background(), "gitlab_integration_enabled") == "true",
		BaseURL:    BaseURL(db),
		DefaultRef: store.NewAppSettingsRepo(db).Value(context.Background(), "gitlab_code_default_ref"),
	}
	s.CatalogGroups = SplitGroups(store.NewAppSettingsRepo(db).Value(context.Background(), "gitlab_catalog_groups"))

	token, _, err := store.NewAppSecretRepo(db).Reveal(context.Background(), enc, "gitlab_code_service_token")
	if err != nil {
		return s, err
	}
	s.ServiceToken = token
	return s, nil
}

// NewServiceClient returns a Client using the shared service PAT, or nil if not configured.
func NewServiceClient(settings Settings) *Client {
	if settings.ServiceToken == "" {
		return nil
	}
	return NewClient(settings.BaseURL, settings.ServiceToken)
}

// SplitGroups reads a list of group paths separated by commas or newlines,
// trimmed, empty entries and surrounding slashes dropped.
func SplitGroups(raw string) []string {
	var out []string
	for _, g := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if g = strings.Trim(strings.TrimSpace(g), "/"); g != "" {
			out = append(out, g)
		}
	}
	return out
}

// BaseURL is the configured GitLab's base URL (DefaultBaseURL when unset),
// read without touching the encrypted token.
func BaseURL(db *sql.DB) string {
	if u := store.NewAppSettingsRepo(db).Value(context.Background(), "auth_gitlab_base_url"); u != "" {
		return u
	}
	return DefaultBaseURL
}

// SameBaseURL compares two GitLab base URLs ignoring case and a trailing slash.
func SameBaseURL(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(a, "/"), strings.TrimRight(b, "/"))
}
