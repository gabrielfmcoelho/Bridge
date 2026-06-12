package auth

import (
	"bytes"
	"context"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestActorSink_RecordAndRead(t *testing.T) {
	ctx, actor := WithActorSink(context.Background())
	if got := actor(); got != "" {
		t.Fatalf("expected empty actor before record, got %q", got)
	}
	recordActor(ctx, "alice")
	if got := actor(); got != "alice" {
		t.Fatalf("expected alice, got %q", got)
	}
}

func TestRecordActor_NoSink_NoPanic(t *testing.T) {
	// recordActor on a context without a sink must be a silent no-op.
	recordActor(context.Background(), "bob")
}

func TestAuditAuthFailure_LogsReasonPathRemote(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	r := httptest.NewRequest("GET", "/api/hosts", nil)
	r.RemoteAddr = "10.1.2.3:5555"
	auditAuthFailure(r, "no session token")

	out := buf.String()
	for _, want := range []string{"denied", "/api/hosts", "no session token", "10.1.2.3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("audit line missing %q: %s", want, out)
		}
	}
}

func TestClientIP_PrefersFirstXForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:9999"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("expected first XFF ip, got %q", got)
	}
}

func TestClientIP_FallsBackToRemoteAddrHost(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.5.9:4321"
	if got := clientIP(r); got != "192.168.5.9" {
		t.Fatalf("expected remote host, got %q", got)
	}
}
