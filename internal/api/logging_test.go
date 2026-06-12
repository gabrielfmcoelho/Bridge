package api

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestLoggingMiddleware_LogsLineWithStatusAndDefaultActor(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/projects", nil))

	out := buf.String()
	for _, want := range []string{"[req]", "POST", "/api/projects", "201", "actor=-"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log line missing %q: %s", want, out)
		}
	}
}

func TestLoggingMiddleware_ScrubsShareToken(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/share/supersecrettoken123", nil))

	if out := buf.String(); strings.Contains(out, "supersecrettoken123") {
		t.Fatalf("share token not scrubbed from log: %s", out)
	}
}

func TestLoggingMiddleware_DefaultStatus200(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok")) // no explicit WriteHeader → defaults to 200
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/health", nil))

	if out := buf.String(); !strings.Contains(out, " 200 ") {
		t.Fatalf("expected default 200 status in log: %s", out)
	}
}
