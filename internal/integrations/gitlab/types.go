package gitlab

import "time"

// GitLabUser represents a GitLab user from /api/v4/user.
type GitLabUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// Project represents a GitLab project.
type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	Description       string `json:"description"`
	DefaultBranch     string `json:"default_branch"`
}

// Commit represents a GitLab commit.
type Commit struct {
	ID             string    `json:"id"`
	ShortID        string    `json:"short_id"`
	Title          string    `json:"title"`
	Message        string    `json:"message"`
	AuthorName     string    `json:"author_name"`
	AuthorEmail    string    `json:"author_email"`
	CommittedDate  time.Time `json:"committed_date"`
	WebURL         string    `json:"web_url"`
}

// Issue represents a GitLab issue.
type Issue struct {
	ID          int       `json:"id"`
	IID         int       `json:"iid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	WebURL      string    `json:"web_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Labels      []string  `json:"labels"`
	Author      struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
}

// CommitListParams are query parameters for listing commits.
type CommitListParams struct {
	RefName string
	PerPage int
	Page    int
}

// IssueListParams are query parameters for listing issues.
type IssueListParams struct {
	State   string // opened, closed, all
	PerPage int
	Page    int
}
