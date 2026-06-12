package httpx

import (
	"context"
	"net"
	"net/http"
	"time"
)

// GracefulServe runs srv on ln until ctx is cancelled (e.g. SIGTERM via
// signal.NotifyContext), then drains in-flight requests with srv.Shutdown,
// bounded by timeout. It returns nil on a clean shutdown, or the serve/shutdown
// error otherwise.
//
// Taking an explicit net.Listener (rather than calling srv.ListenAndServe)
// keeps it testable: a caller can bind 127.0.0.1:0 and drive real requests
// through the drain. cmd/web.go binds srv.Addr and passes the listener here.
func GracefulServe(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err == http.ErrServerClosed {
			err = nil // expected once Shutdown is called
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		// Serve returned before any shutdown signal — a real startup failure.
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
