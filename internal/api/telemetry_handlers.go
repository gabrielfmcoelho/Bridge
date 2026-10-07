package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/signoz"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type telemetryHandlers struct {
	db *database.DB
}

// telemetryRow is a signoz.Row plus a human label (key → "label · owner",
// service uuid → nickname); Label is empty when Key already reads well.
type telemetryRow struct {
	signoz.Row
	Label string `json:"label,omitempty"`
}

type telemetryKeySeries struct {
	signoz.KeySeries
	Label string `json:"label,omitempty"`
}

type telemetryKeyOption struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
}

type telemetryFacets struct {
	Routes []string             `json:"routes"`
	Keys   []telemetryKeyOption `json:"keys,omitempty"`
}

type telemetryResponse struct {
	Available   bool                 `json:"available"`
	Reason      string               `json:"reason,omitempty"` // not_configured | no_gateway_url | no_domain
	Summary     *signoz.Stats        `json:"summary,omitempty"`
	Series      []signoz.Bucket      `json:"series,omitempty"`
	Top         []telemetryRow       `json:"top,omitempty"`
	SeriesByKey []telemetryKeySeries `json:"series_by_key,omitempty"`
	Facets      *telemetryFacets     `json:"facets,omitempty"`
	SignozURL   string               `json:"signoz_url,omitempty"`
}

type telemetrySpan struct {
	signoz.Span
	Label string `json:"label,omitempty"`
}

type telemetrySpansResponse struct {
	Available bool            `json:"available"`
	Reason    string          `json:"reason,omitempty"`
	Spans     []telemetrySpan `json:"spans,omitempty"`
}

type telemetryTraceResponse struct {
	TraceID   string             `json:"trace_id"`
	Spans     []signoz.TraceSpan `json:"spans"`
	Cut       bool               `json:"cut,omitempty"`        // longer than signoz.MaxTraceSpans
	Redacted  bool               `json:"redacted,omitempty"`   // query values hidden (not admin)
	SignozURL string             `json:"signoz_url,omitempty"` // the trace in SigNoz
}

// telemetryAsset is an asset resolved for telemetry: its span filter, labels
// for keys (API) or service uuids (host), and why it can't be read, if so.
type telemetryAsset struct {
	filter signoz.Filter
	labels map[string]string
	reason string
}

// asset resolves kind/id (and the viewer's route/key/status filters) to a
// visible asset, writing 400/404 itself. An invisible asset is 404.
func (h *telemetryHandlers) asset(w http.ResponseWriter, r *http.Request) (*telemetryAsset, bool) {
	q := r.URL.Query()
	ctx := r.Context()
	id := strings.TrimSpace(q.Get("id"))
	a := &telemetryAsset{labels: map[string]string{}}
	switch q.Get("kind") {
	case "api":
		aid, err := parseIntFromString(id)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "api id must be numeric")
			return nil, false
		}
		api, err := store.NewAPICatalogRepo(h.db.SQL).Get(ctx, aid)
		if err != nil {
			jsonServerError(w, r, "get api", err)
			return nil, false
		}
		if api == nil {
			jsonError(w, http.StatusNotFound, "api not found")
			return nil, false
		}
		if a.filter.PathPrefix = gatewayPath(api.BaseURL); a.filter.PathPrefix == "" {
			a.reason = "no_gateway_url"
		}
		keys, err := store.NewAPIKeyRepo(h.db.SQL).List(ctx, api.ID, true)
		if err != nil {
			jsonServerError(w, r, "list api keys", err)
			return nil, false
		}
		for _, k := range keys {
			if k.ExternalLabel == nil {
				continue
			}
			l := k.Label
			if k.OwnerContactName != "" {
				l += " · " + k.OwnerContactName
			}
			a.labels[*k.ExternalLabel] = l
		}
		a.filter.Key = q.Get("key")
	case "service":
		sid, err := parseIntFromString(id)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "service id must be numeric")
			return nil, false
		}
		svc, err := store.NewServiceRepo(h.db.SQL).Get(ctx, sid)
		if err != nil {
			jsonServerError(w, r, "get service", err)
			return nil, false
		}
		if svc == nil {
			jsonError(w, http.StatusNotFound, "service not found")
			return nil, false
		}
		domains, err := h.serviceDomains(ctx, svc.ID, svc.ExternalURL)
		if err != nil {
			jsonServerError(w, r, "service domains", err)
			return nil, false
		}
		if a.filter.Domains = domains; len(domains) == 0 {
			a.reason = "no_domain"
		}
	case "host":
		host, err := store.NewHostRepo(h.db.SQL).GetBySlug(ctx, id)
		if err != nil {
			jsonServerError(w, r, "get host", err)
			return nil, false
		}
		if host == nil {
			jsonError(w, http.StatusNotFound, "host not found")
			return nil, false
		}
		// A host's domains: the DNS records pointing at it, plus its services'
		// own; a domain that belongs to a service is labelled with it.
		seen := map[string]bool{}
		add := func(d string) {
			if d = cleanDomain(d); d != "" && !seen[d] {
				seen[d] = true
				a.filter.Domains = append(a.filter.Domains, d)
			}
		}
		recs, err := store.NewDNSRepo(h.db.SQL).RecordsByHost(ctx, host.ID)
		if err != nil {
			jsonServerError(w, r, "host dns", err)
			return nil, false
		}
		for _, rec := range recs {
			add(rec.Domain)
		}
		svcs, err := store.NewServiceRepo(h.db.SQL).ListByHost(ctx, host.ID)
		if err != nil {
			jsonServerError(w, r, "list host services", err)
			return nil, false
		}
		for _, sv := range svcs {
			ds, err := h.serviceDomains(ctx, sv.ID, sv.ExternalURL)
			if err != nil {
				jsonServerError(w, r, "service domains", err)
				return nil, false
			}
			for _, d := range ds {
				add(d)
				a.labels[d] = sv.Nickname
			}
		}
		if len(a.filter.Domains) == 0 {
			a.reason = "no_domain"
		}
	default:
		jsonError(w, http.StatusBadRequest, "kind must be api, service or host")
		return nil, false
	}
	if q.Get("key") != "" && q.Get("kind") != "api" {
		jsonError(w, http.StatusBadRequest, "key filters APIs only")
		return nil, false
	}
	a.filter.Route = q.Get("route")
	a.filter.Status = q.Get("status")
	return a, true
}

// client loads the SigNoz client, or nil (with the settings) when it is off.
func (h *telemetryHandlers) client(w http.ResponseWriter, r *http.Request) (*signoz.Client, signoz.Settings, bool) {
	s, err := signoz.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "load signoz settings", err)
		return nil, s, false
	}
	return signoz.NewServiceClient(s), s, true
}

// handleRequests returns request telemetry for one API, service or host.
//
//	@Summary		Request telemetry (count, errors, p50/p95/p99)
//	@Description	Any role that can see the asset (404 otherwise). Reads SigNoz: an API's spans come from the APISIX gateway (filtered by the path of its base_url), a service's and a host's from Coolify's Traefik (by requested domain: a service's linked DNS records and external URL; a host's DNS records plus its services'). Optional filters route (normalized, as group_by=route reports it), key (Keycloak client; APIs only) and status. series_by=key adds one line per top-5 key plus "__outros__" (APIs only). facets lists the window's top routes and keys, ignoring the route/key/status filters. Keys are labelled with their Bridge key and owner; a host's domains with their service's nickname. {available:false, reason} when SigNoz isn't configured or the asset has nothing to match on. Latencies in ms; errors = 5xx.
//	@Tags			telemetry
//	@Produce		json
//	@Param			kind		query		string	true	"Asset kind"	Enums(api, service, host)
//	@Param			id			query		string	true	"API or service id, or host slug"
//	@Param			range		query		string	false	"Window"		Enums(1h, 24h, 7d, 30d)				default(24h)
//	@Param			group_by	query		string	false	"Top groups"	Enums(route, key, status, service)	default(route)
//	@Param			route		query		string	false	"Only this normalized route"
//	@Param			key			query		string	false	"Only this Keycloak client (APIs)"
//	@Param			status		query		string	false	"Only this HTTP status"
//	@Param			series_by	query		string	false	"Extra series per key (APIs)"	Enums(key)
//	@Success		200			{object}	telemetryResponse
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Failure		502			{object}	httpx.ErrorResponse
//	@Router			/api/telemetry/requests [get]
func (h *telemetryHandlers) handleRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rng, groupBy := q.Get("range"), q.Get("group_by")
	if rng == "" {
		rng = "24h"
	}
	if groupBy == "" {
		groupBy = "route"
	}
	kind := q.Get("kind")
	byKey := q.Get("series_by") == "key"
	if _, ok := signoz.Ranges[rng]; !ok || !signoz.GroupBys[groupBy] {
		jsonError(w, http.StatusBadRequest, "invalid range or group_by")
		return
	}
	if groupBy == "key" && kind != "api" || groupBy == "service" && kind != "host" || byKey && kind != "api" {
		jsonError(w, http.StatusBadRequest, "group_by key and series_by key are for APIs, group_by service for hosts")
		return
	}
	a, ok := h.asset(w, r)
	if !ok {
		return
	}
	c, settings, ok := h.client(w, r)
	if !ok {
		return
	}
	if c == nil {
		a.reason = "not_configured"
	}
	if a.reason != "" {
		jsonOK(w, telemetryResponse{Reason: a.reason})
		return
	}
	res, err := c.Requests(r.Context(), a.filter, rng, groupBy, byKey)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusBadGateway, "SigNoz query failed", err)
		return
	}
	out := telemetryResponse{
		Available: true, Summary: &res.Summary, Series: res.Series, SignozURL: settings.UIURL,
		Top:    make([]telemetryRow, len(res.Top)),
		Facets: &telemetryFacets{Routes: res.Facets.Routes},
	}
	for i, row := range res.Top {
		out.Top[i] = telemetryRow{Row: row, Label: a.labels[row.Key]}
	}
	for _, s := range res.SeriesByKey {
		out.SeriesByKey = append(out.SeriesByKey, telemetryKeySeries{KeySeries: s, Label: a.labels[s.Key]})
	}
	for _, k := range res.Facets.Keys {
		out.Facets.Keys = append(out.Facets.Keys, telemetryKeyOption{Key: k, Label: a.labels[k]})
	}
	jsonOK(w, out)
}

// handleSpans lists an asset's most recent requests.
//
//	@Summary		Recent requests of an asset
//	@Description	Any role that can see the asset (404 otherwise). The newest gateway (APIs) or Traefik (services, hosts) spans in the window, filtered like /api/telemetry/requests (route, key, status): time, trace and span ids, method, normalized route, path, status, duration and, for APIs, the Keycloak client with its Bridge label. Paths never carry the query string. limit 1–100 (default 50).
//	@Tags			telemetry
//	@Produce		json
//	@Param			kind	query		string	true	"Asset kind"	Enums(api, service, host)
//	@Param			id		query		string	true	"API or service id, or host slug"
//	@Param			range	query		string	false	"Window"	Enums(1h, 24h, 7d, 30d)	default(24h)
//	@Param			route	query		string	false	"Only this normalized route"
//	@Param			key		query		string	false	"Only this Keycloak client (APIs)"
//	@Param			status	query		string	false	"Only this HTTP status"
//	@Param			limit	query		int		false	"Rows (1–100)"	default(50)
//	@Success		200		{object}	telemetrySpansResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/telemetry/spans [get]
func (h *telemetryHandlers) handleSpans(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rng := q.Get("range")
	if rng == "" {
		rng = "24h"
	}
	if _, ok := signoz.Ranges[rng]; !ok {
		jsonError(w, http.StatusBadRequest, "invalid range")
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	a, ok := h.asset(w, r)
	if !ok {
		return
	}
	c, _, ok := h.client(w, r)
	if !ok {
		return
	}
	if c == nil {
		a.reason = "not_configured"
	}
	if a.reason != "" {
		jsonOK(w, telemetrySpansResponse{Reason: a.reason})
		return
	}
	spans, err := c.Spans(r.Context(), a.filter, rng, limit)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusBadGateway, "SigNoz query failed", err)
		return
	}
	out := telemetrySpansResponse{Available: true, Spans: make([]telemetrySpan, len(spans))}
	for i, s := range spans {
		out.Spans[i] = telemetrySpan{Span: s, Label: a.labels[s.Client]}
	}
	jsonOK(w, out)
}

// handleTrace returns one whole trace, as seen from an asset.
//
//	@Summary		One request's whole trace
//	@Description	Any role that can see the asset (kind/id), and only when the trace holds a span of that asset in the last 30 days — 404 otherwise, so a trace id alone opens nothing. Every span of the trace, any service (gateway, the API's own spans, its downstream calls), ordered by start, with offsets and durations in ms. Authorization and cookie attributes are dropped for everyone. Query values (url.query, http.target, http.url, url.full) are shown to admins only; others get the parameter names. At most 500 spans (cut:true beyond).
//	@Tags			telemetry
//	@Produce		json
//	@Param			trace_id	path		string	true	"Trace id (32 hex)"
//	@Param			kind		query		string	true	"Asset kind"	Enums(api, service, host)
//	@Param			id			query		string	true	"API or service id, or host slug"
//	@Success		200			{object}	telemetryTraceResponse
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Failure		502			{object}	httpx.ErrorResponse
//	@Router			/api/telemetry/traces/{trace_id} [get]
func (h *telemetryHandlers) handleTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("trace_id")
	a, ok := h.asset(w, r)
	if !ok {
		return
	}
	a.filter.Route, a.filter.Key, a.filter.Status = "", "", ""
	c, settings, ok := h.client(w, r)
	if !ok {
		return
	}
	if c == nil || a.reason != "" {
		jsonError(w, http.StatusNotFound, "trace not found")
		return
	}
	match, err := c.TraceMatches(r.Context(), a.filter, traceID)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusBadGateway, "SigNoz query failed", err)
		return
	}
	if !match {
		jsonError(w, http.StatusNotFound, "trace not found")
		return
	}
	spans, cut, err := c.Trace(r.Context(), traceID)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusBadGateway, "SigNoz query failed", err)
		return
	}
	u := auth.UserFromContext(r.Context())
	admin := u != nil && u.Role == "admin"
	for i := range spans {
		redactSpan(spans[i].Attributes, !admin)
	}
	out := telemetryTraceResponse{TraceID: traceID, Spans: spans, Cut: cut, Redacted: !admin}
	if settings.UIURL != "" {
		out.SignozURL = settings.UIURL + "/trace/" + traceID
	}
	jsonOK(w, out)
}

// queryAttrs are the span attributes that can carry a query string.
var queryAttrs = []string{"url.query", "http.target", "http.url", "url.full"}

// redactSpan drops credentials from a span's attributes and, when hideValues,
// replaces every query string with its parameter names ("cpf=1&p=2" →
// "?cpf&p" in a URL, "cpf, p" in url.query): a query can carry a CPF.
func redactSpan(attrs map[string]string, hideValues bool) {
	for k := range attrs {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "authorization") || strings.Contains(lk, "cookie") {
			delete(attrs, k)
		}
	}
	if !hideValues {
		return
	}
	for _, k := range queryAttrs {
		v, ok := attrs[k]
		if !ok {
			continue
		}
		if k == "url.query" {
			attrs[k] = strings.Join(queryNames(v), ", ")
			continue
		}
		if base, query, found := strings.Cut(v, "?"); found {
			attrs[k] = base
			if names := queryNames(query); len(names) > 0 {
				attrs[k] += "?" + strings.Join(names, "&")
			}
		}
	}
}

// queryNames is the parameter names of a raw query string, in order, unique.
func queryNames(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, "&") {
		name, _, _ := strings.Cut(part, "=")
		if n, err := url.QueryUnescape(name); err == nil {
			name = n
		}
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// handleStatus says whether request telemetry is on, so pages can show or
// hide their tab (the integration settings themselves are admin-only).
//
//	@Summary		Request telemetry availability
//	@Description	Any role. {enabled: true} when the SigNoz integration is on and has a ClickHouse URL.
//	@Tags			telemetry
//	@Produce		json
//	@Success		200	{object}	map[string]bool
//	@Router			/api/telemetry/status [get]
func (h *telemetryHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	s, err := signoz.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "load signoz settings", err)
		return
	}
	jsonOK(w, map[string]bool{"enabled": signoz.NewServiceClient(s) != nil})
}

// serviceDomains is the domains a service answers on, lower-case: its linked
// DNS records and its external URL's host. Traefik spans are matched on them.
func (h *telemetryHandlers) serviceDomains(ctx context.Context, serviceID int64, externalURL string) ([]string, error) {
	ids, err := store.NewServiceRepo(h.db.SQL).DNSIDs(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	add := func(d string) {
		if d = cleanDomain(d); d != "" && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	dns := store.NewDNSRepo(h.db.SQL)
	for _, id := range ids {
		rec, err := dns.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			add(rec.Domain)
		}
	}
	if u, err := url.Parse(externalURL); err == nil {
		add(u.Hostname())
	}
	return out, nil
}

// cleanDomain lower-cases a domain and drops scheme, port and path; "" when
// it can't be one (or would break the comma-joined parameter).
func cleanDomain(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	d, _, _ = strings.Cut(d, "/")
	d, _, _ = strings.Cut(d, ":")
	if d == "" || strings.ContainsAny(d, ", ") {
		return ""
	}
	return d
}

// gatewayPath is the path of an API's base URL ("/datalakehouse/servidores"),
// which is how its gateway spans are matched; "" when it has none.
func gatewayPath(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || strings.Trim(u.Path, "/") == "" {
		return ""
	}
	return "/" + strings.Trim(u.Path, "/")
}

func (h *telemetryHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/telemetry/status", h.handleStatus)
	rr.auth("GET /api/telemetry/requests", h.handleRequests)
	rr.auth("GET /api/telemetry/spans", h.handleSpans)
	rr.auth("GET /api/telemetry/traces/{trace_id}", h.handleTrace)
}
