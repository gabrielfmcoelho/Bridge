package auth

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// actorSink is a per-request, middleware-writable holder for the authenticated
// username. An OUTER logging middleware installs it before RequireAuth runs;
// RequireAuth writes into it once the user is resolved. Because the sink is a
// pointer carried in the context, the logging middleware sees the value even
// though RequireAuth sets the user on a *downstream* context it can't read back.
type actorSink struct {
	mu   sync.Mutex
	name string
}

type actorSinkKey struct{}

// WithActorSink installs a writable actor holder and returns the derived context
// plus a getter for the recorded username ("" until RequireAuth records one).
func WithActorSink(ctx context.Context) (context.Context, func() string) {
	s := &actorSink{}
	ctx = context.WithValue(ctx, actorSinkKey{}, s)
	return ctx, func() string {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.name
	}
}

// recordActor writes name into the request's actor sink if one is present;
// otherwise it's a silent no-op (e.g. routes invoked without the middleware).
func recordActor(ctx context.Context, name string) {
	if s, ok := ctx.Value(actorSinkKey{}).(*actorSink); ok {
		s.mu.Lock()
		s.name = name
		s.mu.Unlock()
	}
}

// auditAuthFailure emits one structured log line for a denied request so failed
// auth attempts are visible in ops logs. The session token lives in a cookie/
// header (never the path), so logging method+path+remote leaks no secret.
func auditAuthFailure(r *http.Request, reason string) {
	log.Printf("[auth-audit] denied method=%s path=%s reason=%q remote=%s",
		r.Method, r.URL.Path, reason, clientIP(r))
}

// clientIP returns the best-effort caller IP: the first X-Forwarded-For hop when
// present (proxy/ingress), else the RemoteAddr host.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
