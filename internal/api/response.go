package api

import (
	"encoding/json"
	"log"
	"net/http"
)

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func jsonCreated(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// jsonServerError writes a 500 response and logs the underlying error with
// request context so operators can diagnose production failures. The client
// only sees the sanitized `msg`; the real `err` stays in server logs.
func jsonServerError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	log.Printf("[api] %s %s: %s: %v", r.Method, r.URL.Path, msg, err)
	jsonError(w, http.StatusInternalServerError, msg)
}

// jsonBadRequest writes a 400 response. If err is non-nil, it is logged for
// server-side diagnostics (e.g. JSON decode failures, validation surprises).
func jsonBadRequest(w http.ResponseWriter, r *http.Request, msg string, err error) {
	if err != nil {
		log.Printf("[api] %s %s: bad request: %s: %v", r.Method, r.URL.Path, msg, err)
	}
	jsonError(w, http.StatusBadRequest, msg)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
