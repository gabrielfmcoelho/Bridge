// Package nvd reads one CVE record from the NVD CVE API 2.0. No API key:
// the public limit (5 requests / 30s) is plenty for on-demand analysis.
package nvd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the public NVD CVE API endpoint.
const DefaultBaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

// ErrNotFound means NVD has no record for the id.
var ErrNotFound = errors.New("CVE not found in NVD")

// Client fetches CVE records. BaseURL is a field so tests can point it at a
// fake server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for the public NVD API.
func New() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// CVE is the subset of an NVD record the analysis uses.
type CVE struct {
	ID          string
	Description string
	CVSS        *float64
	CVSSVector  string
	CWE         string
	Published   *time.Time
	References  []string
}

const maxRefs = 20

type cvssMetric struct {
	CVSSData struct {
		BaseScore    float64 `json:"baseScore"`
		VectorString string  `json:"vectorString"`
	} `json:"cvssData"`
}

type response struct {
	TotalResults    int `json:"totalResults"`
	Vulnerabilities []struct {
		CVE struct {
			ID           string `json:"id"`
			Published    string `json:"published"`
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			Metrics struct {
				V40 []cvssMetric `json:"cvssMetricV40"`
				V31 []cvssMetric `json:"cvssMetricV31"`
				V30 []cvssMetric `json:"cvssMetricV30"`
				V2  []cvssMetric `json:"cvssMetricV2"`
			} `json:"metrics"`
			Weaknesses []struct {
				Description []struct {
					Value string `json:"value"`
				} `json:"description"`
			} `json:"weaknesses"`
			References []struct {
				URL string `json:"url"`
			} `json:"references"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

// CVE fetches one record by id ("CVE-2024-1234").
func (c *Client) CVE(ctx context.Context, id string) (*CVE, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?cveId="+url.QueryEscape(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nvd: HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	var r response
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("nvd: decode: %w", err)
	}
	if r.TotalResults == 0 || len(r.Vulnerabilities) == 0 {
		return nil, ErrNotFound
	}
	v := r.Vulnerabilities[0].CVE
	out := &CVE{ID: v.ID}
	for _, d := range v.Descriptions {
		if d.Lang == "en" {
			out.Description = d.Value
			break
		}
	}
	// Newest CVSS version wins.
	for _, ms := range [][]cvssMetric{v.Metrics.V40, v.Metrics.V31, v.Metrics.V30, v.Metrics.V2} {
		if len(ms) > 0 {
			score := ms[0].CVSSData.BaseScore
			out.CVSS, out.CVSSVector = &score, ms[0].CVSSData.VectorString
			break
		}
	}
	// NVD-CWE-Other / NVD-CWE-noinfo are placeholders, not weaknesses.
	for _, w := range v.Weaknesses {
		for _, d := range w.Description {
			if out.CWE == "" && strings.HasPrefix(d.Value, "CWE-") {
				out.CWE = d.Value
			}
		}
	}
	for _, ref := range v.References {
		if len(out.References) == maxRefs {
			break
		}
		out.References = append(out.References, ref.URL)
	}
	// NVD timestamps carry no zone; they are UTC.
	if t, err := time.Parse("2006-01-02T15:04:05.000", v.Published); err == nil {
		out.Published = &t
	} else if t, err := time.Parse(time.RFC3339, v.Published); err == nil {
		out.Published = &t
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
