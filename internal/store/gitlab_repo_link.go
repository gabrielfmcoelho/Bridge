package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// GitLabRepoLinkRepo owns SQL for gitlab_repo_links (GitLab repo ↔ DNS record
// or host, or GitLab group ↔ host). Reads join the live parent, so links of a trashed record are
// hidden until it is restored.
type GitLabRepoLinkRepo struct {
	db *sql.DB
}

// NewGitLabRepoLinkRepo constructs the repo over the given DB handle.
func NewGitLabRepoLinkRepo(db *sql.DB) *GitLabRepoLinkRepo {
	return &GitLabRepoLinkRepo{db: db}
}

const gitlabRepoLinkSelect = `SELECT l.id, l.entity_type, l.entity_id, COALESCE(d.domain, h.nickname, ''), l.kind,
		l.gitlab_project_id, l.gitlab_base_url, l.gitlab_path, l.created_at
	FROM gitlab_repo_links l
	LEFT JOIN dns_records d ON l.entity_type = 'dns' AND d.id = l.entity_id AND d.deleted_at IS NULL
	LEFT JOIN hosts h ON l.entity_type = 'host' AND h.id = l.entity_id AND h.deleted_at IS NULL
	WHERE (d.id IS NOT NULL OR h.id IS NOT NULL)`

func (r *GitLabRepoLinkRepo) query(ctx context.Context, where string, args ...any) ([]models.GitLabRepoLink, error) {
	rows, err := r.db.QueryContext(ctx, gitlabRepoLinkSelect+where+` ORDER BY l.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.GitLabRepoLink
	for rows.Next() {
		var l models.GitLabRepoLink
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.EntityName, &l.Kind,
			&l.GitLabProjectID, &l.GitLabBaseURL, &l.GitLabPath, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// List returns every link whose DNS record / host is live. Unscoped: the
// settings inventory is admin-only.
func (r *GitLabRepoLinkRepo) List(ctx context.Context) ([]models.GitLabRepoLink, error) {
	return r.query(ctx, "")
}

// ListFor returns the links of the given entities (entityType "dns" or "host").
// The caller has already checked the entities are visible.
func (r *GitLabRepoLinkRepo) ListFor(ctx context.Context, entityType string, ids []int64) ([]models.GitLabRepoLink, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.query(ctx, ` AND l.entity_type = ? AND l.entity_id = ANY(?)`, entityType, ids)
}

// Create inserts a link and sets l.ID. A duplicate violates the UNIQUE
// constraint and returns its error.
func (r *GitLabRepoLinkRepo) Create(ctx context.Context, l *models.GitLabRepoLink, createdBy *int64) error {
	if l.Kind == "" {
		l.Kind = models.GitLabLinkKindProject // a single repo unless said otherwise
	}
	id, err := database.InsertReturningID(r.db, `
		INSERT INTO gitlab_repo_links (entity_type, entity_id, kind, gitlab_project_id, gitlab_base_url, gitlab_path, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.EntityType, l.EntityID, l.Kind, l.GitLabProjectID, l.GitLabBaseURL, l.GitLabPath, createdBy)
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

// Delete removes a link; reports whether it existed.
func (r *GitLabRepoLinkRepo) Delete(ctx context.Context, id int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM gitlab_repo_links WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
