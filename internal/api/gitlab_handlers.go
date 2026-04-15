package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type gitlabHandlers struct {
	db *database.DB
}

// getClientForUser creates a GitLab API client using the authenticated user's stored token.
func (h *gitlabHandlers) getClientForUser(r *http.Request) (*gitlabclient.Client, string, error) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return nil, "", http.ErrNoCookie
	}

	baseURL := models.GetAppSettingValue(h.db.SQL, "auth_gitlab_base_url")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	token, err := models.GetUserGitLabToken(h.db.SQL, user.ID, baseURL)
	if err != nil || token == nil {
		return nil, baseURL, http.ErrNoCookie
	}

	accessToken, err := h.db.Encryptor.Decrypt(token.AccessTokenCipher, token.AccessTokenNonce)
	if err != nil {
		return nil, baseURL, err
	}

	return gitlabclient.NewClient(baseURL, accessToken), baseURL, nil
}

// handleStatus checks if the user has a valid GitLab token configured.
func (h *gitlabHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	client, _, err := h.getClientForUser(r)
	if err != nil || client == nil {
		jsonOK(w, map[string]any{"connected": false})
		return
	}

	user, err := client.GetCurrentUser()
	if err != nil {
		jsonOK(w, map[string]any{"connected": false, "error": "token invalid or expired"})
		return
	}

	jsonOK(w, map[string]any{
		"connected": true,
		"username":  user.Username,
		"name":      user.Name,
	})
}

// handleSaveToken saves a personal access token for the current user.
func (h *gitlabHandlers) handleSaveToken(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Token   string `json:"token"`
		BaseURL string `json:"base_url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Token == "" {
		jsonError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.BaseURL == "" {
		req.BaseURL = models.GetAppSettingValue(h.db.SQL, "auth_gitlab_base_url")
		if req.BaseURL == "" {
			req.BaseURL = "https://gitlab.com"
		}
	}

	// Validate the token by fetching user info.
	client := gitlabclient.NewClient(req.BaseURL, req.Token)
	glUser, err := client.GetCurrentUser()
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid token: "+err.Error())
		return
	}

	// Encrypt and store.
	cipher, nonce, err := h.db.Encryptor.Encrypt(req.Token)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	t := &models.UserGitLabToken{
		UserID:             user.ID,
		GitLabBaseURL:      req.BaseURL,
		AccessTokenCipher:  cipher,
		AccessTokenNonce:   nonce,
		GitLabUserID:       string(rune(glUser.ID)),
		GitLabUsername:     glUser.Username,
	}
	if err := models.UpsertUserGitLabToken(h.db.SQL, t); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	jsonOK(w, map[string]any{"status": "saved", "username": glUser.Username})
}

// handleDeleteToken removes the user's GitLab token.
func (h *gitlabHandlers) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	baseURL := models.GetAppSettingValue(h.db.SQL, "auth_gitlab_base_url")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	models.DeleteUserGitLabToken(h.db.SQL, user.ID, baseURL)
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleListCommits returns recent commits for a linked GitLab project.
func (h *gitlabHandlers) handleListCommits(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	link, err := models.GetProjectGitLabLink(h.db.SQL, projectID)
	if err != nil || link == nil {
		jsonError(w, http.StatusNotFound, "no GitLab link for this project")
		return
	}

	client, _, err := h.getClientForUser(r)
	if err != nil || client == nil {
		jsonError(w, http.StatusUnauthorized, "GitLab token not configured")
		return
	}

	commits, err := client.ListCommits(link.GitLabProjectID, gitlabclient.CommitListParams{PerPage: 20})
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to fetch commits: "+err.Error())
		return
	}

	jsonOK(w, commits)
}

// handleListIssues returns GitLab issues for a linked project.
func (h *gitlabHandlers) handleListIssues(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	link, err := models.GetProjectGitLabLink(h.db.SQL, projectID)
	if err != nil || link == nil {
		jsonError(w, http.StatusNotFound, "no GitLab link for this project")
		return
	}

	client, _, err := h.getClientForUser(r)
	if err != nil || client == nil {
		jsonError(w, http.StatusUnauthorized, "GitLab token not configured")
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		state = "opened"
	}

	issues, err := client.ListIssues(link.GitLabProjectID, gitlabclient.IssueListParams{State: state, PerPage: 20})
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to fetch issues: "+err.Error())
		return
	}

	jsonOK(w, issues)
}

// handleLinkProject links an SSHCM project to a GitLab project.
func (h *gitlabHandlers) handleLinkProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req struct {
		GitLabPath string `json:"gitlab_path"` // e.g., "org/repo"
	}
	if err := decodeJSON(r, &req); err != nil || req.GitLabPath == "" {
		jsonError(w, http.StatusBadRequest, "gitlab_path is required")
		return
	}

	client, baseURL, err := h.getClientForUser(r)
	if err != nil || client == nil {
		jsonError(w, http.StatusUnauthorized, "GitLab token not configured")
		return
	}

	// Look up the GitLab project by path.
	glProject, err := client.SearchProjectByPath(req.GitLabPath)
	if err != nil {
		jsonError(w, http.StatusNotFound, "GitLab project not found: "+err.Error())
		return
	}

	link := &models.ProjectGitLabLink{
		ProjectID:       projectID,
		GitLabProjectID: glProject.ID,
		GitLabBaseURL:   baseURL,
		GitLabPath:      glProject.PathWithNamespace,
	}
	if err := models.CreateProjectGitLabLink(h.db.SQL, link); err != nil {
		jsonError(w, http.StatusConflict, "link already exists or failed to create")
		return
	}

	jsonCreated(w, link)
}
