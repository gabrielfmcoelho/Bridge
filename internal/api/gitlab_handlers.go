package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
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

	baseURL := store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "auth_gitlab_base_url")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	token, err := store.NewUserGitLabTokenRepo(h.db.SQL).Get(r.Context(), user.ID, baseURL)
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
//
//	@Summary		Caller's GitLab connection status
//	@Description	Any role. {"connected": false[, "error"]} when no usable token, else connected plus username and name.
//	@Tags			gitlab
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/gitlab/status [get]
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

// gitlabSaveTokenRequest is a personal access token; base_url defaults to the
// configured GitLab instance.
type gitlabSaveTokenRequest struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
}

// handleSaveToken saves a personal access token for the current user.
//
//	@Summary		Save the caller's GitLab token
//	@Description	Any role. The token is validated against GitLab, then stored encrypted. Answers {"status": "saved", "username"}.
//	@Tags			gitlab
//	@Accept			json
//	@Produce		json
//	@Param			body	body		gitlabSaveTokenRequest	true	"Personal access token"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Router			/api/gitlab/token [post]
func (h *gitlabHandlers) handleSaveToken(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req gitlabSaveTokenRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if req.Token == "" {
		jsonError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.BaseURL == "" {
		req.BaseURL = store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "auth_gitlab_base_url")
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
		jsonServerError(w, r, "encryption failed", err)
		return
	}

	t := &models.UserGitLabToken{
		UserID:            user.ID,
		GitLabBaseURL:     req.BaseURL,
		AccessTokenCipher: cipher,
		AccessTokenNonce:  nonce,
		GitLabUserID:      string(rune(glUser.ID)),
		GitLabUsername:    glUser.Username,
	}
	if err := store.NewUserGitLabTokenRepo(h.db.SQL).Upsert(r.Context(), t); err != nil {
		jsonServerError(w, r, "failed to save token", err)
		return
	}

	jsonOK(w, map[string]any{"status": "saved", "username": glUser.Username})
}

// handleDeleteToken removes the user's GitLab token.
//
//	@Summary		Remove the caller's GitLab token
//	@Description	Any role.
//	@Tags			gitlab
//	@Produce		json
//	@Success		200	{object}	StatusResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/gitlab/token [delete]
func (h *gitlabHandlers) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	baseURL := store.NewAppSettingsRepo(h.db.SQL).Value(r.Context(), "auth_gitlab_base_url")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	store.NewUserGitLabTokenRepo(h.db.SQL).Delete(r.Context(), user.ID, baseURL)
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleListCommits returns recent commits for a linked GitLab project.
//
//	@Summary		Recent commits of a project's linked GitLab repo
//	@Description	Any role. Uses the caller's own token; 20 most recent commits of the first link.
//	@Tags			gitlab
//	@Produce		json
//	@Param			id	path		int	true	"Project ID"
//	@Success		200	{array}		gitlab.Commit
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/gitlab/projects/{id}/commits [get]
func (h *gitlabHandlers) handleListCommits(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	link, err := store.NewProjectGitLabLinkRepo(h.db.SQL).GetFirst(r.Context(), projectID)
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
//
//	@Summary		GitLab issues of a project's linked repo
//	@Description	Any role. Uses the caller's own token; 20 per call.
//	@Tags			gitlab
//	@Produce		json
//	@Param			id		path		int		true	"Project ID"
//	@Param			state	query		string	false	"opened, closed or all"
//	@Success		200		{array}		gitlab.Issue
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/gitlab/projects/{id}/issues [get]
func (h *gitlabHandlers) handleListIssues(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	link, err := store.NewProjectGitLabLinkRepo(h.db.SQL).GetFirst(r.Context(), projectID)
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

// gitlabLinkProjectRequest names the GitLab project to link by path.
type gitlabLinkProjectRequest struct {
	GitLabPath string `json:"gitlab_path"` // e.g., "org/repo"
}

// handleLinkProject links an SSHCM project to a GitLab project.
//
//	@Summary		Link a project to a GitLab repo (caller's token)
//	@Description	Editor+. The path is resolved with the caller's token.
//	@Tags			gitlab
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Project ID"
//	@Param			body	body		gitlabLinkProjectRequest	true	"GitLab project path"
//	@Success		201		{object}	models.ProjectGitLabLink
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/gitlab/projects/{id}/link [post]
func (h *gitlabHandlers) handleLinkProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	var req gitlabLinkProjectRequest
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
	if err := store.NewProjectGitLabLinkRepo(h.db.SQL).Create(r.Context(), link); err != nil {
		jsonError(w, http.StatusConflict, "link already exists or failed to create")
		return
	}

	jsonCreated(w, link)
}

// registerRoutes binds the per-user GitLab token routes (profile-level).
func (h *gitlabHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/gitlab/status", h.handleStatus)
	rr.auth("POST /api/gitlab/token", h.handleSaveToken)
	rr.auth("DELETE /api/gitlab/token", h.handleDeleteToken)
	rr.auth("GET /api/gitlab/projects/{id}/commits", h.handleListCommits)
	rr.auth("GET /api/gitlab/projects/{id}/issues", h.handleListIssues)
	rr.role("editor", "POST /api/gitlab/projects/{id}/link", h.handleLinkProject)
}
