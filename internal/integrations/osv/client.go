// Package osv queries OSV.dev (https://osv.dev): which advisories affect a
// distro package version (Debian, Ubuntu, Alpine, Rocky, AlmaLinux…), with
// the distro's own fixed versions — so backported fixes are not reported.
// No API key. Only ecosystem, package name and version are sent.
package osv

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

// DefaultBaseURL is the public OSV API.
const DefaultBaseURL = "https://api.osv.dev/v1"

// Client talks to OSV. BaseURL is a field so tests can point it at a fake.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for the public OSV API.
func New() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Query is one package version to look up.
type Query struct {
	Ecosystem string // "Debian:12"
	Name      string // source package
	Version   string
}

// maxBatch is OSV's querybatch limit.
const maxBatch = 1000

type batchQuery struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}

type batchResponse struct {
	Results []struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
		NextPageToken string `json:"next_page_token"`
	} `json:"results"`
}

// QueryBatch returns, per query (same order), the ids of the advisories
// affecting it. Chunks of 1000 and per-query page tokens are handled here.
func (c *Client) QueryBatch(ctx context.Context, qs []Query) ([][]string, error) {
	out := make([][]string, len(qs))
	for start := 0; start < len(qs); start += maxBatch {
		end := min(start+maxBatch, len(qs))
		pending := make([]int, 0, end-start) // indexes into qs still to (re)ask
		tokens := map[int]string{}
		for i := start; i < end; i++ {
			pending = append(pending, i)
		}
		for round := 0; len(pending) > 0 && round < 10; round++ {
			body := struct {
				Queries []batchQuery `json:"queries"`
			}{}
			for _, i := range pending {
				var b batchQuery
				b.Package.Ecosystem, b.Package.Name, b.Version, b.PageToken = qs[i].Ecosystem, qs[i].Name, qs[i].Version, tokens[i]
				body.Queries = append(body.Queries, b)
			}
			var r batchResponse
			if err := c.post(ctx, "/querybatch", body, &r); err != nil {
				return nil, err
			}
			if len(r.Results) != len(pending) {
				return nil, fmt.Errorf("osv: %d results for %d queries", len(r.Results), len(pending))
			}
			var next []int
			for k, res := range r.Results {
				i := pending[k]
				for _, v := range res.Vulns {
					out[i] = append(out[i], v.ID)
				}
				if res.NextPageToken != "" {
					tokens[i] = res.NextPageToken
					next = append(next, i)
				}
			}
			pending = next
		}
	}
	return out, nil
}

// Vuln is the part of an OSV advisory the inventory shows.
type Vuln struct {
	ID       string  `json:"id"`
	CVE      string  `json:"cve"` // the CVE it tracks, or "" when none
	Summary  string  `json:"summary"`
	Severity string  `json:"severity"` // critical | high | medium | low | info
	CVSS     float64 `json:"cvss"`
	// Fixed maps "ecosystem|package" to the first fixed version there.
	Fixed map[string]string `json:"fixed,omitempty"`
}

type vulnRecord struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Aliases  []string `json:"aliases"`
	Upstream []string `json:"upstream"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected []struct {
		Package struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
		Ranges []struct {
			Events []struct {
				Fixed string `json:"fixed"`
			} `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
}

// Vuln fetches one advisory by id.
func (c *Client) Vuln(ctx context.Context, id string) (*Vuln, error) {
	var rec vulnRecord
	if err := c.getJSON(ctx, "/vulns/"+url.PathEscape(id), &rec); err != nil {
		return nil, err
	}
	return fromRecord(rec), nil
}

func fromRecord(rec vulnRecord) *Vuln {
	v := &Vuln{ID: rec.ID, Fixed: map[string]string{}}
	for _, a := range append(append([]string{}, rec.Upstream...), rec.Aliases...) {
		if strings.HasPrefix(a, "CVE-") {
			v.CVE = a
			break
		}
	}
	if v.CVE == "" && strings.HasPrefix(rec.ID, "CVE-") {
		v.CVE = rec.ID
	}
	v.Summary = strings.TrimSpace(rec.Summary)
	if v.Summary == "" {
		v.Summary = strings.TrimSpace(rec.Details)
	}
	if len(v.Summary) > 500 {
		v.Summary = v.Summary[:500] + "…"
	}
	priority := ""
	for _, s := range rec.Severity {
		switch s.Type {
		case "CVSS_V3":
			if score, ok := CVSS3Score(s.Score); ok && score > v.CVSS {
				v.CVSS = score
			}
		case "Ubuntu":
			priority = strings.ToLower(s.Score)
		}
	}
	switch {
	case v.CVSS > 0:
		v.Severity = SeverityFromScore(v.CVSS)
	case priority == "critical" || priority == "high" || priority == "medium" || priority == "low":
		v.Severity = priority
	default:
		v.Severity = "info"
	}
	for _, a := range rec.Affected {
		key := a.Package.Ecosystem + "|" + a.Package.Name
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" && v.Fixed[key] == "" {
					v.Fixed[key] = e.Fixed
				}
			}
		}
	}
	return v
}

// FixedFor returns the fixed version for a package in an ecosystem we
// queried as "Ubuntu:22.04": OSV labels it "Ubuntu:22.04:LTS", so a
// ":"-suffixed variant counts (but not "Ubuntu:Pro:…").
func (v *Vuln) FixedFor(ecosystem, name string) string {
	if f := v.Fixed[ecosystem+"|"+name]; f != "" {
		return f
	}
	for k, f := range v.Fixed {
		eco, pkg, _ := strings.Cut(k, "|")
		if pkg == name && strings.HasPrefix(eco, ecosystem+":") {
			return f
		}
	}
	return ""
}

// SeverityFromScore maps a CVSS base score to our levels.
func SeverityFromScore(s float64) string {
	switch {
	case s >= 9:
		return "critical"
	case s >= 7:
		return "high"
	case s >= 4:
		return "medium"
	case s > 0:
		return "low"
	}
	return "info"
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		s := string(data)
		if len(s) > 300 {
			s = s[:300]
		}
		return fmt.Errorf("osv: HTTP %d: %s", resp.StatusCode, s)
	}
	return json.Unmarshal(data, out)
}
