package api

import (
	"net/http"
	"strings"
)

// This file holds the request-side transport helpers that fold the two most
// repeated handler boilerplate shapes — "decode JSON or 400" and "parse path
// id or 400" — into one call each. They return ok=false AFTER writing the
// error response, so the caller's pattern is uniformly:
//
//	if !decodeBody(w, r, &req) { return }
//	id, ok := pathID(w, r, "id"); if !ok { return }
//
// Legacy handlers still call decodeJSON + pathInt64 directly; they migrate to
// these incrementally as each handler is thinned (Phase 2). New code should
// prefer these.

// decodeBody decodes the JSON request body into v. On failure it writes a 400
// (logging the decode error for diagnostics) and returns false.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := decodeJSON(r, v); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return false
	}
	return true
}

// pathID parses an int64 path parameter (e.g. "id"). On failure it writes a
// 400 and returns ok=false.
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := pathInt64(r, name)
	if err != nil {
		jsonBadRequest(w, r, "invalid "+name, err)
		return 0, false
	}
	return id, true
}

// requireFields verifies that every named field has a non-blank value. The map
// is field-name → value. On the first blank it writes a 400 naming the field
// and returns false. Field iteration order is not guaranteed, so callers that
// need deterministic "first missing" messaging should check individually.
func requireFields(w http.ResponseWriter, fields map[string]string) bool {
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			jsonError(w, http.StatusBadRequest, name+" is required")
			return false
		}
	}
	return true
}
