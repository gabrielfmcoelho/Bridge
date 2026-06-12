package models

import "time"

// ProjectAIAnalysis is the cached markdown summary last generated for a project.
// There's exactly one row per project — regeneration overwrites in place.
// Persistence lives in internal/store.ProjectAIAnalysisRepo — this file is the
// pure data type only.
type ProjectAIAnalysis struct {
	ProjectID   int64     `json:"project_id"`
	Content     string    `json:"content"`
	Locale      string    `json:"locale"`
	CommitsUsed int       `json:"commits_used"`
	ReposUsed   int       `json:"repos_used"`
	GeneratedAt time.Time `json:"generated_at"`
}
