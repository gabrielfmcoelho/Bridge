package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// LLMClient builds the OpenAI-compatible LLM client from the "llm"
// integration settings, or errors when AI is off or incomplete.
func LLMClient(ctx context.Context, db *database.DB) (*llm.Client, error) {
	settings := store.NewAppSettingsRepo(db.SQL)
	if settings.Value(ctx, "llm_enabled") != "true" {
		return nil, fmt.Errorf("AI is not enabled")
	}
	baseURL := settings.Value(ctx, "llm_base_url")
	maxTokens := 2000
	if n, err := strconv.Atoi(settings.Value(ctx, "llm_max_tokens")); err == nil && n > 0 {
		maxTokens = n
	}
	apiKey, ok, err := store.NewAppSecretRepo(db.SQL).Reveal(ctx, db.Encryptor, "llm_api_key")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt llm api key: %w", err)
	}
	if !ok || baseURL == "" {
		return nil, fmt.Errorf("AI is not configured")
	}
	return llm.NewClient(baseURL, apiKey, settings.Value(ctx, "llm_model_text"), maxTokens), nil
}
