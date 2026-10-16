package service

import (
	"context"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/wazuh"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// WazuhClient builds the Wazuh indexer client from the integration
// settings, or errors when the integration is off or incomplete.
func WazuhClient(ctx context.Context, db *database.DB) (*wazuh.Client, error) {
	settings := store.NewAppSettingsRepo(db.SQL)
	if settings.Value(ctx, "wazuh_enabled") != "true" {
		return nil, fmt.Errorf("wazuh integration is not enabled")
	}
	baseURL := settings.Value(ctx, "wazuh_indexer_url")
	user := settings.Value(ctx, "wazuh_indexer_user")
	pass, ok, err := store.NewAppSecretRepo(db.SQL).Reveal(ctx, db.Encryptor, "wazuh_indexer_password")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt wazuh password: %w", err)
	}
	if !ok || baseURL == "" || user == "" {
		return nil, fmt.Errorf("wazuh integration is not configured")
	}
	return wazuh.NewClient(baseURL, user, pass, settings.Value(ctx, "wazuh_tls_skip_verify") == "true"), nil
}
