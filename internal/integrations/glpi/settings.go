package glpi

import (
	"database/sql"
	"encoding/hex"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// Settings captures the instance-level GLPI configuration. User tokens (one per
// profile) live in the glpi_tokens table — not here — because we support multiple.
type Settings struct {
	Enabled          bool
	BaseURL          string
	AppToken         string
	DefaultEntityID  int
}

// LoadSettings reads the glpi_* keys from app_settings, decrypting the App-Token.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	get := func(k string) string { return strings.TrimSpace(models.GetAppSettingValue(db, k)) }
	s := Settings{
		Enabled: models.GetAppSettingValue(db, "glpi_enabled") == "true",
		BaseURL: strings.TrimRight(get("glpi_base_url"), "/"),
	}
	appTok, err := decryptSecret(db, enc, "glpi_app_token")
	if err != nil {
		return s, err
	}
	s.AppToken = appTok

	// Optional default entity id — parse defensively.
	if v := get("glpi_default_entity_id"); v != "" {
		var n int
		for _, r := range v {
			if r < '0' || r > '9' {
				n = 0
				break
			}
			n = n*10 + int(r-'0')
		}
		s.DefaultEntityID = n
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

// NewServiceClient returns a ready-to-use client if the minimum config is present.
// A valid GLPI setup may or may not require an App-Token depending on the admin's
// configuration, so we only require the base URL.
func NewServiceClient(s Settings) *Client {
	if s.BaseURL == "" {
		return nil
	}
	return NewClient(s.BaseURL, s.AppToken)
}
