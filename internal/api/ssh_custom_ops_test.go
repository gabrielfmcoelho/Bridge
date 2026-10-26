package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// renderScript must turn any editor-typed value into one literal shell word.
// The rendered script is run through a real sh to prove it.
func TestRenderScript(t *testing.T) {
	if _, err := renderScript("docker rm -f {{container}} {{x}}", map[string]string{"container": "a", "x": "  "}); err == nil || !strings.Contains(err.Error(), "x") {
		t.Fatalf("blank value: err = %v, want missing x", err)
	}
	if got, err := renderScript("uptime", nil); err != nil || got != "uptime" {
		t.Fatalf("no params = %q, %v", got, err)
	}
	evil := `x'; echo INJECTED #`
	got, err := renderScript("printf '%s|' {{v}} {{v}}", map[string]string{"v": evil, "unused": "1"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", got).Output()
	if err != nil {
		t.Fatalf("sh %q: %v", got, err)
	}
	if want := evil + "|" + evil + "|"; string(out) != want {
		t.Fatalf("sh output = %q, want %q (script %q)", out, want, got)
	}
}

// CRUD round trip and run validation. Roles are enforced by rr.role, which a
// registrar without db skips (TestSelfRegisteredRoutes_Wired covers wiring);
// a run checks its parameters before any SSH connection is attempted.
func TestSSHCustomOps_Handlers(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	users := map[string]*models.User{}
	for _, role := range []string{"admin", "editor"} {
		u := &models.User{Username: role, DisplayName: role, Role: role, Email: role + "@example.com"}
		if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		users[role] = u
	}
	if _, err := d.SQL.Exec(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES ('vm1', 'vm1', '10.0.0.1', 'root')`); err != nil {
		t.Fatal(err)
	}
	mux := newEcho()
	(&sshHandlers{db: d}).registerRoutes(routeRegistrar{e: mux})
	call := func(role, method, path, body string) (int, string) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), users[role])))
		b, _ := io.ReadAll(w.Result().Body)
		return w.Code, string(b)
	}

	const base = "/api/ssh/custom-operations"
	create := `{"name":"Remover Portainer agent","script":"docker rm -f {{container}}","use_sudo":true}`
	if code, _ := call("admin", "POST", base, `{"name":"x","script":"  "}`); code != http.StatusBadRequest {
		t.Fatalf("blank script = %d, want 400", code)
	}
	code, body := call("admin", "POST", base, create)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %s", code, body)
	}
	var op models.SSHCustomOperation
	json.Unmarshal([]byte(body), &op)
	if !op.UseSudo || op.CreatedBy == nil || *op.CreatedBy != users["admin"].ID {
		t.Fatalf("created = %+v", op)
	}
	if code, _ := call("admin", "POST", base, create); code != http.StatusConflict {
		t.Fatalf("duplicate = %d, want 409", code)
	}

	id := strconv.FormatInt(op.ID, 10)
	if code, body := call("admin", "PUT", base+"/"+id, `{"description":"Remove o agente"}`); code != http.StatusOK ||
		!strings.Contains(body, `"name":"Remover Portainer agent"`) || !strings.Contains(body, `"use_sudo":true`) {
		t.Fatalf("partial update = %d %s", code, body)
	}
	if code, body := call("editor", "GET", base, ""); code != http.StatusOK || !strings.Contains(body, `"description":"Remove o agente"`) {
		t.Fatalf("editor list = %d %s", code, body)
	}

	run := base + "/" + id + "/run/"
	if code, _ := call("editor", "POST", run+"nope", `{"params":{"container":"a"}}`); code != http.StatusNotFound {
		t.Fatalf("unknown host = %d, want 404", code)
	}
	if code, _ := call("editor", "POST", base+"/999999/run/vm1", `{}`); code != http.StatusNotFound {
		t.Fatalf("unknown op = %d, want 404", code)
	}
	if code, body := call("editor", "POST", run+"vm1", `{"params":{}}`); code != http.StatusBadRequest || !strings.Contains(body, "container") {
		t.Fatalf("missing param = %d %s, want 400 naming container", code, body)
	}

	if code, _ := call("admin", "DELETE", base+"/"+id, ""); code != http.StatusOK {
		t.Fatalf("delete = %d", code)
	}
	if o, _ := store.NewSSHCustomOperationRepo(d.SQL).Get(ctx, op.ID); o != nil {
		t.Fatal("operation still there")
	}
}
