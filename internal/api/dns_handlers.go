package api

import (
	"errors"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// dnsHandlers is a Phase 2 reference handler: it holds a domain service (not a
// raw *database.DB), and each method is parse → call service → render. All the
// enrichment/orchestration lives in service.DNSService.
type dnsHandlers struct {
	dns *service.DNSService
}

func (h *dnsHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	pp := parsePageParams(r)
	f := models.DNSFilter{
		Search:      r.URL.Query().Get("search"),
		Situacao:    r.URL.Query().Get("situacao"),
		Tag:         r.URL.Query().Get("tag"),
		Responsavel: r.URL.Query().Get("responsavel"),
		HasHTTPS:    r.URL.Query().Get("has_https"),
		SortBy:      r.URL.Query().Get("sort_by"),
		SortDir:     r.URL.Query().Get("sort_dir"),
		Page:        pp.Page,
		PerPage:     pp.PerPage,
	}
	items, err := h.dns.List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list DNS records", err)
		return
	}
	// Real server-side pagination (R4 envelope). When per_page is set we report
	// the matching Count; an unbounded request reports the returned length.
	total := len(items)
	if !pp.Unbounded() {
		if n, err := h.dns.Count(r.Context(), f); err == nil {
			total = n
		}
	}
	jsonList(w, items, metaFor(pp, total))
}

func (h *dnsHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	detail, err := h.dns.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load DNS record", err)
		return
	}
	if detail == nil {
		jsonError(w, http.StatusNotFound, "DNS record not found")
		return
	}
	jsonOK(w, detail)
}

// dnsWriteRequest is the create/update wire shape: a DNSRecord plus its
// relations and entidade grants. Pointer slices distinguish "absent" (leave
// unchanged) from "present but empty" (clear) on update.
type dnsWriteRequest struct {
	models.DNSRecord
	models.AssetGrantsInput
	Tags         *[]string                  `json:"tags"`
	HostIDs      *[]int64                   `json:"host_ids"`
	Responsaveis *[]models.ResponsavelInput `json:"responsaveis"`
}

func (req *dnsWriteRequest) toWrite() *service.DNSWrite {
	return &service.DNSWrite{Record: req.DNSRecord, Tags: req.Tags, HostIDs: req.HostIDs, Responsaveis: req.Responsaveis}
}

func (h *dnsHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req dnsWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"domain": req.Domain}) {
		return
	}
	wr := req.toWrite()
	g, ok := resolveGrants(w, r, req.AssetGrantsInput, nil)
	if !ok {
		return
	}
	wr.Grants = &g
	if err := h.dns.Create(r.Context(), wr); err != nil {
		if errors.Is(err, service.ErrSetEntidades) {
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
		jsonError(w, http.StatusConflict, "domain already exists")
		return
	}
	jsonCreated(w, wr.Record)
}

func (h *dnsHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req dnsWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	wr := req.toWrite()
	if req.AssetGrantsInput.Present() {
		existing, err := h.dns.Grants(r.Context(), id)
		if err != nil {
			jsonServerError(w, r, "failed to load entidades", err)
			return
		}
		g, ok := resolveGrants(w, r, req.AssetGrantsInput, &existing)
		if !ok {
			return
		}
		wr.Grants = &g
	}
	found, err := h.dns.Update(r.Context(), id, wr)
	if err != nil {
		jsonServerError(w, r, "failed to update DNS record", err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "DNS record not found")
		return
	}
	jsonOK(w, wr.Record)
}

func (h *dnsHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.dns.Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete DNS record", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *dnsHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/dns", h.handleList)
	rr.role("editor", "POST /api/dns", h.handleCreate)
	rr.auth("GET /api/dns/{id}", h.handleGet)
	rr.role("editor", "PUT /api/dns/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/dns/{id}", h.handleDelete)
}
