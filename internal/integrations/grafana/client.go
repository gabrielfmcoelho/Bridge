package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client talks to a Grafana OSS HTTP API using a service account / API token.
// The same instance is safe to share across goroutines.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient constructs a Grafana client. The baseURL should include the scheme
// but NOT a trailing slash — e.g. "https://grafana.example.org".
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("grafana api %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, resp.StatusCode, nil
}

// Health hits /api/health — the cheapest check that the base URL is reachable
// and serves Grafana. Does NOT require authentication but we still send the
// token so a 401 surfaces configuration errors early.
func (c *Client) Health(ctx context.Context) (*Health, error) {
	body, _, err := c.do(ctx, "GET", "/api/health", nil)
	if err != nil {
		return nil, err
	}
	var h Health
	if err := json.Unmarshal(body, &h); err != nil {
		return nil, fmt.Errorf("parse health: %w", err)
	}
	return &h, nil
}

// CurrentUser returns the user/service-account the API token is bound to.
// /api/user requires authentication — so a success here proves the token works.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	body, _, err := c.do(ctx, "GET", "/api/user", nil)
	if err != nil {
		return nil, err
	}
	var u User
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}
	return &u, nil
}

// CreateOrUpdateDashboard uploads a dashboard model to Grafana, always with overwrite=true
// so re-provisioning is idempotent. The caller is responsible for building the dashboard
// JSON (via RenderHostDashboard / RenderServiceDashboard).
func (c *Client) CreateOrUpdateDashboard(ctx context.Context, dashboardJSON []byte) (*DashboardCreateResponse, error) {
	// Grafana's /api/dashboards/db expects {dashboard: <model>, folderId: 0, overwrite: true, message: "..."}
	// We can't just embed the rendered bytes as json.RawMessage without tagging because
	// bytes.NewReader won't serialize our envelope — so build the envelope as a struct.
	envelope := struct {
		Dashboard json.RawMessage `json:"dashboard"`
		FolderID  int             `json:"folderId"`
		Overwrite bool            `json:"overwrite"`
		Message   string          `json:"message"`
	}{
		Dashboard: json.RawMessage(dashboardJSON),
		FolderID:  0,
		Overwrite: true,
		Message:   "Provisioned by sshcm",
	}
	body, _, err := c.do(ctx, "POST", "/api/dashboards/db", envelope)
	if err != nil {
		return nil, err
	}
	var resp DashboardCreateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse dashboard response: %w", err)
	}
	return &resp, nil
}

// QueryPrometheusInstant runs a PromQL instant query against a Prometheus datasource
// through Grafana's datasource proxy. Returns parsed vector results; an empty result
// slice means the query ran but matched no series (common when a host is down).
func (c *Client) QueryPrometheusInstant(ctx context.Context, datasourceUID, expr string) (*PrometheusVectorResponse, error) {
	if datasourceUID == "" {
		return nil, fmt.Errorf("datasource UID is required")
	}
	path := fmt.Sprintf("/api/datasources/proxy/uid/%s/api/v1/query?query=%s",
		url.PathEscape(datasourceUID),
		url.QueryEscape(expr),
	)
	body, _, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp PrometheusVectorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse prometheus response: %w", err)
	}
	if resp.Status != "success" {
		detail := resp.Error
		if detail == "" {
			detail = resp.ErrorType
		}
		return nil, fmt.Errorf("prometheus query failed: %s", detail)
	}
	return &resp, nil
}

// FirstScalar extracts the first sample's value as a float64 from a vector response.
// Returns (nil, nil) when the result set is empty (no matching series — caller
// treats as "no data" rather than error).
func FirstScalar(r *PrometheusVectorResponse) (*float64, error) {
	if r == nil || len(r.Data.Result) == 0 {
		return nil, nil
	}
	v := r.Data.Result[0].Value
	// Prometheus returns [unix_time_float, "string_value"].
	s, ok := v[1].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T in prometheus sample", v[1])
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return nil, fmt.Errorf("parse prometheus value %q: %w", s, err)
	}
	return &f, nil
}
