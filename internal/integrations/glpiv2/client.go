package glpiv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client talks to GLPI's High-Level REST API (api.php/v2.x) over OAuth2
// (resource-owner password grant). Unlike the legacy glpi package, there is
// no per-profile user identity here — one service account (super-admin, per
// the operator) backs every call, so there is no SessionCache keyed by
// profile id, just a single cached access token.
type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	username     string
	password     string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
	meID        int // cached id of the authenticated service account (0 = not resolved yet)
}

// NewClient builds a client from Settings. Returns nil if the integration
// isn't fully configured/enabled — mirrors glpi.NewServiceClient's contract.
func NewClient(s Settings) *Client {
	if !s.Enabled {
		return nil
	}
	return &Client{
		baseURL:      s.BaseURL,
		clientID:     s.ClientID,
		clientSecret: s.ClientSecret,
		username:     s.Username,
		password:     s.Password,
		httpClient:   &http.Client{Timeout: 20 * time.Second},
	}
}

// tokenResponse is the standard OAuth2 token endpoint shape (RFC 6749).
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// accessTokenFor returns a cached bearer token, refreshing via the password
// grant when absent or within 30s of expiry.
func (c *Client) accessTokenFor(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.accessToken != "" && time.Now().Add(30*time.Second).Before(c.expiresAt) {
		tok := c.accessToken
		c.mu.Unlock()
		return tok, nil
	}
	c.mu.Unlock()

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("username", c.username)
	form.Set("password", c.password)
	// "user" is required for /Administration/User/Me (used to resolve the
	// service account's own id so it can be set as ticket requester).
	form.Set("scope", "api user")

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("glpi-sead token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("glpi-sead token: %d %s", resp.StatusCode, truncate(string(body), 300))
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("glpi-sead token: parse response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("glpi-sead token: empty access_token")
	}

	c.mu.Lock()
	c.accessToken = tok.AccessToken
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.expiresAt = time.Now().Add(ttl)
	c.mu.Unlock()

	return tok.AccessToken, nil
}

// do executes an authenticated request against the v2 API. On a 401 it
// retries once after forcing a fresh token (covers server-side token
// revocation between our cached expiry and GLPI's own).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doAttempt(ctx, method, path, query, body, out, true)
}

func (c *Client) doAttempt(ctx context.Context, method, path string, query url.Values, body any, out any, retryOn401 bool) error {
	tok, err := c.accessTokenFor(ctx)
	if err != nil {
		return err
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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

	if resp.StatusCode == 401 && retryOn401 {
		c.mu.Lock()
		c.accessToken = ""
		c.mu.Unlock()
		return c.doAttempt(ctx, method, path, query, body, out, false)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("glpi-sead api %s %s: %d %s", method, path, resp.StatusCode, truncate(string(respBody), 500))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// decodeList accepts either a bare JSON array or {"items":[...]} — GLPI v2
// deployments have been observed emitting both depending on version/config.
func decodeList[T any](raw json.RawMessage) ([]T, error) {
	var arr []T
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var env listEnvelope[T]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode list: %w", err)
	}
	return env.Items, nil
}

// ListTickets runs GET /Assistance/Ticket with an optional RSQL filter
// (pass "" for none) and start/limit pagination.
func (c *Client) ListTickets(ctx context.Context, filter string, start, limit int) ([]Ticket, error) {
	q := url.Values{}
	if filter != "" {
		q.Set("filter", filter)
	}
	q.Set("start", fmt.Sprintf("%d", start))
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "date:desc")
	var raw json.RawMessage
	if err := c.do(ctx, "GET", "/Assistance/Ticket", q, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[Ticket](raw)
}

// GetTicket fetches one ticket by id.
func (c *Client) GetTicket(ctx context.Context, id int) (*Ticket, error) {
	var t Ticket
	if err := c.do(ctx, "GET", fmt.Sprintf("/Assistance/Ticket/%d", id), nil, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTicket POSTs a new ticket, returning its id.
func (c *Client) CreateTicket(ctx context.Context, in TicketCreateInput) (int, error) {
	var out struct {
		ID int `json:"id"`
	}
	if err := c.do(ctx, "POST", "/Assistance/Ticket", nil, in, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// GetTicketTimeline returns the raw timeline entries (followups, tasks,
// solutions, documents, validations interleaved) — GLPI v2 doesn't publish a
// fixed schema for this endpoint, so callers inspect the `itemtype` field
// present on each row.
func (c *Client) GetTicketTimeline(ctx context.Context, ticketID int) ([]map[string]any, error) {
	var raw json.RawMessage
	if err := c.do(ctx, "GET", fmt.Sprintf("/Assistance/Ticket/%d/Timeline", ticketID), nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[map[string]any](raw)
}

// AddFollowup posts a new ITILFollowup on the ticket's timeline.
func (c *Client) AddFollowup(ctx context.Context, ticketID int, content string, isPrivate bool) (int, error) {
	var out struct {
		ID int `json:"id"`
	}
	path := fmt.Sprintf("/Assistance/Ticket/%d/Timeline/Followup", ticketID)
	if err := c.do(ctx, "POST", path, nil, FollowupInput{Content: content, IsPrivate: isPrivate}, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// MeID returns the GLPI user id of the authenticated service account
// (cached after the first lookup — it never changes for a given credential).
func (c *Client) MeID(ctx context.Context) (int, error) {
	c.mu.Lock()
	if c.meID != 0 {
		id := c.meID
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()

	var me struct {
		ID int `json:"id"`
	}
	if err := c.do(ctx, "GET", "/Administration/User/Me", nil, nil, &me); err != nil {
		return 0, err
	}
	c.mu.Lock()
	c.meID = me.ID
	c.mu.Unlock()
	return me.ID, nil
}

// AddTeamMember attaches a requester/observer/assignee to a ticket.
// actorType is "User"|"Group"|"Supplier", role is
// "requester"|"observer"|"assigned".
func (c *Client) AddTeamMember(ctx context.Context, ticketID int, actorType string, actorID int, role string) error {
	body := struct {
		Type string `json:"type"`
		ID   int    `json:"id"`
		Role string `json:"role"`
	}{Type: actorType, ID: actorID, Role: role}
	path := fmt.Sprintf("/Assistance/Ticket/%d/TeamMember", ticketID)
	return c.do(ctx, "POST", path, nil, body, nil)
}

// GetTicketTeam returns the ticket's requesters/observers/assignees.
func (c *Client) GetTicketTeam(ctx context.Context, ticketID int) ([]TeamMember, error) {
	var raw json.RawMessage
	if err := c.do(ctx, "GET", fmt.Sprintf("/Assistance/Ticket/%d/TeamMember", ticketID), nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[TeamMember](raw)
}

// nameFilter builds an RSQL "name contains query" filter, or "" for an empty
// query (no filtering — caller still gets the first page).
func nameFilter(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	q = strings.ReplaceAll(q, `"`, `\"`)
	return fmt.Sprintf(`name=like="*%s*"`, q)
}

// SearchDropdown queries /Dropdowns/{itemtype} (ITILCategory, Location,
// RequestType, TaskCategory, ...) by name substring.
func (c *Client) SearchDropdown(ctx context.Context, itemtype, query string, limit int) ([]DropdownItem, error) {
	q := url.Values{}
	if f := nameFilter(query); f != "" {
		q.Set("filter", f)
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	var raw json.RawMessage
	if err := c.do(ctx, "GET", "/Dropdowns/"+itemtype, q, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[DropdownItem](raw)
}

// SearchAdministration queries /Administration/{itemtype} (User, Entity,
// Group) by name substring — same shape as SearchDropdown, different
// top-level namespace in the v2 API.
func (c *Client) SearchAdministration(ctx context.Context, itemtype, query string, limit int) ([]DropdownItem, error) {
	q := url.Values{}
	if f := nameFilter(query); f != "" {
		q.Set("filter", f)
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	var raw json.RawMessage
	if err := c.do(ctx, "GET", "/Administration/"+itemtype, q, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[DropdownItem](raw)
}
