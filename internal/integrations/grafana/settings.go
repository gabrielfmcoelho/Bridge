package grafana

import (
	"database/sql"
	"encoding/hex"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// Settings is the resolved Grafana configuration. Empty strings mean "not set";
// callers should treat missing pieces as "not configured" rather than erroring.
type Settings struct {
	Enabled                       bool
	BaseURL                       string
	APIToken                      string
	WebhookSecret                 string
	HostDefaultDashboardUID       string
	ServiceDefaultDashboardUID    string
	PromRemoteWriteURL            string
	PromRemoteWriteUsername       string
	PromRemoteWritePassword       string
	DatasourceUID                 string
}

// LoadSettings reads every grafana_* key from app_settings, decrypting the
// three secrets. Mirrors gitlab.LoadSettings / handleTestLDAP pattern.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	get := func(k string) string {
		return strings.TrimSpace(models.GetAppSettingValue(db, k))
	}
	s := Settings{
		Enabled:                    models.GetAppSettingValue(db, "grafana_enabled") == "true",
		BaseURL:                    strings.TrimRight(get("grafana_base_url"), "/"),
		HostDefaultDashboardUID:    get("grafana_host_default_dashboard_uid"),
		ServiceDefaultDashboardUID: get("grafana_service_default_dashboard_uid"),
		PromRemoteWriteURL:         get("grafana_prom_remote_write_url"),
		PromRemoteWriteUsername:    get("grafana_prom_remote_write_username"),
		DatasourceUID:              get("grafana_datasource_uid"),
	}

	if tok, err := decryptSecret(db, enc, "grafana_api_token"); err != nil {
		return s, err
	} else {
		s.APIToken = tok
	}
	if sec, err := decryptSecret(db, enc, "grafana_webhook_secret"); err != nil {
		return s, err
	} else {
		s.WebhookSecret = sec
	}
	if pw, err := decryptSecret(db, enc, "grafana_prom_remote_write_password"); err != nil {
		return s, err
	} else {
		s.PromRemoteWritePassword = pw
	}
	return s, nil
}

func decryptSecret(db *sql.DB, enc *database.Encryptor, prefix string) (string, error) {
	cipherHex := models.GetAppSettingValue(db, prefix+"_cipher")
	nonceHex := models.GetAppSettingValue(db, prefix+"_nonce")
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
	return enc.Decrypt(cipher, nonce)
}

// NewServiceClient returns a Client using the resolved settings, or nil if the
// integration lacks the minimum needed to talk to Grafana.
func NewServiceClient(s Settings) *Client {
	if s.BaseURL == "" || s.APIToken == "" {
		return nil
	}
	return NewClient(s.BaseURL, s.APIToken)
}
