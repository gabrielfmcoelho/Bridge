package providers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
)

// LocalProvider authenticates users against the local database (bcrypt passwords).
type LocalProvider struct {
	db *sql.DB
}

// NewLocalProvider creates a local authentication provider.
func NewLocalProvider(db *sql.DB) *LocalProvider {
	return &LocalProvider{db: db}
}

func (p *LocalProvider) Name() string           { return "local" }
func (p *LocalProvider) Enabled() bool           { return true } // always enabled
func (p *LocalProvider) SupportsDirectLogin() bool { return true }

func (p *LocalProvider) Authenticate(_ context.Context, username, password string) (*auth.ExternalIdentity, error) {
	user, err := auth.Login(p.db, username, password)
	if err != nil {
		return nil, err
	}
	return &auth.ExternalIdentity{
		ProviderName: "local",
		ExternalID:   fmt.Sprintf("%d", user.ID),
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
	}, nil
}

func (p *LocalProvider) AuthorizationURL(_, _ string) (string, error) {
	return "", fmt.Errorf("local provider does not support OAuth")
}

func (p *LocalProvider) ExchangeCode(_ context.Context, _, _ string) (*auth.ExternalIdentity, error) {
	return nil, fmt.Errorf("local provider does not support OAuth")
}

func (p *LocalProvider) DisplayInfo() auth.ProviderDisplayInfo {
	return auth.ProviderDisplayInfo{
		Label: "Login",
		Icon:  "key",
		Color: "#06b6d4",
	}
}
