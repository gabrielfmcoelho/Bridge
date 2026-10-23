// Package endoflife reads release cycles from endoflife.date: when each
// version line of a product (php 7.4, nginx 1.29, debian 11) stops being
// supported upstream.
package endoflife

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the public endoflife.date API.
const DefaultBaseURL = "https://endoflife.date/api"

// Client reads endoflife.date. BaseURL is a field so tests can fake it.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for the public API.
func New() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Cycle is one release line. EOL is "" while supported with no date, a
// "YYYY-MM-DD" date, or "true" when ended with no date given.
type Cycle struct {
	Cycle string `json:"cycle"`
	EOL   string `json:"eol"`
}

// Cycles lists a product's release lines, newest first.
func (c *Client) Cycles(ctx context.Context, product string) ([]Cycle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+url.PathEscape(product)+".json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endoflife: %s: HTTP %d", product, resp.StatusCode)
	}
	var raw []struct {
		Cycle json.RawMessage `json:"cycle"`
		EOL   json.RawMessage `json:"eol"` // a date string or a bool
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("endoflife: decode: %w", err)
	}
	out := make([]Cycle, 0, len(raw))
	for _, r := range raw {
		cy := Cycle{Cycle: strings.Trim(string(r.Cycle), `"`)}
		switch e := strings.Trim(string(r.EOL), `"`); e {
		case "false", "":
		default:
			cy.EOL = e // "true" or a date
		}
		out = append(out, cy)
	}
	return out, nil
}

// Status says whether version is past its cycle's end of life on day now.
// The cycle is the longest one that prefixes the version on a component
// boundary ("1.1.1w" → "1.1.1", "18.19.0" → "18", never "1.1.10" → "1.1.1").
// ok is false when no cycle matches.
func Status(cycles []Cycle, version string, now time.Time) (eol bool, date string, ok bool) {
	best := -1
	for i, c := range cycles {
		if !prefixCycle(version, c.Cycle) {
			continue
		}
		if best < 0 || len(c.Cycle) > len(cycles[best].Cycle) {
			best = i
		}
	}
	if best < 0 {
		return false, "", false
	}
	switch e := cycles[best].EOL; e {
	case "":
		return false, "", true
	case "true":
		return true, "", true
	default:
		t, err := time.Parse("2006-01-02", e)
		if err != nil {
			return false, "", true
		}
		return !now.Before(t), e, true
	}
}

func prefixCycle(version, cycle string) bool {
	if cycle == "" || !strings.HasPrefix(version, cycle) {
		return false
	}
	if len(version) == len(cycle) {
		return true
	}
	next := version[len(cycle)]
	return next < '0' || next > '9'
}
