// Package glpiv2 talks to GLPI's newer "High-Level REST API" (introduced in
// GLPI 11, served at /api.php/v2.x), used by the SEAD/NTGD GLPI instance.
// This is a different protocol from internal/integrations/glpi (the legacy
// /apirest.php client used for the ETIPI GLPI instance): OAuth2 instead of
// App-Token/user-token/session-token, nested {id,name} refs instead of flat
// numeric FKs, RSQL filters instead of criteria[] arrays. The two packages
// are intentionally independent — see internal/spec note in CLAUDE.md.
package glpiv2

import (
	"os"
	"strings"
)

// Settings is sourced entirely from environment variables (GLPI_SEAD_*), not
// from the app_settings table — this integration was stood up with
// operator-managed .env values rather than a Settings-UI flow, since the
// credential (a super-admin OAuth2 password-grant client) is high-privilege
// and the operator asked to keep it out of the database/UI entirely.
type Settings struct {
	Enabled      bool
	BaseURL      string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// LoadSettingsFromEnv reads GLPI_SEAD_* env vars. Enabled requires both the
// explicit opt-in flag and every credential field to be non-empty — a half
// configured integration silently stays disabled rather than erroring.
func LoadSettingsFromEnv() Settings {
	s := Settings{
		Enabled:      strings.EqualFold(strings.TrimSpace(os.Getenv("GLPI_SEAD_ENABLED")), "true"),
		BaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("GLPI_SEAD_BASE_URL")), "/"),
		ClientID:     strings.TrimSpace(os.Getenv("GLPI_SEAD_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("GLPI_SEAD_CLIENT_SECRET")),
		Username:     strings.TrimSpace(os.Getenv("GLPI_SEAD_USERNAME")),
		Password:     strings.TrimSpace(os.Getenv("GLPI_SEAD_PASSWORD")),
	}
	if s.BaseURL == "" || s.ClientID == "" || s.ClientSecret == "" || s.Username == "" || s.Password == "" {
		s.Enabled = false
	}
	return s
}
