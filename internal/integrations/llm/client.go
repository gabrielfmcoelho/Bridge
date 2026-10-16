package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client interacts with an OpenAI-compatible chat completion API.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

// NewClient creates an LLM client.
func NewClient(baseURL, apiKey, model string, maxTokens int) *Client {
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Model is the configured text model id.
func (c *Client) Model() string { return c.model }

// ListModels calls GET /models on the OpenAI-compatible API and returns the list
// of available model ids. Used as a cheap connectivity + auth check.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// APIError is a non-2xx answer from the API. Callers can tell a request the
// gateway rejected (4xx, e.g. a model without tool support) from an outage.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("api error %d: %s", e.Status, e.Body) }

// Complete sends a chat completion request and returns the assistant's reply.
func (c *Client) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	msg, err := c.Chat(ctx, messages, nil)
	return msg.Content, err
}

// Chat sends a chat completion request with optional tools and returns the
// whole assistant message (text, or the tool calls it asks for).
func (c *Client) Chat(ctx context.Context, messages []ChatMessage, tools []Tool) (ChatMessage, error) {
	req := CompletionRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: c.maxTokens,
		Tools:     tools,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return ChatMessage{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return ChatMessage{}, &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}

	var result CompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ChatMessage{}, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return ChatMessage{}, fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message, nil
}
