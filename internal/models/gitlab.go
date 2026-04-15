package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// UserGitLabToken stores an encrypted GitLab access token for a user.
type UserGitLabToken struct {
	ID                  int64      `json:"id"`
	UserID              int64      `json:"user_id"`
	GitLabBaseURL       string     `json:"gitlab_base_url"`
	AccessTokenCipher   []byte     `json:"-"`
	AccessTokenNonce    []byte     `json:"-"`
	RefreshTokenCipher  []byte     `json:"-"`
	RefreshTokenNonce   []byte     `json:"-"`
	GitLabUserID        string     `json:"gitlab_user_id"`
	GitLabUsername       string     `json:"gitlab_username"`
	ExpiresAt           *time.Time `json:"expires_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func UpsertUserGitLabToken(db *sql.DB, t *UserGitLabToken) error {
	_, err := db.Exec(`
		INSERT INTO user_gitlab_tokens (user_id, gitlab_base_url, access_token_cipher, access_token_nonce,
			refresh_token_cipher, refresh_token_nonce, gitlab_user_id, gitlab_username, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, gitlab_base_url) DO UPDATE SET
			access_token_cipher = excluded.access_token_cipher,
			access_token_nonce = excluded.access_token_nonce,
			refresh_token_cipher = excluded.refresh_token_cipher,
			refresh_token_nonce = excluded.refresh_token_nonce,
			gitlab_user_id = excluded.gitlab_user_id,
			gitlab_username = excluded.gitlab_username,
			expires_at = excluded.expires_at,
			updated_at = CURRENT_TIMESTAMP`,
		t.UserID, t.GitLabBaseURL, t.AccessTokenCipher, t.AccessTokenNonce,
		t.RefreshTokenCipher, t.RefreshTokenNonce, t.GitLabUserID, t.GitLabUsername, t.ExpiresAt,
	)
	return err
}

func GetUserGitLabToken(db *sql.DB, userID int64, baseURL string) (*UserGitLabToken, error) {
	t := &UserGitLabToken{}
	err := db.QueryRow(`
		SELECT id, user_id, gitlab_base_url, access_token_cipher, access_token_nonce,
			refresh_token_cipher, refresh_token_nonce, gitlab_user_id, gitlab_username,
			expires_at, created_at, updated_at
		FROM user_gitlab_tokens WHERE user_id = ? AND gitlab_base_url = ?`,
		userID, baseURL,
	).Scan(&t.ID, &t.UserID, &t.GitLabBaseURL, &t.AccessTokenCipher, &t.AccessTokenNonce,
		&t.RefreshTokenCipher, &t.RefreshTokenNonce, &t.GitLabUserID, &t.GitLabUsername,
		&t.ExpiresAt, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func DeleteUserGitLabToken(db *sql.DB, userID int64, baseURL string) error {
	_, err := db.Exec(`DELETE FROM user_gitlab_tokens WHERE user_id = ? AND gitlab_base_url = ?`, userID, baseURL)
	return err
}

// ProjectGitLabLink links an SSHCM project to a GitLab project.
type ProjectGitLabLink struct {
	ID              int64      `json:"id"`
	ProjectID       int64      `json:"project_id"`
	GitLabProjectID int        `json:"gitlab_project_id"`
	GitLabBaseURL   string     `json:"gitlab_base_url"`
	GitLabPath      string     `json:"gitlab_path"`
	SyncIssues      bool       `json:"sync_issues"`
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func CreateProjectGitLabLink(db *sql.DB, l *ProjectGitLabLink) error {
	id, err := database.InsertReturningID(db, `
		INSERT INTO project_gitlab_links (project_id, gitlab_project_id, gitlab_base_url, gitlab_path, sync_issues)
		VALUES (?, ?, ?, ?, ?)`,
		l.ProjectID, l.GitLabProjectID, l.GitLabBaseURL, l.GitLabPath, l.SyncIssues,
	)
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

func GetProjectGitLabLink(db *sql.DB, projectID int64) (*ProjectGitLabLink, error) {
	l := &ProjectGitLabLink{}
	err := db.QueryRow(`
		SELECT id, project_id, gitlab_project_id, gitlab_base_url, gitlab_path, sync_issues, last_synced_at, created_at
		FROM project_gitlab_links WHERE project_id = ?`, projectID,
	).Scan(&l.ID, &l.ProjectID, &l.GitLabProjectID, &l.GitLabBaseURL, &l.GitLabPath, &l.SyncIssues, &l.LastSyncedAt, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

func DeleteProjectGitLabLink(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM project_gitlab_links WHERE id = ?`, id)
	return err
}
