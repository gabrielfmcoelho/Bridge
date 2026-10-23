package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/nvd"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vulnimport"
)

// maxETIPIBytes caps an uploaded ETIPI report (the real ones are ~20 KB).
const maxETIPIBytes = 5 << 20

type vulnHandlers struct {
	db  *database.DB
	nvd *nvd.Client
}

var vulnStatuses = map[string]bool{"open": true, "accepted": true, "false_positive": true, "resolved": true}
var vulnEntityTypes = map[string]bool{"host": true, "dns": true, "service": true, "project": true}

func (h *vulnHandlers) repo() *store.VulnRepo { return store.NewVulnRepo(h.db.SQL) }

// handleList godoc
//
//	@Summary		List vulnerability findings
//	@Description	Findings from ETIPI reports and Wazuh, most severe first. Visible through the host / DNS / service / project they link to; unlinked ones are admin-only. With entity_type + entity_id: that entity's tab, including findings on its related hosts / DNS records (via). Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			status		query		string	false	"Comma-separated: open, accepted, false_positive, resolved"
//	@Param			severity	query		string	false	"Comma-separated: critical, high, medium, low, info"
//	@Param			source		query		string	false	"etipi or wazuh"
//	@Param			search		query		string	false	"Title, CVE, target or VM"
//	@Param			unmatched	query		bool	false	"Only findings linked to nothing (admin)"
//	@Param			entity_type	query		string	false	"host, dns, service or project"
//	@Param			entity_id	query		int		false	"Entity ID (with entity_type)"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200, default 50)"
//	@Success		200			{object}	ListEnvelope[models.VulnFinding]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities [get]
func (h *vulnHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pp := parsePageParams(r)
	if pp.Unbounded() {
		pp.PerPage = 50
	}
	f := models.VulnFilter{
		Status:     q.Get("status"),
		Severity:   q.Get("severity"),
		Source:     q.Get("source"),
		Search:     q.Get("search"),
		Unmatched:  q.Get("unmatched") == "true",
		EntityType: q.Get("entity_type"),
		Page:       pp.Page,
		PerPage:    pp.PerPage,
	}
	if f.EntityType != "" {
		if !vulnEntityTypes[f.EntityType] {
			jsonError(w, http.StatusBadRequest, "invalid entity_type")
			return
		}
		id, err := strconv.ParseInt(q.Get("entity_id"), 10, 64)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid entity_id")
			return
		}
		f.EntityID = id
	}
	items, total, err := h.repo().List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list vulnerabilities", err)
		return
	}
	jsonList(w, items, metaFor(pp, total))
}

// handleGet godoc
//
//	@Summary		Get a vulnerability finding
//	@Description	With its links. 404 when absent or invisible. Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			id	path		int	true	"Finding ID"
//	@Success		200	{object}	models.VulnFinding
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/{id} [get]
func (h *vulnHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	f, ok := h.load(w, r)
	if !ok {
		return
	}
	jsonOK(w, f)
}

// load reads the {id} finding, answering 404 when absent or invisible.
func (h *vulnHandlers) load(w http.ResponseWriter, r *http.Request) (*models.VulnFinding, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return nil, false
	}
	f, err := h.repo().Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load vulnerability", err)
		return nil, false
	}
	if f == nil {
		jsonError(w, http.StatusNotFound, "vulnerability not found")
		return nil, false
	}
	return f, true
}

// vulnUpdateRequest is a partial update: an omitted field keeps its value.
// issue_id / glpi_ticket_id 0 clears the link.
type vulnUpdateRequest struct {
	Status        *string `json:"status"`
	StatusNote    *string `json:"status_note"`
	IssueID       *int64  `json:"issue_id"`
	GlpiTicketID  *int64  `json:"glpi_ticket_id"`
	GlpiTicketURL *string `json:"glpi_ticket_url"`
}

// handleUpdate godoc
//
//	@Summary		Update a vulnerability finding
//	@Description	Editor+. Partial: status (open, accepted, false_positive, resolved), status_note, and the issue / GLPI ticket opened from it. accepted / false_positive survive re-imports.
//	@Tags			vulnerabilities
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Finding ID"
//	@Param			body	body		vulnUpdateRequest	true	"Fields to change"
//	@Success		200		{object}	models.VulnFinding
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/{id} [patch]
func (h *vulnHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	f, ok := h.load(w, r)
	if !ok {
		return
	}
	var req vulnUpdateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Status != nil && !vulnStatuses[*req.Status] {
		jsonError(w, http.StatusBadRequest, "invalid status")
		return
	}
	user := auth.UserFromContext(r.Context())
	if err := h.repo().Update(r.Context(), f.ID, store.VulnUpdate{
		Status: req.Status, StatusNote: req.StatusNote, IssueID: req.IssueID,
		GlpiTicketID: req.GlpiTicketID, GlpiTicketURL: req.GlpiTicketURL,
	}, user.ID); err != nil {
		jsonServerError(w, r, "failed to update vulnerability", err)
		return
	}
	updated, err := h.repo().Get(r.Context(), f.ID)
	if err != nil || updated == nil {
		jsonServerError(w, r, "failed to reload vulnerability", err)
		return
	}
	jsonOK(w, updated)
}

// vulnLinkRequest names the asset to link a finding to (or unlink from).
type vulnLinkRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
}

// handleAddLink godoc
//
//	@Summary		Link a finding to an asset
//	@Description	Editor+. Manual link to a host, DNS record, service or project, kept across imports. The asset must exist and be visible (404 otherwise).
//	@Tags			vulnerabilities
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"Finding ID"
//	@Param			body	body		vulnLinkRequest	true	"Asset"
//	@Success		200		{object}	models.VulnFinding
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/{id}/links [post]
func (h *vulnHandlers) handleAddLink(w http.ResponseWriter, r *http.Request) {
	h.changeLink(w, r, true)
}

// handleRemoveLink godoc
//
//	@Summary		Unlink a finding from an asset
//	@Description	Editor+. Removes a manual or auto link; an auto link returns on the next import that still matches it. The asset must be visible (404 otherwise).
//	@Tags			vulnerabilities
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"Finding ID"
//	@Param			body	body		vulnLinkRequest	true	"Asset"
//	@Success		200		{object}	models.VulnFinding
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/{id}/links [delete]
func (h *vulnHandlers) handleRemoveLink(w http.ResponseWriter, r *http.Request) {
	h.changeLink(w, r, false)
}

func (h *vulnHandlers) changeLink(w http.ResponseWriter, r *http.Request, add bool) {
	f, ok := h.load(w, r)
	if !ok {
		return
	}
	var req vulnLinkRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !vulnEntityTypes[req.EntityType] || req.EntityID <= 0 {
		jsonError(w, http.StatusBadRequest, "entity_type must be host, dns, service or project, with entity_id")
		return
	}
	ctx := r.Context()
	repo := h.repo()
	// Admins skip the visibility predicate, so check the row exists too.
	visible, err := store.CanSee(ctx, h.db.SQL, store.AssetType(req.EntityType), req.EntityID)
	if err == nil && visible {
		visible, err = repo.EntityExists(ctx, req.EntityType, req.EntityID)
	}
	if err != nil {
		jsonServerError(w, r, "failed to check asset", err)
		return
	}
	if !visible {
		jsonError(w, http.StatusNotFound, "asset not found")
		return
	}
	if add {
		err = repo.AddLink(ctx, f.ID, req.EntityType, req.EntityID)
	} else {
		err = repo.RemoveLink(ctx, f.ID, req.EntityType, req.EntityID)
	}
	if err != nil {
		jsonServerError(w, r, "failed to change link", err)
		return
	}
	// Unlinking the caller's last visible asset makes the finding invisible
	// to them: answer with what they sent rather than a 404.
	updated, err := repo.Get(ctx, f.ID)
	if err != nil {
		jsonServerError(w, r, "failed to reload vulnerability", err)
		return
	}
	if updated == nil {
		f.Links = nil
		updated = f
	}
	jsonOK(w, updated)
}

// handleImportETIPI godoc
//
//	@Summary		Import an ETIPI report
//	@Description	Admin. Multipart field "file": the ETIPI text report. The file is rejected when its counts don't add up. Findings are matched to DNS records (target) and hosts (VM). An open ETIPI finding missing from the file is resolved only when the file still lists its host with that severity, unless complete=true (the file is every open finding). With dry_run=true nothing is written and the counts are returned.
//	@Tags			vulnerabilities
//	@Accept			mpfd
//	@Produce		json
//	@Param			file		formData	file	true	"ETIPI report (.txt)"
//	@Param			dry_run		query		bool	false	"Count only, write nothing"
//	@Param			complete	query		bool	false	"The file lists every open finding: resolve absent ones on any host"
//	@Success		200			{object}	models.VulnImportResult
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/import/etipi [post]
func (h *vulnHandlers) handleImportETIPI(w http.ResponseWriter, r *http.Request) {
	raw, ok := readUpload(w, r, "file", maxETIPIBytes)
	if !ok {
		return
	}
	rep, err := vulnimport.ParseETIPI(raw)
	if err != nil {
		jsonBadRequest(w, r, "could not parse ETIPI report: "+err.Error(), err)
		return
	}
	dryRun := r.URL.Query().Get("dry_run") == "true"
	cover := &store.VulnCoverage{Targets: rep.Targets, Severities: rep.Severities}
	if r.URL.Query().Get("complete") == "true" {
		cover = nil
	}
	res, err := h.repo().ApplyImport(store.WithSystemScope(r.Context()), vulnimport.SourceETIPI, rep.Findings, cover, dryRun)
	if err != nil {
		jsonServerError(w, r, "failed to import ETIPI report", err)
		return
	}
	jsonOK(w, res)
}

// handleWazuhTest godoc
//
//	@Summary		Test the Wazuh integration
//	@Description	Admin. Counts the vulnerability states in the saved Wazuh indexer. Always 200: {"success": false, "error"} or {"success": true, "count"}.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/wazuh/test [post]
func (h *vulnHandlers) handleWazuhTest(w http.ResponseWriter, r *http.Request) {
	client, err := service.WazuhClient(r.Context(), h.db)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	n, err := client.Count(ctx)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]any{"success": true, "count": n})
}

// handleWazuhSync godoc
//
//	@Summary		Sync vulnerabilities from Wazuh
//	@Description	Admin. Reads every vulnerability state from the Wazuh 4.8+ indexer and imports it: one finding per agent + CVE + package version, matched to hosts by agent name. Open Wazuh findings the indexer no longer has are resolved; a read that doesn't add up to the indexer's total writes nothing. With dry_run=true nothing is written.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			dry_run	query		bool	false	"Count only, write nothing"
//	@Success		200		{object}	models.VulnImportResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/wazuh/sync [post]
func (h *vulnHandlers) handleWazuhSync(w http.ResponseWriter, r *http.Request) {
	client, err := service.WazuhClient(r.Context(), h.db)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	hits, err := client.Vulnerabilities(ctx)
	if err != nil {
		jsonErrorLogged(w, r, http.StatusBadGateway, "failed to read Wazuh: "+err.Error(), err)
		return
	}
	res, err := h.repo().ApplyImport(store.WithSystemScope(ctx), vulnimport.SourceWazuh, vulnimport.FromWazuh(hits), nil,
		r.URL.Query().Get("dry_run") == "true")
	if err != nil {
		jsonServerError(w, r, "failed to import Wazuh vulnerabilities", err)
		return
	}
	jsonOK(w, res)
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *vulnHandlers) registerRoutes(rr routeRegistrar) {
	rr.role("admin", "POST /api/wazuh/test", h.handleWazuhTest)
	rr.role("admin", "POST /api/wazuh/sync", h.handleWazuhSync)
	rr.auth("GET /api/vulnerabilities", h.handleList)
	rr.role("admin", "POST /api/vulnerabilities/import/etipi", h.handleImportETIPI)
	rr.auth("GET /api/vulnerabilities/cves", h.handleListCVEs)
	rr.auth("GET /api/vulnerabilities/cves/{cve}", h.handleGetCVE)
	rr.auth("GET /api/vulnerabilities/dependencies", h.handleListDependencies)
	rr.auth("GET /api/vulnerabilities/dependencies/{id}", h.handleGetDependency)
	rr.auth("GET /api/vulnerabilities/{id}", h.handleGet)
	rr.perm("ai.use", "POST /api/vulnerabilities/{id}/cve-analysis", h.handleAnalyzeCVE)
	rr.auth("GET /api/vulnerabilities/{id}/fix-plan", h.handleGetFixPlan)
	rr.perm("ai.use", "POST /api/vulnerabilities/{id}/fix-plan", h.handleGenerateFixPlan)
	rr.role("editor", "PATCH /api/vulnerabilities/{id}", h.handleUpdate)
	rr.role("editor", "POST /api/vulnerabilities/{id}/links", h.handleAddLink)
	rr.role("editor", "DELETE /api/vulnerabilities/{id}/links", h.handleRemoveLink)
}
