package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	grafanaclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/grafana"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type grafanaWebhookHandlers struct {
	db *database.DB
}

// grafanaWebhookPayload matches Grafana 9+/10+ Unified Alerting contact-point payload.
// Only the fields we actually use are parsed — extra fields in the JSON are ignored.
type grafanaWebhookPayload struct {
	Status string         `json:"status"`
	Alerts []grafanaAlert `json:"alerts"`
}

type grafanaAlert struct {
	Status       string            `json:"status"` // "firing" | "resolved"
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// handleAlertWebhook ingests alert notifications from Grafana.
// Public route (no auth middleware) — protected instead by HMAC-SHA256 over the
// raw request body using the admin-configured grafana_webhook_secret.
//
// Idempotency: every alert keyed by its Grafana fingerprint. Repeat deliveries
// (Grafana retries, receiver config dupes) UPDATE in place instead of creating
// duplicate rows. A "resolved" status transitions the persisted alert to resolved.
func (h *grafanaWebhookHandlers) handleAlertWebhook(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[grafana-webhook] panic: %v", rv)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}()

	// Read the raw body FIRST — we need it for HMAC verification before JSON decoding.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	settings, err := grafanaclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		log.Printf("[grafana-webhook] load settings: %v", err)
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if settings.WebhookSecret == "" {
		// Fail closed — if no secret is configured, we can't verify anything.
		http.Error(w, "webhook secret not configured", http.StatusServiceUnavailable)
		return
	}
	if !verifyHMAC(r.Header.Get("X-Sshcm-Signature"), settings.WebhookSecret, body) {
		log.Printf("[grafana-webhook] HMAC verification failed from %s", r.RemoteAddr)
		http.Error(w, "signature mismatch", http.StatusUnauthorized)
		return
	}

	var payload grafanaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[grafana-webhook] parse payload: %v", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	processed := 0
	skipped := 0
	unmatched := []string{}

	for _, alert := range payload.Alerts {
		if alert.Fingerprint == "" {
			// Without a fingerprint we can't dedupe. Skip rather than risk duplicates.
			skipped++
			continue
		}
		slug := extractHostLabel(alert.Labels)
		if slug == "" {
			skipped++
			continue
		}
		host, err := findHostByLabel(h.db, slug)
		if err != nil {
			log.Printf("[grafana-webhook] host lookup for %q: %v", slug, err)
			skipped++
			continue
		}
		if host == nil {
			unmatched = append(unmatched, slug)
			continue
		}

		record := &models.HostAlert{
			HostID:         host.ID,
			Type:           alertType(alert),
			Level:          alertLevel(alert),
			Message:        alertMessage(alert),
			Description:    alert.Annotations["description"],
			Source:         "grafana",
			Status:         mapAlertStatus(alert.Status),
			ExternalID:     alert.Fingerprint,
			ExternalSource: "grafana",
		}
		if _, err := models.UpsertExternalHostAlert(h.db.SQL, record); err != nil {
			log.Printf("[grafana-webhook] upsert for host %s fp=%s: %v", host.OficialSlug, alert.Fingerprint, err)
			skipped++
			continue
		}
		processed++
	}

	resp := map[string]any{
		"processed": processed,
		"skipped":   skipped,
	}
	if len(unmatched) > 0 {
		resp["unmatched"] = unmatched
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// verifyHMAC compares the received signature header against an HMAC-SHA256
// computed over the raw body with the configured secret. Accepts either
// "sha256=<hex>" (prefix style, which matches most webhook platforms) or
// a bare hex digest for flexibility. Constant-time compare — timing-attack
// resistant.
func verifyHMAC(header, secret string, body []byte) bool {
	if header == "" {
		return false
	}
	provided := strings.TrimPrefix(header, "sha256=")
	providedBytes, err := hex.DecodeString(provided)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, providedBytes)
}

// extractHostLabel checks Grafana alert labels for the host identifier in a
// priority order. `sshcm_slug` wins because it's explicitly ours; `host` and
// `instance` are conventional in the Prometheus ecosystem.
func extractHostLabel(labels map[string]string) string {
	for _, key := range []string{"sshcm_slug", "host", "instance"} {
		if v := strings.TrimSpace(labels[key]); v != "" {
			return v
		}
	}
	return ""
}

// findHostByLabel tries to match a host row by its oficial_slug, using two strategies:
//  1. exact match on the full label value
//  2. prefix before the first ":" or "." (covers Prometheus instance="myhost:9100"
//     and FQDN-style instance="myhost.example.com")
func findHostByLabel(db *database.DB, raw string) (*models.Host, error) {
	host, err := models.GetHostBySlug(db.SQL, raw)
	if err != nil {
		return nil, err
	}
	if host != nil {
		return host, nil
	}
	trimmed := raw
	if idx := strings.IndexAny(trimmed, ":"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if idx := strings.IndexAny(trimmed, "."); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if trimmed == raw {
		return nil, nil
	}
	return models.GetHostBySlug(db.SQL, trimmed)
}

// alertType returns a short category label. Prefer the Grafana alertname label,
// fall back to "grafana" so the type column is never empty.
func alertType(a grafanaAlert) string {
	if n := strings.TrimSpace(a.Labels["alertname"]); n != "" {
		return n
	}
	return "grafana"
}

// alertLevel maps Grafana's severity label (info/warning/critical) to sshcm's
// level column. Unknown values default to "warning" since a ringing alert
// usually deserves at least that weight.
func alertLevel(a grafanaAlert) string {
	sev := strings.ToLower(strings.TrimSpace(a.Labels["severity"]))
	switch sev {
	case "info", "notice":
		return "info"
	case "warn", "warning":
		return "warning"
	case "crit", "critical", "high", "emergency":
		return "critical"
	}
	return "warning"
}

// alertMessage picks the most informative short string: summary annotation
// wins, then alertname label, then a generic placeholder.
func alertMessage(a grafanaAlert) string {
	if s := strings.TrimSpace(a.Annotations["summary"]); s != "" {
		return s
	}
	if s := strings.TrimSpace(a.Annotations["message"]); s != "" {
		return s
	}
	if n := strings.TrimSpace(a.Labels["alertname"]); n != "" {
		return n
	}
	return "Grafana alert"
}

// mapAlertStatus translates Grafana's alert status into sshcm's status column.
// Grafana emits "firing" or "resolved"; sshcm uses "active" or "resolved".
func mapAlertStatus(status string) string {
	if strings.EqualFold(status, "resolved") {
		return "resolved"
	}
	return "active"
}
