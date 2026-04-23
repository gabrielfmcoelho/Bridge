package grafana

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.json
var dashboardTemplates embed.FS

// DashboardVars feeds into the dashboard JSON templates. Every string goes through
// text/template's HTML-esque escaping — but since the destination is a JSON document
// we must ensure none of these contain raw double-quotes or backslashes. We JSON-escape
// defensively in the renderer.
type DashboardVars struct {
	UID           string
	Title         string
	Slug          string
	DatasourceUID string
}

// HostUID returns the deterministic dashboard UID for a host. Grafana UIDs must be
// <= 40 chars and match [a-zA-Z0-9_-]+. The sshcm-host-<slug> form is stable so
// re-provisioning always targets the same dashboard.
func HostUID(slug string) string {
	return truncateUID("sshcm-host-" + sanitizeUIDPart(slug))
}

// ServiceUID mirrors HostUID but for services, keyed by numeric id so nickname
// changes don't orphan the dashboard.
func ServiceUID(serviceID int64) string {
	return truncateUID(fmt.Sprintf("sshcm-service-%d", serviceID))
}

func sanitizeUIDPart(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

func truncateUID(s string) string {
	if len(s) > 40 {
		return s[:40]
	}
	return s
}

// RenderHostDashboard substitutes the template vars into the embedded host
// template and validates the output parses as JSON. The returned bytes are
// the complete `dashboard` payload — the caller wraps them into the
// `{dashboard: ..., overwrite: true}` envelope before POSTing.
func RenderHostDashboard(vars DashboardVars) ([]byte, error) {
	return renderDashboard("templates/host_default.json", vars)
}

// RenderServiceDashboard is the service equivalent.
func RenderServiceDashboard(vars DashboardVars) ([]byte, error) {
	return renderDashboard("templates/service_default.json", vars)
}

func renderDashboard(path string, vars DashboardVars) ([]byte, error) {
	raw, err := dashboardTemplates.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}

	// Escape every string field to be JSON-safe (double-quotes, backslashes, control chars)
	// so an admin-chosen title with a quote in it can't break the template.
	esc := func(s string) string {
		b, _ := json.Marshal(s)
		// json.Marshal wraps in quotes — strip them.
		if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
			return string(b[1 : len(b)-1])
		}
		return string(b)
	}
	escaped := DashboardVars{
		UID:           esc(vars.UID),
		Title:         esc(vars.Title),
		Slug:          esc(vars.Slug),
		DatasourceUID: esc(vars.DatasourceUID),
	}

	tmpl, err := template.New(path).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, escaped); err != nil {
		return nil, fmt.Errorf("render template %s: %w", path, err)
	}

	// Validate JSON before shipping. A template bug should surface here, not
	// in Grafana's error response.
	var check any
	if err := json.Unmarshal(buf.Bytes(), &check); err != nil {
		return nil, fmt.Errorf("rendered dashboard is not valid JSON: %w", err)
	}
	return buf.Bytes(), nil
}
