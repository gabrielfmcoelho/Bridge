package gestor

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// DefaultBaseURL is the Gestor behind the SEAD gateway.
const DefaultBaseURL = "https://gateway.sead.pi.gov.br/gestor"

// Settings is the resolved "gestor" settings group.
type Settings struct {
	Enabled                         bool
	BaseURL, ClientID, ClientSecret string
	GitLabGrupos                    []string
}

// LoadSettings reads the group, decrypting the client secret.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	ctx, repo := context.Background(), store.NewAppSettingsRepo(db)
	s := Settings{
		Enabled:  repo.Value(ctx, "gestor_integration_enabled") == "true",
		BaseURL:  strings.TrimRight(strings.TrimSpace(repo.Value(ctx, "gestor_base_url")), "/"),
		ClientID: strings.TrimSpace(repo.Value(ctx, "gestor_client_id")),
	}
	if s.BaseURL == "" {
		s.BaseURL = DefaultBaseURL
	}
	for _, g := range strings.FieldsFunc(repo.Value(ctx, "gestor_gitlab_grupos"), func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if g = strings.TrimSpace(g); g != "" {
			s.GitLabGrupos = append(s.GitLabGrupos, g)
		}
	}
	if len(s.GitLabGrupos) == 0 {
		s.GitLabGrupos = []string{"seadpi"}
	}
	secret, _, err := store.NewAppSecretRepo(db).Reveal(ctx, enc, "gestor_client_secret")
	s.ClientSecret = secret
	return s, err
}

// KeycloakFromSettings builds the token source from the keycloak_apis group
// (plain settings only: Gestor's own client secret does the authenticating).
func KeycloakFromSettings(db *sql.DB) *kcadmin.Client {
	repo := store.NewAppSettingsRepo(db)
	return kcadmin.New(kcadmin.FromSettings(func(k string) string {
		if k == kcadmin.SettingClientSecret || k == kcadmin.SettingUsageSecret {
			return ""
		}
		return repo.Value(context.Background(), k)
	}))
}
