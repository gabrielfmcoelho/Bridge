package models

import "time"

// ProjectEmbed is a BI or observability tool shown in an iframe on a
// project's Observabilidade tab.
type ProjectEmbed struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Height    int       `json:"height"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
