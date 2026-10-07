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
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
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
	admin        *models.User
	contactID    int64
	d            *database.DB
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
	admin := seed("admin", "admin")
	var contactID int64
	if err := d.SQL.QueryRow(`INSERT INTO contacts (name, phone) VALUES (?, '') RETURNING id`, "Fulano").Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

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
	return &bundleHandlerFixture{repo: repo, owner: owner, other: other, admin: admin, contactID: contactID, d: d, actor: actor, secretID: secretID}
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

// TestBundleHandlers_RevealAndRevokeAll covers the admin routes: reveal answers
// the token, URL and passphrase (404 for a non-admin, 409 for a bundle without
// a stored cipher) and is logged; revoke-all reports how many it revoked. Also
// checks create's recipient fields: a known contact is kept, an unknown one is 400.
func TestBundleHandlers_RevealAndRevokeAll(t *testing.T) {
	f := newBundleHandlerFixture(t)
	h := &bundleHandlers{repo: f.repo}
	secretRef := strconv.FormatInt(f.secretID, 10)

	// Create with a recipient contact and a passphrase.
	body := `{"title":"for fulano","ttl_seconds":3600,"passphrase":"pw-123","recipient_contact_id":` +
		strconv.FormatInt(f.contactID, 10) + `,"recipient_label":"Equipe X","items":[{"type":"secret","ref_id":` + secretRef + `}]}`
	rec := httptest.NewRecorder()
	h.handleCreate(rec, f.as(f.owner, httptest.NewRequest(http.MethodPost, "/api/share-bundles", strings.NewReader(body))))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	// Unknown recipient contact -> 400.
	bad := `{"ttl_seconds":3600,"recipient_contact_id":999999,"items":[{"type":"secret","ref_id":` + secretRef + `}]}`
	recBad := httptest.NewRecorder()
	h.handleCreate(recBad, f.as(f.owner, httptest.NewRequest(http.MethodPost, "/api/share-bundles", strings.NewReader(bad))))
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("unknown recipient status = %d, want 400", recBad.Code)
	}

	// The admin's list carries the recipient and recoverability.
	recList := httptest.NewRecorder()
	h.handleList(recList, f.as(f.admin, httptest.NewRequest(http.MethodGet, "/api/share-bundles", nil)))
	rows := decodeEnvelopeData(t, recList)
	if len(rows) != 1 || rows[0]["recipient_name"] != "Fulano" || rows[0]["recipient_label"] != "Equipe X" ||
		rows[0]["recoverable"] != true || rows[0]["created_by_name"] != "owner" {
		t.Fatalf("admin list row = %+v", rows)
	}

	reveal := func(u *models.User, id string) *httptest.ResponseRecorder {
		req := f.as(u, httptest.NewRequest(http.MethodPost, "/api/share-bundles/"+id+"/reveal", nil))
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h.handleReveal(rec, req)
		return rec
	}
	recRev := reveal(f.admin, idStr)
	if recRev.Code != http.StatusOK {
		t.Fatalf("reveal status = %d, body=%s", recRev.Code, recRev.Body.String())
	}
	var got bundleRevealResponse
	if err := json.Unmarshal(recRev.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode reveal: %v", err)
	}
	if got.Token != created.Token || got.URL != "/share/"+created.Token || got.Passphrase != "pw-123" {
		t.Errorf("reveal = %+v, want token %q and passphrase", got, created.Token)
	}
	if c := reveal(f.owner, idStr).Code; c != http.StatusNotFound {
		t.Errorf("non-admin reveal status = %d, want 404", c)
	}

	// A bundle without a stored cipher (pre-v105) -> 409.
	_, legacy, err := f.repo.CreateBundle(context.Background(), f.actor,
		[]vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: f.secretID}},
		vault.CreateBundleOpts{Title: "legacy", TTL: time.Hour})
	if err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	if _, err := f.d.SQL.Exec(`UPDATE share_bundles SET token_cipher = NULL, token_nonce = NULL WHERE id = ?`, legacy.ID); err != nil {
		t.Fatalf("strip cipher: %v", err)
	}
	if c := reveal(f.admin, strconv.FormatInt(legacy.ID, 10)).Code; c != http.StatusConflict {
		t.Errorf("legacy reveal status = %d, want 409", c)
	}

	// The reveal shows up in the access log with the admin's name.
	reqLog := f.as(f.admin, httptest.NewRequest(http.MethodGet, "/api/share-bundles/"+idStr+"/access-log", nil))
	reqLog.SetPathValue("id", idStr)
	recLog := httptest.NewRecorder()
	h.handleAccessLog(recLog, reqLog)
	logRows := decodeEnvelopeData(t, recLog)
	if len(logRows) != 1 || logRows[0]["action"] != "reveal" || logRows[0]["actor_name"] != "admin" {
		t.Errorf("access log = %+v, want one reveal by admin", logRows)
	}

	// revoke-all revokes both live bundles; a second call finds none.
	revokeAll := func() int64 {
		rec := httptest.NewRecorder()
		h.handleRevokeAll(rec, f.as(f.admin, httptest.NewRequest(http.MethodPost, "/api/share-bundles/revoke-all", nil)))
		if rec.Code != http.StatusOK {
			t.Fatalf("revoke-all status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var out struct {
			Revoked int64 `json:"revoked"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out.Revoked
	}
	if n := revokeAll(); n != 2 {
		t.Errorf("revoke-all revoked %d, want 2", n)
	}
	if n := revokeAll(); n != 0 {
		t.Errorf("second revoke-all revoked %d, want 0", n)
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

// Editing a bundle's details is partial and keeps the token: omitted fields
// stay, the recipient can be set and cleared, and a new passphrase replaces
// the old one for redeem and for the admin reveal.
func TestBundleHandlers_UpdateDetails(t *testing.T) {
	f := newBundleHandlerFixture(t)
	h := &bundleHandlers{repo: f.repo}
	ctx := context.Background()
	tok, view, err := f.repo.CreateBundle(ctx, f.actor,
		[]vault.BundleItemInput{{Type: vault.BundleItemSecret, RefID: f.secretID}},
		vault.CreateBundleOpts{Title: "antigo", Description: "nota", TTL: time.Hour, Passphrase: "velha"})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	idStr := strconv.FormatInt(view.ID, 10)
	put := func(u *models.User, body string) (int, vault.BundleView) {
		t.Helper()
		req := f.as(u, httptest.NewRequest(http.MethodPut, "/api/share-bundles/"+idStr+"/details", strings.NewReader(body)))
		req.SetPathValue("id", idStr)
		rec := httptest.NewRecorder()
		h.handleUpdateDetails(rec, req)
		var v vault.BundleView
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
		return rec.Code, v
	}
	redeems := func(pass string) bool {
		_, err := f.repo.RedeemBundle(ctx, tok, pass, vault.RedeemMeta{})
		return err == nil
	}

	code, v := put(f.owner, `{"title":"novo","recipient_contact_id":`+strconv.FormatInt(f.contactID, 10)+`,"recipient_label":"PGE"}`)
	if code != http.StatusOK || v.Title != "novo" || v.Description != "nota" || !v.HasPassphrase ||
		v.RecipientContactID == nil || *v.RecipientContactID != f.contactID || v.RecipientLabel != "PGE" {
		t.Fatalf("partial edit = %d %+v", code, v)
	}

	if code, _ := put(f.owner, `{"passphrase":"nova"}`); code != http.StatusOK {
		t.Fatalf("passphrase change = %d", code)
	}
	if redeems("velha") || !redeems("nova") {
		t.Error("after the change, the old passphrase must fail and the new one open the link")
	}
	adminActor := vault.ActorContext{UserID: f.admin.ID, Role: f.admin.Role}
	if sec, err := f.repo.RevealBundle(ctx, adminActor, view.ID); err != nil || sec.Passphrase != "nova" || sec.Token != tok {
		t.Errorf("reveal after change = %+v %v (want the new passphrase, same token)", sec, err)
	}

	code, v = put(f.owner, `{"passphrase":"","recipient_contact_id":0}`)
	if code != http.StatusOK || v.HasPassphrase || v.RecipientContactID != nil || v.Title != "novo" {
		t.Fatalf("clear passphrase/contact = %d %+v", code, v)
	}
	if !redeems("") {
		t.Error("link without passphrase should open without one")
	}

	if code, _ := put(f.other, `{"title":"x"}`); code != http.StatusNotFound {
		t.Errorf("non-owner edit = %d, want 404", code)
	}
	if code, _ := put(f.owner, `{"recipient_contact_id":999999}`); code != http.StatusBadRequest {
		t.Errorf("unknown contact = %d, want 400", code)
	}
}
