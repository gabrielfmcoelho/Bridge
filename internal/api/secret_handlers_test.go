package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// secretAPIEnv bundles the test server and a fixture of users + seeded
// secrets for every ACL matrix combination required by Plans.md Task 1.6.
//
// Fixture summary:
//   - users: alice(admin), bob(editor), carol(viewer), dave(viewer)
//   - secrets:
//     shared1   visibility=shared,   type=cred,     scope=service, parent=1, owner=bob
//     personalC visibility=personal, type=password, scope=avulso,  owner=carol
//     personalD visibility=personal, type=password, scope=avulso,  owner=dave
//
// This shape is enough to assert every cell of the two ACL tables in spec §5.
type secretAPIEnv struct {
	t      *testing.T
	d      *database.DB
	server *httptest.Server

	alice, bob, carol, dave *models.User

	sharedID    int64
	personalCID int64 // owned by carol
	personalDID int64 // owned by dave
}

func newSecretAPIEnv(t *testing.T) *secretAPIEnv {
	t.Helper()
	dir := t.TempDir()
	d, err := database.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	mkUser := func(name, role string) *models.User {
		res, err := d.SQL.Exec(
			`INSERT INTO users (username, password_hash, role) VALUES (?,?,?)`,
			name, "x", role,
		)
		if err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		id, _ := res.LastInsertId()
		return &models.User{ID: id, Username: name, Role: role}
	}
	alice := mkUser("alice", "admin")
	bob := mkUser("bob", "editor")
	carol := mkUser("carol", "viewer")
	dave := mkUser("dave", "viewer")

	// Parent service for the shared secret.
	svcRes, err := d.SQL.Exec(`INSERT INTO services (nickname) VALUES (?)`, "billing-api")
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	serviceID, _ := svcRes.LastInsertId()

	repo := vault.NewSecretRepo(d)
	ctx := context.Background()

	// shared secret: bob (editor) creates a shared cred on the service.
	sharedID, err := repo.Create(ctx,
		vault.ActorContext{UserID: bob.ID, Role: bob.Role},
		&models.Secret{
			Type:        models.SecretTypeCred,
			Scope:       models.SecretScopeService,
			Visibility:  models.SecretVisibilityShared,
			ParentID:    int64Ptr(serviceID),
			OwnerUserID: bob.ID,
			Name:        "primary",
			KeyVersion:  1,
			CreatedBy:   bob.ID,
		},
		"shared-cred-plaintext",
	)
	if err != nil {
		t.Fatalf("seed shared: %v", err)
	}

	// personal for carol (avulso).
	personalCID, err := repo.Create(ctx,
		vault.ActorContext{UserID: carol.ID, Role: carol.Role},
		&models.Secret{
			Type:        models.SecretTypePassword,
			Scope:       models.SecretScopeAvulso,
			Visibility:  models.SecretVisibilityPersonal,
			OwnerUserID: carol.ID,
			Name:        "carol-secret",
			Description: stringPtr("carol's note"),
			KeyVersion:  1,
			CreatedBy:   carol.ID,
		},
		"carol-plain",
	)
	if err != nil {
		t.Fatalf("seed personalC: %v", err)
	}

	// personal for dave (avulso).
	personalDID, err := repo.Create(ctx,
		vault.ActorContext{UserID: dave.ID, Role: dave.Role},
		&models.Secret{
			Type:        models.SecretTypePassword,
			Scope:       models.SecretScopeAvulso,
			Visibility:  models.SecretVisibilityPersonal,
			OwnerUserID: dave.ID,
			Name:        "dave-secret",
			Description: stringPtr("dave's note"),
			KeyVersion:  1,
			CreatedBy:   dave.ID,
		},
		"dave-plain",
	)
	if err != nil {
		t.Fatalf("seed personalD: %v", err)
	}

	// Build a mux with the secret routes wired AND a test-only middleware
	// that pulls the actor from the X-Test-User-ID header (one of the
	// seeded user IDs) and stamps the context just like RequireAuth would.
	mux := http.NewServeMux()
	sh := &secretHandlers{db: d, repo: repo}
	// Identity wrap: test middleware (below) sits outside the mux and injects
	// the actor per-request. Production wires the same register() call with
	// authenticated(db, ...) as the wrapper — single source of truth for routes.
	sh.register(mux, func(h http.Handler) http.Handler { return h })
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get("X-Test-User-ID")
		if idStr == "" {
			http.Error(w, `{"error":"missing test user header"}`, http.StatusUnauthorized)
			return
		}
		uid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, `{"error":"bad test user header"}`, http.StatusUnauthorized)
			return
		}
		u, err := store.NewUserRepo(d.SQL).GetByID(context.Background(), uid)
		if err != nil || u == nil {
			http.Error(w, `{"error":"test user not found"}`, http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	})
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)

	return &secretAPIEnv{
		t: t, d: d, server: srv,
		alice: alice, bob: bob, carol: carol, dave: dave,
		sharedID: sharedID, personalCID: personalCID, personalDID: personalDID,
	}
}

func (e *secretAPIEnv) do(actor *models.User, method, path, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		e.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(actor.ID, 10))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	rec.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	bodyBytes, _ := io.ReadAll(resp.Body)
	rec.Body = bytes.NewBuffer(bodyBytes)
	return rec
}

func int64Ptr(v int64) *int64    { return &v }
func stringPtr(v string) *string { return &v }

// decodeMap unmarshals the recorder body into a generic map for assertion.
func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}
	return out
}

// decodeSlice unwraps the R4 list envelope {data:[...], meta:{...}} and returns
// the data rows. (List endpoints no longer return a bare array.)
func decodeSlice(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode slice: %v (body=%q)", err, rec.Body.String())
	}
	return env.Data
}

// ---------------------------------------------------------------------------
// GET /api/secrets — list
// ---------------------------------------------------------------------------

func TestSecretHandlers_List(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("viewer sees shared + own personal but not others'", func(t *testing.T) {
		rec := env.do(env.carol, "GET", "/api/secrets", "")
		if rec.Code != 200 {
			t.Fatalf("status: %d body=%s", rec.Code, rec.Body)
		}
		items := decodeSlice(t, rec)
		names := map[string]bool{}
		for _, it := range items {
			names[it["name"].(string)] = true
		}
		if !names["primary"] {
			t.Error("carol should see shared 'primary'")
		}
		if !names["carol-secret"] {
			t.Error("carol should see own 'carol-secret'")
		}
		if names["dave-secret"] {
			t.Error("carol must NOT see dave's personal")
		}
	})

	t.Run("admin sees others' personal metadata-only (description redacted)", func(t *testing.T) {
		rec := env.do(env.alice, "GET", "/api/secrets", "")
		items := decodeSlice(t, rec)
		var daveRow map[string]any
		for _, it := range items {
			if it["name"] == "dave-secret" {
				daveRow = it
				break
			}
		}
		if daveRow == nil {
			t.Fatal("admin should see dave-secret in list")
		}
		if d, ok := daveRow["description"]; ok && d != nil {
			t.Errorf("admin viewing others' personal: description must be redacted, got %v", d)
		}
	})
}

// ---------------------------------------------------------------------------
// POST /api/secrets — create
// ---------------------------------------------------------------------------

func TestSecretHandlers_Create(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("viewer cannot create shared", func(t *testing.T) {
		body := `{"type":"password","scope":"avulso","visibility":"shared","name":"x","payload":"v"}`
		rec := env.do(env.carol, "POST", "/api/secrets", body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})

	t.Run("editor can create shared", func(t *testing.T) {
		body := `{"type":"password","scope":"avulso","visibility":"shared","name":"new-shared","payload":"v"}`
		rec := env.do(env.bob, "POST", "/api/secrets", body)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("viewer can create personal for self", func(t *testing.T) {
		body := `{"type":"password","scope":"avulso","visibility":"personal","name":"my-personal","payload":"v"}`
		rec := env.do(env.carol, "POST", "/api/secrets", body)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("creating personal owned-by-someone-else rejected", func(t *testing.T) {
		body := fmt.Sprintf(
			`{"type":"password","scope":"avulso","visibility":"personal","name":"impersonate","payload":"v","owner_user_id":%d}`,
			env.dave.ID,
		)
		rec := env.do(env.carol, "POST", "/api/secrets", body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/secrets/{id} — metadata
// ---------------------------------------------------------------------------

func TestSecretHandlers_GetMetadata(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("404 for non-existent", func(t *testing.T) {
		rec := env.do(env.alice, "GET", "/api/secrets/999999", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("admin viewing others' personal: 200 with redacted description", func(t *testing.T) {
		rec := env.do(env.alice, "GET", fmt.Sprintf("/api/secrets/%d", env.personalDID), "")
		if rec.Code != 200 {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		body := decodeMap(t, rec)
		if _, hasPayload := body["payload"]; hasPayload {
			t.Error("response must not include payload on metadata GET")
		}
		if d, ok := body["description"]; ok && d != nil {
			t.Errorf("admin must not see others' personal description, got %v", d)
		}
	})

	t.Run("viewer viewing others' personal: 404", func(t *testing.T) {
		rec := env.do(env.carol, "GET", fmt.Sprintf("/api/secrets/%d", env.personalDID), "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/secrets/{id}/reveal
// ---------------------------------------------------------------------------

func TestSecretHandlers_Reveal(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("viewer can reveal shared", func(t *testing.T) {
		rec := env.do(env.carol, "GET", fmt.Sprintf("/api/secrets/%d/reveal", env.sharedID), "")
		if rec.Code != 200 {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		body := decodeMap(t, rec)
		if body["payload"] != "shared-cred-plaintext" {
			t.Errorf("payload mismatch: %v", body["payload"])
		}
	})

	t.Run("admin reveal of others' personal: 403 (D1, no break-glass)", func(t *testing.T) {
		rec := env.do(env.alice, "GET", fmt.Sprintf("/api/secrets/%d/reveal", env.personalDID), "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})

	t.Run("owner can reveal own personal", func(t *testing.T) {
		rec := env.do(env.carol, "GET", fmt.Sprintf("/api/secrets/%d/reveal", env.personalCID), "")
		if rec.Code != 200 {
			t.Errorf("got %d, want 200; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("non-owner viewer can't see others' personal exists: 404", func(t *testing.T) {
		rec := env.do(env.carol, "GET", fmt.Sprintf("/api/secrets/%d/reveal", env.personalDID), "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// PUT /api/secrets/{id} — update
// ---------------------------------------------------------------------------

func TestSecretHandlers_Update(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("viewer cannot update shared: 403", func(t *testing.T) {
		rec := env.do(env.carol, "PUT",
			fmt.Sprintf("/api/secrets/%d", env.sharedID),
			`{"name":"renamed"}`,
		)
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})

	t.Run("editor can update shared", func(t *testing.T) {
		rec := env.do(env.bob, "PUT",
			fmt.Sprintf("/api/secrets/%d", env.sharedID),
			`{"name":"renamed-by-bob"}`,
		)
		if rec.Code != 200 {
			t.Errorf("got %d, want 200; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("visibility change rejected with 422", func(t *testing.T) {
		rec := env.do(env.carol, "PUT",
			fmt.Sprintf("/api/secrets/%d", env.personalCID),
			`{"visibility":"shared"}`,
		)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("got %d, want 422", rec.Code)
		}
	})

	t.Run("admin updating others' personal: 403", func(t *testing.T) {
		rec := env.do(env.alice, "PUT",
			fmt.Sprintf("/api/secrets/%d", env.personalDID),
			`{"name":"hijack"}`,
		)
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// DELETE + restore
// ---------------------------------------------------------------------------

func TestSecretHandlers_DeleteRestore(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("viewer cannot delete shared: 403", func(t *testing.T) {
		rec := env.do(env.carol, "DELETE", fmt.Sprintf("/api/secrets/%d", env.sharedID), "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})

	t.Run("admin deletes shared then restores", func(t *testing.T) {
		rec := env.do(env.alice, "DELETE", fmt.Sprintf("/api/secrets/%d", env.sharedID), "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("delete got %d, want 204", rec.Code)
		}
		rec = env.do(env.alice, "POST", fmt.Sprintf("/api/secrets/%d/restore", env.sharedID), "")
		if rec.Code != 200 && rec.Code != http.StatusNoContent {
			t.Errorf("restore got %d", rec.Code)
		}
	})

	t.Run("owner deletes own personal", func(t *testing.T) {
		rec := env.do(env.carol, "DELETE", fmt.Sprintf("/api/secrets/%d", env.personalCID), "")
		if rec.Code != http.StatusNoContent {
			t.Errorf("got %d, want 204; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("admin cannot delete others' personal: 403", func(t *testing.T) {
		rec := env.do(env.alice, "DELETE", fmt.Sprintf("/api/secrets/%d", env.personalDID), "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 403", rec.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/secrets/trash
// ---------------------------------------------------------------------------

func TestSecretHandlers_Trash(t *testing.T) {
	env := newSecretAPIEnv(t)

	// dave deletes his own personal so it lands in trash.
	rec := env.do(env.dave, "DELETE", fmt.Sprintf("/api/secrets/%d", env.personalDID), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("setup delete: %d", rec.Code)
	}

	t.Run("dave sees his deleted personal in trash", func(t *testing.T) {
		rec := env.do(env.dave, "GET", "/api/secrets/trash", "")
		items := decodeSlice(t, rec)
		if len(items) != 1 || items[0]["name"] != "dave-secret" {
			t.Errorf("trash: got %v", items)
		}
	})

	t.Run("carol does not see dave's deleted personal in trash", func(t *testing.T) {
		rec := env.do(env.carol, "GET", "/api/secrets/trash", "")
		items := decodeSlice(t, rec)
		for _, it := range items {
			if it["name"] == "dave-secret" {
				t.Error("carol should not see dave's trash")
			}
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/secrets/mine
// ---------------------------------------------------------------------------

func TestSecretHandlers_Mine(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("returns only visibility=personal AND owner=me", func(t *testing.T) {
		rec := env.do(env.carol, "GET", "/api/secrets/mine", "")
		items := decodeSlice(t, rec)
		if len(items) != 1 {
			t.Fatalf("expected 1 row for carol, got %d: %v", len(items), items)
		}
		if items[0]["name"] != "carol-secret" {
			t.Errorf("wrong row: %v", items[0])
		}
		if items[0]["visibility"] != "personal" {
			t.Errorf("expected personal, got %v", items[0]["visibility"])
		}
		if int64(items[0]["owner_user_id"].(float64)) != env.carol.ID {
			t.Errorf("owner mismatch: %v", items[0]["owner_user_id"])
		}
	})

	t.Run("excludes shared even though caller can see it elsewhere", func(t *testing.T) {
		rec := env.do(env.bob, "GET", "/api/secrets/mine", "")
		items := decodeSlice(t, rec)
		for _, it := range items {
			if it["visibility"] != "personal" {
				t.Errorf("/mine returned non-personal: %v", it)
			}
			if int64(it["owner_user_id"].(float64)) != env.bob.ID {
				t.Errorf("/mine returned secret owned by %v, not bob", it["owner_user_id"])
			}
		}
	})
}

// ---------------------------------------------------------------------------
// GET /api/secrets/{id}/history
// ---------------------------------------------------------------------------

func TestSecretHandlers_History(t *testing.T) {
	env := newSecretAPIEnv(t)

	t.Run("owner sees history (>=1 create row)", func(t *testing.T) {
		rec := env.do(env.carol, "GET", fmt.Sprintf("/api/secrets/%d/history", env.personalCID), "")
		if rec.Code != 200 {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		items := decodeSlice(t, rec)
		if len(items) < 1 {
			t.Error("expected at least one audit row")
		}
	})

	t.Run("admin sees no audit for others' personal: 404", func(t *testing.T) {
		// D6: admin sees others' personal metadata only — audit log is part
		// of the "values + audit + share-link" tranche they cannot see.
		rec := env.do(env.alice, "GET", fmt.Sprintf("/api/secrets/%d/history", env.personalDID), "")
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
			t.Errorf("got %d, want 404/403", rec.Code)
		}
	})
}
