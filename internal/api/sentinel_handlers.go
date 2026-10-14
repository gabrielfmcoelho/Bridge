package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/sentinel"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// sentinelHandlers expose Coolify Sentinel (host/container metrics, traffic
// analytics): live reads for the host and service pages, the manual sync,
// and the per-host status the Integrações page lists.
type sentinelHandlers struct {
	db       *database.DB
	sentinel *service.SentinelService
}

// sentinelResponse is every Sentinel read: the data, or why there is none
// (reason: service.SentinelReason*; the UI shows sentinel.reason.<reason>).
type sentinelResponse struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Data      any    `json:"data,omitempty"`
}

// sentinelReply answers a read: its data, or why there is none (200
// {available:false, reason}) — Sentinel being unreadable is a state of the
// host, not a failure of the request.
func sentinelReply(w http.ResponseWriter, data any, err error) {
	var un *service.SentinelUnavailable
	switch {
	case err == nil:
		jsonOK(w, sentinelResponse{Available: true, Data: data})
	case errors.As(err, &un):
		jsonOK(w, sentinelResponse{Reason: un.Reason, Detail: un.Detail})
	default:
		jsonOK(w, sentinelResponse{Reason: service.SentinelReasonError, Detail: err.Error()})
	}
}

func (h *sentinelHandlers) host(w http.ResponseWriter, r *http.Request) *models.Host {
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		jsonServerError(w, r, "failed to load host", err)
		return nil
	}
	if host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
	}
	return host
}

func (h *sentinelHandlers) service(w http.ResponseWriter, r *http.Request) *models.Service {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid service id")
		return nil
	}
	svc, err := store.NewServiceRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load service", err)
		return nil
	}
	if svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
	}
	return svc
}

// sentinelHostView is the host card: live status, last sync and its alerts.
type sentinelHostView struct {
	Status   service.SentinelStatus                   `json:"status"`
	Snapshot *models.SentinelHostSnapshot             `json:"snapshot"`
	Services map[int64]models.SentinelServiceSnapshot `json:"services"`
	Alerts   []service.AlertView                      `json:"alerts"`
}

// handleHostStatus godoc
//
//	@Summary		Host Sentinel status
//	@Description	Any role. Probes the host's Coolify Sentinel live (never cached) and returns it with the last sync's snapshots and the alerts they raise.
//	@Tags			sentinel
//	@Produce		json
//	@Param			slug	path		string	true	"Host oficial slug"
//	@Success		200		{object}	sentinelHostView
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/sentinel [get]
func (h *sentinelHandlers) handleHostStatus(w http.ResponseWriter, r *http.Request) {
	host := h.host(w, r)
	if host == nil {
		return
	}
	ctx := r.Context()
	repo := store.NewSentinelRepo(h.db.SQL)
	v := sentinelHostView{Status: h.sentinel.Status(ctx, host), Alerts: []service.AlertView{}}
	var err error
	if v.Snapshot, err = repo.GetHost(ctx, host.ID); err != nil {
		jsonServerError(w, r, "failed to load sentinel snapshot", err)
		return
	}
	if v.Services, err = repo.ServicesBulk(ctx, host.ID); err != nil {
		jsonServerError(w, r, "failed to load sentinel snapshots", err)
		return
	}
	if v.Snapshot != nil {
		t, err := store.NewAlertSettingsRepo(h.db.SQL).GetThresholds(ctx)
		if err != nil {
			jsonServerError(w, r, "failed to load alert thresholds", err)
			return
		}
		svcs := make([]models.SentinelServiceSnapshot, 0, len(v.Services))
		for _, s := range v.Services {
			svcs = append(svcs, s)
		}
		if a := service.SentinelAlerts(v.Snapshot, svcs, t); a != nil {
			v.Alerts = a
		}
	}
	jsonOK(w, v)
}

// handleHostMetrics godoc
//
//	@Summary		Host metrics history
//	@Description	Any role. Host CPU / memory (percent) and 1-minute load from Sentinel, downsampled to ≤500 points, plus the latest sample. Answers {available:false, reason} when Sentinel can't be read.
//	@Tags			sentinel
//	@Produce		json
//	@Param			slug	path		string	true	"Host oficial slug"
//	@Param			range	query		string	false	"1h | 24h (default) | 7d"
//	@Param			fresh	query		bool	false	"Skip the read cache (live mode)"
//	@Success		200		{object}	sentinelResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/sentinel/metrics [get]
func (h *sentinelHandlers) handleHostMetrics(w http.ResponseWriter, r *http.Request) {
	host := h.host(w, r)
	if host == nil {
		return
	}
	rng := r.URL.Query().Get("range")
	if _, _, err := service.MetricsRange(rng); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.forgetIfFresh(r, host)
	m, err := h.sentinel.HostMetrics(r.Context(), host, rng)
	sentinelReply(w, m, err)
}

// handleHostContainers godoc
//
//	@Summary		Host containers (live)
//	@Description	Any role. Every live container Sentinel sees on the host — state, health, restarts, CPU (percent of one core) and memory — each linked to its Bridge service when one matches.
//	@Tags			sentinel
//	@Produce		json
//	@Param			slug	path		string	true	"Host oficial slug"
//	@Param			fresh	query		bool	false	"Skip the read cache (live mode)"
//	@Success		200		{object}	sentinelResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/sentinel/containers [get]
func (h *sentinelHandlers) handleHostContainers(w http.ResponseWriter, r *http.Request) {
	host := h.host(w, r)
	if host == nil {
		return
	}
	h.forgetIfFresh(r, host)
	cs, err := h.sentinel.Containers(r.Context(), host)
	sentinelReply(w, cs, err)
}

// handleServiceTraffic godoc
//
//	@Summary		Service traffic
//	@Description	Any role. Requests, status classes, latency percentiles, a status-class series and top paths for the service, summed over the Sentinel app keys (Traefik routers) that belong to it. keys is empty when no traffic goes through the proxy.
//	@Tags			sentinel
//	@Produce		json
//	@Param			id		path		int		true	"Service id"
//	@Param			range	query		string	false	"24h (default) | 7d | 30d"
//	@Success		200		{object}	sentinelResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/services/{id}/sentinel/traffic [get]
func (h *sentinelHandlers) handleServiceTraffic(w http.ResponseWriter, r *http.Request) {
	svc := h.service(w, r)
	if svc == nil {
		return
	}
	rng := r.URL.Query().Get("range")
	if _, _, err := service.TrafficRange(rng); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	t, err := h.sentinel.ServiceTraffic(r.Context(), svc, rng)
	sentinelReply(w, t, err)
}

// handleServiceBreakdown godoc
//
//	@Summary		Service traffic by dimension
//	@Description	Any role. Top values of one dimension (status, method, country, referer, browser, os, device, protocol, scheme, tls, cache, bot). Visitor-identifying dimensions (ip, user agent) are not exposed.
//	@Tags			sentinel
//	@Produce		json
//	@Param			id		path		int		true	"Service id"
//	@Param			dim		path		string	true	"Dimension"
//	@Param			range	query		string	false	"24h (default) | 7d | 30d"
//	@Success		200		{object}	sentinelResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/services/{id}/sentinel/traffic/breakdown/{dim} [get]
func (h *sentinelHandlers) handleServiceBreakdown(w http.ResponseWriter, r *http.Request) {
	svc := h.service(w, r)
	if svc == nil {
		return
	}
	rng := r.URL.Query().Get("range")
	if _, _, err := service.TrafficRange(rng); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	dim := r.PathValue("dim")
	if !sentinel.Dimensions[dim] {
		jsonError(w, http.StatusBadRequest, "unknown dimension")
		return
	}
	rows, err := h.sentinel.ServiceBreakdown(r.Context(), svc, dim, rng)
	sentinelReply(w, rows, err)
}

// forgetIfFresh drops the host's cached reads on ?fresh=1 (live mode: the
// default 60s cache would freeze charts Sentinel updates every 10s).
func (h *sentinelHandlers) forgetIfFresh(r *http.Request, host *models.Host) {
	if r.URL.Query().Get("fresh") == "1" {
		h.sentinel.ForgetHost(host.ID)
	}
}

func isUnavailable(err error) bool {
	var un *service.SentinelUnavailable
	return errors.As(err, &un)
}

// handleServiceResources godoc
//
//	@Summary		Service container metrics
//	@Description	Any role. CPU (percent of one core) and memory (percent) history of the service's live container. container is empty when none matches.
//	@Tags			sentinel
//	@Produce		json
//	@Param			id		path		int		true	"Service id"
//	@Param			range	query		string	false	"1h | 24h (default) | 7d"
//	@Success		200		{object}	sentinelResponse
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/services/{id}/sentinel/resources [get]
func (h *sentinelHandlers) handleServiceResources(w http.ResponseWriter, r *http.Request) {
	svc := h.service(w, r)
	if svc == nil {
		return
	}
	rng := r.URL.Query().Get("range")
	if _, _, err := service.MetricsRange(rng); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.sentinel.ServiceResources(r.Context(), svc, rng)
	sentinelReply(w, res, err)
}

// handleSync godoc
//
//	@Summary		Sync Sentinel snapshots
//	@Description	Admin. Reads every Coolify-linked host's Sentinel (4 at a time) and stores the host and service snapshots lists sort on and alerts read. A host that can't be read records why; it never fails the sync.
//	@Tags			sentinel
//	@Produce		json
//	@Success		200	{object}	service.SentinelSyncReport
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/sentinel/sync [post]
func (h *sentinelHandlers) handleSync(w http.ResponseWriter, r *http.Request) {
	rep, err := h.sentinel.Sync(r.Context())
	if isUnavailable(err) {
		jsonError(w, http.StatusBadRequest, "sentinel integration is not enabled")
		return
	}
	if err != nil {
		jsonServerError(w, r, "sentinel sync failed", err)
		return
	}
	jsonOK(w, rep)
}

// handleHostSync godoc
//
//	@Summary		Sync one host's Sentinel snapshots
//	@Description	Editor+. Reads the host's Sentinel now and stores its host and service snapshots (what lists sort on and alerts read), like the full sync does for every host. A host that can't be read records why.
//	@Tags			sentinel
//	@Produce		json
//	@Param			slug	path		string	true	"Host oficial slug"
//	@Success		200		{object}	service.SentinelSyncHost
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/sentinel/sync [post]
func (h *sentinelHandlers) handleHostSync(w http.ResponseWriter, r *http.Request) {
	host := h.host(w, r)
	if host == nil {
		return
	}
	res, err := h.sentinel.SyncHost(r.Context(), host)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "sentinel integration is not enabled")
		return
	}
	jsonOK(w, res)
}

// sentinelHostRow is one Coolify-linked host on the Integrações page.
type sentinelHostRow struct {
	ID       int64                        `json:"id"`
	Slug     string                       `json:"slug"`
	Nickname string                       `json:"nickname"`
	Hostname string                       `json:"hostname"`
	Snapshot *models.SentinelHostSnapshot `json:"snapshot"`
}

// handleHosts godoc
//
//	@Summary		Coolify-linked hosts with their Sentinel state
//	@Description	Admin. Every host linked to a Coolify server and its last Sentinel sync (snapshot null = never synced).
//	@Tags			sentinel
//	@Produce		json
//	@Success		200	{object}	ListEnvelope[sentinelHostRow]
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/sentinel/hosts [get]
func (h *sentinelHandlers) handleHosts(w http.ResponseWriter, r *http.Request) {
	repo := store.NewSentinelRepo(h.db.SQL)
	hosts, err := repo.CoolifyHosts(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list hosts", err)
		return
	}
	snaps, err := repo.HostsBulk(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to load sentinel snapshots", err)
		return
	}
	rows := make([]sentinelHostRow, len(hosts))
	for i, host := range hosts {
		rows[i] = sentinelHostRow{ID: host.ID, Slug: host.OficialSlug, Nickname: host.Nickname, Hostname: host.Hostname}
		if s, ok := snaps[host.ID]; ok {
			rows[i].Snapshot = &s
		}
	}
	jsonPaged(w, r, rows)
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *sentinelHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/hosts/{slug}/sentinel", h.handleHostStatus)
	rr.auth("GET /api/hosts/{slug}/sentinel/metrics", h.handleHostMetrics)
	rr.auth("GET /api/hosts/{slug}/sentinel/containers", h.handleHostContainers)
	rr.role("editor", "POST /api/hosts/{slug}/sentinel/sync", h.handleHostSync)
	rr.auth("GET /api/services/{id}/sentinel/traffic", h.handleServiceTraffic)
	rr.auth("GET /api/services/{id}/sentinel/traffic/breakdown/{dim}", h.handleServiceBreakdown)
	rr.auth("GET /api/services/{id}/sentinel/resources", h.handleServiceResources)
	rr.role("admin", "GET /api/sentinel/hosts", h.handleHosts)
	rr.role("admin", "POST /api/sentinel/sync", h.handleSync)
}
