package glpi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client talks to a GLPI instance. Auth is layered:
//  1. App-Token header identifies the sshcm application (instance-wide, single value).
//  2. A per-request Authorization: user_token <userToken> starts a session via initSession.
//  3. Subsequent calls use Session-Token: <sessionToken> returned by (2).
//
// Sessions have an expiry on GLPI's side. We cache them keyed by profile id (see
// SessionCache below) and auto-refresh on 401 responses.
type Client struct {
	baseURL    string
	appToken   string
	httpClient *http.Client
}

// NewClient constructs a client. baseURL should include scheme and end with /apirest.php
// (or just the domain — we normalise). Providing an empty appToken is valid for GLPI
// instances that disable the App-Token requirement, but most do require one.
func NewClient(baseURL, appToken string) *Client {
	return &Client{
		baseURL:  normaliseBaseURL(baseURL),
		appToken: appToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) BaseURL() string { return c.baseURL }
func (c *Client) WebBaseURL() string {
	// Strip "/apirest.php" to get the browser-facing URL for deep-links.
	return strings.TrimSuffix(c.baseURL, "/apirest.php")
}

func normaliseBaseURL(s string) string {
	s = strings.TrimRight(s, "/")
	if !strings.HasSuffix(s, "/apirest.php") {
		s = s + "/apirest.php"
	}
	return s
}

// do executes the request. sessionToken may be empty for endpoints that only
// need the App-Token (like initSession). Non-2xx responses are surfaced with the
// GLPI-provided error body so callers can map them into friendly messages.
//
// We append `sanitize=false` to every GET so GLPI 10+ returns raw HTML in text
// fields instead of entity-encoded (&lt;div&gt;…). Older GLPI ignores the flag.
// For POST/PUT we don't want to strip the sanitizer since the field values
// usually come from sshcm and don't need un-encoding.
func (c *Client) do(ctx context.Context, method, path, sessionToken string, query url.Values, body any, out any) error {
	if method == "GET" {
		if query == nil {
			query = url.Values{}
		}
		if query.Get("sanitize") == "" {
			query.Set("sanitize", "false")
		}
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
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.appToken != "" {
		req.Header.Set("App-Token", c.appToken)
	}
	if sessionToken != "" {
		req.Header.Set("Session-Token", sessionToken)
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
		return fmt.Errorf("glpi api %s %s: %d %s", method, path, resp.StatusCode, truncate(string(respBody), 500))
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

// InitSession exchanges a user_token for a session_token. GLPI's response is
// {"session_token":"..."}. Used by SessionCache — callers normally don't call this directly.
func (c *Client) InitSession(ctx context.Context, userToken string) (string, error) {
	// Use Authorization: user_token <t> — this is the init-only form, per GLPI docs.
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/initSession", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "user_token "+userToken)
	if c.appToken != "" {
		req.Header.Set("App-Token", c.appToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("glpi initSession: %d %s", resp.StatusCode, truncate(string(body), 300))
	}
	var out initSessionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parse initSession: %w", err)
	}
	if out.SessionToken == "" {
		return "", fmt.Errorf("glpi initSession: empty session_token")
	}
	return out.SessionToken, nil
}

// KillSession invalidates a session on the GLPI side. Best-effort; errors are logged by callers.
func (c *Client) KillSession(ctx context.Context, sessionToken string) error {
	return c.do(ctx, "GET", "/killSession", sessionToken, nil, nil, nil)
}

// GetMyProfiles returns the profiles available to the authenticated session —
// useful on Test-connection so admins see who the token belongs to.
func (c *Client) GetMyProfiles(ctx context.Context, sessionToken string) (*MyProfilesResponse, error) {
	var out MyProfilesResponse
	if err := c.do(ctx, "GET", "/getMyProfiles", sessionToken, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get is a generic passthrough for GET endpoints the handlers need that aren't
// otherwise wrapped — e.g. relation paths like /Computer/{id}/Ticket. Prefer
// the typed helpers above when they exist.
func (c *Client) Get(ctx context.Context, sessionToken, path string, out any) error {
	return c.do(ctx, "GET", path, sessionToken, nil, nil, out)
}

// GetTicket fetches one ticket by id. Returns a pointer so callers can distinguish
// "not found" (404 mapped to error) from "empty fields".
func (c *Client) GetTicket(ctx context.Context, sessionToken string, id int) (*Ticket, error) {
	var t Ticket
	path := fmt.Sprintf("/Ticket/%d", id)
	if err := c.do(ctx, "GET", path, sessionToken, nil, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTicketFollowups returns the ITILFollowup records attached to a ticket —
// the thread of comments/messages visible to the requester.
func (c *Client) GetTicketFollowups(ctx context.Context, sessionToken string, ticketID int) ([]Followup, error) {
	var out []Followup
	path := fmt.Sprintf("/Ticket/%d/ITILFollowup", ticketID)
	if err := c.do(ctx, "GET", path, sessionToken, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTicketTasks returns TicketTask records — planned/completed work items.
func (c *Client) GetTicketTasks(ctx context.Context, sessionToken string, ticketID int) ([]Task, error) {
	var out []Task
	path := fmt.Sprintf("/Ticket/%d/TicketTask", ticketID)
	if err := c.do(ctx, "GET", path, sessionToken, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTicketSolutions returns ITILSolution records — the proposed/accepted
// resolution text, if any.
func (c *Client) GetTicketSolutions(ctx context.Context, sessionToken string, ticketID int) ([]Solution, error) {
	var out []Solution
	path := fmt.Sprintf("/Ticket/%d/ITILSolution", ticketID)
	if err := c.do(ctx, "GET", path, sessionToken, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDocumentBinary streams the bytes for an uploaded GLPI Document (attachment
// or inline image). GLPI requires an authenticated session, so the caller
// passes the session token it already manages. Returns the live response —
// callers own Body.Close(). Content-Type/Disposition headers on the response
// are what the HTTP proxy handler forwards to its own client.
func (c *Client) GetDocumentBinary(ctx context.Context, sessionToken string, docID int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/Document/%d", c.baseURL, docID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	if c.appToken != "" {
		req.Header.Set("App-Token", c.appToken)
	}
	if sessionToken != "" {
		req.Header.Set("Session-Token", sessionToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("glpi document %d: %d %s", docID, resp.StatusCode, truncate(string(body), 300))
	}
	return resp, nil
}

// SearchDropdownItems returns items of an arbitrary GLPI itemtype, optionally
// filtered by a name substring. Used by the Formcreator dropdown/glpiselect/
// itemtype question pickers. Itemtype must be a safe identifier — callers are
// expected to allow-list the value against something like Entity, ITILCategory,
// Location, Supplier, User, Computer, … before passing it in.
//
// Returns raw maps so the handler can pick the right display column per type
// (User uses firstname+realname; most others use name or completename).
func (c *Client) SearchDropdownItems(ctx context.Context, sessionToken, itemtype, query string, rangeStart, rangeEnd int) ([]map[string]any, error) {
	q := url.Values{}
	// field id 1 = "name" for most itemtypes. Formcreator's own pickers use the
	// same default, so this matches the web UI's behaviour.
	if query != "" {
		q.Set("criteria[0][field]", "1")
		q.Set("criteria[0][searchtype]", "contains")
		q.Set("criteria[0][value]", query)
	}
	q.Set("forcedisplay[0]", "2")  // id
	q.Set("forcedisplay[1]", "1")  // name
	q.Set("forcedisplay[2]", "80") // entity (when applicable)
	q.Set("range", fmt.Sprintf("%d-%d", rangeStart, rangeEnd))
	path := "/search/" + itemtype
	var raw struct {
		Totalcount int              `json:"totalcount"`
		Data       []map[string]any `json:"data"`
	}
	if err := c.do(ctx, "GET", path, sessionToken, q, nil, &raw); err != nil {
		return nil, err
	}
	return raw.Data, nil
}

// SearchUsers is a focused helper for actor-style pickers. Unlike the generic
// SearchDropdownItems, it requests the columns (firstname, realname, name)
// Formcreator's actor field renders.
func (c *Client) SearchUsers(ctx context.Context, sessionToken, query string, rangeStart, rangeEnd int) ([]map[string]any, error) {
	q := url.Values{}
	if query != "" {
		// Field 1 = name on User. Easier to combine with firstname/realname here.
		q.Set("criteria[0][field]", "1")
		q.Set("criteria[0][searchtype]", "contains")
		q.Set("criteria[0][value]", query)
	}
	q.Set("forcedisplay[0]", "2")  // id
	q.Set("forcedisplay[1]", "1")  // name (login)
	q.Set("forcedisplay[2]", "9")  // firstname
	q.Set("forcedisplay[3]", "34") // realname
	q.Set("forcedisplay[4]", "5")  // email
	q.Set("range", fmt.Sprintf("%d-%d", rangeStart, rangeEnd))
	var raw struct {
		Data []map[string]any `json:"data"`
	}
	if err := c.do(ctx, "GET", "/search/User", sessionToken, q, nil, &raw); err != nil {
		return nil, err
	}
	return raw.Data, nil
}

// UploadDocumentMultipart creates a GLPI Document by posting a multipart body
// (the only flow GLPI's REST accepts for attaching real file bytes). Returns
// the new document id. The Formcreator file question's answer is the list of
// these ids.
//
// We accept a reader + filename + mime so callers don't have to buffer huge
// files in memory more than once.
func (c *Client) UploadDocumentMultipart(ctx context.Context, sessionToken, filename, mimeType string, fileReader io.Reader) (int, error) {
	if filename == "" {
		return 0, fmt.Errorf("filename required")
	}
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	// Part 1: JSON manifest ("uploadManifest") telling GLPI what fields to set.
	manifest, err := mw.CreateFormField("uploadManifest")
	if err != nil {
		return 0, err
	}
	payload := map[string]any{
		"input": map[string]any{
			"name":       filename,
			"_filename":  []string{filename},
		},
	}
	if err := json.NewEncoder(manifest).Encode(payload); err != nil {
		return 0, err
	}

	// Part 2: the file bytes, keyed as "filename[0]".
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="filename[0]"; filename="%s"`, escapeQuotes(filename)))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	h.Set("Content-Type", mimeType)
	part, err := mw.CreatePart(h)
	if err != nil {
		return 0, err
	}
	if _, err := io.Copy(part, fileReader); err != nil {
		return 0, err
	}
	if err := mw.Close(); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/Document", body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	if c.appToken != "" {
		req.Header.Set("App-Token", c.appToken)
	}
	if sessionToken != "" {
		req.Header.Set("Session-Token", sessionToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("glpi Document upload: %d %s", resp.StatusCode, truncate(string(respBody), 300))
	}
	// Response can be either {"id":N, ...} (single) or [{"id":N, ...}] (array — GLPI wraps uploads).
	var single struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(respBody, &single); err == nil && single.ID > 0 {
		return single.ID, nil
	}
	var list []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(respBody, &list); err == nil && len(list) > 0 && list[0].ID > 0 {
		return list[0].ID, nil
	}
	return 0, fmt.Errorf("glpi Document upload: no id in response: %s", truncate(string(respBody), 200))
}

func escapeQuotes(s string) string { return strings.ReplaceAll(s, `"`, `\"`) }

// GetUser resolves a GLPI User so we can render display names on followups etc.
// Returns nil + nil if the user can't be read (permission denied) so callers
// can gracefully fall back to the numeric id.
func (c *Client) GetUser(ctx context.Context, sessionToken string, id int) (*GlpiUser, error) {
	if id <= 0 {
		return nil, nil
	}
	var u GlpiUser
	path := fmt.Sprintf("/User/%d", id)
	if err := c.do(ctx, "GET", path, sessionToken, nil, nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// SearchTickets runs GLPI's search API scoped to the Ticket itemtype.
// criteria is a list of {field_id, searchtype, value} maps. We accept the list
// pre-built to keep the client agnostic of GLPI's numeric field ids.
// When both entityID and linkedComputerID are >0 the ticket must match both.
func (c *Client) SearchTickets(ctx context.Context, sessionToken string, criteria []map[string]string, rangeStart, rangeEnd int) ([]Ticket, error) {
	q := url.Values{}
	for i, cr := range criteria {
		for k, v := range cr {
			q.Add(fmt.Sprintf("criteria[%d][%s]", i, k), v)
		}
	}
	// Columns we want surfaced in the result — match the Ticket struct fields.
	// GLPI's search output formatting is quirky; easier: pass forcedisplay for the
	// important ids and decode from the map-array it returns.
	q.Add("forcedisplay[0]", "2")  // id
	q.Add("forcedisplay[1]", "1")  // name
	q.Add("forcedisplay[2]", "12") // status
	q.Add("forcedisplay[3]", "3")  // priority
	q.Add("forcedisplay[4]", "15") // date
	q.Add("forcedisplay[5]", "80") // entities_id
	// Sort newest-first — sshcm surfaces open tickets by creation date.
	q.Add("sort", "15")
	q.Add("order", "DESC")
	q.Add("range", fmt.Sprintf("%d-%d", rangeStart, rangeEnd))

	var raw struct {
		Totalcount int                       `json:"totalcount"`
		Data       []map[string]any          `json:"data"`
	}
	if err := c.do(ctx, "GET", "/search/Ticket", sessionToken, q, nil, &raw); err != nil {
		return nil, err
	}

	// Map GLPI field ids back into Ticket fields.
	out := make([]Ticket, 0, len(raw.Data))
	for _, row := range raw.Data {
		t := Ticket{}
		if v, ok := row["2"]; ok {
			t.ID = toInt(v)
		}
		if v, ok := row["1"]; ok {
			t.Name = toString(v)
		}
		if v, ok := row["12"]; ok {
			t.Status = toInt(v)
		}
		if v, ok := row["3"]; ok {
			t.Priority = toInt(v)
		}
		if v, ok := row["15"]; ok {
			t.Date = toString(v)
		}
		if v, ok := row["80"]; ok {
			t.EntitiesID = toInt(v)
		}
		out = append(out, t)
	}
	return out, nil
}

// CreateTicket POSTs a new ticket. GLPI accepts the fields wrapped in {"input": ...}.
// Returns the newly created ticket id.
func (c *Client) CreateTicket(ctx context.Context, sessionToken string, in TicketCreateInput) (int, error) {
	body := struct {
		Input TicketCreateInput `json:"input"`
	}{Input: in}
	var out struct {
		ID int `json:"id"`
	}
	if err := c.do(ctx, "POST", "/Ticket", sessionToken, nil, body, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// ListEntities returns the entities the session can see.
func (c *Client) ListEntities(ctx context.Context, sessionToken string) ([]Entity, error) {
	var out []Entity
	q := url.Values{}
	q.Set("range", "0-200")
	if err := c.do(ctx, "GET", "/Entity", sessionToken, q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindComputerByName searches for a Computer asset by name. Returns nil if not found.
func (c *Client) FindComputerByName(ctx context.Context, sessionToken, name string) (*Computer, error) {
	// Use /search/Computer with name criteria. GLPI field_id 1 = name.
	q := url.Values{}
	q.Add("criteria[0][field]", "1")
	q.Add("criteria[0][searchtype]", "equals")
	q.Add("criteria[0][value]", name)
	q.Add("forcedisplay[0]", "2") // id
	q.Add("forcedisplay[1]", "1") // name
	q.Add("forcedisplay[2]", "80") // entities_id
	q.Add("range", "0-1")
	var raw struct {
		Totalcount int              `json:"totalcount"`
		Data       []map[string]any `json:"data"`
	}
	if err := c.do(ctx, "GET", "/search/Computer", sessionToken, q, nil, &raw); err != nil {
		return nil, err
	}
	if raw.Totalcount == 0 || len(raw.Data) == 0 {
		return nil, nil
	}
	row := raw.Data[0]
	return &Computer{
		ID:         toInt(row["2"]),
		Name:       toString(row["1"]),
		EntitiesID: toInt(row["80"]),
	}, nil
}

// LinkComputerToTicket attaches a Computer asset to a Ticket via Ticket_Item.
// Best-effort — on failure we still return the ticket ID so the caller can link
// sshcm-side and degrade gracefully.
func (c *Client) LinkComputerToTicket(ctx context.Context, sessionToken string, ticketID, computerID int) error {
	body := struct {
		Input struct {
			TicketsID int    `json:"tickets_id"`
			Itemtype  string `json:"itemtype"`
			ItemsID   int    `json:"items_id"`
		} `json:"input"`
	}{}
	body.Input.TicketsID = ticketID
	body.Input.Itemtype = "Computer"
	body.Input.ItemsID = computerID
	return c.do(ctx, "POST", "/Ticket_Item", sessionToken, nil, body, nil)
}

// toInt is a defensive converter for GLPI's search API results: field values
// come back as strings, float64 (json numbers), or sometimes nested objects.
func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n := 0
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%g", x)
	case int:
		return fmt.Sprintf("%d", x)
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// sessionEntry holds a cached session-token for a given profile.
type sessionEntry struct {
	token   string
	expires time.Time
}

// SessionCache is an in-memory map of profile-id → session-token. Safe for
// concurrent use. The TTL is intentionally short of GLPI's own (GLPI defaults to
// a few hours) so we refresh on our own schedule, and we'll still re-initSession
// on any 401 returned downstream as a belt-and-suspenders.
type SessionCache struct {
	mu      sync.Mutex
	entries map[int64]sessionEntry
	ttl     time.Duration
}

// NewSessionCache returns a cache with the given TTL. 30 minutes is plenty —
// re-authenticating is cheap.
func NewSessionCache(ttl time.Duration) *SessionCache {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &SessionCache{entries: make(map[int64]sessionEntry), ttl: ttl}
}

// Get returns a cached session-token, initialising one if absent or expired.
// The tokenFetcher callback is invoked when a refresh is needed; it receives
// the profile id and must return the profile's decrypted user_token.
func (s *SessionCache) Get(ctx context.Context, client *Client, profileID int64, tokenFetcher func(int64) (string, error)) (string, error) {
	s.mu.Lock()
	e, ok := s.entries[profileID]
	s.mu.Unlock()
	if ok && time.Now().Before(e.expires) {
		return e.token, nil
	}

	userTok, err := tokenFetcher(profileID)
	if err != nil {
		return "", fmt.Errorf("load user token: %w", err)
	}
	if userTok == "" {
		return "", fmt.Errorf("profile has no user token configured")
	}
	sess, err := client.InitSession(ctx, userTok)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.entries[profileID] = sessionEntry{token: sess, expires: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return sess, nil
}

// Invalidate drops the cached session for a profile (e.g. after a 401 response).
func (s *SessionCache) Invalidate(profileID int64) {
	s.mu.Lock()
	delete(s.entries, profileID)
	s.mu.Unlock()
}

// Clear drops all cached sessions. Used on settings changes (e.g. base URL or App-Token updated).
func (s *SessionCache) Clear() {
	s.mu.Lock()
	s.entries = make(map[int64]sessionEntry)
	s.mu.Unlock()
}
