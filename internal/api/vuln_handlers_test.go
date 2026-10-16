package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// newVulnEnv serves the vulnerability routes as user `name`; entidade != 0
// limits the scope to that entidade, 0 is unscoped (admin-like).
func newVulnEnv(t *testing.T, d *database.DB, name string, entidade int64) *catalogEnv {
	t.Helper()
	u := &models.User{Username: name, DisplayName: name, Role: "editor", Email: name + "@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	mux := newEcho()
	(&vulnHandlers{db: d}).registerRoutes(routeRegistrar{e: mux})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithUser(r.Context(), u)
		if entidade != 0 {
			ctx = store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{entidade}, PrimaryEntidadeID: entidade})
		}
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return &catalogEnv{t: t, d: d, server: srv, user: u}
}

func (e *catalogEnv) uploadETIPI(query string, data []byte) *http.Response {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "report.txt")
	_, _ = io.Copy(fw, bytes.NewReader(data))
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, e.server.URL+"/api/vulnerabilities/import/etipi"+query, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("upload: %v", err)
	}
	return resp
}

func TestVuln_ImportScopeAndUpdate(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	sample, err := os.ReadFile("../vulnimport/testdata/etipi_sample.txt")
	if err != nil {
		t.Fatal(err)
	}
	sga := entidadeID(t, d, "sga")
	etipi := entidadeID(t, d, "etipi")
	grants := store.NewAssetEntidadeRepo(d.SQL)
	seedDNS := func(domain string, ent int64) int64 {
		t.Helper()
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES (?) RETURNING id`, domain).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := grants.Replace(context.Background(), d.SQL, store.AssetDNS, id, models.AssetGrants{CreatorEntidadeID: &ent}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	dadosDNS := seedDNS("dados.example.test", sga)
	otherDNS := seedDNS("wiki.example.test", etipi)

	admin := newVulnEnv(t, d, "admin", 0)
	scoped := newVulnEnv(t, d, "sga-user", sga)

	// Dry run counts but writes nothing.
	resp := admin.uploadETIPI("?dry_run=true", sample)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dry run = %d %s", resp.StatusCode, readBody(resp))
	}
	var res models.VulnImportResult
	_ = json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	if res.New != 6 || !res.DryRun {
		t.Fatalf("dry run result = %+v", res)
	}
	if got := decodeArr(t, admin.do("GET", "/api/vulnerabilities", "")); len(got) != 0 {
		t.Fatalf("dry run persisted %d", len(got))
	}

	// A file whose counts don't add up is a 400.
	if resp := admin.uploadETIPI("", bytes.Replace(sample, []byte("aberto: 7"), []byte("aberto: 9"), 1)); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad checksum = %d", resp.StatusCode)
	}

	if resp := admin.uploadETIPI("", sample); resp.StatusCode != http.StatusOK {
		t.Fatalf("import = %d %s", resp.StatusCode, readBody(resp))
	}
	all := decodeArr(t, admin.do("GET", "/api/vulnerabilities", ""))
	if len(all) != 6 || all[0]["severity"] != "critical" {
		t.Fatalf("admin list = %d rows, first %v", len(all), all[0]["severity"])
	}

	// The SGA user sees only the finding on its DNS record.
	mine := decodeArr(t, scoped.do("GET", "/api/vulnerabilities", ""))
	if len(mine) != 1 || mine[0]["cve"] != "CVE-2026-0001" {
		t.Fatalf("scoped list = %+v", mine)
	}
	var wikiID int64
	for _, f := range all {
		if f["target"] == "wiki.example.test:443" {
			wikiID = int64(f["id"].(float64))
			break
		}
	}
	if resp := scoped.do("GET", "/api/vulnerabilities/"+itoa(wikiID), ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("invisible get = %d, want 404", resp.StatusCode)
	}
	if resp := scoped.do("PATCH", "/api/vulnerabilities/"+itoa(wikiID), `{"status":"accepted"}`); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("invisible patch = %d, want 404", resp.StatusCode)
	}

	// Linking its own finding to an asset it can't see is a 404, too.
	mineID := itoa(int64(mine[0]["id"].(float64)))
	if resp := scoped.do("POST", "/api/vulnerabilities/"+mineID+"/links", `{"entity_type":"dns","entity_id":`+itoa(otherDNS)+`}`); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("link to invisible asset = %d, want 404", resp.StatusCode)
	}
	if resp := admin.do("POST", "/api/vulnerabilities/"+mineID+"/links", `{"entity_type":"host","entity_id":999999}`); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("link to missing asset = %d, want 404", resp.StatusCode)
	}

	// Partial update: status + note; omitted fields keep their value.
	if resp := scoped.do("PATCH", "/api/vulnerabilities/"+mineID, `{"status":"bogus"}`); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad status = %d", resp.StatusCode)
	}
	got := decodeObj(t, scoped.do("PATCH", "/api/vulnerabilities/"+mineID, `{"status":"accepted","status_note":"WAF rule in place"}`))
	if got["status"] != "accepted" || got["status_note"] != "WAF rule in place" || got["cve"] != "CVE-2026-0001" {
		t.Fatalf("patch = %+v", got)
	}
	got = decodeObj(t, scoped.do("PATCH", "/api/vulnerabilities/"+mineID, `{"glpi_ticket_id":42}`))
	if got["status"] != "accepted" || got["glpi_ticket_id"] != float64(42) {
		t.Fatalf("partial patch lost fields: %+v", got)
	}

	// The DNS entity tab lists it.
	tab := decodeArr(t, scoped.do("GET", "/api/vulnerabilities?entity_type=dns&entity_id="+itoa(dadosDNS), ""))
	if len(tab) != 1 {
		t.Fatalf("dns tab = %d", len(tab))
	}
}

// The Wazuh sync reads every page from the indexer, matches agents to hosts
// by name, and resolves what the indexer no longer reports.
func TestVuln_WazuhSync(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES ('web-01', 'web-01', '10.0.0.9') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatal(err)
	}

	hits := []string{
		`{"_source":{"agent":{"id":"001","name":"web-01"},"package":{"name":"openssl","version":"3.0.2"},"vulnerability":{"id":"CVE-2024-0001","severity":"Critical","score":{"base":9.8}}}}`,
		`{"_source":{"agent":{"id":"002","name":"db-unknown"},"package":{"name":"curl","version":"7.0"},"vulnerability":{"id":"CVE-2024-0002","severity":"Medium"}}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/wazuh-states-vulnerabilities-*/_search":
			body := `{"_scroll_id":"s","hits":{"total":{"value":` + itoa(int64(len(hits))) + `},"hits":[` + strings.Join(hits, ",") + `]}}`
			_, _ = w.Write([]byte(body))
		case r.Method == http.MethodPost && r.URL.Path == "/_search/scroll":
			_, _ = w.Write([]byte(`{"_scroll_id":"s","hits":{"total":{"value":0},"hits":[]}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)

	admin := newVulnEnv(t, d, "admin", 0)
	if resp := admin.do("POST", "/api/wazuh/sync", ""); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("sync while disabled = %d, want 400", resp.StatusCode)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"wazuh_enabled": "true", "wazuh_indexer_url": srv.URL, "wazuh_indexer_user": "reader"} {
		if err := settings.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "wazuh_indexer_password", "pw"); err != nil {
		t.Fatal(err)
	}

	res := decodeObj(t, admin.do("POST", "/api/wazuh/sync", ""))
	if res["new"] != float64(2) || res["unmatched"] != float64(1) {
		t.Fatalf("first sync = %+v", res)
	}
	if u, _ := res["unmatched_targets"].([]any); len(u) != 1 || u[0] != "db-unknown" {
		t.Fatalf("unmatched targets = %v", res["unmatched_targets"])
	}
	tab := decodeArr(t, admin.do("GET", "/api/vulnerabilities?entity_type=host&entity_id="+itoa(hostID), ""))
	if len(tab) != 1 || tab[0]["severity"] != "critical" || tab[0]["source"] != "wazuh" {
		t.Fatalf("host tab = %+v", tab)
	}

	hits = hits[:1] // db-unknown's CVE is fixed
	res = decodeObj(t, admin.do("POST", "/api/wazuh/sync", ""))
	if res["resolved"] != float64(1) || res["updated"] != float64(1) {
		t.Fatalf("second sync = %+v", res)
	}
}
