package outline

import (
	"database/sql"
	"encoding/hex"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// Settings is the resolved Outline configuration.
// Empty fields mean "not configured" — callers check before instantiating a client.
type Settings struct {
	Enabled             bool
	BaseURL             string
	APIToken            string
	CommonCollectionIDs []string
}

// LoadSettings reads all outline_* settings from app_settings, decrypting the API token.
// Mirrors grafana.LoadSettings.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	get := func(k string) string { return strings.TrimSpace(models.GetAppSettingValue(db, k)) }
	s := Settings{
		Enabled:             models.GetAppSettingValue(db, "outline_enabled") == "true",
		BaseURL:             normaliseOutlineBaseURL(get("outline_base_url")),
		CommonCollectionIDs: parseCommonCollectionIDs(get("outline_common_collection_id")),
	}
	tok, err := decryptSecret(db, enc, "outline_api_token")
	if err != nil {
		return s, err
	}
	s.APIToken = tok
	return s, nil
}

// parseCommonCollectionIDs splits the stored setting on commas, trims whitespace,
// and drops empty entries. A single UUID (no commas) preserves the old behaviour.
func parseCommonCollectionIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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

// NewServiceClient returns a ready-to-use client backed by the stored service token,
// or nil if the integration lacks the minimum required config (base URL + token).
func NewServiceClient(s Settings) *Client {
	if s.BaseURL == "" || s.APIToken == "" {
		return nil
	}
	return NewClient(s.BaseURL, s.APIToken)
}
