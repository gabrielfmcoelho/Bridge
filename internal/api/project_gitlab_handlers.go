package api

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type projectGitLabHandlers struct {
	db *database.DB
}

// resolveClient loads the shared GitLab settings and returns a ready-to-use client,
// plus the resolved settings. Returns (nil, settings, nil) when not configured —
// callers decide whether to 404 or return an empty envelope.
func (h *projectGitLabHandlers) resolveClient() (*gitlabclient.Client, gitlabclient.Settings, error) {
	settings, err := gitlabclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		return nil, settings, err
	}
	client := gitlabclient.NewServiceClient(settings)
	return client, settings, nil
}

// linkWithHealth augments a stored link with live reachability info from GitLab.
type linkWithHealth struct {
	models.ProjectGitLabLink
	Reachable   *bool  `json:"reachable,omitempty"`
	HealthError string `json:"health_error,omitempty"`
}

type linksEnvelope struct {
	Enabled    bool             `json:"enabled"`
	Configured bool             `json:"configured"`
	Links      []linkWithHealth `json:"links"`
}

// handleListLinks returns every GitLab link attached to a project, along with the
// integration's active status and a live reachability check for each link (one
// GitLab API call per link, fanned out with bounded concurrency).
func (h *projectGitLabHandlers) handleListLinks(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	raw, err := models.ListProjectGitLabLinks(h.db.SQL, projectID)
	if err != nil {
		jsonServerError(w, r, "failed to list gitlab links", err)
		return
	}

	settings, err := gitlabclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load gitlab settings", err)
		return
	}

	env := linksEnvelope{
		Enabled:    settings.Enabled,
		Configured: settings.ServiceToken != "",
		Links:      make([]linkWithHealth, len(raw)),
	}
	for i, l := range raw {
		env.Links[i] = linkWithHealth{ProjectGitLabLink: l}
	}

	// Only hit GitLab if we're enabled AND have a token — otherwise the reachability
	// field stays nil (unknown) and the UI will fall back on the enabled/configured flags.
	client := gitlabclient.NewServiceClient(settings)
	if client != nil && env.Enabled && len(env.Links) > 0 {
		const maxConcurrency = 5
		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup
		for i := range env.Links {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int) {
				defer wg.Done()
				defer func() { <-sem }()
				link := env.Links[idx]
				var err error
				if link.Kind == models.GitLabLinkKindGroup {
					_, err = client.GetGroup(link.GitLabProjectID)
				} else {
					_, err = client.GetProject(link.GitLabProjectID)
				}
				ok := err == nil
				env.Links[idx].Reachable = &ok
				if err != nil {
					env.Links[idx].HealthError = err.Error()
				}
			}(i)
		}
		wg.Wait()
	}

	jsonOK(w, env)
}

// handleCreateLink attaches a GitLab project or subgroup to an SSHCM project.
// The client sends a path ("org/repo" or "org/subgroup") plus a kind.
// The backend resolves the path to a stable numeric ID before persisting.
func (h *projectGitLabHandlers) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	var req struct {
		Kind    string `json:"kind"`     // "project" (default) or "group"
		Path    string `json:"path"`     // "org/repo" or "org/subgroup"
		RefName string `json:"ref_name"` // optional branch override
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	rawPath := req.Path
	req.Path = strings.TrimSpace(strings.Trim(req.Path, "/"))
	if req.Path == "" {
		jsonError(w, http.StatusBadRequest, "path is required")
		return
	}
	log.Printf("[gitlab-links] create project=%d kind=%s raw=%q cleaned=%q", projectID, req.Kind, rawPath, req.Path)
	if req.Kind == "" {
		req.Kind = models.GitLabLinkKindProject
	}
	if req.Kind != models.GitLabLinkKindProject && req.Kind != models.GitLabLinkKindGroup {
		jsonError(w, http.StatusBadRequest, "kind must be 'project' or 'group'")
		return
	}

	client, settings, err := h.resolveClient()
	if err != nil {
		jsonServerError(w, r, "failed to load gitlab settings", err)
		return
	}
	if client == nil {
		jsonError(w, http.StatusBadRequest, "GitLab service token not configured")
		return
	}

	link := &models.ProjectGitLabLink{
		ProjectID:     projectID,
		GitLabBaseURL: settings.BaseURL,
		Kind:          req.Kind,
		RefName:       strings.TrimSpace(req.RefName),
	}

	if req.Kind == models.GitLabLinkKindGroup {
		group, err := client.SearchGroupByPath(req.Path)
		if err != nil {
			log.Printf("[gitlab-links] SearchGroupByPath(%q) failed: %v", req.Path, err)
			jsonError(w, http.StatusNotFound, "GitLab group not found: "+err.Error())
			return
		}
		link.GitLabProjectID = group.ID
		link.GitLabPath = group.FullPath
		link.DisplayName = group.Name
		log.Printf("[gitlab-links] resolved group id=%d path=%q name=%q", group.ID, group.FullPath, group.Name)
	} else {
		project, err := client.SearchProjectByPath(req.Path)
		if err != nil {
			log.Printf("[gitlab-links] SearchProjectByPath(%q) failed: %v", req.Path, err)
			jsonError(w, http.StatusNotFound, "GitLab project not found: "+err.Error())
			return
		}
		link.GitLabProjectID = project.ID
		link.GitLabPath = project.PathWithNamespace
		link.DisplayName = project.Name
		log.Printf("[gitlab-links] resolved project id=%d path=%q name=%q", project.ID, project.PathWithNamespace, project.Name)
	}

	if err := models.CreateProjectGitLabLink(h.db.SQL, link); err != nil {
		log.Printf("[gitlab-links] create db insert failed project=%d gitlab_id=%d path=%q: %v", projectID, link.GitLabProjectID, link.GitLabPath, err)
		jsonError(w, http.StatusConflict, "link already exists or failed to create")
		return
	}
	jsonCreated(w, link)
}

// handleDeleteLink removes a link, enforcing that it belongs to the URL's project.
func (h *projectGitLabHandlers) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	linkID, err := pathInt64(r, "linkId")
	if err != nil {
		jsonBadRequest(w, r, "invalid link id", err)
		return
	}
	ok, err := models.DeleteProjectGitLabLinkByID(h.db.SQL, linkID, projectID)
	if err != nil {
		jsonServerError(w, r, "failed to delete link", err)
		return
	}
	if !ok {
		jsonError(w, http.StatusNotFound, "link not found")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// commitEnvelope is the response shape for the commits tab.
type commitEnvelope struct {
	Enabled    bool             `json:"enabled"`
	Configured bool             `json:"configured"`
	Commits    []enrichedCommit `json:"commits"`
	Warnings   []string         `json:"warnings,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type enrichedCommit struct {
	gitlabclient.Commit
	ProjectID   int      `json:"source_project_id"`
	ProjectName string   `json:"source_project_name"`
	ProjectPath string   `json:"source_project_path"`
	Branches    []string `json:"branches,omitempty"`
}

// commitsPerRepo is the fixed number of recent commits fetched for each linked repo.
// The tab groups commits by repo, so this caps the per-group list size.
const commitsPerRepo = 10

// handleListCommits returns up to `commitsPerRepo` recent commits for every repo
// attached to the SSHCM project, expanding subgroup links into their member repos.
// Each returned commit is enriched with the branches that contain it (one extra
// API call per commit, fanned out with bounded concurrency).
func (h *projectGitLabHandlers) handleListCommits(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}

	env := commitEnvelope{Commits: []enrichedCommit{}}

	settings, err := gitlabclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load gitlab settings", err)
		return
	}
	env.Enabled = settings.Enabled
	env.Configured = settings.ServiceToken != ""
	if !env.Enabled || !env.Configured {
		jsonOK(w, env)
		return
	}

	client := gitlabclient.NewServiceClient(settings)
	if client == nil {
		jsonOK(w, env)
		return
	}

	links, err := models.ListProjectGitLabLinks(h.db.SQL, projectID)
	if err != nil {
		jsonServerError(w, r, "failed to list gitlab links", err)
		return
	}
	if len(links) == 0 {
		jsonOK(w, env)
		return
	}

	// Resolve group links into their member projects. Each target is one repo that
	// will be fetched with its own commits-list call.
	type target struct {
		ID   int
		Path string
		Name string
		Ref  string
	}
	var targets []target
	var warnings []string

	for _, l := range links {
		switch l.Kind {
		case models.GitLabLinkKindGroup:
			projects, err := client.ListGroupProjects(l.GitLabProjectID, true)
			if err != nil {
				warnings = append(warnings, "group "+l.GitLabPath+": "+err.Error())
				continue
			}
			for _, p := range projects {
				targets = append(targets, target{
					ID: p.ID, Path: p.PathWithNamespace, Name: p.Name,
					Ref: strOr(l.RefName, settings.DefaultRef),
				})
			}
		default: // project
			targets = append(targets, target{
				ID: l.GitLabProjectID, Path: l.GitLabPath, Name: pickName(l.DisplayName, l.GitLabPath),
				Ref: strOr(l.RefName, settings.DefaultRef),
			})
		}
	}

	if len(targets) == 0 {
		env.Warnings = warnings
		jsonOK(w, env)
		return
	}

	// Fan-out stage 1: fetch up to commitsPerRepo commits per repo with bounded concurrency.
	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	all := make([]enrichedCommit, 0, len(targets)*commitsPerRepo)

	for _, tgt := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t target) {
			defer wg.Done()
			defer func() { <-sem }()

			// When no branch override is set, fetch commits across every branch via all=true.
			batch, err := client.ListCommits(t.ID, gitlabclient.CommitListParams{
				RefName: t.Ref,
				All:     t.Ref == "",
				PerPage: commitsPerRepo,
			})
			if err != nil {
				mu.Lock()
				warnings = append(warnings, t.Path+": "+err.Error())
				mu.Unlock()
				return
			}
			enriched := make([]enrichedCommit, 0, len(batch))
			for _, c := range batch {
				enriched = append(enriched, enrichedCommit{
					Commit:      c,
					ProjectID:   t.ID,
					ProjectName: t.Name,
					ProjectPath: t.Path,
				})
			}
			mu.Lock()
			all = append(all, enriched...)
			mu.Unlock()
		}(tgt)
	}
	wg.Wait()

	// Fan-out stage 2: enrich each commit with the branches that contain it.
	// This is one API call per commit, so we keep the same 5-wide concurrency pool.
	// Errors here are non-fatal — we just leave Branches empty.
	var enrichWg sync.WaitGroup
	for i := range all {
		enrichWg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer enrichWg.Done()
			defer func() { <-sem }()
			refs, err := client.ListCommitRefs(all[idx].ProjectID, all[idx].Commit.ID, "branch")
			if err != nil {
				return
			}
			names := make([]string, 0, len(refs))
			for _, ref := range refs {
				names = append(names, ref.Name)
			}
			all[idx].Branches = names
		}(i)
	}
	enrichWg.Wait()

	// Detect auth failures and short-circuit with a helpful error.
	for _, msg := range warnings {
		if strings.Contains(msg, "401") {
			env.Error = "auth_failed"
			break
		}
	}

	// Return commits sorted globally newest-first; the frontend re-groups by repo.
	sort.Slice(all, func(i, j int) bool {
		return all[i].CommittedDate.After(all[j].CommittedDate)
	})

	env.Commits = all
	if len(warnings) > 0 {
		env.Warnings = warnings
	}
	jsonOK(w, env)
}

func strOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func pickName(display, path string) string {
	if display != "" {
		return display
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		return path[idx+1:]
	}
	return path
}
