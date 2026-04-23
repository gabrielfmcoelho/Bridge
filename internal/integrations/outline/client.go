package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an Outline instance. Outline's REST API is a little unusual:
// every endpoint is a POST with JSON body (even reads), and responses wrap the
// payload in {data, ok, pagination?}. This client hides both details.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient constructs a client. The base URL should be the Outline server root
// (e.g. "https://wiki.example.org") — the client owns the "/api/" prefix for
// every endpoint it calls.
//
// Many Outline users (reasonably) paste "https://wiki.example.org/api" because
// that's what the Outline docs show in examples. We defensively strip a trailing
// "/api" or "/api/" so both forms work — otherwise we'd double-prefix and 404.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    normaliseOutlineBaseURL(baseURL),
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// normaliseOutlineBaseURL strips a trailing slash and, if present, a trailing
// "/api" segment. The client's own endpoint methods already prepend "/api/…".
func normaliseOutlineBaseURL(s string) string {
	s = strings.TrimRight(s, "/")
	if strings.HasSuffix(s, "/api") {
		s = strings.TrimSuffix(s, "/api")
	}
	return strings.TrimRight(s, "/")
}

// BaseURL returns the normalized base URL (useful for building deep-links).
func (c *Client) BaseURL() string { return c.baseURL }

// post executes a POST /api/<path> with a JSON body and decodes the data field
// into out. Outline returns 4xx/5xx with {ok:false, error, message} — we surface
// both the HTTP status and the server-provided error to the caller.
//
// Retries once on transient transport errors (unexpected EOF, connection reset).
// Outline's server behind its reverse proxy occasionally drops reused keep-alive
// connections, and Go's net/http doesn't auto-retry POST requests. Retrying at
// our layer is safe because every Outline endpoint we call is semantically
// idempotent (auth.info, *.info, *.list, *.documents, documents.search).
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	var marshaled []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		marshaled = b
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			// Small backoff lets the far side recover from whatever caused the EOF.
			select {
			case <-ctx.Done():
				return lastErr
			case <-time.After(200 * time.Millisecond):
			}
		}
		err := c.postOnce(ctx, path, marshaled, body != nil, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableTransportError(err) {
			return err
		}
	}
	return lastErr
}

func (c *Client) postOnce(ctx context.Context, path string, marshaled []byte, hasBody bool, out any) error {
	var reader io.Reader
	if hasBody {
		reader = bytes.NewReader(marshaled)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var env envelope
		if json.Unmarshal(respBody, &env) == nil && env.Error != "" {
			return fmt.Errorf("outline api %s: %d %s: %s", path, resp.StatusCode, env.Error, env.Message)
		}
		return fmt.Errorf("outline api %s: %d %s", path, resp.StatusCode, string(respBody))
	}

	if out == nil {
		return nil
	}
	var env envelope
	env.Data = json.RawMessage(nil)
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}
	raw, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("parse data: %w", err)
	}
	return nil
}

// isRetryableTransportError matches the network-level failures that warrant a
// single retry (the server closed a keep-alive connection before sending a
// response, or a TCP reset). Does NOT match HTTP status errors — those come
// wrapped with "outline api" and fail fast.
func isRetryableTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "unexpected EOF") ||
		strings.Contains(s, "EOF") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "broken pipe")
}

// AuthInfo returns the authenticated user + workspace for the service token.
// Used as the cheapest possible "is this token valid" check.
func (c *Client) AuthInfo(ctx context.Context) (*AuthInfo, error) {
	var out AuthInfo
	if err := c.post(ctx, "/api/auth.info", map[string]any{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CollectionsList returns all collections the token has access to, paginated.
// Pass limit <= 0 to use Outline's default (usually 25).
func (c *Client) CollectionsList(ctx context.Context, limit, offset int) ([]Collection, error) {
	body := map[string]any{}
	if limit > 0 {
		body["limit"] = limit
	}
	if offset > 0 {
		body["offset"] = offset
	}
	var out []Collection
	if err := c.post(ctx, "/api/collections.list", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CollectionInfo fetches one collection by id.
func (c *Client) CollectionInfo(ctx context.Context, id string) (*Collection, error) {
	var out Collection
	if err := c.post(ctx, "/api/collections.info", map[string]any{"id": id}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DocumentsList returns docs in a collection, newest-updated first.
func (c *Client) DocumentsList(ctx context.Context, collectionID string, limit int) ([]Document, error) {
	body := map[string]any{
		"collectionId": collectionID,
		"sort":         "updatedAt",
		"direction":    "DESC",
	}
	if limit > 0 {
		body["limit"] = limit
	}
	var out []Document
	if err := c.post(ctx, "/api/documents.list", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DocumentCreate creates a new document. Publish=true skips the draft state —
// we set this to true so the doc is immediately visible to everyone with
// collection access, matching admin expectations when creating from sshcm.
func (c *Client) DocumentCreate(ctx context.Context, in DocumentCreateInput) (*Document, error) {
	if in.CollectionID == "" {
		return nil, fmt.Errorf("collectionId is required")
	}
	if in.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	var out Document
	if err := c.post(ctx, "/api/documents.create", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DocumentInfo fetches a single document by id, including its full markdown body.
// Outline's shareId/slug/id all resolve here; we pass the UUID.
func (c *Client) DocumentInfo(ctx context.Context, id string) (*Document, error) {
	var out Document
	if err := c.post(ctx, "/api/documents.info", map[string]any{"id": id}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CollectionDocuments returns the nested document tree for a collection in the
// same order Outline renders its own sidebar. Each node carries only the minimum
// needed for a nav: id/title/url + children. For full markdown use DocumentInfo.
func (c *Client) CollectionDocuments(ctx context.Context, collectionID string) ([]DocumentNode, error) {
	var out []DocumentNode
	if err := c.post(ctx, "/api/collections.documents", map[string]any{"id": collectionID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DocumentsSearch runs Outline's full-text search. collectionID is optional;
// empty string searches the whole workspace.
func (c *Client) DocumentsSearch(ctx context.Context, query, collectionID string, limit int) ([]DocumentSearchResult, error) {
	body := map[string]any{"query": query}
	if collectionID != "" {
		body["collectionId"] = collectionID
	}
	if limit > 0 {
		body["limit"] = limit
	}
	var out []DocumentSearchResult
	if err := c.post(ctx, "/api/documents.search", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
