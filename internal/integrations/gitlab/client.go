package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// encodeNamespacedPath URL-encodes a GitLab namespaced path for use as a single
// path parameter. GitLab's API requires slashes to be encoded as %2F (url.PathEscape
// leaves them intact because it treats them as path separators), so we do it
// explicitly here. Used for /projects/:path and /groups/:path lookups.
func encodeNamespacedPath(p string) string {
	return strings.ReplaceAll(url.PathEscape(p), "/", "%2F")
}

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
	body, err := c.get(fmt.Sprintf("/projects/%s", encodeNamespacedPath(path)), nil)
	if err != nil {
		return nil, err
	}
	var project Project
	return &project, json.Unmarshal(body, &project)
}

// SearchGroupByPath finds a group by its full path (e.g., "org/subgroup").
func (c *Client) SearchGroupByPath(path string) (*Group, error) {
	body, err := c.get(fmt.Sprintf("/groups/%s", encodeNamespacedPath(path)), nil)
	if err != nil {
		return nil, err
	}
	var group Group
	return &group, json.Unmarshal(body, &group)
}

// GetGroup returns a group by numeric ID — used for health verification of stored group links.
func (c *Client) GetGroup(groupID int) (*Group, error) {
	body, err := c.get(fmt.Sprintf("/groups/%d", groupID), nil)
	if err != nil {
		return nil, err
	}
	var group Group
	return &group, json.Unmarshal(body, &group)
}

// ListGroupProjects returns projects under a group, optionally recursing into subgroups.
// Fetches up to 100 projects per page and stops at maxPages to keep latency bounded.
func (c *Client) ListGroupProjects(groupID int, includeSubgroups bool) ([]Project, error) {
	const maxPages = 5
	var all []Project
	for page := 1; page <= maxPages; page++ {
		q := url.Values{}
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		q.Set("simple", "true")
		q.Set("archived", "false")
		if includeSubgroups {
			q.Set("include_subgroups", "true")
		}

		body, err := c.get(fmt.Sprintf("/groups/%d/projects", groupID), q)
		if err != nil {
			return nil, err
		}
		var batch []Project
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

// ListCommits returns recent commits for a project.
// Pass params.All=true to include commits from every branch (default is the repo's default branch only).
func (c *Client) ListCommits(projectID int, params CommitListParams) ([]Commit, error) {
	q := url.Values{}
	if params.RefName != "" {
		q.Set("ref_name", params.RefName)
	} else if params.All {
		q.Set("all", "true")
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

// ListCommitRefs returns the branches (and/or tags) that contain a given commit.
// Pass refType = "branch" to limit the response to branches only.
func (c *Client) ListCommitRefs(projectID int, sha, refType string) ([]CommitRef, error) {
	q := url.Values{}
	if refType != "" {
		q.Set("type", refType)
	}
	q.Set("per_page", "20")
	body, err := c.get(fmt.Sprintf("/projects/%d/repository/commits/%s/refs", projectID, sha), q)
	if err != nil {
		return nil, err
	}
	var refs []CommitRef
	return refs, json.Unmarshal(body, &refs)
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
