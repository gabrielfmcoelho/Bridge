package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type aiHandlers struct {
	db *database.DB
}

func (h *aiHandlers) getClient() (*llm.Client, error) {
	return service.LLMClient(context.Background(), h.db)
}

// handleStatus checks if the LLM integration is configured.
//
//	@Summary		LLM integration status
//	@Description	Any role. Returns {enabled, configured, model}.
//	@Tags			ai
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/ai/status [get]
func (h *aiHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	enabled := store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "llm_enabled") == "true"
	// Same conditions as getClient: a key and a base URL.
	configured := store.NewAppSecretRepo(h.db.SQL).Configured(r.Context(), "llm_api_key") &&
		store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "llm_base_url") != ""
	jsonOK(w, map[string]any{
		"enabled":    enabled,
		"configured": configured,
		"model":      store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "llm_model_text"),
	})
}

// aiAssistIssueRequest is the issue-draft body.
type aiAssistIssueRequest struct {
	Summary string `json:"summary"`
	Context string `json:"context"`
}

// handleAssistIssue generates a structured issue description from a brief summary.
//
//	@Summary		Draft an issue description
//	@Description	Requires permission ai.use. Returns {description}. 503 when the LLM is not configured.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		aiAssistIssueRequest	true	"Summary and optional context"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/ai/assist/issue [post]
func (h *aiHandlers) handleAssistIssue(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req aiAssistIssueRequest
	if err := decodeJSON(r, &req); err != nil || req.Summary == "" {
		jsonError(w, http.StatusBadRequest, "summary is required")
		return
	}

	userMsg := req.Summary
	if req.Context != "" {
		userMsg += "\n\nAdditional context:\n" + req.Context
	}

	messages := []llm.ChatMessage{
		{Role: "system", Content: llm.IssueAssistSystem},
		{Role: "user", Content: userMsg},
	}

	result, err := client.Complete(r.Context(), messages)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}

	jsonOK(w, map[string]string{"description": result})
}

// aiAssistHostDocRequest names the host to document.
type aiAssistHostDocRequest struct {
	HostSlug string `json:"host_slug"`
}

// handleAssistHostDoc generates documentation for a host from its data.
//
//	@Summary		Draft host documentation
//	@Description	Requires permission ai.use. Returns {documentation}. 503 when the LLM is not configured.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		aiAssistHostDocRequest	true	"Host to document"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/ai/assist/host-doc [post]
func (h *aiHandlers) handleAssistHostDoc(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req aiAssistHostDocRequest
	if err := decodeJSON(r, &req); err != nil || req.HostSlug == "" {
		jsonError(w, http.StatusBadRequest, "host_slug is required")
		return
	}

	// Fetch host data.
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), req.HostSlug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	// Build context from host data.
	hostInfo := "Host: " + host.Nickname + " (" + host.Hostname + ")\n"
	hostInfo += "Type: " + host.TipoMaquina + "\n"
	hostInfo += "Hosting: " + host.Hospedagem + "\n"
	if host.Description != "" {
		hostInfo += "Description: " + host.Description + "\n"
	}

	messages := []llm.ChatMessage{
		{Role: "system", Content: llm.HostDocSystem},
		{Role: "user", Content: "Generate documentation for this host:\n\n" + hostInfo},
	}

	result, err := client.Complete(r.Context(), messages)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}

	jsonOK(w, map[string]string{"documentation": result})
}

// aiAnalyzeProjectRequest is the optional analyze body.
type aiAnalyzeProjectRequest struct {
	Locale string `json:"locale"`
}

// handleAnalyzeProject summarizes what the team is currently working on across
// the project's linked GitLab repos, based on the most recent commits. It returns
// a markdown string in the locale supplied by the caller.
//
//	@Summary		Generate a project AI analysis
//	@Description	Requires permission ai.use. Summarizes the recent commits of the project's linked GitLab repos (markdown, in the requested locale) and caches the result. The body is optional. 400 when GitLab is disabled/unconfigured or no repos are linked; 503 when the LLM is not configured.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"Project ID"
//	@Param			body	body		aiAnalyzeProjectRequest	false	"Response locale (pt* → Portuguese, else English)"
//	@Success		200		{object}	models.ProjectAIAnalysis
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/projects/{id}/ai/analyze [post]
func (h *aiHandlers) handleAnalyzeProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	if ok, err := store.CanSee(r.Context(), h.db.SQL, store.AssetProject, projectID); err != nil || !ok {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req aiAnalyzeProjectRequest
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &req)
	}
	locale := strings.TrimSpace(req.Locale)
	languageInstruction := "Respond in English."
	if strings.HasPrefix(strings.ToLower(locale), "pt") {
		languageInstruction = "Responda em português do Brasil."
	}

	// Load GitLab Code Management settings — without a service token we can't fetch commits.
	settings, err := gitlabclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load gitlab settings", err)
		return
	}
	if !settings.Enabled {
		jsonError(w, http.StatusBadRequest, "GitLab Code Management is disabled")
		return
	}
	glClient := gitlabclient.NewServiceClient(settings)
	if glClient == nil {
		jsonError(w, http.StatusBadRequest, "GitLab service token not configured")
		return
	}

	links, err := store.NewProjectGitLabLinkRepo(h.db.SQL).List(r.Context(), projectID)
	if err != nil {
		jsonServerError(w, r, "failed to list project links", err)
		return
	}
	if len(links) == 0 {
		jsonError(w, http.StatusBadRequest, "project has no linked GitLab repos")
		return
	}

	targets, _ := gitlabTargets(links, glClient, settings.DefaultRef)
	if len(targets) == 0 {
		jsonError(w, http.StatusBadGateway, "no reachable GitLab repos for this project")
		return
	}

	// Fetch 10 recent commits per target with bounded concurrency (matches the Commits tab).
	const perRepo = 10
	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	type enriched struct {
		gitlabclient.Commit
		RepoName string
		RepoPath string
	}
	var all []enriched

	for _, tgt := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t gitlabTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			batch, err := glClient.ListCommits(t.ID, gitlabclient.CommitListParams{
				RefName: t.Ref,
				All:     t.Ref == "",
				PerPage: perRepo,
			})
			if err != nil {
				return
			}
			mu.Lock()
			for _, c := range batch {
				all = append(all, enriched{Commit: c, RepoName: t.Name, RepoPath: t.Path})
			}
			mu.Unlock()
		}(tgt)
	}
	wg.Wait()

	if len(all) == 0 {
		jsonError(w, http.StatusBadGateway, "no commits found on the linked repos")
		return
	}

	// Newest first, cap at 50 commits total so the prompt stays compact.
	sort.Slice(all, func(i, j int) bool {
		return all[i].CommittedDate.After(all[j].CommittedDate)
	})
	if len(all) > 50 {
		all = all[:50]
	}

	// Compact the commit list into a single block of lines: repo | date | author | title.
	var buf strings.Builder
	buf.WriteString("Recent commits across the project's linked repositories:\n\n")
	for _, c := range all {
		buf.WriteString(fmt.Sprintf("- [%s] %s · %s · %s\n",
			c.RepoName,
			c.CommittedDate.Format("2006-01-02"),
			strings.TrimSpace(c.AuthorName),
			strings.TrimSpace(c.Title),
		))
	}

	// Build the user prompt with the locale instruction up front so the model can't miss it.
	project, err := store.NewProjectRepo(h.db.SQL).Get(r.Context(), projectID)
	projectName := ""
	if err == nil && project != nil {
		projectName = project.Name
	}
	userMsg := languageInstruction + "\n\n"
	if projectName != "" {
		userMsg += fmt.Sprintf("Project: %s\n\n", projectName)
	}
	userMsg += buf.String()
	userMsg += "\nSummarize what the team is actively working on."

	messages := []llm.ChatMessage{
		{Role: "system", Content: llm.ProjectAnalysisSystem},
		{Role: "user", Content: userMsg},
	}

	// Give reasoning models breathing room.
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	reply, err := client.Complete(ctx, messages)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}

	analysis := strings.TrimSpace(reply)
	record := &models.ProjectAIAnalysis{
		ProjectID:   projectID,
		Content:     analysis,
		Locale:      locale,
		CommitsUsed: len(all),
		ReposUsed:   len(targets),
	}
	if err := store.NewProjectAIAnalysisRepo(h.db.SQL).Upsert(r.Context(), record); err != nil {
		// Persistence failure shouldn't hide the freshly-generated result from the user,
		// so log and still return the analysis. Next visit will see no cached version.
		log.Printf("[ai] failed to cache project analysis project=%d: %v", projectID, err)
	}

	// Re-read to get the DB-generated generated_at timestamp.
	if saved, err := store.NewProjectAIAnalysisRepo(h.db.SQL).Get(r.Context(), projectID); err == nil && saved != nil {
		jsonOK(w, saved)
		return
	}
	jsonOK(w, map[string]any{
		"project_id":   projectID,
		"content":      analysis,
		"locale":       locale,
		"commits_used": len(all),
		"repos_used":   len(targets),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleGetProjectAnalysis returns the cached analysis (if any). Never calls the LLM.
// Returns 200 with null content when no cached analysis exists — the frontend treats
// that as "show Generate button" rather than an error.
//
//	@Summary		Cached project AI analysis
//	@Description	Any role. Never calls the LLM. Answers 200 with a null body when no analysis is cached.
//	@Tags			ai
//	@Produce		json
//	@Param			id	path		int	true	"Project ID"
//	@Success		200	{object}	models.ProjectAIAnalysis
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/projects/{id}/ai/analyze [get]
func (h *aiHandlers) handleGetProjectAnalysis(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	if ok, err := store.CanSee(r.Context(), h.db.SQL, store.AssetProject, projectID); err != nil || !ok {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	cached, err := store.NewProjectAIAnalysisRepo(h.db.SQL).Get(r.Context(), projectID)
	if err != nil {
		jsonServerError(w, r, "failed to read cached analysis", err)
		return
	}
	if cached == nil {
		jsonOK(w, nil)
		return
	}
	jsonOK(w, cached)
}

// aiChatRequest is the chat body.
type aiChatRequest struct {
	Message string `json:"message"`
	// History is the conversation so far, oldest first (roles user/assistant),
	// so follow-up questions keep their context. Optional.
	History []llm.ChatMessage `json:"history"`
}

// Chat history limits: the last maxChatHistory turns, each cut to
// maxChatTurnChars, so a long session can't grow the prompt without bound.
const (
	maxChatHistory   = 20
	maxChatTurnChars = 8000
)

// chatMessages builds the completion messages: one leading system message
// (some OpenAI-compatible gateways, e.g. LiteLLM in front of certain models,
// reject more than one), then the recent history, then the new message.
// History turns with any other role are dropped, so a client can't inject a
// system prompt.
func chatMessages(system string, history []llm.ChatMessage, message string) []llm.ChatMessage {
	if len(history) > maxChatHistory {
		history = history[len(history)-maxChatHistory:]
	}
	out := []llm.ChatMessage{{Role: "system", Content: system}}
	for _, m := range history {
		if (m.Role != "user" && m.Role != "assistant") || m.Content == "" {
			continue
		}
		if len(m.Content) > maxChatTurnChars {
			// Cut on bytes, then drop a rune the cut split in half.
			m.Content = strings.ToValidUTF8(m.Content[:maxChatTurnChars], "")
		}
		out = append(out, m)
	}
	return append(out, llm.ChatMessage{Role: "user", Content: message})
}

// handleChat handles a natural language query about infrastructure.
//
//	@Summary		Ask about the infrastructure
//	@Description	Requires permission ai.use. Body {message, history?: [{role: user|assistant, content}]} (last 20 turns kept). The model may call entidade-scoped search tools and draft issue/chamado proposals (never executed server-side). Returns {response, tools, steps, refs, proposals}. 503 when the LLM is not configured.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		aiChatRequest	true	"User message"
//	@Success		200		{object}	aiChatResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/ai/chat [post]
func (h *aiHandlers) handleChat(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req aiChatRequest
	if err := decodeJSON(r, &req); err != nil || req.Message == "" {
		jsonError(w, http.StatusBadRequest, "message is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second) // keeps the caller's scope
	defer cancel()
	user := auth.UserFromContext(ctx)
	messages := chatMessages(llm.ChatSystem+"\n\n"+h.chatContext(ctx, user), req.History, req.Message)

	result, err := runChat(ctx, client, aiToolEnv{db: h.db, user: user}, messages)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}
	jsonOK(w, result)
}

// chatContext tells the model who is asking and what they can see: name,
// role, entidades and the size of their (entidade-scoped) inventory.
func (h *aiHandlers) chatContext(ctx context.Context, user *models.User) string {
	var b strings.Builder
	if user != nil {
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		fmt.Fprintf(&b, "Current user: %s (role %s).", name, user.Role)
		if ents, _ := store.NewUserEntidadeRepo(h.db.SQL).ListForUser(ctx, user.ID); len(ents) > 0 {
			names := make([]string, len(ents))
			for i, e := range ents {
				names[i] = e.Name
			}
			fmt.Fprintf(&b, " Entidades: %s.", strings.Join(names, ", "))
		}
		if roleLevel[user.Role] < roleLevel["editor"] {
			b.WriteString(" This user is a viewer: they cannot create issues or open chamados in Bridge, so draft text instead of proposing.")
		}
	}
	hosts, _ := store.NewHostRepo(h.db.SQL).CountAll(ctx)
	svcs, _ := store.NewServiceRepo(h.db.SQL).Count(ctx)
	dns, _ := store.NewDNSRepo(h.db.SQL).Count(ctx)
	projects, _ := store.NewProjectRepo(h.db.SQL).Count(ctx)
	visSQL, visArgs := store.VisibleExprDyn(ctx, "entity_type", "entity_id")
	issues, _ := models.OpenIssueCountVisible(h.db.SQL, visSQL, visArgs)
	fmt.Fprintf(&b, "\nVisible to this user: %d hosts, %d services, %d DNS records, %d projects, %d open issues. Today is %s.",
		hosts, svcs, dns, projects, issues, time.Now().Format("2006-01-02"))
	return b.String()
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *aiHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/ai/status", h.handleStatus)
	rr.perm("ai.use", "POST /api/ai/assist/issue", h.handleAssistIssue)
	rr.perm("ai.use", "POST /api/ai/assist/host-doc", h.handleAssistHostDoc)
	rr.perm("ai.use", "POST /api/ai/chat", h.handleChat)
	rr.auth("GET /api/projects/{id}/ai/analyze", h.handleGetProjectAnalysis)
	rr.perm("ai.use", "POST /api/projects/{id}/ai/analyze", h.handleAnalyzeProject)
}
