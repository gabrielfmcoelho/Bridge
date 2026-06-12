package api

import (
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
)

// R4: every plain-list endpoint returns a uniform envelope
//
//	{ "data": [ ... ], "meta": { "page": N, "per_page": N, "total": N } }
//
// so the frontend (and any client) can treat all lists identically. Detail
// (single-object) responses and bespoke integration payloads keep their own
// shapes — the envelope is for generic collections only.

// Meta is the pagination block of a list envelope. For an unbounded request
// (per_page omitted or 0) it reports the full count with page=1.
type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// PageParams is the parsed pagination request. PerPage<=0 means "unbounded"
// (return everything) — the mode dropdowns/selects and internal callers rely
// on so they never get truncated to a single page.
type PageParams struct {
	Page    int
	PerPage int
}

// Unbounded reports whether the caller asked for the full list (no LIMIT).
func (p PageParams) Unbounded() bool { return p.PerPage <= 0 }

// Limit/Offset translate the params into SQL clause values. Only meaningful
// when !Unbounded().
func (p PageParams) Limit() int  { return p.PerPage }
func (p PageParams) Offset() int { return (p.Page - 1) * p.PerPage }

const maxPerPage = 200

// parsePageParams reads ?page and ?per_page. Defaults: page=1, per_page=0
// (unbounded). per_page is clamped to [0, maxPerPage]; page to >=1. A bare
// request (no params) therefore returns the whole list, matching the
// pre-R4 behavior for any consumer that hasn't adopted server pagination.
func parsePageParams(r *http.Request) PageParams {
	pp := PageParams{Page: 1, PerPage: 0}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 1 {
			pp.Page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > maxPerPage {
				n = maxPerPage
			}
			pp.PerPage = n
		}
	}
	return pp
}

// metaFor builds the envelope meta for the given params and total row count.
// Unbounded requests report page=1, per_page=total.
func metaFor(p PageParams, total int) Meta {
	if p.Unbounded() {
		return Meta{Page: 1, PerPage: total, Total: total}
	}
	return Meta{Page: p.Page, PerPage: p.PerPage, Total: total}
}

// jsonList writes a 200 list envelope. data should be a non-nil slice; callers
// already guard nil→empty, and jsonListAny normalizes the common nil cases.
func jsonList[T any](w http.ResponseWriter, data []T, meta Meta) {
	if data == nil {
		data = []T{}
	}
	httpx.WriteJSON(w, http.StatusOK, listEnvelope[T]{Data: data, Meta: meta})
}

type listEnvelope[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

// jsonPaged is the common list-endpoint tail: parse ?page/?per_page from r,
// window items in memory, and write the list envelope with the full slice
// length as meta.total. A 1:1 replacement for `jsonOK(w, slice)`.
//
// In-memory windowing (rather than SQL LIMIT/OFFSET) is deliberate: these list
// endpoints do bulk enrichment (all-entity joins regardless of page), so
// LIMIT on the base query would save almost nothing at this data scale. The
// large hosts table keeps its own SQL pagination path.
func jsonPaged[T any](w http.ResponseWriter, r *http.Request, items []T) {
	pp := parsePageParams(r)
	jsonList(w, pageSlice(items, pp), metaFor(pp, len(items)))
}

// pageSlice paginates an already-materialized slice in memory. Used by the
// long-tail list endpoints whose repos return enriched rows (the page is small
// enough that a full fetch + slice is fine); the big tables use SQL LIMIT/OFFSET
// instead. Returns the page window for pp (or all of items when unbounded).
func pageSlice[T any](items []T, pp PageParams) []T {
	if pp.Unbounded() {
		return items
	}
	start := pp.Offset()
	if start >= len(items) {
		return []T{}
	}
	end := start + pp.PerPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
