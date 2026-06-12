package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
)

// scrubPath redacts share-link tokens from URL paths before they hit logs.
// /api/share/{token} contains the SECRET CAPABILITY for the share link
// (the token is also the HKDF salt — see secretshare). Logging it would
// hand a copy to anyone with log access, defeating the D4 property.
//
// Returns "/api/share/[REDACTED]" for any path under /api/share/; passes
// other paths through unchanged. Cheap enough to apply unconditionally
// at every log call site.
func scrubPath(path string) string {
	const prefix = "/api/share/"
	if strings.HasPrefix(path, prefix) {
		return prefix + "[REDACTED]"
	}
	return path
}

func jsonOK(w http.ResponseWriter, data any) {
	httpx.WriteJSON(w, http.StatusOK, data)
}

func jsonCreated(w http.ResponseWriter, data any) {
	httpx.WriteJSON(w, http.StatusCreated, data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	httpx.WriteError(w, status, msg)
}

// jsonErrorLogged writes `status` with the sanitized client message `msg` and
// logs the underlying `err` (when non-nil) with request context. Use it for
// 4xx responses where the real error must NOT reach the client but operators
// still need it — e.g. a unique-constraint violation surfaced as 409. This is
// the non-500 sibling of jsonServerError.
func jsonErrorLogged(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	if err != nil {
		log.Printf("[api] %s %s: %s: %v", r.Method, scrubPath(r.URL.Path), msg, err)
	}
	jsonError(w, status, msg)
}

// jsonServerError writes a 500 response and logs the underlying error with
// request context so operators can diagnose production failures. The client
// only sees the sanitized `msg`; the real `err` stays in server logs.
func jsonServerError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	log.Printf("[api] %s %s: %s: %v", r.Method, scrubPath(r.URL.Path), msg, err)
	jsonError(w, http.StatusInternalServerError, msg)
}

// jsonBadRequest writes a 400 response. If err is non-nil, it is logged for
// server-side diagnostics (e.g. JSON decode failures, validation surprises).
func jsonBadRequest(w http.ResponseWriter, r *http.Request, msg string, err error) {
	if err != nil {
		log.Printf("[api] %s %s: bad request: %s: %v", r.Method, scrubPath(r.URL.Path), msg, err)
	}
	jsonError(w, http.StatusBadRequest, msg)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
