package llm

import "encoding/json"

// ChatMessage represents a message in a chat completion request.
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant", "tool"
	Content string `json:"content"`
	// ToolCalls: an assistant turn asking for tools (its Content may be empty).
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID: on a "tool" turn, the call this result answers.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall is one function call the model asked for.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON object, as text
	} `json:"function"`
}

// Tool declares a function the model may call (OpenAI "tools" format).
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction is a tool's name, description and JSON-schema parameters.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// CompletionRequest is the request body for the OpenAI-compatible chat completion API.
type CompletionRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Tools     []Tool        `json:"tools,omitempty"`
}

// CompletionResponse is the response from the chat completion API.
type CompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}
