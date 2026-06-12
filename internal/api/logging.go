package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
)

// statusRecorder wraps a ResponseWriter to capture the final status code for the
// access log. A handler that writes a body without calling WriteHeader gets the
// implicit 200 that net/http would send.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}

// loggingMiddleware emits one structured line per request: method, token-scrubbed
// path, status, latency, and actor. The actor is filled in by auth.RequireAuth
// via the sink installed here (RequireAuth sets the user on a downstream context
// this middleware can't read back, so the shared sink carries the name up).
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, actor := auth.WithActorSink(r.Context())
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r.WithContext(ctx))

		name := actor()
		if name == "" {
			name = "-"
		}
		log.Printf("[req] %s %s %d %s actor=%s",
			r.Method, scrubPath(r.URL.Path), rec.status,
			time.Since(start).Round(time.Millisecond), name)
	})
}
