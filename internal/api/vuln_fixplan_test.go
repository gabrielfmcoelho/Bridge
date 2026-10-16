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
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestGitlabRepoPath(t *testing.T) {
	base := "https://gitlab.sead.pi.gov.br"
	for raw, want := range map[string]string{
		"git@gitlab.sead.pi.gov.br:sead/app.git":                    "sead/app",
		"ssh://git@gitlab.sead.pi.gov.br:2222/sead/sub/app.git":     "sead/sub/app",
		"https://gitlab.sead.pi.gov.br/sead/app":                    "sead/app",
		"https://gitlab.sead.pi.gov.br/sead/app/-/tree/main/docker": "sead/app",
		"https://github.com/sead/app":                               "", // another host
		"https://gitlab.sead.pi.gov.br/sead":                        "", // a group, not a repo
		"":                                                          "",
	} {
		got, ok := gitlabRepoPath(raw, base)
		if got != want || ok != (want != "") {
			t.Errorf("gitlabRepoPath(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
}

func TestReadLines(t *testing.T) {
	body := strings.Repeat("x\n", 250) + "end"
	m := readLines("a.txt", body, 0)
	if m["from"] != 1 || m["to"] != 200 || m["more"] != true || !strings.HasPrefix(m["content"].(string), "1: x\n") {
		t.Fatalf("first page = %+v", m)
	}
	m = readLines("a.txt", body, 201)
	if m["to"] != 251 || m["more"] != false || !strings.Contains(m["content"].(string), "251: end") {
		t.Fatalf("second page from=%v to=%v more=%v", m["from"], m["to"], m["more"])
	}
}

// fakeRepoServer is a GitLab with one repo (id 7, "sead/app") holding a
// Dockerfile, at ref main.
func fakeRepoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/projects/7/repository/tree":
			_, _ = w.Write([]byte(`[{"name":"Dockerfile","type":"blob","path":"Dockerfile"},{"name":"src","type":"tree","path":"src"}]`))
		case r.URL.EscapedPath() == "/api/v4/projects/7/repository/files/Dockerfile/raw":
			if r.URL.Query().Get("ref") != "main" {
				t.Errorf("raw read ref = %q, want main", r.URL.Query().Get("ref"))
			}
			_, _ = w.Write([]byte("FROM metabase/metabase:v0.46.1\nEXPOSE 3000"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The loop reads only allowlisted repos and hosts, and the last round
// forces an answer.
func TestFixTools_ThroughToolLoop(t *testing.T) {
	srv := fakeRepoServer(t)
	sc := &fixScope{
		repos: []fixRepo{{Key: "sead/app", ID: 7, Name: "app", Ref: "main"}},
		hosts: map[string]*sshtest.VMInfo{"web-01": {InstalledPackages: []sshtest.InstalledPackage{{Name: "openssl", Version: "3.0.2", Source: "apt"}}}},
		trees: map[int][]gitlabclient.TreeEntry{},
	}
	tools := fixTools(sc, gitlabclient.NewClient(srv.URL, "tok"))
	toolReply := func(msgs []llm.ChatMessage, id string) string {
		for _, m := range msgs {
			if m.Role == "tool" && m.ToolCallID == id {
				return m.Content
			}
		}
		t.Fatalf("no tool reply %s", id)
		return ""
	}
	m := &fakeModel{replies: []func([]llm.ChatMessage, []llm.Tool) (llm.ChatMessage, error){
		func(_ []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error) {
			if len(tools) != 5 {
				t.Fatalf("offered %d tools, want the 5 fix tools", len(tools))
			}
			return llm.ChatMessage{Role: "assistant", ToolCalls: []llm.ToolCall{
				toolCall("a", "find_files", `{"repo":"sead/app","name":"docker"}`),
				toolCall("b", "read_file", `{"repo":"other/secret","path":"x"}`),
				toolCall("c", "host_packages", `{"host":"web-01","query":"ssl"}`),
				toolCall("d", "host_packages", `{"host":"db-99","query":"ssl"}`),
			}}, nil
		},
		func(msgs []llm.ChatMessage, _ []llm.Tool) (llm.ChatMessage, error) {
			if !strings.Contains(toolReply(msgs, "a"), `"path":"Dockerfile"`) {
				t.Fatalf("find_files = %s", toolReply(msgs, "a"))
			}
			if !strings.Contains(toolReply(msgs, "b"), "not available") {
				t.Fatalf("a repo outside the scope must be refused: %s", toolReply(msgs, "b"))
			}
			if !strings.Contains(toolReply(msgs, "c"), "3.0.2") || !strings.Contains(toolReply(msgs, "d"), "no scan") {
				t.Fatalf("host_packages = %s / %s", toolReply(msgs, "c"), toolReply(msgs, "d"))
			}
			return llm.ChatMessage{Role: "assistant", ToolCalls: []llm.ToolCall{toolCall("e", "read_file", `{"repo":"SEAD/app","path":"Dockerfile"}`)}}, nil
		},
		func(msgs []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error) {
			if !strings.Contains(toolReply(msgs, "e"), "1: FROM metabase/metabase:v0.46.1") {
				t.Fatalf("read_file = %s", toolReply(msgs, "e"))
			}
			if tools != nil {
				t.Fatal("the last round must offer no tools")
			}
			return llm.ChatMessage{Role: "assistant", Content: "### Diagnóstico neste ambiente\nv0.46.1"}, nil
		},
	}}
	res, err := runToolLoop(context.Background(), m, aiToolEnv{}, []llm.ChatMessage{{Role: "user", Content: "x"}}, tools, 2)
	if err != nil || !strings.Contains(res.Response, "v0.46.1") || len(res.Steps) != 5 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.Steps[4].Query != "SEAD/app:Dockerfile" || res.Steps[1].Error == "" {
		t.Fatalf("steps = %+v", res.Steps)
	}
}

// TestVuln_FixPlan: end to end through the handler with a fake GitLab and a
// fake LLM that reads the Dockerfile, then answers.
func TestVuln_FixPlan(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatal(err)
	}
	gl := fakeRepoServer(t)

	var llmCalls atomic.Int32
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llm.CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		var msg map[string]any
		if llmCalls.Add(1) == 1 {
			if len(req.Tools) == 0 || !strings.Contains(req.Messages[1].Content, "sead/app") || !strings.Contains(req.Messages[1].Content, "Catálogo") {
				t.Errorf("first request lacks tools, repo list or topology: %s", req.Messages[1].Content)
			}
			msg = map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{
				"id": "c1", "type": "function", "function": map[string]any{"name": "read_file", "arguments": `{"repo":"sead/app","path":"Dockerfile"}`},
			}}}
		} else {
			last := req.Messages[len(req.Messages)-1]
			if last.Role != "tool" || !strings.Contains(last.Content, "metabase:v0.46.1") {
				t.Errorf("second request lacks the file read: %+v", last)
			}
			msg = map[string]any{"role": "assistant", "content": "### Diagnóstico neste ambiente\nImagem metabase:v0.46.1 em sead/app:Dockerfile:1"}
		}
		out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": msg}}})
		_, _ = w.Write(out)
	}))
	t.Cleanup(llmSrv.Close)

	// Finding → project "Catálogo" → GitLab repo sead/app (id 7, ref main).
	var projectID int64
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('Catálogo') RETURNING id`).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO project_gitlab_links (project_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind, ref_name)
		VALUES (?, 7, ?, 'sead/app', 'project', 'main')`, projectID, gl.URL); err != nil {
		t.Fatal(err)
	}
	vulns := store.NewVulnRepo(d.SQL)
	if _, err := vulns.ApplyImport(ctx, "etipi", []models.VulnFinding{{Fingerprint: "sqli", Title: "Metabase - SQL Injection", Severity: "critical", Target: "x"}}, nil, false); err != nil {
		t.Fatal(err)
	}
	all, _, _ := vulns.List(ctx, models.VulnFilter{})
	fid := all[0].ID
	if err := vulns.AddLink(ctx, fid, "project", projectID); err != nil {
		t.Fatal(err)
	}

	admin := newVulnEnvWith(t, &vulnHandlers{db: d}, "admin", 0)
	path := "/api/vulnerabilities/" + itoa(fid) + "/fix-plan"
	if resp := admin.do("POST", path, ""); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("AI off = %d, want 503", resp.StatusCode)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	secrets := store.NewAppSecretRepo(d.SQL)
	for k, v := range map[string]string{"llm_enabled": "true", "llm_base_url": llmSrv.URL, "llm_model_text": "qwen",
		"gitlab_integration_enabled": "true", "auth_gitlab_base_url": gl.URL} {
		if err := settings.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range map[string]string{"llm_api_key": "k", "gitlab_code_service_token": "tok"} {
		if err := secrets.Store(ctx, d.Encryptor, k, v); err != nil {
			t.Fatal(err)
		}
	}

	if got := readBody(admin.do("GET", path, "")); strings.TrimSpace(got) != "null" {
		t.Fatalf("plan before generating = %s", got)
	}
	resp := admin.do("POST", path, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("generate = %d %s", resp.StatusCode, readBody(resp))
	}
	p := decodeObj(t, admin.do("GET", path, ""))
	if !strings.Contains(p["content"].(string), "sead/app:Dockerfile:1") || p["model"] != "qwen" {
		t.Fatalf("plan = %+v", p)
	}
	if repos, _ := p["repos"].([]any); len(repos) != 1 || repos[0] != "sead/app" {
		t.Fatalf("repos = %v", p["repos"])
	}
	if steps, _ := p["steps"].([]any); len(steps) != 1 || steps[0].(map[string]any)["query"] != "sead/app:Dockerfile" {
		t.Fatalf("steps = %v", p["steps"])
	}

	// Unlinked-to-sga finding: a scoped user can neither read nor generate.
	scoped := newVulnEnvWith(t, &vulnHandlers{db: d}, "sga-user", entidadeID(t, d, "sga"))
	for _, m := range []string{"GET", "POST"} {
		if resp := scoped.do(m, path, ""); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("scoped %s = %d, want 404", m, resp.StatusCode)
		}
	}
}
