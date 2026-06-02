package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// secretShareEnv wraps secretAPIEnv with the public redemption route
// mounted alongside the authenticated mux, so tests can hit BOTH the
// owner-only mgmt endpoints AND the public GET /api/share/{token}.
type secretShareEnv struct {
	*secretAPIEnv
	publicURL string
}

func newSecretShareEnv(t *testing.T) *secretShareEnv {
	base := newSecretAPIEnv(t)

	// Spin up an additional httptest server for the public redemption
	// endpoint. The base test middleware demands X-Test-User-ID, which
	// would defeat the "anonymous redemption" contract — we need a clean
	// mux without that gate.
	// Construct a fresh repo against the same DB — repos are stateless
	// wrappers, so the authenticated mux and the public mux can hold
	// independent instances without interfering.
	publicMux := http.NewServeMux()
	publicH := &publicShareHandlers{repo: vault.NewSecretRepo(base.d)}
	publicMux.HandleFunc("GET /api/share/{token}", publicH.handleRedeem)
	srv := httptest.NewServer(publicMux)
	t.Cleanup(srv.Close)

	return &secretShareEnv{secretAPIEnv: base, publicURL: srv.URL}
}

// doPublic hits the public redemption endpoint without auth.
func (e *secretShareEnv) doPublic(token string, passphrase string) *httptest.ResponseRecorder {
	e.t.Helper()
	u := e.publicURL + "/api/share/" + token
	if passphrase != "" {
		u += "?passphrase=" + passphrase
	}
	resp, err := http.Get(u)
	if err != nil {
		e.t.Fatalf("public get: %v", err)
	}
	defer resp.Body.Close()
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	for k, vs := range resp.Header {
		for _, v := range vs {
			rec.Header().Add(k, v)
		}
	}
	b, _ := io.ReadAll(resp.Body)
	_, _ = rec.Body.Write(b)
	return rec
}

// extractToken pulls the raw token out of a CreateShareLink response body.
func extractToken(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body)
	}
	tok, _ := m["token"].(string)
	if tok == "" {
		t.Fatalf("no token in response: %s", rec.Body)
	}
	return tok
}

// ---------------------------------------------------------------------------
// POST /api/secrets/{id}/share-links
// ---------------------------------------------------------------------------

func TestShareLink_AllowsSharedSecret(t *testing.T) {
	env := newSecretShareEnv(t)
	// Policy relaxation: a shared secret IS now externally shareable by
	// anyone who can reveal it (env.bob can reveal shared secrets).
	rec := env.do(env.bob, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.sharedID),
		`{"ttl_seconds":3600}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", rec.Code, rec.Body)
	}
	if tok, _ := decodeMap(t, rec)["token"].(string); tok == "" {
		t.Errorf("expected a token in the response; got %s", rec.Body)
	}
}

func TestShareLink_OwnerCanCreate(t *testing.T) {
	env := newSecretShareEnv(t)
	rec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{"ttl_seconds":86400}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("owner create: got %d body=%s", rec.Code, rec.Body)
	}
	out := decodeMap(t, rec)
	if out["token"].(string) == "" {
		t.Error("response must include raw token (only emitted once)")
	}
}

func TestShareLink_NonOwnerCannotCreate(t *testing.T) {
	env := newSecretShareEnv(t)
	// dave is a viewer trying to share carol's personal secret.
	rec := env.do(env.dave, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{"ttl_seconds":86400}`)
	// 404 (not 403) — non-owners can't even see others' personal secrets
	// per §5.2 (the repo's loadView returns ErrSecretNotFound).
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 404/403", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// GET / DELETE /api/secrets/{id}/share-links
// ---------------------------------------------------------------------------

func TestShareLink_ListAndRevoke(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: %d", createRec.Code)
	}
	createBody := decodeMap(t, createRec)
	linkID := int64(createBody["id"].(float64))

	listRec := env.do(env.carol, "GET",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID), "")
	if listRec.Code != 200 {
		t.Fatalf("list: %d", listRec.Code)
	}
	items := decodeSlice(t, listRec)
	if len(items) != 1 {
		t.Errorf("expected 1 link, got %d", len(items))
	}

	revokeRec := env.do(env.carol, "DELETE",
		fmt.Sprintf("/api/secrets/%d/share-links/%d", env.personalCID, linkID), "")
	if revokeRec.Code != http.StatusNoContent {
		t.Errorf("revoke: got %d, want 204", revokeRec.Code)
	}
}

// admin viewing another user's personal cannot list their share links —
// §5.2 ownership rule applies (admin sees metadata but NOT share state per D6).
func TestShareLink_AdminCannotListOthersShareLinks(t *testing.T) {
	env := newSecretShareEnv(t)
	rec := env.do(env.alice, "GET",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalDID), "")
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Errorf("admin listing others' share links should be 403/404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /api/share/{token} — public redemption
// ---------------------------------------------------------------------------

func TestShareLink_PublicRedemption_HappyPath(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{"ttl_seconds":3600}`)
	tok := extractToken(t, createRec)

	rec := env.doPublic(tok, "")
	if rec.Code != 200 {
		t.Fatalf("redeem: %d body=%s", rec.Code, rec.Body)
	}
	body := decodeMap(t, rec)
	if body["payload"] != "carol-plain" {
		t.Errorf("payload mismatch: %v", body["payload"])
	}
	// Hardening sanity check (preview of Task 3.5).
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("missing Referrer-Policy: no-referrer header")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing Cache-Control: no-store header")
	}
}

func TestShareLink_PublicRedemption_BadTokenIs404(t *testing.T) {
	env := newSecretShareEnv(t)
	rec := env.doPublic("nonsense-not-a-real-token", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestShareLink_PublicRedemption_RevokedIs404(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{}`)
	tok := extractToken(t, createRec)
	linkID := int64(decodeMap(t, createRec)["id"].(float64))

	if rec := env.do(env.carol, "DELETE",
		fmt.Sprintf("/api/secrets/%d/share-links/%d", env.personalCID, linkID), ""); rec.Code != 204 {
		t.Fatalf("revoke: %d", rec.Code)
	}
	rec := env.doPublic(tok, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("revoked redemption should be 404, got %d", rec.Code)
	}
}

func TestShareLink_PublicRedemption_MaxViewsExceeded(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{"max_views":2}`)
	tok := extractToken(t, createRec)

	// Two successful redeems exhaust the budget.
	for i := 0; i < 2; i++ {
		if rec := env.doPublic(tok, ""); rec.Code != 200 {
			t.Fatalf("redeem %d: %d", i, rec.Code)
		}
	}
	rec := env.doPublic(tok, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("max-views exceeded should be 404, got %d", rec.Code)
	}
}

func TestShareLink_PublicRedemption_WrongPassphrase(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{"passphrase":"correct horse battery staple"}`)
	tok := extractToken(t, createRec)

	rec := env.doPublic(tok, "wrong-pass")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong passphrase should be 401, got %d body=%s", rec.Code, rec.Body)
	}

	rec = env.doPublic(tok, "correct%20horse%20battery%20staple")
	if rec.Code != 200 {
		t.Errorf("right passphrase should redeem, got %d body=%s", rec.Code, rec.Body)
	}
}

func TestShareLink_PublicRedemption_ExpiredIs404(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{"ttl_seconds":1}`)
	tok := extractToken(t, createRec)
	// Wait for expiry. 1s TTL + a hair so the comparison flips reliably.
	time.Sleep(1500 * time.Millisecond)
	rec := env.doPublic(tok, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expired redemption should be 404, got %d", rec.Code)
	}
}

// TestShareLink_RedemptionWritesAuditRow asserts the spec §6 / Task 3.3
// requirement that every redemption writes a `share_redeem` audit row.
func TestShareLink_RedemptionWritesAuditRow(t *testing.T) {
	env := newSecretShareEnv(t)
	createRec := env.do(env.carol, "POST",
		fmt.Sprintf("/api/secrets/%d/share-links", env.personalCID),
		`{}`)
	tok := extractToken(t, createRec)
	if rec := env.doPublic(tok, ""); rec.Code != 200 {
		t.Fatalf("redeem: %d", rec.Code)
	}

	var n int
	err := env.d.SQL.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM secret_audit_log WHERE secret_id = ? AND action = 'share_redeem'`,
		env.personalCID).Scan(&n)
	if err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 share_redeem audit row, got %d", n)
	}
}

