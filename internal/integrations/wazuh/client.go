// Package wazuh reads vulnerability states from a Wazuh 4.8+ indexer
// (OpenSearch, index pattern wazuh-states-vulnerabilities-*). Read-only:
// the indexer user only needs read on that index pattern.
package wazuh

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Index is the vulnerability-state index pattern of Wazuh 4.8+.
const Index = "wazuh-states-vulnerabilities-*"

const pageSize = 1000

// Client talks to the Wazuh indexer with HTTP basic auth.
type Client struct {
	baseURL    string
	user, pass string
	httpClient *http.Client
}

// NewClient builds a client for the indexer at baseURL
// (e.g. https://wazuh-indexer:9200). skipVerify accepts the indexer's
// default self-signed certificate.
func NewClient(baseURL, user, pass string, skipVerify bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // admin opt-in, self-signed indexer
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		user:       user,
		pass:       pass,
		httpClient: &http.Client{Timeout: 60 * time.Second, Transport: tr},
	}
}

// Hit is one vulnerability state: a CVE affecting a package on an agent.
type Hit struct {
	Agent struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"agent"`
	Package struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"package"`
	Vulnerability struct {
		ID          string `json:"id"`
		Severity    string `json:"severity"`
		Description string `json:"description"`
		Reference   string `json:"reference"`
		Score       struct {
			Base *float64 `json:"base"`
		} `json:"score"`
	} `json:"vulnerability"`
}

var sourceFields = []string{
	"agent.id", "agent.name", "package.name", "package.version",
	"vulnerability.id", "vulnerability.severity", "vulnerability.description",
	"vulnerability.reference", "vulnerability.score.base",
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("wazuh indexer %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(data), 300))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Count returns how many vulnerability states the indexer holds (the
// connection test).
func (c *Client) Count(ctx context.Context) (int, error) {
	var out struct {
		Count int `json:"count"`
	}
	err := c.post(ctx, "/"+Index+"/_count", map[string]any{"query": map[string]any{"match_all": map[string]any{}}}, &out)
	return out.Count, err
}

type searchPage struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source Hit `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// Vulnerabilities returns every vulnerability state, paging with the scroll
// API. It fails unless the pages add up to the reported total: the sync
// auto-resolves whatever is missing, so a partial read must never pass.
func (c *Client) Vulnerabilities(ctx context.Context) ([]Hit, error) {
	var page searchPage
	if err := c.post(ctx, "/"+Index+"/_search?scroll=2m", map[string]any{
		"size":             pageSize,
		"track_total_hits": true,
		"_source":          sourceFields,
		"query":            map[string]any{"match_all": map[string]any{}},
	}, &page); err != nil {
		return nil, err
	}
	total := page.Hits.Total.Value
	var out []Hit
	defer func() {
		if page.ScrollID != "" {
			// Best effort: an unreleased scroll expires on its own after 2m.
			_ = c.del(context.WithoutCancel(ctx), page.ScrollID)
		}
	}()
	for {
		for _, h := range page.Hits.Hits {
			out = append(out, h.Source)
		}
		if len(page.Hits.Hits) == 0 || len(out) >= total {
			break
		}
		id := page.ScrollID
		page = searchPage{}
		if err := c.post(ctx, "/_search/scroll", map[string]any{"scroll": "2m", "scroll_id": id}, &page); err != nil {
			return nil, err
		}
	}
	if len(out) != total {
		return nil, fmt.Errorf("wazuh indexer returned %d of %d vulnerabilities", len(out), total)
	}
	return out, nil
}

func (c *Client) del(ctx context.Context, scrollID string) error {
	b, _ := json.Marshal(map[string]any{"scroll_id": scrollID})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/_search/scroll", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
