package api

import (
	"errors"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// DomainError lets a domain/service error declare the HTTP status it should map
// to, so the transport layer doesn't need a hand-written switch per call site.
// Implement it on sentinel error types as the service layer grows (Phase 1/2);
// writeErr will pick up the status automatically.
type DomainError interface {
	error
	HTTPStatus() int
	// ClientMessage is the safe, caller-facing message. It must not leak
	// internal detail (SQL text, file paths, upstream error strings).
	ClientMessage() string
}

// writeErr is the single error responder for handlers. Resolution order:
//  1. if err implements DomainError, use its status + client message;
//  2. else map known vault sentinels (the current domain errors);
//  3. else treat as an unexpected 500 (real error logged, sanitized to client).
//
// It replaces the per-handler writeRepoErr plus the duplicated errors.Is
// switches in secret_share_handlers.go and share_bundle_handlers.go.
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var de DomainError
	if errors.As(err, &de) {
		jsonError(w, de.HTTPStatus(), de.ClientMessage())
		return
	}
	switch {
	case errors.Is(err, vault.ErrSecretNotFound):
		jsonError(w, http.StatusNotFound, "secret not found")
	case errors.Is(err, vault.ErrSecretForbidden):
		jsonError(w, http.StatusForbidden, "forbidden")
	default:
		jsonServerError(w, r, "request failed", err)
	}
}
