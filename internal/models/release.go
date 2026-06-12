package models

import "time"

// Release is a planned project release/milestone with linked issues
// (release_issues). Persistence lives in internal/store.ReleaseRepo — this file
// is the pure data type only.
type Release struct {
	ID          int64     `json:"id"`
	ProjectID   *int64    `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	TargetDate  string    `json:"target_date"`
	LiveDate    string    `json:"live_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
