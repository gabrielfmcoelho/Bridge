package api

import (
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
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

	messages := []llm.ChatMessage{
		{Role: "system", Content: llm.ChatSystem},
		{Role: "system", Content: statsContext},
		{Role: "user", Content: req.Message},
	}

	result, err := client.Complete(r.Context(), messages)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "AI request failed: "+err.Error())
		return
	}

	jsonOK(w, map[string]string{"response": result})
}
