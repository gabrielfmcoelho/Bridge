package apicatalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// MaxSpecBytes caps how much we'll read from a remote spec URL (and is a
	// sensible upload ceiling too). 10 MiB comfortably fits very large specs
	// while bounding memory + DB blob size.
	MaxSpecBytes = 10 << 20
	fetchTimeout = 15 * time.Second
)

// ErrBlockedHost is returned when an import-from-URL target resolves to a
// private, loopback, or link-local address. This is the SSRF guard: without
// it, a user could coerce the server into fetching internal-only endpoints.
var ErrBlockedHost = errors.New("refusing to fetch spec from a private, loopback, or link-local address")

// FetchSpec downloads a spec from rawURL with a timeout, a size cap, and an
// SSRF guard. allowPrivate bypasses the guard for trusted/internal
// deployments (wired from a setting by the handler).
//
// NOTE: the guard resolves the host and inspects the returned IPs before the
// request; this is best-effort and does not fully close DNS-rebinding /
// TOCTOU gaps. Documented as a known v1 limitation.
func FetchSpec(ctx context.Context, rawURL string, allowPrivate bool) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q (want http or https)", u.Scheme)
	}
	if !allowPrivate {
		if err := guardHost(u.Hostname()); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, text/plain, */*")
	req.Header.Set("User-Agent", "Bridge-Atlas-API-Catalog/1.0")

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch spec: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxSpecBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read spec body: %w", err)
	}
	if len(body) > MaxSpecBytes {
		return nil, fmt.Errorf("spec exceeds maximum size of %d bytes", MaxSpecBytes)
	}
	if looksLikeHTML(body, resp.Header.Get("Content-Type")) {
		return nil, fmt.Errorf("the URL returned an HTML page, not an OpenAPI/Swagger document — point the Spec URL at the raw JSON/YAML (e.g. /openapi.json or /swagger.json), and use the Docs URL field for the docs page")
	}
	return body, nil
}

// looksLikeHTML detects when a spec URL actually returned a docs UI page
// (the common /docs vs /openapi.json mistake) so we can surface an
// actionable error instead of a cryptic YAML/JSON parse failure.
func looksLikeHTML(body []byte, contentType string) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	n := 64
	if len(trimmed) < n {
		n = len(trimmed)
	}
	head := bytes.ToLower(trimmed[:n])
	return bytes.HasPrefix(head, []byte("<!doctype html")) ||
		bytes.HasPrefix(head, []byte("<html")) ||
		bytes.HasPrefix(head, []byte("<!--")) ||
		bytes.HasPrefix(head, []byte("<head"))
}

func guardHost(host string) error {
	if host == "" {
		return ErrBlockedHost
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return ErrBlockedHost
		}
	}
	return nil
}
