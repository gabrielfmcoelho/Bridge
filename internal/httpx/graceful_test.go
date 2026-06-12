package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestGracefulServe_DrainsInflight verifies the core graceful-shutdown contract:
// cancelling the context lets an already-in-flight request run to completion
// (drain) rather than killing it, and then GracefulServe returns nil.
func TestGracefulServe_DrainsInflight(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	handlerStarted := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		time.Sleep(200 * time.Millisecond) // still running when we cancel
		fmt.Fprint(w, "ok")
	})}

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- GracefulServe(ctx, srv, ln, 5*time.Second) }()

	var wg sync.WaitGroup
	wg.Add(1)
	var status int
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://" + addr + "/")
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
		}
	}()

	<-handlerStarted // request is mid-flight
	cancel()         // trigger graceful shutdown
	wg.Wait()        // the in-flight request must still finish

	if status != http.StatusOK {
		t.Fatalf("in-flight request was not drained: status=%d", status)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("GracefulServe returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GracefulServe did not return after shutdown")
	}
}

// TestGracefulServe_ReturnsListenError verifies a serve failure propagates
// instead of hanging.
func TestGracefulServe_ReturnsListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close() // closed listener → Serve fails immediately

	srv := &http.Server{Handler: http.NotFoundHandler()}
	done := make(chan error, 1)
	go func() { done <- GracefulServe(context.Background(), srv, ln, time.Second) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a serve error on a closed listener, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GracefulServe hung on a serve error")
	}
}
