package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"

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

const gitlabRepoLinkSelect = `SELECT l.id, l.entity_type, l.entity_id, COALESCE(d.domain, h.nickname, ''), COALESCE(h.oficial_slug, ''), COALESCE(h.hostname, ''), l.kind,
		l.gitlab_project_id, l.gitlab_base_url, l.gitlab_path, l.source, l.created_at
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
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.EntityName, &l.EntitySlug, &l.EntityAddress, &l.Kind,
			&l.GitLabProjectID, &l.GitLabBaseURL, &l.GitLabPath, &l.Source, &l.CreatedAt); err != nil {
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

// SyncScanLinks makes the host's scan links (source 'scan', GitLab id 0,
// keyed by lowercase path) on baseURL match the repo paths its latest scan
// found: adds the missing ones, skipping a repo the host already links by
// hand, and, when prune (the scan's list is complete), deletes the scan
// links whose repo is gone. Manual links and other GitLabs are never touched.
func (r *GitLabRepoLinkRepo) SyncScanLinks(ctx context.Context, hostID int64, baseURL string, paths []string, prune bool) (added, removed int, err error) {
	baseURL = strings.TrimRight(baseURL, "/")
	keys, found := []string{}, map[string]string{}
	for _, p := range paths {
		if k := strings.ToLower(p); found[k] == "" {
			found[k] = p
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	const sameHost = `entity_type = 'host' AND entity_id = ? AND lower(rtrim(gitlab_base_url, '/')) = lower(?)`
	if prune {
		res, err := tx.ExecContext(ctx, `DELETE FROM gitlab_repo_links WHERE `+sameHost+`
			AND source = 'scan' AND NOT (lower(gitlab_path) = ANY(?))`, hostID, baseURL, keys)
		if err != nil {
			return 0, 0, err
		}
		n, _ := res.RowsAffected()
		removed = int(n)
	}
	for _, k := range keys {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO gitlab_repo_links (entity_type, entity_id, kind, gitlab_project_id, gitlab_base_url, gitlab_path, source)
			SELECT 'host', ?::bigint, 'project', 0, ?::text, ?::text, 'scan'
			WHERE NOT EXISTS (SELECT 1 FROM gitlab_repo_links WHERE `+sameHost+` AND kind = 'project' AND lower(gitlab_path) = ?)
			ON CONFLICT DO NOTHING`, hostID, baseURL, found[k], hostID, baseURL, k)
		if err != nil {
			return 0, 0, err
		}
		n, _ := res.RowsAffected()
		added += int(n)
	}
	return added, removed, tx.Commit()
}
