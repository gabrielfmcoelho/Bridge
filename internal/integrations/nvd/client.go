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
	"slices"
	"strconv"
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
	// Affected is one line per vulnerable vendor:product with its version
	// ranges, applications first: "apache:log4j >=2.0.1 <2.3.1; =2.15.0".
	// In "<X", X is the first fixed version.
	Affected []string
}

const (
	maxRefs             = 20
	maxAffectedProducts = 10
	maxRangesPerProduct = 8
)

type cpeMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
}

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
			Configurations []struct {
				Nodes []struct {
					CPEMatch []cpeMatch `json:"cpeMatch"`
				} `json:"nodes"`
			} `json:"configurations"`
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
	var matches []cpeMatch
	for _, c := range v.Configurations {
		for _, n := range c.Nodes {
			matches = append(matches, n.CPEMatch...)
		}
	}
	out.Affected = affected(matches)
	// NVD timestamps carry no zone; they are UTC.
	if t, err := time.Parse("2006-01-02T15:04:05.000", v.Published); err == nil {
		out.Published = &t
	} else if t, err := time.Parse(time.RFC3339, v.Published); err == nil {
		out.Published = &t
	}
	return out, nil
}

// affected groups the vulnerable CPE matches by vendor:product, keeping
// first-seen order with applications ("a") ahead of OS / hardware — a CVE
// like Log4Shell lists dozens of firmware entries before the library.
// Exact versions (NVD often enumerates them instead of a range) are sorted
// and, when many, shown as their ends: the newest affected one is the line
// that matters for picking a fix.
func affected(matches []cpeMatch) []string {
	type product struct {
		name   string
		app    bool
		ranges []string
		exact  []string
	}
	var products []*product
	byName := map[string]*product{}
	for _, m := range matches {
		// cpe:2.3:part:vendor:product:version:...
		f := strings.Split(m.Criteria, ":")
		if !m.Vulnerable || len(f) < 6 {
			continue
		}
		name := f[3] + ":" + f[4]
		p := byName[name]
		if p == nil {
			p = &product{name: name, app: f[2] == "a"}
			byName[name] = p
			products = append(products, p)
		}
		var r []string
		for _, b := range [][2]string{{">=", m.VersionStartIncluding}, {">", m.VersionStartExcluding}, {"<=", m.VersionEndIncluding}, {"<", m.VersionEndExcluding}} {
			if b[1] != "" {
				r = append(r, b[0]+b[1])
			}
		}
		switch {
		case len(r) > 0:
			if rng := strings.Join(r, " "); !slices.Contains(p.ranges, rng) {
				p.ranges = append(p.ranges, rng)
			}
		case f[5] != "*" && f[5] != "-":
			if !slices.Contains(p.exact, f[5]) {
				p.exact = append(p.exact, f[5])
			}
		default:
			if !slices.Contains(p.ranges, "all versions") {
				p.ranges = append(p.ranges, "all versions")
			}
		}
	}
	slices.SortStableFunc(products, func(a, b *product) int {
		switch {
		case a.app == b.app:
			return 0
		case a.app:
			return -1
		}
		return 1
	})
	var out []string
	for i, p := range products {
		if i == maxAffectedProducts {
			out = append(out, fmt.Sprintf("… %d more products", len(products)-i))
			break
		}
		parts := p.ranges
		if len(parts) > maxRangesPerProduct {
			parts = append(parts[:maxRangesPerProduct:maxRangesPerProduct], "…")
		}
		if len(p.exact) > 0 {
			slices.SortFunc(p.exact, compareVersions)
			if len(p.exact) > 4 {
				parts = append(parts, fmt.Sprintf("=%s; =%s … =%s; =%s (%d listed versions)",
					p.exact[0], p.exact[1], p.exact[len(p.exact)-2], p.exact[len(p.exact)-1], len(p.exact)))
			} else {
				for _, v := range p.exact {
					parts = append(parts, "="+v)
				}
			}
		}
		out = append(out, p.name+" "+strings.Join(parts, "; "))
	}
	return out
}

// compareVersions orders dotted versions numerically where it can
// ("2.4.9" < "2.4.10"), falling back to text for non-numeric parts.
func compareVersions(a, b string) int {
	as, bs := strings.FieldsFunc(a, isSep), strings.FieldsFunc(b, isSep)
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, errX := strconv.Atoi(as[i])
		y, errY := strconv.Atoi(bs[i])
		if errX == nil && errY == nil {
			if x != y {
				return x - y
			}
			continue
		}
		if c := strings.Compare(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return len(as) - len(bs)
}

func isSep(r rune) bool { return r == '.' || r == '-' || r == '_' }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
