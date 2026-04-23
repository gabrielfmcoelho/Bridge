package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	grafanaclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/grafana"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type grafanaHandlers struct {
	db *database.DB
}

// handleEmbedURL returns the iframe URL for a host's or service's Grafana dashboard.
// Falls back to the group default UID if the entity doesn't have its own set.
// Callers: frontend MetricsTab component. Requires authentication.
func (h *grafanaHandlers) handleEmbedURL(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity")
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		jsonError(w, http.StatusBadRequest, "id is required")
		return
	}

	settings, err := grafanaclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load grafana settings", err)
		return
	}
	if !settings.Enabled || settings.BaseURL == "" {
		jsonError(w, http.StatusServiceUnavailable, "Grafana integration is disabled")
		return
	}

	var dashboardUID, varName, varValue string

	switch entity {
	case "host":
		host, err := models.GetHostBySlug(h.db.SQL, id)
		if err != nil {
			jsonServerError(w, r, "host lookup failed", err)
			return
		}
		if host == nil {
			jsonError(w, http.StatusNotFound, "host not found")
			return
		}
		if host.GrafanaDashboardUID != "" {
			dashboardUID = host.GrafanaDashboardUID
		} else {
			dashboardUID = settings.HostDefaultDashboardUID
		}
		varName = "host"
		varValue = host.OficialSlug
	case "service":
		sid, err := parseIntFromString(id)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "service id must be numeric")
			return
		}
		svc, err := models.GetService(h.db.SQL, sid)
		if err != nil {
			jsonServerError(w, r, "service lookup failed", err)
			return
		}
		if svc == nil {
			jsonError(w, http.StatusNotFound, "service not found")
			return
		}
		if svc.GrafanaDashboardUID != "" {
			dashboardUID = svc.GrafanaDashboardUID
		} else {
			dashboardUID = settings.ServiceDefaultDashboardUID
		}
		varName = "service"
		varValue = svc.Nickname
	default:
		jsonError(w, http.StatusBadRequest, "entity must be 'host' or 'service'")
		return
	}

	if dashboardUID == "" {
		jsonOK(w, map[string]any{"configured": false})
		return
	}

	// Build the embed URL. kiosk strips Grafana's own chrome so it looks native
	// inside our iframe; theme=dark matches sshcm's default; refresh keeps the
	// panel auto-updating without user interaction.
	q := url.Values{}
	q.Set("var-"+varName, varValue)
	q.Set("kiosk", "")
	q.Set("theme", "dark")
	q.Set("refresh", "30s")
	embedURL := fmt.Sprintf("%s/d/%s?%s",
		settings.BaseURL,
		url.PathEscape(dashboardUID),
		q.Encode(),
	)
	// Grafana's kiosk param is a valueless flag; url.Values encodes empty values as "kiosk=".
	// That still works but looks ugly — clean up.
	embedURL = strings.Replace(embedURL, "kiosk=&", "kiosk&", 1)
	embedURL = strings.TrimSuffix(embedURL, "&kiosk=")

	jsonOK(w, map[string]any{
		"configured":    true,
		"url":           embedURL,
		"dashboard_uid": dashboardUID,
		"variable":      varName,
		"value":         varValue,
	})
}

func parseIntFromString(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// hostLiveMetrics is what we return from GET /api/hosts/{slug}/metrics/live.
// Every metric is a pointer so the JSON `null` distinguishes "series missing"
// from "value is zero" — the UI uses that to render dimmed tiles.
type hostLiveMetrics struct {
	Enabled        bool     `json:"enabled"`
	Configured     bool     `json:"configured"`
	HostUp         *bool    `json:"host_up"`
	CPUPct         *float64 `json:"cpu_pct"`
	RAMPct         *float64 `json:"ram_pct"`
	DiskPct        *float64 `json:"disk_pct"`
	Load1m         *float64 `json:"load_1m"`
	UptimeSeconds  *float64 `json:"uptime_seconds"`
	FetchedAt      string   `json:"fetched_at"`
	Warnings       []string `json:"warnings,omitempty"`
}

// handleHostLiveMetrics fans out a small pack of PromQL queries against the
// configured Prometheus datasource and returns the results as a flat struct.
// Label convention: the Grafana Agent we install (Phase G) sets host="<slug>"
// as an external label, so all selectors use {host=<slug>}.
func (h *grafanaHandlers) handleHostLiveMetrics(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil {
		jsonServerError(w, r, "host lookup failed", err)
		return
	}
	if host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	settings, err := grafanaclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load grafana settings", err)
		return
	}

	out := hostLiveMetrics{
		Enabled:    settings.Enabled,
		Configured: settings.APIToken != "" && settings.DatasourceUID != "",
		FetchedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if !out.Enabled || !out.Configured {
		jsonOK(w, out)
		return
	}

	client := grafanaclient.NewServiceClient(settings)
	if client == nil {
		jsonOK(w, out)
		return
	}

	// PromQL pack — each entry's expr uses {host="<slug>"} as the sole label
	// selector so it matches exactly one series (except node_cpu_seconds_total,
	// which aggregates across all CPUs via avg()).
	type task struct {
		key    string  // field on out to populate
		expr   string
		out    **float64
	}
	slugQuoted := host.OficialSlug
	tasks := []task{
		{key: "host_up",
			expr: fmt.Sprintf(`up{host=%q}`, slugQuoted),
			// host_up set below via a dedicated assignment since it's a *bool
		},
		{key: "cpu_pct",
			expr: fmt.Sprintf(`100 - (avg by (host) (rate(node_cpu_seconds_total{mode="idle",host=%q}[1m])) * 100)`, slugQuoted),
			out:  &out.CPUPct},
		{key: "ram_pct",
			expr: fmt.Sprintf(`100 * (1 - (node_memory_MemAvailable_bytes{host=%q} / node_memory_MemTotal_bytes{host=%q}))`, slugQuoted, slugQuoted),
			out:  &out.RAMPct},
		{key: "disk_pct",
			expr: fmt.Sprintf(`100 - (node_filesystem_avail_bytes{host=%q,mountpoint="/"} / node_filesystem_size_bytes{host=%q,mountpoint="/"}) * 100`, slugQuoted, slugQuoted),
			out:  &out.DiskPct},
		{key: "load_1m",
			expr: fmt.Sprintf(`node_load1{host=%q}`, slugQuoted),
			out:  &out.Load1m},
		{key: "uptime_seconds",
			expr: fmt.Sprintf(`node_time_seconds{host=%q} - node_boot_time_seconds{host=%q}`, slugQuoted, slugQuoted),
			out:  &out.UptimeSeconds},
	}

	// Fan out with bounded concurrency (5) — shields Grafana from sudden load.
	// Individual query failures are non-fatal: they land in warnings and the
	// corresponding output field stays nil, which the UI renders as "no data".
	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	warnings := []string{}

	// Dedicated slot for host_up — it becomes a bool.
	for _, tk := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(tk task) {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			defer cancel()

			resp, err := client.QueryPrometheusInstant(ctx, settings.DatasourceUID, tk.expr)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, tk.key+": "+err.Error())
				mu.Unlock()
				return
			}
			v, err := grafanaclient.FirstScalar(resp)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, tk.key+": "+err.Error())
				mu.Unlock()
				return
			}

			if tk.key == "host_up" {
				mu.Lock()
				up := v != nil && *v == 1
				out.HostUp = &up
				mu.Unlock()
				return
			}
			if tk.out != nil {
				mu.Lock()
				*tk.out = v
				mu.Unlock()
			}
		}(tk)
	}
	wg.Wait()

	if len(warnings) > 0 {
		out.Warnings = warnings
	}
	jsonOK(w, out)
}

// ProvisionHostDashboard uploads the default dashboard template for a host to Grafana
// and persists the returned UID back onto the host row. Safe to call repeatedly —
// overwrite=true means the second call just updates in place. Used both by the
// explicit admin endpoint and the fire-and-forget goroutine on host create.
func ProvisionHostDashboard(ctx context.Context, db *database.DB, host *models.Host) (string, error) {
	settings, err := grafanaclient.LoadSettings(db.SQL, db.Encryptor)
	if err != nil {
		return "", fmt.Errorf("load grafana settings: %w", err)
	}
	if !settings.Enabled {
		return "", fmt.Errorf("grafana integration is disabled")
	}
	if settings.APIToken == "" {
		return "", fmt.Errorf("grafana API token not configured")
	}
	if settings.DatasourceUID == "" {
		return "", fmt.Errorf("grafana datasource UID not configured")
	}
	client := grafanaclient.NewServiceClient(settings)
	if client == nil {
		return "", fmt.Errorf("grafana client unavailable")
	}

	uid := grafanaclient.HostUID(host.OficialSlug)
	title := fmt.Sprintf("Host – %s", host.Nickname)
	if host.Nickname == "" {
		title = fmt.Sprintf("Host – %s", host.OficialSlug)
	}

	dashboardJSON, err := grafanaclient.RenderHostDashboard(grafanaclient.DashboardVars{
		UID:           uid,
		Title:         title,
		Slug:          host.OficialSlug,
		DatasourceUID: settings.DatasourceUID,
	})
	if err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	resp, err := client.CreateOrUpdateDashboard(ctx, dashboardJSON)
	if err != nil {
		return "", fmt.Errorf("upload dashboard: %w", err)
	}
	returnedUID := resp.UID
	if returnedUID == "" {
		returnedUID = uid
	}

	// Persist the UID so the Metrics tab picks it up immediately.
	host.GrafanaDashboardUID = returnedUID
	if err := models.UpdateHost(db.SQL, host); err != nil {
		log.Printf("[grafana-provision] host %d: uploaded dashboard %s but failed to persist UID: %v", host.ID, returnedUID, err)
	}
	return returnedUID, nil
}

// ProvisionServiceDashboard mirrors ProvisionHostDashboard for services.
func ProvisionServiceDashboard(ctx context.Context, db *database.DB, svc *models.Service) (string, error) {
	settings, err := grafanaclient.LoadSettings(db.SQL, db.Encryptor)
	if err != nil {
		return "", fmt.Errorf("load grafana settings: %w", err)
	}
	if !settings.Enabled {
		return "", fmt.Errorf("grafana integration is disabled")
	}
	if settings.APIToken == "" {
		return "", fmt.Errorf("grafana API token not configured")
	}
	if settings.DatasourceUID == "" {
		return "", fmt.Errorf("grafana datasource UID not configured")
	}
	client := grafanaclient.NewServiceClient(settings)
	if client == nil {
		return "", fmt.Errorf("grafana client unavailable")
	}

	uid := grafanaclient.ServiceUID(svc.ID)
	title := fmt.Sprintf("Service – %s", svc.Nickname)

	dashboardJSON, err := grafanaclient.RenderServiceDashboard(grafanaclient.DashboardVars{
		UID:           uid,
		Title:         title,
		Slug:          svc.Nickname,
		DatasourceUID: settings.DatasourceUID,
	})
	if err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	resp, err := client.CreateOrUpdateDashboard(ctx, dashboardJSON)
	if err != nil {
		return "", fmt.Errorf("upload dashboard: %w", err)
	}
	returnedUID := resp.UID
	if returnedUID == "" {
		returnedUID = uid
	}

	svc.GrafanaDashboardUID = returnedUID
	if err := models.UpdateService(db.SQL, svc); err != nil {
		log.Printf("[grafana-provision] service %d: uploaded dashboard %s but failed to persist UID: %v", svc.ID, returnedUID, err)
	}
	return returnedUID, nil
}

// handleProvisionHostDashboard is the HTTP entry point — admin clicks the "Provision
// default dashboard" button in the host form. Synchronous; returns the new UID.
func (h *grafanaHandlers) handleProvisionHostDashboard(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil {
		jsonServerError(w, r, "host lookup failed", err)
		return
	}
	if host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	uid, err := ProvisionHostDashboard(ctx, h.db, host)
	if err != nil {
		log.Printf("[grafana-provision] host %s: %v", slug, err)
		jsonError(w, http.StatusBadGateway, "provision failed: "+err.Error())
		return
	}

	jsonOK(w, map[string]any{
		"uid":     uid,
		"message": fmt.Sprintf("Dashboard %q provisioned. The Metrics tab will use it on next load.", uid),
	})
}

// handleProvisionServiceDashboard is the service equivalent.
func (h *grafanaHandlers) handleProvisionServiceDashboard(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid service id", err)
		return
	}
	svc, err := models.GetService(h.db.SQL, id)
	if err != nil {
		jsonServerError(w, r, "service lookup failed", err)
		return
	}
	if svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	uid, err := ProvisionServiceDashboard(ctx, h.db, svc)
	if err != nil {
		log.Printf("[grafana-provision] service %d: %v", id, err)
		jsonError(w, http.StatusBadGateway, "provision failed: "+err.Error())
		return
	}

	jsonOK(w, map[string]any{
		"uid":     uid,
		"message": fmt.Sprintf("Dashboard %q provisioned. The Metrics tab will use it on next load.", uid),
	})
}
