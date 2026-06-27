package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// bundleHandlerFixture seeds two users + a personal secret and returns the
// pieces needed to exercise the bundle HTTP handlers directly (the actor is
// injected via context, exactly as actorFrom expects).
type bundleHandlerFixture struct {
	repo         *vault.SecretRepo
	owner, other *models.User
	actor        vault.ActorContext
	secretID     int64
}

func newBundleHandlerFixture(t *testing.T) *bundleHandlerFixture {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	seed := func(name, role string) *models.User {
		var id int64
		if err := d.SQL.QueryRow(
			`INSERT INTO users (username, password_hash, role) VALUES (?,?,?) RETURNING id`,
			name, "x", role).Scan(&id); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		return &models.User{ID: id, Username: name, Role: role}
	}
	owner := seed("owner", "editor")
	other := seed("other", "editor")

	repo := vault.NewSecretRepo(d)
	actor := vault.ActorContext{UserID: owner.ID, Role: owner.Role}
	secretID, err := repo.Create(context.Background(), actor, &models.Secret{
		Type:        models.SecretTypePassword,
		Scope:       models.SecretScopeAvulso,
		Visibility:  models.SecretVisibilityPersonal,
		OwnerUserID: owner.ID,
		Name:        "db-pass",
		KeyVersion:  1,
		CreatedBy:   owner.ID,
	}, "s3cr3t")
	if err != nil {
		t.Fatalf("seed secret: %v", err)
	}
	return &bundleHandlerFixture{repo: repo, owner: owner, other: other, actor: actor, secretID: secretID}
}

func (f *bundleHandlerFixture) as(u *models.User, r *http.Request) *http.Request {
	return r.WithContext(auth.WithUser(r.Context(), u))
}

// TestBundleHandlers_ListByItem verifies GET /api/share-bundles?item_type=&ref_id=
// returns only bundles containing that item (the per-API "already emitted
// links" list), and an empty envelope for a non-matching ref.
func TestBundleHandlers_ListByItem(t *testing.T) {
	f := newBundleHandlerFixture(t)
	h := &bundleHandlers{repo: f.repo}

	if _, _, err := f.repo.CreateBundle(context.Background(), f.actor,
		[]vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: f.secretID}},
		vault.CreateBundleOpts{Title: "link", TTL: time.Hour}); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	// Matching item -> exactly one row in the data envelope.
	req := f.as(f.owner, httptest.NewRequest(http.MethodGet,
		"/api/share-bundles?item_type=secret&ref_id="+strconv.FormatInt(f.secretID, 10), nil))
	rec := httptest.NewRecorder()
	h.handleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if n := len(decodeEnvelopeData(t, rec)); n != 1 {
		t.Errorf("expected 1 bundle for the secret, got %d", n)
	}

	// Non-matching ref -> empty data.
	req2 := f.as(f.owner, httptest.NewRequest(http.MethodGet,
		"/api/share-bundles?item_type=secret&ref_id=999999", nil))
	rec2 := httptest.NewRecorder()
	h.handleList(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("list2 status = %d", rec2.Code)
	}
	if n := len(decodeEnvelopeData(t, rec2)); n != 0 {
		t.Errorf("expected 0 bundles for bogus ref, got %d", n)
	}
}

// TestBundleHandlers_Renew verifies PATCH /api/share-bundles/{id} extends the
// expiry (owner) and rejects a non-owner with 404.
func TestBundleHandlers_Renew(t *testing.T) {
	f := newBundleHandlerFixture(t)
	h := &bundleHandlers{repo: f.repo}

	_, view, err := f.repo.CreateBundle(context.Background(), f.actor,
		[]vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: f.secretID}},
		vault.CreateBundleOpts{Title: "renew", TTL: time.Minute})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	idStr := strconv.FormatInt(view.ID, 10)

	// Owner renews to +1h.
	req := f.as(f.owner, httptest.NewRequest(http.MethodPatch,
		"/api/share-bundles/"+idStr, strings.NewReader(`{"ttl_seconds":3600}`)))
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()
	h.handleRenew(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("renew status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.ExpiresAt.After(time.Now().Add(30 * time.Minute)) {
		t.Errorf("expected expiry ~1h out, got %v", got.ExpiresAt)
	}

	// Non-owner gets 404.
	req2 := f.as(f.other, httptest.NewRequest(http.MethodPatch,
		"/api/share-bundles/"+idStr, strings.NewReader(`{"ttl_seconds":3600}`)))
	req2.SetPathValue("id", idStr)
	rec2 := httptest.NewRecorder()
	h.handleRenew(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("non-owner renew status = %d, want 404", rec2.Code)
	}
}

// TestBundleHandlers_Reissue verifies POST /api/share-bundles/reissue rebuilds a
// link under a supplied token, and rejects a token already in use with 409.
func TestBundleHandlers_Reissue(t *testing.T) {
	f := newBundleHandlerFixture(t)
	h := &bundleHandlers{repo: f.repo}

	rawToken := "AOD2ud7zvCDBq65zmM-1ovtxyr4ZYsfu6wQ6uj0N8Pg"
	body := `{"token":"` + rawToken + `","ttl_seconds":3600,"items":[{"type":"secret","ref_id":` +
		strconv.FormatInt(f.secretID, 10) + `}]}`

	req := f.as(f.owner, httptest.NewRequest(http.MethodPost,
		"/api/share-bundles/reissue", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	h.handleReissue(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("reissue status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// Second reissue of the now-live token must conflict.
	req2 := f.as(f.owner, httptest.NewRequest(http.MethodPost,
		"/api/share-bundles/reissue", strings.NewReader(body)))
	rec2 := httptest.NewRecorder()
	h.handleReissue(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("duplicate reissue status = %d, want 409", rec2.Code)
	}
}

// TestBundleHandlers_AccessLog verifies GET /api/share-bundles/{id}/access-log
// returns the owner's recorded accesses (200) and 404s a non-owner.
func TestBundleHandlers_AccessLog(t *testing.T) {
	f := newBundleHandlerFixture(t)
	h := &bundleHandlers{repo: f.repo}

	tok, view, err := f.repo.CreateBundle(context.Background(), f.actor,
		[]vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: f.secretID}},
		vault.CreateBundleOpts{Title: "logged", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	// Anonymous redeem records one access-log row.
	if _, err := f.repo.RedeemBundle(context.Background(), tok, "",
		vault.RedeemMeta{RemoteIP: "198.51.100.4", UserAgent: "probe/1.0"}); err != nil {
		t.Fatalf("redeem: %v", err)
	}
	idStr := strconv.FormatInt(view.ID, 10)

	// Owner sees exactly one row.
	req := f.as(f.owner, httptest.NewRequest(http.MethodGet,
		"/api/share-bundles/"+idStr+"/access-log", nil))
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()
	h.handleAccessLog(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("access-log status = %d, body=%s", rec.Code, rec.Body.String())
	}
	rows := decodeEnvelopeData(t, rec)
	if len(rows) != 1 {
		t.Fatalf("expected 1 access-log row, got %d", len(rows))
	}
	if rows[0]["remote_ip"] != "198.51.100.4" {
		t.Errorf("remote_ip not surfaced: %+v", rows[0])
	}

	// Non-owner gets 404.
	req2 := f.as(f.other, httptest.NewRequest(http.MethodGet,
		"/api/share-bundles/"+idStr+"/access-log", nil))
	req2.SetPathValue("id", idStr)
	rec2 := httptest.NewRecorder()
	h.handleAccessLog(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("non-owner access-log status = %d, want 404", rec2.Code)
	}
}

func decodeEnvelopeData(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%q)", err, rec.Body.String())
	}
	return env.Data
}
