package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	q := r.URL.Query()
	f := models.DependencyFilter{Search: q.Get("search"), HasCVEs: q.Get("has_cves") == "true", EOL: q.Get("eol") == "true"}
	for _, k := range strings.Split(q.Get("kind"), ",") {
		if k = strings.TrimSpace(k); k == "" {
			continue
		}
		if !dependencyKinds[k] {
			jsonError(w, http.StatusBadRequest, "invalid kind "+k)
			return
		}
		f.Kinds = append(f.Kinds, k)
	}
	f.HostID, _ = strconv.ParseInt(q.Get("host_id"), 10, 64)
	f.ServiceID, _ = strconv.ParseInt(q.Get("service_id"), 10, 64)
	p := parsePageParams(r)
	f.Page, f.PerPage = p.Page, p.PerPage
	rows, total, err := store.NewDependencyRepo(h.db.SQL).List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list dependencies", err)
		return
	}
	jsonList(w, rows, metaFor(p, total))
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

// handleEvaluateDependencies godoc
//
//	@Summary		Check dependency versions for CVEs
//	@Description	Admin. Asks OSV (distro packages), NVD (upstream builds) and endoflife.date about every inventoried version whose answer is missing or older than a day — only ecosystem, product and version are sent — then turns every matched CVE into an "inventory" finding. NVD calls are rate-limited: what doesn't fit this run is reported as nvd_deferred and picked up by the next one. 400 when the integration is off.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Success		200	{object}	service.DependencyEvalResult
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
	// Like the Wazuh sync: synchronous, bounded; the client waits up to 5 min.
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	res, err := service.EvaluateDependencies(ctx, h.db.SQL, clients)
	if err != nil {
		jsonServerError(w, r, "failed to evaluate dependencies", err)
		return
	}
	jsonOK(w, res)
}
