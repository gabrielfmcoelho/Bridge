package models

import (
	"database/sql"
	"time"
)

// ProjectAIAnalysis is the cached markdown summary last generated for a project.
// There's exactly one row per project — regeneration overwrites in place.
type ProjectAIAnalysis struct {
	ProjectID    int64     `json:"project_id"`
	Content      string    `json:"content"`
	Locale       string    `json:"locale"`
	CommitsUsed  int       `json:"commits_used"`
	ReposUsed    int       `json:"repos_used"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// GetProjectAIAnalysis returns the cached analysis for a project, or nil if none exists.
func GetProjectAIAnalysis(db *sql.DB, projectID int64) (*ProjectAIAnalysis, error) {
	a := &ProjectAIAnalysis{}
	err := db.QueryRow(`
		SELECT project_id, content, locale, commits_used, repos_used, generated_at
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

// UpsertProjectAIAnalysis inserts or replaces the cached analysis for a project.
func UpsertProjectAIAnalysis(db *sql.DB, a *ProjectAIAnalysis) error {
	_, err := db.Exec(`
		INSERT INTO project_ai_analyses (project_id, content, locale, commits_used, repos_used, generated_at)
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
