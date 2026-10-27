package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vulnimport"
)

var dependencyKinds = map[string]bool{models.DepKindOS: true, models.DepKindRuntime: true, models.DepKindPackage: true}

// handleListDependencies godoc
//
//	@Summary		List dependency versions
//	@Description	OS, runtime and package versions the SSH scan found on hosts and inside their containers, with their CVE count and EOL flag. Visible through the host or the container's service. Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			kind		query		string	false	"Comma-separated: os, runtime, package (default os,runtime)"
//	@Param			host_id		query		int		false	"Only this host"
//	@Param			service_id	query		int		false	"Only this (container) service"
//	@Param			has_cves	query		bool	false	"Only rows with known CVEs"
//	@Param			eol			query		bool	false	"Only end-of-life versions"
//	@Param			search		query		string	false	"Name, version, container, image or host"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.Dependency]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/dependencies [get]
func (h *vulnHandlers) handleListDependencies(w http.ResponseWriter, r *http.Request) {
	f, p, ok := dependencyFilter(w, r)
	if !ok {
		return
	}
	rows, total, err := store.NewDependencyRepo(h.db.SQL).List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list dependencies", err)
		return
	}
	jsonList(w, rows, metaFor(p, total))
}

// dependencyFilter reads the list / groups query: kind, host_id, service_id,
// has_cves, eol, search and paging. Writes 400 on an unknown kind.
func dependencyFilter(w http.ResponseWriter, r *http.Request) (models.DependencyFilter, PageParams, bool) {
	q := r.URL.Query()
	f := models.DependencyFilter{Search: q.Get("search"), HasCVEs: q.Get("has_cves") == "true", EOL: q.Get("eol") == "true"}
	for _, k := range strings.Split(q.Get("kind"), ",") {
		if k = strings.TrimSpace(k); k == "" {
			continue
		}
		if !dependencyKinds[k] {
			jsonError(w, http.StatusBadRequest, "invalid kind "+k)
			return f, PageParams{}, false
		}
		f.Kinds = append(f.Kinds, k)
	}
	f.HostID, _ = strconv.ParseInt(q.Get("host_id"), 10, 64)
	f.ServiceID, _ = strconv.ParseInt(q.Get("service_id"), 10, 64)
	p := parsePageParams(r)
	f.Page, f.PerPage = p.Page, p.PerPage
	return f, p, true
}

// handleListDependencyGroups godoc
//
//	@Summary		List dependency versions grouped by component
//	@Description	The same rows and filters as the dependency list, one entry per component version (kind, name, version) with how many hosts and installations have it, the worst CVE count and severity among them, EOL, and up to 50 of its installations. Worst first, then the most widespread. Any role; visibility as the list.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			kind		query		string	false	"Comma-separated: os, runtime, package (default os,runtime)"
//	@Param			host_id		query		int		false	"Only this host"
//	@Param			service_id	query		int		false	"Only this service's container"
//	@Param			has_cves	query		bool	false	"Only versions with CVEs"
//	@Param			eol			query		bool	false	"Only end-of-life versions"
//	@Param			search		query		string	false	"Name, version, container, image or host"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.DependencyGroup]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/dependencies/groups [get]
func (h *vulnHandlers) handleListDependencyGroups(w http.ResponseWriter, r *http.Request) {
	f, p, ok := dependencyFilter(w, r)
	if !ok {
		return
	}
	groups, total, err := store.NewDependencyRepo(h.db.SQL).ListGroups(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list dependency groups", err)
		return
	}
	jsonList(w, groups, metaFor(p, total))
}

// handleGetDependency godoc
//
//	@Summary		Get a dependency version
//	@Description	One inventory row with its known advisories and the findings they produced. Invisible rows answer 404. Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			id	path		int	true	"Dependency ID"
//	@Success		200	{object}	models.DependencyDetail
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/dependencies/{id} [get]
func (h *vulnHandlers) handleGetDependency(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d, err := store.NewDependencyRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to get dependency", err)
		return
	}
	if d == nil {
		jsonError(w, http.StatusNotFound, "dependency not found")
		return
	}
	vulns, err := service.DependencyVulns(r.Context(), h.db.SQL, *d)
	if err != nil {
		jsonServerError(w, r, "failed to load dependency advisories", err)
		return
	}
	fps := make([]string, len(vulns))
	for i, v := range vulns {
		fps[i] = vulnimport.InventoryFingerprint(*d, v.ID)
	}
	findings, err := h.repo().ByFingerprints(r.Context(), vulnimport.SourceInventory, fps)
	if err != nil {
		jsonServerError(w, r, "failed to load dependency findings", err)
		return
	}
	jsonOK(w, models.DependencyDetail{Dependency: *d, Vulns: vulns, Findings: findings})
}

// DependencyEvalStatus is the background "Evaluate CVEs" run: under way, or
// how the last one ended.
type DependencyEvalStatus struct {
	Running    bool                          `json:"running"`
	StartedAt  *time.Time                    `json:"started_at,omitempty"`
	FinishedAt *time.Time                    `json:"finished_at,omitempty"`
	Result     *service.DependencyEvalResult `json:"result,omitempty"`
	Error      string                        `json:"error,omitempty"`
}

// evalJob runs one evaluation at a time in the background.
// ponytail: in memory, one process — a restart forgets a run under way (the
// next click starts over); move it to a table if Bridge ever runs replicated.
type evalJob struct {
	mu     sync.Mutex
	status DependencyEvalStatus
}

var depEvalJob = &evalJob{}

const (
	// A first run on a large fleet needs ~20k OSV calls plus a fleet-wide
	// findings rewrite; outbound calls stop depEvalReserve before the end
	// so the rewrite always has time.
	depEvalTimeout = 45 * time.Minute
	depEvalReserve = 10 * time.Minute
)

func (j *evalJob) get() DependencyEvalStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

// start launches run unless one is under way; either way it returns the
// current status and whether this call started it.
func (j *evalJob) start(run func() (*service.DependencyEvalResult, error)) (DependencyEvalStatus, bool) {
	j.mu.Lock()
	if j.status.Running {
		s := j.status
		j.mu.Unlock()
		return s, false
	}
	now := time.Now().UTC()
	j.status = DependencyEvalStatus{Running: true, StartedAt: &now}
	s := j.status
	j.mu.Unlock()

	go func() {
		var res *service.DependencyEvalResult
		var err error
		defer func() {
			if rv := recover(); rv != nil {
				err = fmt.Errorf("panic: %v", rv)
			}
			end := time.Now().UTC()
			j.mu.Lock()
			j.status.Running, j.status.FinishedAt, j.status.Result = false, &end, res
			if err != nil {
				j.status.Error = err.Error()
			}
			j.mu.Unlock()
		}()
		res, err = run()
	}()
	return s, true
}

// handleEvaluateDependencies godoc
//
//	@Summary		Check dependency versions for CVEs (background)
//	@Description	Admin. Starts, in the background, a run that asks NVD (upstream builds), endoflife.date and OSV (distro packages) about every inventoried version whose answer is missing or older than a day — only ecosystem, product and version are sent — then turns every matched CVE into an "inventory" finding. Answers 202 with the run's status at once; a run already under way is returned instead of starting another. Poll GET for the result. Up to 45 minutes; what doesn't fit is reported as *_deferred for the next run. 400 when the integration is off.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Success		202	{object}	DependencyEvalStatus
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/dependencies/evaluate [post]
func (h *vulnHandlers) handleEvaluateDependencies(w http.ResponseWriter, r *http.Request) {
	clients, err := service.DependencyClientsFromSettings(r.Context(), h.db)
	if err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	clients.Reserve = depEvalReserve
	// Detached: the run outlives this request (and a proxy cutting it).
	base := context.WithoutCancel(r.Context())
	st, _ := depEvalJob.start(func() (*service.DependencyEvalResult, error) {
		ctx, cancel := context.WithTimeout(base, depEvalTimeout)
		defer cancel()
		start := time.Now()
		res, err := service.EvaluateDependencies(ctx, h.db.SQL, clients)
		if err != nil {
			log.Printf("[deps] evaluate failed after %s: %v", time.Since(start).Round(time.Second), err)
		} else {
			log.Printf("[deps] evaluate done in %s: osv=%d details=%d (deferred %d) nvd=%d (deferred %d) eol=%d (deferred %d) errors=%d",
				time.Since(start).Round(time.Second), res.OSVQueried, res.OSVVulns, res.OSVVulnsDeferred,
				res.NVDQueried, res.NVDDeferred, res.EOLQueried, res.EOLDeferred, len(res.Errors))
		}
		return res, err
	})
	httpx.WriteJSON(w, http.StatusAccepted, st)
}

// handleEvaluateStatus godoc
//
//	@Summary		Status of the background CVE evaluation
//	@Description	Admin. Whether a run is under way, and the result or error of the last one (since the server started).
//	@Tags			vulnerabilities
//	@Produce		json
//	@Success		200	{object}	DependencyEvalStatus
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/dependencies/evaluate [get]
func (h *vulnHandlers) handleEvaluateStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, depEvalJob.get())
}
