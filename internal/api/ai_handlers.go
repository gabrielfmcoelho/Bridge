package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/llm"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type aiHandlers struct {
	db *database.DB
}

func (h *aiHandlers) getClient() (*llm.Client, error) {
	get := func(key string) string { return models.GetAppSettingValue(h.db.SQL, key) }

	if get("llm_enabled") != "true" {
		return nil, http.ErrAbortHandler
	}

	baseURL := get("llm_base_url")
	model := get("llm_model_text")
	maxTokensStr := get("llm_max_tokens")
	maxTokens := 2000
	if n, err := strconv.Atoi(maxTokensStr); err == nil && n > 0 {
		maxTokens = n
	}

	// Decrypt API key.
	cipherHex := get("llm_api_key_cipher")
	nonceHex := get("llm_api_key_nonce")
	if cipherHex == "" || nonceHex == "" || baseURL == "" {
		return nil, http.ErrAbortHandler
	}
	cipher, _ := hex.DecodeString(cipherHex)
	nonce, _ := hex.DecodeString(nonceHex)
	apiKey, err := h.db.Encryptor.Decrypt(cipher, nonce)
	if err != nil {
		return nil, err
	}

	return llm.NewClient(baseURL, apiKey, model, maxTokens), nil
}

// handleStatus checks if the LLM integration is configured.
func (h *aiHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	enabled := models.GetAppSettingValue(h.db.SQL, "llm_enabled") == "true"
	configured := models.GetAppSettingValue(h.db.SQL, "llm_api_key_cipher") != ""
	jsonOK(w, map[string]any{
		"enabled":    enabled,
		"configured": configured,
		"model":      models.GetAppSettingValue(h.db.SQL, "llm_model_text"),
	})
}

// handleAssistIssue generates a structured issue description from a brief summary.
func (h *aiHandlers) handleAssistIssue(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req struct {
		Summary string `json:"summary"`
		Context string `json:"context"`
	}
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

// handleAssistHostDoc generates documentation for a host from its data.
func (h *aiHandlers) handleAssistHostDoc(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req struct {
		HostSlug string `json:"host_slug"`
	}
	if err := decodeJSON(r, &req); err != nil || req.HostSlug == "" {
		jsonError(w, http.StatusBadRequest, "host_slug is required")
		return
	}

	// Fetch host data.
	host, err := models.GetHostBySlug(h.db.SQL, req.HostSlug)
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

// handleAnalyzeProject summarizes what the team is currently working on across
// the project's linked GitLab repos, based on the most recent commits. It returns
// a markdown string in the locale supplied by the caller.
func (h *aiHandlers) handleAnalyzeProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req struct {
		Locale string `json:"locale"`
	}
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

	links, err := models.ListProjectGitLabLinks(h.db.SQL, projectID)
	if err != nil {
		jsonServerError(w, r, "failed to list project links", err)
		return
	}
	if len(links) == 0 {
		jsonError(w, http.StatusBadRequest, "project has no linked GitLab repos")
		return
	}

	// Resolve group links into their member projects (same pattern as the commits tab).
	type target struct {
		ID   int
		Path string
		Name string
		Ref  string
	}
	var targets []target
	for _, l := range links {
		if l.Kind == models.GitLabLinkKindGroup {
			projects, err := glClient.ListGroupProjects(l.GitLabProjectID, true)
			if err != nil {
				continue
			}
			for _, p := range projects {
				targets = append(targets, target{
					ID: p.ID, Path: p.PathWithNamespace, Name: p.Name,
					Ref: l.RefName,
				})
			}
		} else {
			name := l.DisplayName
			if name == "" {
				name = l.GitLabPath
			}
			targets = append(targets, target{
				ID: l.GitLabProjectID, Path: l.GitLabPath, Name: name,
				Ref: l.RefName,
			})
		}
	}
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
		go func(t target) {
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
	project, err := models.GetProject(h.db.SQL, projectID)
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
		ProjectID:    projectID,
		Content:      analysis,
		Locale:       locale,
		CommitsUsed:  len(all),
		ReposUsed:    len(targets),
	}
	if err := models.UpsertProjectAIAnalysis(h.db.SQL, record); err != nil {
		// Persistence failure shouldn't hide the freshly-generated result from the user,
		// so log and still return the analysis. Next visit will see no cached version.
		log.Printf("[ai] failed to cache project analysis project=%d: %v", projectID, err)
	}

	// Re-read to get the DB-generated generated_at timestamp.
	if saved, err := models.GetProjectAIAnalysis(h.db.SQL, projectID); err == nil && saved != nil {
		jsonOK(w, saved)
		return
	}
	jsonOK(w, map[string]any{
		"project_id":    projectID,
		"content":       analysis,
		"locale":        locale,
		"commits_used":  len(all),
		"repos_used":    len(targets),
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

// handleGetProjectAnalysis returns the cached analysis (if any). Never calls the LLM.
// Returns 200 with null content when no cached analysis exists — the frontend treats
// that as "show Generate button" rather than an error.
func (h *aiHandlers) handleGetProjectAnalysis(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	cached, err := models.GetProjectAIAnalysis(h.db.SQL, projectID)
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

// handleChat handles a natural language query about infrastructure.
func (h *aiHandlers) handleChat(w http.ResponseWriter, r *http.Request) {
	client, err := h.getClient()
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Message == "" {
		jsonError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Build context with high-level stats.
	var hostCount, svcCount, dnsCount, projCount int
	h.db.SQL.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&hostCount)
	h.db.SQL.QueryRow(`SELECT COUNT(*) FROM services`).Scan(&svcCount)
	h.db.SQL.QueryRow(`SELECT COUNT(*) FROM dns_records`).Scan(&dnsCount)
	h.db.SQL.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projCount)
	statsContext := "Infrastructure summary: " +
		strconv.Itoa(hostCount) + " hosts, " +
		strconv.Itoa(svcCount) + " services, " +
		strconv.Itoa(dnsCount) + " DNS records, " +
		strconv.Itoa(projCount) + " projects."

	// Some OpenAI-compatible gateways (e.g. LiteLLM in front of certain models)
	// reject multiple system messages with "System message must be at the beginning".
	// Merge them into a single leading system message to stay compatible everywhere.
	messages := []llm.ChatMessage{
		{Role: "system", Content: llm.ChatSystem + "\n\n" + statsContext},
		{Role: "user", Content: req.Message},
	}

	result, err := client.Complete(r.Context(), messages)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}

	jsonOK(w, map[string]string{"response": result})
}
