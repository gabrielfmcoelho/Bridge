package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestAITools_Scope: the assistant's tools see exactly what the caller's
// entidades see. One project + host + issue granted to sga, a twin of each
// granted to nobody (admin-only): a sga user finds only theirs, can't
// propose an issue under the hidden project, and an admin sees both.
func TestAITools_Scope(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if d == nil {
		return // skipped inside dbtest
	}
	ctx := context.Background()
	var sga, uid int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga); err != nil {
		t.Fatalf("sga: %v", err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('ana', 'x', 'editor') RETURNING id`).Scan(&uid); err != nil {
		t.Fatalf("user: %v", err)
	}
	grants := store.NewAssetEntidadeRepo(d.SQL)
	ids := map[string]int64{}
	for _, name := range []string{"portal-visivel", "portal-oculto"} {
		var pid, hid int64
		if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ($1) RETURNING id`, name).Scan(&pid); err != nil {
			t.Fatalf("project: %v", err)
		}
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES ($1, $1, '10.0.0.1') RETURNING id`, name).Scan(&hid); err != nil {
			t.Fatalf("host: %v", err)
		}
		issue := models.Issue{ProjectID: &pid, Title: "issue " + name, Status: "todo", Priority: "high", CreatedBy: uid}
		if err := models.CreateIssue(d.SQL, &issue); err != nil {
			t.Fatalf("issue: %v", err)
		}
		if name == "portal-visivel" {
			for at, id := range map[store.AssetType]int64{store.AssetProject: pid, store.AssetHost: hid} {
				if err := grants.Replace(ctx, d.SQL, at, id, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
					t.Fatalf("grant: %v", err)
				}
			}
		}
		ids[name] = pid
	}

	user := &models.User{ID: uid, Username: "ana", Role: "editor"}
	env := aiToolEnv{db: d, user: user}
	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}})
	admin := store.WithScope(ctx, store.Scope{Admin: true})
	q := json.RawMessage(`{"query":"portal"}`)

	for _, c := range []struct {
		name string
		run  func(context.Context, aiToolEnv, json.RawMessage) (toolOutput, error)
		args json.RawMessage
	}{
		{"projects", searchProjects, q},
		{"hosts", searchHosts, q},
		{"issues", searchIssues, json.RawMessage(`{"query":"portal"}`)},
	} {
		out, err := c.run(scoped, env, c.args)
		if err != nil || out.Total != 1 || !strings.Contains(out.Refs[0].Label, "visivel") {
			t.Errorf("%s scoped: total=%d refs=%+v err=%v, want only the visible one", c.name, out.Total, out.Refs, err)
		}
		if out, err := c.run(admin, env, c.args); err != nil || out.Total != 2 {
			t.Errorf("%s admin: total=%d err=%v, want 2", c.name, out.Total, err)
		}
	}

	propose := func(pid int64) (toolOutput, error) {
		return proposeIssue(scoped, env, json.RawMessage(`{"title":"Renovar certificado","priority":"urgent","parent_type":"project","parent_id":`+jsonInt(pid)+`}`))
	}
	if _, err := propose(ids["portal-oculto"]); !errors.Is(err, errToolNotFound) {
		t.Fatalf("propose under hidden project: err=%v, want errToolNotFound", err)
	}
	out, err := propose(ids["portal-visivel"])
	if err != nil || out.Proposal == nil || out.Proposal.Parent.Href != "/projects/"+jsonInt(ids["portal-visivel"]) || out.Proposal.Priority != "medium" || out.Proposal.ParentID != ids["portal-visivel"] {
		t.Fatalf("propose under visible project: %+v err=%v", out.Proposal, err)
	}
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestToolsFor_ProposalsNeedEditor(t *testing.T) {
	names := func(u *models.User) string {
		var out []string
		for _, tl := range toolsFor(u) {
			out = append(out, tl.def.Function.Name)
		}
		return strings.Join(out, ",")
	}
	if v := names(&models.User{Role: "viewer"}); strings.Contains(v, "propose_") {
		t.Fatalf("viewer tools = %s, want no proposals", v)
	}
	if e := names(&models.User{Role: "editor"}); !strings.Contains(e, "propose_issue") || !strings.Contains(e, "propose_chamado") {
		t.Fatalf("editor tools = %s, want proposals", e)
	}
}

// fakeModel replays scripted answers and records what it was sent.
type fakeModel struct {
	replies []func(msgs []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error)
	calls   int
}

func (f *fakeModel) Chat(_ context.Context, msgs []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error) {
	r := f.replies[f.calls]
	f.calls++
	return r(msgs, tools)
}

func toolCall(id, name, args string) llm.ToolCall {
	var c llm.ToolCall
	c.ID, c.Type = id, "function"
	c.Function.Name, c.Function.Arguments = name, args
	return c
}

// TestRunChat: a tool round then an answer; unknown tools come back as an
// error the model reads; a gateway that rejects tools gets a plain answer.
func TestRunChat(t *testing.T) {
	env := aiToolEnv{user: &models.User{ID: 7, Role: "viewer"}}
	m := &fakeModel{replies: []func([]llm.ChatMessage, []llm.Tool) (llm.ChatMessage, error){
		func(_ []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error) {
			if len(tools) == 0 {
				t.Fatal("first round has no tools")
			}
			return llm.ChatMessage{Role: "assistant", ToolCalls: []llm.ToolCall{toolCall("c1", "drop_database", `{}`)}}, nil
		},
		func(msgs []llm.ChatMessage, _ []llm.Tool) (llm.ChatMessage, error) {
			last := msgs[len(msgs)-1]
			if last.Role != "tool" || last.ToolCallID != "c1" || !strings.Contains(last.Content, "unknown tool") {
				t.Fatalf("tool turn = %+v", last)
			}
			return llm.ChatMessage{Role: "assistant", Content: "pronto"}, nil
		},
	}}
	res, err := runChat(context.Background(), m, env, []llm.ChatMessage{{Role: "user", Content: "oi"}})
	if err != nil || res.Response != "pronto" || !res.Tools || len(res.Steps) != 1 || res.Steps[0].Error == "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}

	rejecting := &fakeModel{replies: []func([]llm.ChatMessage, []llm.Tool) (llm.ChatMessage, error){
		func(_ []llm.ChatMessage, _ []llm.Tool) (llm.ChatMessage, error) {
			return llm.ChatMessage{}, &llm.APIError{Status: 400, Body: `"tool_choice" is not supported`}
		},
		func(_ []llm.ChatMessage, tools []llm.Tool) (llm.ChatMessage, error) {
			if tools != nil {
				t.Fatal("retry still sends tools")
			}
			return llm.ChatMessage{Role: "assistant", Content: "sem ferramentas"}, nil
		},
	}}
	res, err = runChat(context.Background(), rejecting, env, []llm.ChatMessage{{Role: "user", Content: "oi"}})
	if err != nil || res.Tools || res.Response != "sem ferramentas" {
		t.Fatalf("fallback res=%+v err=%v", res, err)
	}

	down := &fakeModel{replies: []func([]llm.ChatMessage, []llm.Tool) (llm.ChatMessage, error){
		func(_ []llm.ChatMessage, _ []llm.Tool) (llm.ChatMessage, error) {
			return llm.ChatMessage{}, &llm.APIError{Status: 503, Body: "down"}
		},
	}}
	if _, err := runChat(context.Background(), down, env, nil); err == nil {
		t.Fatal("a 5xx must surface as an error, not a silent fallback")
	}
}

// TestLLMClientChat: tools go out in the request, tool_calls come back, and
// a 4xx is an *llm.APIError the chat loop can recognize.
func TestLLMClientChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llm.CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "no-tools" {
			http.Error(w, `{"error":"tools not supported"}`, http.StatusBadRequest)
			return
		}
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "search_hosts" {
			http.Error(w, "missing tools", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"x","type":"function","function":{"name":"search_hosts","arguments":"{\"query\":\"sei\"}"}}]}}]}`))
	}))
	defer srv.Close()

	tools := []llm.Tool{aiTools[0].def}
	msg, err := llm.NewClient(srv.URL, "k", "m", 0).Chat(context.Background(), nil, tools)
	if err != nil || len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Arguments != `{"query":"sei"}` {
		t.Fatalf("msg=%+v err=%v", msg, err)
	}
	_, err = llm.NewClient(srv.URL, "k", "no-tools", 0).Chat(context.Background(), nil, tools)
	var apiErr *llm.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 400 {
		t.Fatalf("err=%v, want *llm.APIError 400", err)
	}
}
