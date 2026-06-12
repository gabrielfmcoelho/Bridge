package models

import "time"

// UserGitLabToken stores an encrypted GitLab access token for a user.
// Persistence lives in internal/store.UserGitLabTokenRepo — this file is the
// pure data types + constants only.
type UserGitLabToken struct {
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	GitLabBaseURL      string     `json:"gitlab_base_url"`
	AccessTokenCipher  []byte     `json:"-"`
	AccessTokenNonce   []byte     `json:"-"`
	RefreshTokenCipher []byte     `json:"-"`
	RefreshTokenNonce  []byte     `json:"-"`
	GitLabUserID       string     `json:"gitlab_user_id"`
	GitLabUsername     string     `json:"gitlab_username"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Link kinds stored in project_gitlab_links.kind.
const (
	GitLabLinkKindProject = "project"
	GitLabLinkKindGroup   = "group"
)

// ProjectGitLabLink links an SSHCM project to either a single GitLab project
// or a whole GitLab group (expanded to all repos at read time).
// When Kind == "group", GitLabProjectID holds the group ID.
type ProjectGitLabLink struct {
	ID              int64      `json:"id"`
	ProjectID       int64      `json:"project_id"`
	GitLabProjectID int        `json:"gitlab_project_id"`
	GitLabBaseURL   string     `json:"gitlab_base_url"`
	GitLabPath      string     `json:"gitlab_path"`
	Kind            string     `json:"kind"`
	RefName         string     `json:"ref_name"`
	DisplayName     string     `json:"display_name"`
	SyncIssues      bool       `json:"sync_issues"`
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	CreatedAt       time.Time  `json:"created_at"`
}
