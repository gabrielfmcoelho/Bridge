package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/nvd"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestVuln_CVEAnalysis: the button fetches NVD, writes pt + en together, a
// failure saves nothing, and the catalog follows finding visibility.
func TestVuln_CVEAnalysis(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}

	nvdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cveId") != "CVE-2024-0001" {
			_, _ = w.Write([]byte(`{"totalResults":0,"vulnerabilities":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"totalResults":1,"vulnerabilities":[{"cve":{"id":"CVE-2024-0001","published":"2024-01-02T03:04:05.000",
			"descriptions":[{"lang":"en","value":"OpenSSL overflow"}],
			"metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":9.8,"vectorString":"CVSS:3.1/AV:N"}}]},
			"weaknesses":[{"description":[{"value":"CWE-787"}]}],"references":[{"url":"https://openssl.org/news"}]}}]}`))
	}))
	t.Cleanup(nvdSrv.Close)

	// The fake LLM answers with the language line it was given, so each
	// column proves which prompt filled it.
	var llmFail atomic.Bool
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if llmFail.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		var req llm.CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		first, _, _ := strings.Cut(req.Messages[len(req.Messages)-1].Content, "\n")
		out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "### " + first}}}})
		_, _ = w.Write(out)
	}))
	t.Cleanup(llmSrv.Close)

	repo := store.NewVulnRepo(d.SQL)
	if _, err := repo.ApplyImport(ctx, "etipi", []models.VulnFinding{
		{Fingerprint: "a", Title: "openssl", Severity: "critical", Target: "x", CVE: "CVE-2024-0001"},
		{Fingerprint: "b", Title: "sqli", Severity: "high", Target: "x"},
		{Fingerprint: "c", Title: "ghost", Severity: "high", Target: "x", CVE: "CVE-2099-9999"},
	}, nil, false); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	all, _, _ := repo.List(ctx, models.VulnFilter{PerPage: 50})
	for _, f := range all {
		ids[f.Fingerprint] = itoa(f.ID)
	}

	h := &vulnHandlers{db: d, nvd: &nvd.Client{BaseURL: nvdSrv.URL, HTTP: nvdSrv.Client()}}
	admin := newVulnEnvWith(t, h, "admin", 0)
	post := func(fp string, want int) {
		t.Helper()
		resp := admin.do("POST", "/api/vulnerabilities/"+ids[fp]+"/cve-analysis", "")
		if resp.StatusCode != want {
			t.Fatalf("analyse %s = %d %s, want %d", fp, resp.StatusCode, readBody(resp), want)
		}
		resp.Body.Close()
	}

	post("a", http.StatusServiceUnavailable) // AI off
	settings := store.NewAppSettingsRepo(d.SQL)
	for k, v := range map[string]string{"llm_enabled": "true", "llm_base_url": llmSrv.URL, "llm_model_text": "qwen"} {
		if err := settings.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.NewAppSecretRepo(d.SQL).Store(ctx, d.Encryptor, "llm_api_key", "k"); err != nil {
		t.Fatal(err)
	}

	post("b", http.StatusBadRequest) // no CVE
	post("c", http.StatusNotFound)   // not in NVD
	llmFail.Store(true)
	post("a", http.StatusBadGateway)
	if got := readBody(admin.do("GET", "/api/vulnerabilities/cves/CVE-2024-0001", "")); strings.TrimSpace(got) != "null" {
		t.Fatalf("failed analysis was saved: %s", got)
	}

	llmFail.Store(false)
	post("a", http.StatusOK)
	e := decodeObj(t, admin.do("GET", "/api/vulnerabilities/cves/cve-2024-0001", ""))
	if e["analysis_pt"] != "### Responda em português do Brasil." || e["analysis_en"] != "### Respond in English." {
		t.Fatalf("analyses = %q / %q", e["analysis_pt"], e["analysis_en"])
	}
	if e["model"] != "qwen" || e["cvss"] != 9.8 || e["cwe"] != "CWE-787" || e["open_findings"] != float64(1) {
		t.Fatalf("entry = %+v", e)
	}
	if refs, _ := e["references"].([]any); len(refs) != 1 {
		t.Fatalf("references = %v", e["references"])
	}
	if list := decodeArr(t, admin.do("GET", "/api/vulnerabilities/cves", "")); len(list) != 1 || list[0]["analysis_pt"] != "" {
		t.Fatalf("catalog = %+v", list)
	}
	if resp := admin.do("GET", "/api/vulnerabilities/cves/not-a-cve", ""); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad id = %d, want 400", resp.StatusCode)
	}

	// The findings link to nothing, so they are admin-only: a scoped user
	// can neither analyse nor read the CVE.
	scoped := newVulnEnvWith(t, h, "sga-user", entidadeID(t, d, "sga"))
	if resp := scoped.do("POST", "/api/vulnerabilities/"+ids["a"]+"/cve-analysis", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("scoped analyse = %d, want 404", resp.StatusCode)
	}
	if got := readBody(scoped.do("GET", "/api/vulnerabilities/cves/CVE-2024-0001", "")); strings.TrimSpace(got) != "null" {
		t.Fatalf("scoped read leaked: %s", got)
	}
	if list := decodeArr(t, scoped.do("GET", "/api/vulnerabilities/cves", "")); len(list) != 0 {
		t.Fatalf("scoped catalog leaked: %+v", list)
	}
}
