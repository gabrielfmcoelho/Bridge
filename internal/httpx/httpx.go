// Package httpx holds the lowest-level HTTP response primitives shared across
// the codebase. It deliberately depends on nothing inside internal/ so that
// both the transport layer (internal/api) and the auth middleware
// (internal/auth) can emit identical JSON responses without an import cycle.
//
// Before this package existed, auth middleware wrote errors via http.Error —
// which sets Content-Type: text/plain and appends a newline — while api
// handlers wrote application/json. Clients therefore saw two different error
// shapes depending on whether the request failed at the middleware or the
// handler. Everything now funnels through WriteError so the shape is uniform.
package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSON encodes v as JSON with the given status code. Encoding errors are
// swallowed: the header is already committed by WriteHeader, so there is no
// meaningful recovery beyond what the http.Server logs.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes a {"error": msg} body with the given status. This is the
// single canonical error shape for the whole HTTP surface.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}
