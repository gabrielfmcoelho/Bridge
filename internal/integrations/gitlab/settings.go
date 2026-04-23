package gitlab

import (
	"database/sql"
	"encoding/hex"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// Default GitLab base URL used when no admin setting is configured.
const DefaultBaseURL = "https://gitlab.com"

// Settings holds resolved GitLab Code Management configuration.
type Settings struct {
	Enabled      bool
	BaseURL      string
	ServiceToken string
	DefaultRef   string
}

// LoadSettings resolves the shared GitLab Code Management configuration from app_settings,
// decrypting the service PAT. Empty ServiceToken means no token configured — callers
// should treat that as "not configured" rather than an error.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	s := Settings{
		Enabled:    models.GetAppSettingValue(db, "gitlab_integration_enabled") == "true",
		BaseURL:    models.GetAppSettingValue(db, "auth_gitlab_base_url"),
		DefaultRef: models.GetAppSettingValue(db, "gitlab_code_default_ref"),
	}
	if s.BaseURL == "" {
		s.BaseURL = DefaultBaseURL
	}

	cipherHex := models.GetAppSettingValue(db, "gitlab_code_service_token_cipher")
	nonceHex := models.GetAppSettingValue(db, "gitlab_code_service_token_nonce")
	if cipherHex == "" || nonceHex == "" {
		return s, nil
	}

	cipher, err := hex.DecodeString(cipherHex)
	if err != nil {
		return s, err
	}
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return s, err
	}
	token, err := enc.Decrypt(cipher, nonce)
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
