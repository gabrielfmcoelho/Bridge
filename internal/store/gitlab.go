package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// UserGitLabTokenRepo owns SQL for user_gitlab_tokens (per-user encrypted
// GitLab access tokens).
type UserGitLabTokenRepo struct {
	db *sql.DB
}

// NewUserGitLabTokenRepo constructs the repo over the given DB handle.
func NewUserGitLabTokenRepo(db *sql.DB) *UserGitLabTokenRepo { return &UserGitLabTokenRepo{db: db} }

// Upsert inserts or updates the token for (user_id, gitlab_base_url).
func (r *UserGitLabTokenRepo) Upsert(ctx context.Context, t *models.UserGitLabToken) error {
	_, err := r.db.ExecContext(ctx, `
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

// Get returns the token for (user_id, gitlab_base_url), or (nil, nil).
func (r *UserGitLabTokenRepo) Get(ctx context.Context, userID int64, baseURL string) (*models.UserGitLabToken, error) {
	t := &models.UserGitLabToken{}
	err := r.db.QueryRowContext(ctx, `
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

// Delete removes the token for (user_id, gitlab_base_url).
func (r *UserGitLabTokenRepo) Delete(ctx context.Context, userID int64, baseURL string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_gitlab_tokens WHERE user_id = ? AND gitlab_base_url = ?`, userID, baseURL)
	return err
}

// ProjectGitLabLinkRepo owns SQL for project_gitlab_links (SSHCM project ↔
// GitLab project/group bindings).
type ProjectGitLabLinkRepo struct {
	db *sql.DB
}

// NewProjectGitLabLinkRepo constructs the repo over the given DB handle.
func NewProjectGitLabLinkRepo(db *sql.DB) *ProjectGitLabLinkRepo { return &ProjectGitLabLinkRepo{db: db} }

const projectGitLabLinkCols = `id, project_id, gitlab_project_id, gitlab_base_url, gitlab_path,
	kind, ref_name, display_name, sync_issues, last_synced_at, created_at`

func scanProjectGitLabLink(scanner interface{ Scan(...any) error }, l *models.ProjectGitLabLink) error {
	return scanner.Scan(&l.ID, &l.ProjectID, &l.GitLabProjectID, &l.GitLabBaseURL, &l.GitLabPath,
		&l.Kind, &l.RefName, &l.DisplayName, &l.SyncIssues, &l.LastSyncedAt, &l.CreatedAt)
}

// Create inserts a link (defaulting Kind to project) and sets l.ID.
func (r *ProjectGitLabLinkRepo) Create(ctx context.Context, l *models.ProjectGitLabLink) error {
	if l.Kind == "" {
		l.Kind = models.GitLabLinkKindProject
	}
	id, err := database.InsertReturningID(r.db, `
		INSERT INTO project_gitlab_links
			(project_id, gitlab_project_id, gitlab_base_url, gitlab_path, kind, ref_name, display_name, sync_issues)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ProjectID, l.GitLabProjectID, l.GitLabBaseURL, l.GitLabPath,
		l.Kind, l.RefName, l.DisplayName, l.SyncIssues,
	)
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

// GetFirst returns the first link for a project (legacy single-link callers).
func (r *ProjectGitLabLinkRepo) GetFirst(ctx context.Context, projectID int64) (*models.ProjectGitLabLink, error) {
	l := &models.ProjectGitLabLink{}
	err := scanProjectGitLabLink(r.db.QueryRowContext(ctx,
		`SELECT `+projectGitLabLinkCols+` FROM project_gitlab_links WHERE project_id = ? ORDER BY id ASC LIMIT 1`, projectID), l)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

// List returns every link attached to a project.
func (r *ProjectGitLabLinkRepo) List(ctx context.Context, projectID int64) ([]models.ProjectGitLabLink, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+projectGitLabLinkCols+` FROM project_gitlab_links WHERE project_id = ? ORDER BY id ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []models.ProjectGitLabLink
	for rows.Next() {
		var l models.ProjectGitLabLink
		if err := scanProjectGitLabLink(rows, &l); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// DeleteByID removes a link only if it belongs to the given project (prevents
// cross-project id tampering). Returns whether a row was deleted.
func (r *ProjectGitLabLinkRepo) DeleteByID(ctx context.Context, linkID, projectID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM project_gitlab_links WHERE id = ? AND project_id = ?`, linkID, projectID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
