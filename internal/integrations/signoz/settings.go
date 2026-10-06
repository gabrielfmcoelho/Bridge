package signoz

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Settings is the resolved SigNoz configuration. Bridge reads SigNoz's
// ClickHouse over its HTTP interface (port 8123) with a read-only user; UIURL
// is only for the "open in SigNoz" link.
type Settings struct {
	Enabled bool
	CHURL   string
	CHUser  string
	CHPass  string
	UIURL   string
}

// LoadSettings reads the signoz_* keys from app_settings (password decrypted).
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	repo := store.NewAppSettingsRepo(db)
	get := func(k string) string { return strings.TrimSpace(repo.Value(context.Background(), k)) }
	s := Settings{
		Enabled: get("signoz_enabled") == "true",
		CHURL:   strings.TrimRight(get("signoz_ch_url"), "/"),
		CHUser:  get("signoz_ch_user"),
		UIURL:   strings.TrimRight(get("signoz_ui_url"), "/"),
	}
	pw, _, err := store.NewAppSecretRepo(db).Reveal(context.Background(), enc, "signoz_ch_password")
	s.CHPass = pw
	return s, err
}

// NewServiceClient returns a client, or nil when the integration is off or
// lacks a ClickHouse URL.
func NewServiceClient(s Settings) *Client {
	if !s.Enabled || s.CHURL == "" {
		return nil
	}
	return NewClient(s.CHURL, s.CHUser, s.CHPass)
}
