package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Proxmox server CRUD: the secret is stored encrypted and never returned;
// an update is partial and a masked or empty token_secret keeps the stored one.
func TestProxmoxServerHandlers_CRUD(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	h := &proxmoxHandlers{db: d}
	call := func(fn http.HandlerFunc, method, id, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/proxmox/servers", strings.NewReader(body))
		if id != "" {
			r.SetPathValue("id", id)
		}
		w := httptest.NewRecorder()
		fn(w, r)
		return w
	}
	secret := func(id int64) string {
		t.Helper()
		s, err := store.NewProxmoxServerRepo(d.SQL).Get(context.Background(), id)
		if err != nil || s == nil {
			t.Fatalf("get %d: %v", id, err)
		}
		plain, err := d.Encryptor.Decrypt(s.TokenCipher, s.TokenNonce)
		if err != nil {
			t.Fatal(err)
		}
		return plain
	}

	if w := call(h.handleCreateServer, "POST", "", `{"name":"pve-a","base_url":"https://a:8006"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("create without token = %d, want 400", w.Code)
	}
	w := call(h.handleCreateServer, "POST", "", `{"name":"pve-a","base_url":" https://a:8006/ ","token_id":"bridge@pve!sync","token_secret":"s1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "s1") {
		t.Fatalf("secret leaked: %s", w.Body)
	}
	var a models.ProxmoxServer
	json.Unmarshal(w.Body.Bytes(), &a)
	if a.BaseURL != "https://a:8006" || !a.HasToken || !a.Enabled || secret(a.ID) != "s1" {
		t.Fatalf("created = %+v", a)
	}
	if w := call(h.handleCreateServer, "POST", "", `{"name":"pve-a","base_url":"https://x","token_id":"x","token_secret":"x"}`); w.Code != http.StatusConflict {
		t.Fatalf("duplicate name = %d, want 409", w.Code)
	}

	id := strconv.FormatInt(a.ID, 10)
	// Masked secret + only enabled: everything else is kept.
	if w := call(h.handleUpdateServer, "PUT", id, `{"enabled":false,"token_secret":"••••••••"}`); w.Code != http.StatusOK {
		t.Fatalf("update = %d %s", w.Code, w.Body)
	}
	got, _ := store.NewProxmoxServerRepo(d.SQL).Get(context.Background(), a.ID)
	if got.Enabled || got.Name != "pve-a" || got.TokenID != "bridge@pve!sync" || secret(a.ID) != "s1" {
		t.Fatalf("after masked update = %+v secret %q", got, secret(a.ID))
	}
	if w := call(h.handleUpdateServer, "PUT", id, `{"token_secret":""}`); w.Code != http.StatusOK || secret(a.ID) != "s1" {
		t.Fatalf("empty secret = %d, secret %q", w.Code, secret(a.ID))
	}
	if w := call(h.handleUpdateServer, "PUT", id, `{"token_secret":"s2"}`); w.Code != http.StatusOK || secret(a.ID) != "s2" {
		t.Fatalf("new secret = %d, secret %q", w.Code, secret(a.ID))
	}
	if w := call(h.handleUpdateServer, "PUT", id, `{"name":""}`); w.Code != http.StatusBadRequest {
		t.Fatalf("blank name = %d, want 400", w.Code)
	}
	if w := call(h.handleUpdateServer, "PUT", "999999", `{}`); w.Code != http.StatusNotFound {
		t.Fatalf("missing = %d, want 404", w.Code)
	}

	w = call(h.handleListServers, "GET", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"pve-a"`) || strings.Contains(w.Body.String(), "token_cipher") {
		t.Fatalf("list = %d %s", w.Code, w.Body)
	}
	if enabled, _ := store.NewProxmoxServerRepo(d.SQL).List(context.Background(), true); len(enabled) != 0 {
		t.Fatalf("enabled-only list = %+v, want none", enabled)
	}

	if w := call(h.handleDeleteServer, "DELETE", id, ""); w.Code != http.StatusOK {
		t.Fatalf("delete = %d", w.Code)
	}
	if s, _ := store.NewProxmoxServerRepo(d.SQL).Get(context.Background(), a.ID); s != nil {
		t.Fatal("server still there")
	}
}
