// Package seadkeys talks to the built-in key-management API that SEAD's
// derived APIs (dlhsead-api-servidores, dlhsead-api-sei) expose at
// /admin/keys, authenticated by the service's ADMIN_MASTER_KEY in the
// X-Admin-Key header.
//
// Quirks it absorbs:
//   - Admin routes may also demand a valid X-API-Key when the service runs
//     behind a path prefix (its admin bypass checks the raw path), so the
//     client sends one when configured.
//   - Errors come as FastAPI {"detail": ...} on admin routes and as
//     {"error", "detail", "hint"} elsewhere; both are read.
//   - There is no update endpoint: changing a key means revoke + create.
package seadkeys

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a configured SEAD key-management client for one service.
type Client struct {
	baseURL  string // e.g. https://api.folha.sead.gov.br/folha (no trailing slash)
	adminKey string
	apiKey   string // optional X-API-Key
	http     *http.Client
}

// New returns a client for baseURL. apiKey may be empty.
func New(baseURL, adminKey, apiKey string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		adminKey: adminKey,
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Error is a non-2xx answer from the service.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("sead keys: %d %s", e.Status, e.Message) }

// List returns every key the service knows, revoked and rotated ones included.
func (c *Client) List(ctx context.Context) ([]KeySummary, error) {
	var out []KeySummary
	return out, c.do(ctx, http.MethodGet, "/admin/keys", nil, &out)
}

// Get returns one key by label.
func (c *Client) Get(ctx context.Context, label string) (*KeySummary, error) {
	var out KeySummary
	if err := c.do(ctx, http.MethodGet, "/admin/keys/"+url.PathEscape(label), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create issues a key; the plaintext is only in this response.
func (c *Client) Create(ctx context.Context, req CreateRequest) (*CreateResponse, error) {
	var out CreateResponse
	if err := c.do(ctx, http.MethodPost, "/admin/keys", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke soft-revokes a key (the record stays, status REVOKED).
func (c *Client) Revoke(ctx context.Context, label string) (*KeySummary, error) {
	var out KeySummary
	if err := c.do(ctx, http.MethodDelete, "/admin/keys/"+url.PathEscape(label), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Rotate issues a new key under the same label; the old one is renamed
// "<label>__rotated__<hash8>" and keeps working for graceDays.
func (c *Client) Rotate(ctx context.Context, label string, graceDays int) (*CreateResponse, error) {
	var out CreateResponse
	if err := c.do(ctx, http.MethodPost, "/admin/keys/"+url.PathEscape(label)+"/rotate", map[string]int{"grace_days": graceDays}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Usage returns a key's lifetime and daily request counts for the last days.
func (c *Client) Usage(ctx context.Context, label string, days int) (*Usage, error) {
	var out Usage
	path := fmt.Sprintf("/admin/keys/%s/usage?days=%d", url.PathEscape(label), days)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if c.baseURL == "" || c.adminKey == "" {
		return fmt.Errorf("sead keys: admin base URL and master key are required")
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Admin-Key", c.adminKey)
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sead keys: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &Error{Status: resp.StatusCode, Message: errorMessage(raw)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("sead keys: unexpected response: %w", err)
	}
	return nil
}

// errorMessage reads either error shape the service uses, falling back to
// the raw (truncated) body.
func errorMessage(raw []byte) string {
	var e struct {
		Detail any    `json:"detail"`
		Error  string `json:"error"`
		Hint   string `json:"hint"`
	}
	if json.Unmarshal(raw, &e) == nil {
		parts := []string{}
		if e.Error != "" {
			parts = append(parts, e.Error)
		}
		switch d := e.Detail.(type) {
		case string:
			parts = append(parts, d)
		case nil:
		default: // FastAPI validation errors: a list of objects
			if b, err := json.Marshal(d); err == nil {
				parts = append(parts, string(b))
			}
		}
		if e.Hint != "" {
			parts = append(parts, "("+e.Hint+")")
		}
		if len(parts) > 0 {
			return strings.Join(parts, ": ")
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
