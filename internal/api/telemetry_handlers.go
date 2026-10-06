package api

import (
	"net/http"
	"net/url"
	"strings"

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

type telemetryResponse struct {
	Available bool            `json:"available"`
	Reason    string          `json:"reason,omitempty"` // not_configured | no_gateway_url | no_coolify_uuid
	Summary   *signoz.Stats   `json:"summary,omitempty"`
	Series    []signoz.Bucket `json:"series,omitempty"`
	Top       []telemetryRow  `json:"top,omitempty"`
	SignozURL string          `json:"signoz_url,omitempty"`
}

// handleRequests returns request telemetry for one API, service or host.
//
//	@Summary		Request telemetry (count, errors, p50/p95/p99)
//	@Description	Any role that can see the asset (404 otherwise). Reads SigNoz: an API's spans come from the APISIX gateway (filtered by the path of its base_url), a service's and a host's from Coolify's Traefik (by Coolify resource uuid; a host = its services). group_by key (APIs only) labels each Keycloak client with its Bridge key and owner; service (hosts) with the service nickname. {available:false, reason} when SigNoz isn't configured or the asset has nothing to match on. Latencies in ms; errors = 5xx.
//	@Tags			telemetry
//	@Produce		json
//	@Param			kind		query		string	true	"Asset kind"	Enums(api, service, host)
//	@Param			id			query		string	true	"API or service id, or host slug"
//	@Param			range		query		string	false	"Window"		Enums(1h, 24h, 7d, 30d)				default(24h)
//	@Param			group_by	query		string	false	"Top groups"	Enums(route, key, status, service)	default(route)
//	@Success		200			{object}	telemetryResponse
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Failure		502			{object}	httpx.ErrorResponse
//	@Router			/api/telemetry/requests [get]
func (h *telemetryHandlers) handleRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()
	id := strings.TrimSpace(q.Get("id"))
	rng, groupBy := q.Get("range"), q.Get("group_by")
	if rng == "" {
		rng = "24h"
	}
	if groupBy == "" {
		groupBy = "route"
	}
	if _, ok := signoz.Ranges[rng]; !ok || !signoz.GroupBys[groupBy] {
		jsonError(w, http.StatusBadRequest, "invalid range or group_by")
		return
	}
	kind := q.Get("kind")
	if groupBy == "key" && kind != "api" || groupBy == "service" && kind != "host" {
		jsonError(w, http.StatusBadRequest, "group_by key is for APIs, service for hosts")
		return
	}

	// Resolve the asset first: an invisible one is 404 even when SigNoz is off.
	var f signoz.Filter
	reason := ""
	labels := map[string]string{}
	switch kind {
	case "api":
		aid, err := parseIntFromString(id)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "api id must be numeric")
			return
		}
		a, err := store.NewAPICatalogRepo(h.db.SQL).Get(ctx, aid)
		if err != nil {
			jsonServerError(w, r, "get api", err)
			return
		}
		if a == nil {
			jsonError(w, http.StatusNotFound, "api not found")
			return
		}
		if f.PathPrefix = gatewayPath(a.BaseURL); f.PathPrefix == "" {
			reason = "no_gateway_url"
		}
		if groupBy == "key" {
			keys, err := store.NewAPIKeyRepo(h.db.SQL).List(ctx, a.ID, true)
			if err != nil {
				jsonServerError(w, r, "list api keys", err)
				return
			}
			for _, k := range keys {
				if k.ExternalLabel == nil {
					continue
				}
				l := k.Label
				if k.OwnerContactName != "" {
					l += " · " + k.OwnerContactName
				}
				labels[*k.ExternalLabel] = l
			}
		}
	case "service":
		sid, err := parseIntFromString(id)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "service id must be numeric")
			return
		}
		svc, err := store.NewServiceRepo(h.db.SQL).Get(ctx, sid)
		if err != nil {
			jsonServerError(w, r, "get service", err)
			return
		}
		if svc == nil {
			jsonError(w, http.StatusNotFound, "service not found")
			return
		}
		if svc.CoolifyResourceUUID == "" {
			reason = "no_coolify_uuid"
		} else {
			f.RouterUUIDs = []string{svc.CoolifyResourceUUID}
		}
	case "host":
		host, err := store.NewHostRepo(h.db.SQL).GetBySlug(ctx, id)
		if err != nil {
			jsonServerError(w, r, "get host", err)
			return
		}
		if host == nil {
			jsonError(w, http.StatusNotFound, "host not found")
			return
		}
		svcs, err := store.NewServiceRepo(h.db.SQL).ListByHost(ctx, host.ID)
		if err != nil {
			jsonServerError(w, r, "list host services", err)
			return
		}
		for _, s := range svcs {
			if s.CoolifyResourceUUID != "" {
				f.RouterUUIDs = append(f.RouterUUIDs, s.CoolifyResourceUUID)
				labels[s.CoolifyResourceUUID] = s.Nickname
			}
		}
		if len(f.RouterUUIDs) == 0 {
			reason = "no_coolify_uuid"
		}
	default:
		jsonError(w, http.StatusBadRequest, "kind must be api, service or host")
		return
	}
	settings, err := signoz.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "load signoz settings", err)
		return
	}
	client := signoz.NewServiceClient(settings)
	if client == nil {
		reason = "not_configured"
	}
	if reason != "" {
		jsonOK(w, telemetryResponse{Reason: reason})
		return
	}

	res, err := client.Requests(ctx, f, rng, groupBy)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusBadGateway, "SigNoz query failed", err)
		return
	}
	top := make([]telemetryRow, len(res.Top))
	for i, row := range res.Top {
		top[i] = telemetryRow{Row: row, Label: labels[row.Key]}
	}
	jsonOK(w, telemetryResponse{
		Available: true, Summary: &res.Summary, Series: res.Series, Top: top, SignozURL: settings.UIURL,
	})
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
}
