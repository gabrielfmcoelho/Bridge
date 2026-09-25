package proxmox

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Settings is the resolved Proxmox VE configuration. TokenID is the full
// "user@realm!tokenname"; TokenSecret is the UUID Proxmox shows once.
type Settings struct {
	Enabled     bool
	BaseURL     string
	TokenID     string
	TokenSecret string
	SkipVerify  bool
}

// LoadSettings reads the proxmox_* keys from app_settings, decrypting the
// token secret.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	repo := store.NewAppSettingsRepo(db)
	get := func(k string) string { return strings.TrimSpace(repo.Value(context.Background(), k)) }
	s := Settings{
		Enabled:    get("proxmox_enabled") == "true",
		BaseURL:    strings.TrimRight(get("proxmox_base_url"), "/"),
		TokenID:    get("proxmox_token_id"),
		SkipVerify: get("proxmox_skip_verify") == "true",
	}
	secret, _, err := store.NewAppSecretRepo(db).Reveal(context.Background(), enc, "proxmox_token_secret")
	s.TokenSecret = secret
	return s, err
}
