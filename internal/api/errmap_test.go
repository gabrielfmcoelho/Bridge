package api

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// stubDomainErr implements DomainError for the typed-mapping test.
type stubDomainErr struct{}

func (stubDomainErr) Error() string         { return "internal: row 5 conflict" }
func (stubDomainErr) HTTPStatus() int       { return 409 }
func (stubDomainErr) ClientMessage() string { return "already exists" }

func TestWriteErr_Mapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string // substring expected in body
	}{
		{"not found", vault.ErrSecretNotFound, 404, "secret not found"},
		{"forbidden", vault.ErrSecretForbidden, 403, "forbidden"},
		{"wrapped not found", fmt.Errorf("get: %w", vault.ErrSecretNotFound), 404, "secret not found"},
		{"domain error", stubDomainErr{}, 409, "already exists"},
		{"unknown -> 500", errors.New("boom"), 500, "request failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/secrets/1", nil)
			writeErr(rec, req, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tc.wantMsg) {
				t.Fatalf("body = %q, want substring %q", body, tc.wantMsg)
			}
		})
	}
}

// stubDomainErr must NOT leak its internal Error() text to the client.
func TestWriteErr_DomainErrorDoesNotLeakInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	writeErr(rec, req, stubDomainErr{})
	if strings.Contains(rec.Body.String(), "row 5 conflict") {
		t.Fatalf("internal detail leaked to client: %q", rec.Body.String())
	}
}
