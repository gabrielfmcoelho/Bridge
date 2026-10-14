package service

import (
	"context"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// CoolifyClient builds the Coolify API client from the integration settings,
// or errors when the integration is off or incomplete.
func CoolifyClient(ctx context.Context, db *database.DB) (*coolify.Client, error) {
	settings := store.NewAppSettingsRepo(db.SQL)
	if settings.Value(ctx, "coolify_enabled") != "true" {
		return nil, fmt.Errorf("coolify integration is not enabled")
	}
	baseURL := settings.Value(ctx, "coolify_base_url")
	apiToken, ok, err := store.NewAppSecretRepo(db.SQL).Reveal(ctx, db.Encryptor, "coolify_api_token")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt coolify token: %w", err)
	}
	if !ok || baseURL == "" {
		return nil, fmt.Errorf("coolify integration is not configured")
	}
	return coolify.NewClient(baseURL, apiToken), nil
}
