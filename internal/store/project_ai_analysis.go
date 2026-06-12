package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ProjectAIAnalysisRepo owns SQL for project_ai_analyses (one cached LLM summary
// per project; regeneration overwrites in place).
type ProjectAIAnalysisRepo struct {
	db *sql.DB
}

// NewProjectAIAnalysisRepo constructs a ProjectAIAnalysisRepo over the DB handle.
func NewProjectAIAnalysisRepo(db *sql.DB) *ProjectAIAnalysisRepo {
	return &ProjectAIAnalysisRepo{db: db}
}

// Get returns the cached analysis for a project, or (nil, nil) if none exists.
func (r *ProjectAIAnalysisRepo) Get(ctx context.Context, projectID int64) (*models.ProjectAIAnalysis, error) {
	a := &models.ProjectAIAnalysis{}
	err := r.db.QueryRowContext(ctx,
		`SELECT project_id, content, locale, commits_used, repos_used, generated_at
		 FROM project_ai_analyses WHERE project_id = ?`, projectID,
	).Scan(&a.ProjectID, &a.Content, &a.Locale, &a.CommitsUsed, &a.ReposUsed, &a.GeneratedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Upsert inserts or replaces the cached analysis for a project, stamping
// generated_at to now.
func (r *ProjectAIAnalysisRepo) Upsert(ctx context.Context, a *models.ProjectAIAnalysis) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_ai_analyses (project_id, content, locale, commits_used, repos_used, generated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(project_id) DO UPDATE SET
			content = excluded.content,
			locale = excluded.locale,
			commits_used = excluded.commits_used,
			repos_used = excluded.repos_used,
			generated_at = CURRENT_TIMESTAMP`,
		a.ProjectID, a.Content, a.Locale, a.CommitsUsed, a.ReposUsed,
	)
	return err
}
