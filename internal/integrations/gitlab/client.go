package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client interacts with the GitLab REST API v4.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a GitLab API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) get(path string, params url.Values) ([]byte, error) {
	u := fmt.Sprintf("%s/api/v4%s", c.baseURL, path)
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitlab api %s: %d %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}

// GetCurrentUser returns the authenticated user.
func (c *Client) GetCurrentUser() (*GitLabUser, error) {
	body, err := c.get("/user", nil)
	if err != nil {
		return nil, err
	}
	var user GitLabUser
	return &user, json.Unmarshal(body, &user)
}

// GetProject returns a project by ID.
func (c *Client) GetProject(projectID int) (*Project, error) {
	body, err := c.get(fmt.Sprintf("/projects/%d", projectID), nil)
	if err != nil {
		return nil, err
	}
	var project Project
	return &project, json.Unmarshal(body, &project)
}

// SearchProjectByPath finds a project by its path (e.g., "org/repo").
func (c *Client) SearchProjectByPath(path string) (*Project, error) {
	encoded := url.PathEscape(path)
	body, err := c.get(fmt.Sprintf("/projects/%s", encoded), nil)
	if err != nil {
		return nil, err
	}
	var project Project
	return &project, json.Unmarshal(body, &project)
}

// ListCommits returns recent commits for a project.
func (c *Client) ListCommits(projectID int, params CommitListParams) ([]Commit, error) {
	q := url.Values{}
	if params.RefName != "" {
		q.Set("ref_name", params.RefName)
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	} else {
		q.Set("per_page", "20")
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}

	body, err := c.get(fmt.Sprintf("/projects/%d/repository/commits", projectID), q)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	return commits, json.Unmarshal(body, &commits)
}

// ListIssues returns issues for a project.
func (c *Client) ListIssues(projectID int, params IssueListParams) ([]Issue, error) {
	q := url.Values{}
	if params.State != "" {
		q.Set("state", params.State)
	} else {
		q.Set("state", "opened")
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	} else {
		q.Set("per_page", "20")
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	q.Set("order_by", "updated_at")
	q.Set("sort", "desc")

	body, err := c.get(fmt.Sprintf("/projects/%d/issues", projectID), q)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	return issues, json.Unmarshal(body, &issues)
}
