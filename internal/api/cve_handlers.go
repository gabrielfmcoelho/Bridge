package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/nvd"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

var cveIDRe = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

func (h *vulnHandlers) cveRepo() *store.CVECatalogRepo { return store.NewCVECatalogRepo(h.db.SQL) }

// handleListCVEs godoc
//
//	@Summary		List the CVE catalog
//	@Description	Analysed CVEs, most recent first, without the analysis text. A non-admin sees a CVE only while a finding they can see carries it. Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			search		query		string	false	"CVE id or description"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (default 50)"
//	@Success		200			{object}	ListEnvelope[models.CVECatalogEntry]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/cves [get]
func (h *vulnHandlers) handleListCVEs(w http.ResponseWriter, r *http.Request) {
	pp := parsePageParams(r)
	if pp.Unbounded() {
		pp.PerPage = 50
	}
	items, total, err := h.cveRepo().List(r.Context(), r.URL.Query().Get("search"), pp.Page, pp.PerPage)
	if err != nil {
		jsonServerError(w, r, "failed to list CVE catalog", err)
		return
	}
	jsonList(w, items, metaFor(pp, total))
}

// handleGetCVE godoc
//
//	@Summary		Get a CVE catalog entry
//	@Description	The cached NVD text and pt-BR / en analysis. Never calls NVD or the LLM. Answers 200 with a null body when the CVE was never analysed or is not visible. Any role.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			cve	path		string	true	"CVE id, e.g. CVE-2024-1234"
//	@Success		200	{object}	models.CVECatalogEntry
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/vulnerabilities/cves/{cve} [get]
func (h *vulnHandlers) handleGetCVE(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(strings.TrimSpace(r.PathValue("cve")))
	if !cveIDRe.MatchString(id) {
		jsonError(w, http.StatusBadRequest, "invalid CVE id")
		return
	}
	e, err := h.cveRepo().Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load CVE", err)
		return
	}
	if e == nil {
		jsonOK(w, nil)
		return
	}
	jsonOK(w, e)
}

// handleAnalyzeCVE godoc
//
//	@Summary		Analyse a finding's CVE
//	@Description	Fetches the CVE from NVD and asks the LLM for a generic analysis in pt-BR and en (in parallel), then saves it to the CVE catalog, replacing any previous one. Keyed by finding so the caller must see it. Requires ai.use.
//	@Tags			vulnerabilities
//	@Produce		json
//	@Param			id	path		int	true	"Finding ID"
//	@Success		200	{object}	models.CVECatalogEntry
//	@Failure		400	{object}	httpx.ErrorResponse	"Finding has no valid CVE"
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse	"Finding not visible, or CVE not in NVD"
//	@Failure		502	{object}	httpx.ErrorResponse	"NVD or LLM failed"
//	@Failure		503	{object}	httpx.ErrorResponse	"AI not configured"
//	@Router			/api/vulnerabilities/{id}/cve-analysis [post]
func (h *vulnHandlers) handleAnalyzeCVE(w http.ResponseWriter, r *http.Request) {
	f, ok := h.load(w, r)
	if !ok {
		return
	}
	id := strings.ToUpper(strings.TrimSpace(f.CVE))
	if !cveIDRe.MatchString(id) {
		jsonError(w, http.StatusBadRequest, "finding has no valid CVE")
		return
	}
	client, err := service.LLMClient(r.Context(), h.db)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	rec, err := h.nvd.CVE(ctx, id)
	if errors.Is(err, nvd.ErrNotFound) {
		jsonError(w, http.StatusNotFound, id+" not found in NVD")
		return
	}
	if err != nil {
		jsonError(w, http.StatusBadGateway, "NVD request failed: "+err.Error())
		return
	}

	// Both languages at once; one failure saves nothing.
	var wg sync.WaitGroup
	var pt, en string
	var ptErr, enErr error
	wg.Add(2)
	go func() { defer wg.Done(); pt, ptErr = analyzeCVE(ctx, client, rec, "Responda em português do Brasil.") }()
	go func() { defer wg.Done(); en, enErr = analyzeCVE(ctx, client, rec, "Respond in English.") }()
	wg.Wait()
	if err := errors.Join(ptErr, enErr); err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}

	user := auth.UserFromContext(r.Context())
	entry := &models.CVECatalogEntry{
		CVEID: id, Description: rec.Description, CVSS: rec.CVSS, CVSSVector: rec.CVSSVector, CWE: rec.CWE,
		PublishedAt: rec.Published, References: rec.References,
		AnalysisPT: pt, AnalysisEN: en, Model: client.Model(), AnalyzedBy: &user.ID,
	}
	if err := h.cveRepo().Upsert(r.Context(), entry); err != nil {
		jsonServerError(w, r, "failed to save CVE analysis", err)
		return
	}
	saved, err := h.cveRepo().Get(r.Context(), id)
	if err != nil || saved == nil {
		jsonServerError(w, r, "failed to reload CVE analysis", err)
		return
	}
	jsonOK(w, saved)
}

// analyzeCVE runs one language of the analysis over the NVD record.
func analyzeCVE(ctx context.Context, client *llm.Client, rec *nvd.CVE, language string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nCVE: %s\nDescription: %s\n", language, rec.ID, rec.Description)
	if rec.CVSS != nil {
		fmt.Fprintf(&b, "CVSS: %.1f (%s)\n", *rec.CVSS, rec.CVSSVector)
	}
	if rec.CWE != "" {
		fmt.Fprintf(&b, "CWE: %s\n", rec.CWE)
	}
	if rec.Published != nil {
		fmt.Fprintf(&b, "Published: %s\n", rec.Published.Format("2006-01-02"))
	}
	if len(rec.References) > 0 {
		b.WriteString("References:\n- " + strings.Join(rec.References, "\n- ") + "\n")
	}
	reply, err := client.Complete(ctx, []llm.ChatMessage{
		{Role: "system", Content: llm.CVEAnalysisSystem},
		{Role: "user", Content: b.String()},
	})
	if err != nil {
		return "", err
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return "", errors.New("empty answer")
	}
	return reply, nil
}
