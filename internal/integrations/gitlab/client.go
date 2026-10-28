package gitlab

import (
	"encoding/json"
	"errors"
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

// maxBody caps one API response read into memory.
const maxBody = 8 << 20

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

// StatusError is a GitLab API response with status >= 400.
type StatusError struct {
	Code       int
	Path, Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("gitlab api %s: %d %s", e.Path, e.Code, e.Body)
}

func (c *Client) get(path string, params url.Values) ([]byte, error) {
	body, _, err := c.getResp(path, params)
	return body, err
}

func (c *Client) getResp(path string, params url.Values) ([]byte, http.Header, error) {
	u := fmt.Sprintf("%s/api/v4%s", c.baseURL, path)
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	// ponytail: 8 MiB covers any manifest or source file the API returns; a
	// bigger body is cut, not streamed.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, nil, &StatusError{resp.StatusCode, path, string(body)}
	}
	return body, resp.Header, nil
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

// maxListPages bounds ListGroups / ListProjects: 2000 entries each.
const maxListPages = 20

// listAll walks a paged list endpoint 100 at a time. truncated reports that
// the cap stopped it before the last page.
func listAll[T any](c *Client, path string, q url.Values) (all []T, truncated bool, err error) {
	q.Set("per_page", "100")
	for page := 1; page <= maxListPages; page++ {
		q.Set("page", strconv.Itoa(page))
		body, err := c.get(path, q)
		if err != nil {
			return nil, false, err
		}
		var batch []T
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, false, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			return all, false, nil
		}
	}
	return all, true, nil
}

// ListGroups returns every group and subgroup the token can see as a member
// (guest or above), with parent_id to rebuild the tree.
func (c *Client) ListGroups() ([]Group, bool, error) {
	return listAll[Group](c, "/groups", url.Values{"min_access_level": {"10"}, "order_by": {"path"}})
}

// ListProjects returns every non-archived project the token is a member of.
func (c *Client) ListProjects() ([]Project, bool, error) {
	return listAll[Project](c, "/projects", url.Values{"membership": {"true"}, "simple": {"true"}, "archived": {"false"}, "order_by": {"path"}, "sort": {"asc"}})
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

// ListTree lists a repository directory ("" = root) at ref ("" = the
// default branch). recursive walks every subdirectory; at most
// maxTreePages × 100 entries come back, enough to find files by name.
func (c *Client) ListTree(projectID int, ref, path string, recursive bool) ([]TreeEntry, error) {
	const maxTreePages = 10
	var all []TreeEntry
	for page := 1; page <= maxTreePages; page++ {
		q := url.Values{}
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		if ref != "" {
			q.Set("ref", ref)
		}
		if path != "" {
			q.Set("path", path)
		}
		if recursive {
			q.Set("recursive", "true")
		}
		body, err := c.get(fmt.Sprintf("/projects/%d/repository/tree", projectID), q)
		if err != nil {
			return nil, err
		}
		var batch []TreeEntry
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

// GetRawFile returns a file's content at ref ("" = HEAD).
func (c *Client) GetRawFile(projectID int, filePath, ref string) ([]byte, error) {
	if ref == "" {
		ref = "HEAD"
	}
	return c.get(fmt.Sprintf("/projects/%d/repository/files/%s/raw", projectID, encodeNamespacedPath(filePath)),
		url.Values{"ref": {ref}})
}

// SearchBlobs searches file contents in one project at ref ("" = the
// default branch). Needs the instance's code search (basic search works).
func (c *Client) SearchBlobs(projectID int, query, ref string) ([]BlobHit, error) {
	q := url.Values{}
	q.Set("scope", "blobs")
	q.Set("search", query)
	q.Set("per_page", "20")
	if ref != "" {
		q.Set("ref", ref)
	}
	body, err := c.get(fmt.Sprintf("/projects/%d/search", projectID), q)
	if err != nil {
		return nil, err
	}
	var hits []BlobHit
	return hits, json.Unmarshal(body, &hits)
}

// maxFullPages is the safety cap of listEvery: 200 pages = 20k items.
const maxFullPages = 200

// listEvery walks a paged endpoint to the end; unlike listAll it errors,
// instead of truncating, when the safety cap is hit.
func listEvery[T any](c *Client, path string, q url.Values) ([]T, error) {
	q.Set("per_page", "100")
	var all []T
	for page := 1; page <= maxFullPages; page++ {
		q.Set("page", strconv.Itoa(page))
		body, h, err := c.getResp(path, q)
		if err != nil {
			return nil, err
		}
		var batch []T
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if h.Get("X-Next-Page") == "" {
			return all, nil
		}
	}
	return nil, fmt.Errorf("gitlab api %s: page cap of %d reached, result would be truncated", path, maxFullPages)
}

// ListGroupProjectsAll returns every project of a group (numeric id or full
// path) including subgroups, with no page cap beyond the safety limit.
// Archived projects are included only when includeArchived.
func (c *Client) ListGroupProjectsAll(group string, includeArchived bool) ([]Project, error) {
	q := url.Values{}
	q.Set("include_subgroups", "true")
	q.Set("simple", "true")
	if !includeArchived {
		q.Set("archived", "false")
	}
	return listEvery[Project](c, "/groups/"+encodeNamespacedPath(group)+"/projects", q)
}

// ListContributors returns the repository contributors of a project. A 404
// (repository without default branch) means no contributors.
func (c *Client) ListContributors(projectID int) ([]Contributor, error) {
	out, err := listEvery[Contributor](c, fmt.Sprintf("/projects/%d/repository/contributors", projectID), url.Values{})
	var se *StatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return []Contributor{}, nil
	}
	return out, err
}
