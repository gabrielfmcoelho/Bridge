package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// fakeSEAD mimics the /admin/keys surface of dlhsead-api-servidores closely
// enough for the client and handlers: master key check, create (409-ish 400
// on a taken label), revoke, rotate with a renamed leftover, usage.
type fakeSEAD struct {
	mu   sync.Mutex
	keys map[string]map[string]any // label → KeySummary-ish record
	seq  int
}

func (f *fakeSEAD) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("X-Admin-Key") != "master" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"detail":"X-Admin-Key inválida ou ausente"}`)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/folha/admin/keys")
		label := strings.Trim(strings.TrimSuffix(strings.TrimSuffix(path, "/rotate"), "/usage"), "/")
		now := time.Now().UTC().Format(time.RFC3339Nano)
		newKey := func(label, owner string) map[string]any {
			f.seq++
			return map[string]any{"label": label, "owner": owner, "created_at": now, "scopes": []string{"*"},
				"rate_limit_per_minute": 1000, "notes": "", "lifetime_uses": 0, "status": "ACTIVE",
				"plaintext": fmt.Sprintf("tok-%s-%d", label, f.seq)}
		}
		summary := func(k map[string]any) map[string]any {
			out := map[string]any{}
			for key, v := range k {
				if key != "plaintext" {
					out[key] = v
				}
			}
			return out
		}
		switch {
		case r.Method == http.MethodGet && label == "":
			list := []map[string]any{}
			for _, k := range f.keys {
				list = append(list, summary(k))
			}
			json.NewEncoder(w).Encode(list)
		case r.Method == http.MethodPost && label == "":
			var req struct{ Label, Owner string }
			json.NewDecoder(r.Body).Decode(&req)
			if _, taken := f.keys[req.Label]; taken {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"detail":"label já existe"}`)
				return
			}
			k := newKey(req.Label, req.Owner)
			f.keys[req.Label] = k
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(k)
		case f.keys[label] == nil:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"detail":"chave não encontrada"}`)
		case r.Method == http.MethodDelete:
			f.keys[label]["revoked_at"] = now
			f.keys[label]["status"] = "REVOKED"
			json.NewEncoder(w).Encode(summary(f.keys[label]))
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/rotate"):
			old := f.keys[label]
			old["label"] = label + "__rotated__abcd1234"
			old["grace_until"] = time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
			old["status"] = "GRACE"
			f.keys[old["label"].(string)] = old
			k := newKey(label, old["owner"].(string))
			f.keys[label] = k
			json.NewEncoder(w).Encode(k)
		case strings.HasSuffix(path, "/usage"):
			json.NewEncoder(w).Encode(map[string]any{"label": label, "lifetime": 3, "daily": map[string]int{"20260930": 3}})
		default:
			t.Errorf("fake sead: unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
}

func TestAPIKeys_SEADLifecycle(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	router := NewRouter(d, "/tmp/sshcm-test-config")

	fake := &fakeSEAD{keys: map[string]map[string]any{}}
	sead := httptest.NewServer(fake.handler(t))
	t.Cleanup(sead.Close)

	session := func(name, role string) string {
		u := &models.User{Username: name, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatalf("user: %v", err)
		}
		tok, _, err := auth.CreateSession(d.SQL, u.ID)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}
	admin, editor := session("ana", "admin"), session("bia", "editor")

	do := func(cookie, method, path, body string) (int, map[string]any, string) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var obj map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &obj)
		return rec.Code, obj, rec.Body.String()
	}

	// A global API, visible to the editor too.
	var adminID int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ana'`).Scan(&adminID)
	a := &models.APICatalog{Name: "Folha", SourceType: models.APICatalogSourceUpload, SpecJSON: `{"openapi":"3.0.0"}`,
		OwnerUserID: adminID, CreatedBy: adminID}
	if err := store.NewAPICatalogRepo(d.SQL).Create(ctx, a, nil); err != nil {
		t.Fatalf("api: %v", err)
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetAPICatalog, a.ID, models.AssetGrants{IsGlobal: true}); err != nil {
		t.Fatalf("grants: %v", err)
	}
	base := "/api/api-catalog/" + itoa(a.ID)

	// Connection: admin-only, write-only.
	cfg := `{"key_management":"sead","admin_base_url":"` + sead.URL + `/folha","admin_key":"master"}`
	if code, _, _ := do(editor, "PUT", base+"/key-management", cfg); code != http.StatusForbidden {
		t.Fatalf("editor sets key management = %d, want 403", code)
	}
	code, obj, raw := do(admin, "PUT", base+"/key-management", cfg)
	if code != http.StatusOK || obj["has_admin_key"] != true || strings.Contains(raw, "master") {
		t.Fatalf("set key management = %d %s", code, raw)
	}
	if code, obj, _ := do(admin, "POST", base+"/key-management/test", ""); code != http.StatusOK || obj["success"] != true {
		t.Fatalf("test connection = %d %v", code, obj)
	}
	if _, obj, _ := do(admin, "POST", base+"/key-management/test", `{"admin_key":"wrong"}`); obj["success"] != false {
		t.Errorf("test with a wrong master key = %v, want success false", obj)
	}

	// Issue: editors without apis.keys.manage can list but not issue.
	if code, _, _ := do(editor, "POST", base+"/keys", `{"label":"painel"}`); code != http.StatusForbidden {
		t.Fatalf("editor issues key = %d, want 403", code)
	}
	code, obj, raw = do(admin, "POST", base+"/keys", `{"label":"painel","owner":"rh@sead"}`)
	if code != http.StatusCreated {
		t.Fatalf("issue = %d %s", code, raw)
	}
	plain := obj["plaintext"].(string)
	key := obj["key"].(map[string]any)
	keyID, secretID := itoa(int64(key["id"].(float64))), itoa(int64(key["secret_id"].(float64)))
	if fake.keys["painel"] == nil || key["status"] != "active" || key["external_label"] != "painel" {
		t.Fatalf("issued key = %v (fake has %v)", key, fake.keys["painel"] != nil)
	}
	if code, _, _ := do(editor, "GET", base+"/keys", ""); code != http.StatusOK {
		t.Errorf("editor lists keys = %d, want 200", code)
	}

	// The vault copy: revealable with apis.keys.manage (admin), not by an editor.
	reveal := func(cookie string) (int, string) {
		code, obj, _ := do(cookie, "GET", "/api/secrets/"+secretID+"/reveal", "")
		p, _ := obj["payload"].(string)
		return code, p
	}
	if code, p := reveal(admin); code != http.StatusOK || !strings.Contains(p, plain) {
		t.Fatalf("admin reveal = %d %q, want the plaintext", code, p)
	}
	if code, _ := reveal(editor); code != http.StatusForbidden {
		t.Errorf("editor reveal = %d, want 403", code)
	}

	// Rotate: new plaintext on the same row and in the vault; sync picks up
	// the rotated-away key (in grace).
	code, obj, raw = do(admin, "POST", base+"/keys/"+keyID+"/rotate", `{"grace_days":7}`)
	if code != http.StatusOK || obj["plaintext"] == plain {
		t.Fatalf("rotate = %d %s", code, raw)
	}
	rotated := obj["plaintext"].(string)
	if _, p := reveal(admin); !strings.Contains(p, rotated) {
		t.Errorf("vault still holds the old key after rotate: %q", p)
	}
	_, obj, _ = do(admin, "GET", base+"/keys", "")
	if n := len(obj["data"].([]any)); n != 2 {
		t.Errorf("keys after rotate = %d rows, want 2 (new + rotated in grace)", n)
	}

	// A key created on the service by someone else shows up on sync.
	fake.mu.Lock()
	fake.keys["cli-key"] = map[string]any{"label": "cli-key", "owner": "ops", "created_at": time.Now().UTC().Format(time.RFC3339),
		"scopes": []string{"*"}, "status": "ACTIVE"}
	fake.mu.Unlock()
	if code, obj, _ := do(admin, "POST", base+"/keys/sync", ""); code != http.StatusOK || obj["created"].(float64) != 1 {
		t.Fatalf("sync = %d %v, want 1 created", code, obj)
	}

	// Usage comes straight from the service.
	if code, obj, _ := do(editor, "GET", base+"/keys/"+keyID+"/usage", ""); code != http.StatusOK || obj["lifetime"].(float64) != 3 {
		t.Errorf("usage = %d %v", code, obj)
	}

	// Revoke: remote first, then hidden from the default list.
	if code, obj, _ := do(admin, "POST", base+"/keys/"+keyID+"/revoke", ""); code != http.StatusOK || obj["status"] != "revoked" {
		t.Fatalf("revoke = %d %v", code, obj)
	}
	if fake.keys["painel"]["revoked_at"] == nil {
		t.Error("revoke did not reach the service")
	}

	// If the vault can't take the key, the remote key is revoked again: a
	// shared secret named "clash" already sits on this API.
	if _, err := d.SQL.Exec(`INSERT INTO secrets (type, scope, visibility, parent_id, owner_user_id, name, payload_ciphertext, payload_nonce, key_version, created_by)
		VALUES ('api_key','api_catalog','shared',?,?,'clash','\x00','\x00',1,?)`, a.ID, adminID, adminID); err != nil {
		t.Fatalf("seed clash: %v", err)
	}
	if code, _, _ := do(admin, "POST", base+"/keys", `{"label":"clash"}`); code < 400 {
		t.Fatalf("issue into a vault clash = %d, want an error", code)
	}
	if fake.keys["clash"] == nil || fake.keys["clash"]["revoked_at"] == nil {
		t.Error("remote key left live after the vault write failed")
	}

	// Manual mode: register an existing key.
	do(admin, "PUT", base+"/key-management", `{"key_management":"manual"}`)
	if code, _, _ := do(admin, "POST", base+"/keys", `{"label":"legacy"}`); code != http.StatusBadRequest {
		t.Errorf("manual key without value = %d, want 400", code)
	}
	if code, obj, raw := do(admin, "POST", base+"/keys", `{"label":"legacy","value":"abc123"}`); code != http.StatusCreated || obj["key"].(map[string]any)["source"] != "manual" {
		t.Fatalf("manual key = %d %s", code, raw)
	}

	// Deleting the API trashes its key secrets with it.
	if code, _, _ := do(admin, "DELETE", base, ""); code != http.StatusNoContent {
		t.Fatalf("delete api = %d", code)
	}
	if code, _ := reveal(admin); code != http.StatusNotFound {
		t.Errorf("key secret after api delete = %d, want 404", code)
	}
}
