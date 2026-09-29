package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// putAsAdmin calls a handler directly with the {id} path value and an admin
// actor in context (what the router + RequireAuth would provide).
func putAsAdmin(t *testing.T, env *secretAPIEnv, h http.HandlerFunc, id int64, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PUT", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", itoa(id))
	req = req.WithContext(auth.WithUser(req.Context(), env.alice))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// Saving an edit form must not wipe what the form doesn't carry: a service's
// scan state and links, a DNS record's legacy responsavel. And an explicit
// empty link list still unlinks.
func TestUpdate_KeepsOmittedFields(t *testing.T) {
	env := newSecretAPIEnv(t)
	q := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := env.d.SQL.QueryRow(sql, args...).Scan(&id); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return id
	}
	host := q(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h') RETURNING id`)
	svc := q(`INSERT INTO services (nickname, source, discovery_kind, discovery_key, container_status, container_name, container_image, last_seen_at, gitlab_url)
		VALUES ('pg', 'auto', 'container', 'pg-1', 'online', 'pg-1', 'postgres:16', now(), 'https://git/x') RETURNING id`)
	if _, err := env.d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svc, host); err != nil {
		t.Fatalf("link: %v", err)
	}

	// The edit form sends its own fields only.
	sh := &serviceHandlers{service: service.NewServiceService(env.d.SQL), db: env.d}
	rec := putAsAdmin(t, env, sh.handleUpdate, svc, `{"nickname":"Banco principal","service_kind":"database"}`)
	if rec.Code != 200 {
		t.Fatalf("PUT service: %d %s", rec.Code, rec.Body)
	}
	var nick, status, image, kind, gitlab string
	var seen *string
	if err := env.d.SQL.QueryRow(`SELECT nickname, container_status, container_image, service_kind, last_seen_at::text, gitlab_url FROM services WHERE id = ?`, svc).
		Scan(&nick, &status, &image, &kind, &seen, &gitlab); err != nil {
		t.Fatalf("read service: %v", err)
	}
	if gitlab != "https://git/x" {
		t.Fatalf("gitlab_url = %q — a field the form omits must be kept", gitlab)
	}
	if nick != "Banco principal" || kind != "database" || status != "online" || image != "postgres:16" || seen == nil {
		t.Fatalf("after edit: nick=%q kind=%q status=%q image=%q seen=%v — scan state must survive", nick, kind, status, image, seen)
	}
	var links int
	env.d.SQL.QueryRow(`SELECT COUNT(*) FROM service_host_links WHERE service_id = ?`, svc).Scan(&links)
	if links != 1 {
		t.Fatalf("host link dropped by an edit that didn't send host_ids (links=%d)", links)
	}

	dns := q(`INSERT INTO dns_records (domain, responsavel) VALUES ('a.gov', 'Equipe legado') RETURNING id`)
	if _, err := env.d.SQL.Exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dns, host); err != nil {
		t.Fatalf("dns link: %v", err)
	}
	dh := &dnsHandlers{dns: service.NewDNSService(env.d.SQL)}
	rec = putAsAdmin(t, env, dh.handleUpdate, dns, `{"domain":"a.gov","has_https":true,"host_ids":[]}`)
	if rec.Code != 200 {
		t.Fatalf("PUT dns: %d %s", rec.Code, rec.Body)
	}
	var resp string
	env.d.SQL.QueryRow(`SELECT responsavel FROM dns_records WHERE id = ?`, dns).Scan(&resp)
	if resp != "Equipe legado" {
		t.Fatalf("legacy responsavel = %q, want kept", resp)
	}
	env.d.SQL.QueryRow(`SELECT COUNT(*) FROM dns_host_links WHERE dns_id = ?`, dns).Scan(&links)
	if links != 0 {
		t.Fatalf("host_ids:[] should unlink the last host (links=%d)", links)
	}
}
