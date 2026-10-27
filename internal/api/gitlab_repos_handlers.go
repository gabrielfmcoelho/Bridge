package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// gitlabRepoRef is one Bridge entity a repo or group is attached to. LinkID
// is the link row to delete (projeto and dns/host links); 0 for services,
// which hold the repo in a field.
type gitlabRepoRef struct {
	LinkID int64  `json:"link_id,omitempty"`
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	// Field is where a service holds the repo: repository_url (editable) or
	// gitlab_url / git_repository (legacy / written by the Coolify sync).
	Field string `json:"field,omitempty"`
}

type gitlabInventoryGroup struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	FullPath string          `json:"full_path"`
	ParentID *int            `json:"parent_id"`
	WebURL   string          `json:"web_url"`
	Projetos []gitlabRepoRef `json:"projetos"`
	// Hosts linked to the whole group (every repo under it).
	Hosts []gitlabRepoRef `json:"hosts"`
}

type gitlabInventoryRepo struct {
	// ID is 0 for a repo attached in Bridge that the token does not list.
	ID                int             `json:"id"`
	Name              string          `json:"name"`
	PathWithNamespace string          `json:"path_with_namespace"`
	NamespaceID       int             `json:"namespace_id"`
	WebURL            string          `json:"web_url"`
	DefaultBranch     string          `json:"default_branch"`
	Projetos          []gitlabRepoRef `json:"projetos"`
	Services          []gitlabRepoRef `json:"services"`
	DNS               []gitlabRepoRef `json:"dns"`
	Hosts             []gitlabRepoRef `json:"hosts"`
}

type gitlabInventory struct {
	BaseURL string                 `json:"base_url"`
	Groups  []gitlabInventoryGroup `json:"groups"`
	Repos   []gitlabInventoryRepo  `json:"repos"`
	// Unlisted are repos attached in Bridge that the token does not list:
	// moved, renamed, or out of the token's reach (GitLab answers 404).
	Unlisted  []gitlabInventoryRepo `json:"unlisted"`
	Truncated bool                  `json:"truncated"`
}

// sameGitLab compares two GitLab base URLs ignoring case and a trailing slash.
func sameGitLab(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(a, "/"), strings.TrimRight(b, "/"))
}

// handleRepoInventory lists every GitLab group and repo the service token
// sees, each with the projetos, services, DNS records and hosts it is
// attached to in Bridge.
//
//	@Summary		GitLab repo inventory
//	@Description	Admin. Groups and repos visible to the GitLab service token, with their Bridge attachments (projeto links, service repo fields, DNS/host links). unlisted holds repos attached in Bridge that the token does not list. 400 when the integration is disabled or has no token.
//	@Tags			gitlab
//	@Produce		json
//	@Success		200	{object}	gitlabInventory
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/settings/gitlab/repos [get]
func (h *projectGitLabHandlers) handleRepoInventory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client, settings, err := h.resolveClient()
	if err != nil {
		jsonServerError(w, r, "failed to load gitlab settings", err)
		return
	}
	if client == nil || !settings.Enabled {
		jsonError(w, http.StatusBadRequest, "GitLab Code Management disabled or without a service token")
		return
	}
	groups, gTrunc, err := client.ListGroups()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list gitlab groups: "+err.Error())
		return
	}
	projects, pTrunc, err := client.ListProjects()
	if err != nil {
		jsonError(w, http.StatusBadGateway, "failed to list gitlab projects: "+err.Error())
		return
	}

	inv := gitlabInventory{BaseURL: settings.BaseURL, Truncated: gTrunc || pTrunc,
		Groups: []gitlabInventoryGroup{}, Repos: []gitlabInventoryRepo{}, Unlisted: []gitlabInventoryRepo{}}
	groupAt := map[int]int{}
	for _, g := range groups {
		groupAt[g.ID] = len(inv.Groups)
		inv.Groups = append(inv.Groups, gitlabInventoryGroup{ID: g.ID, Name: g.Name, FullPath: g.FullPath, ParentID: g.ParentID, WebURL: g.WebURL, Projetos: []gitlabRepoRef{}, Hosts: []gitlabRepoRef{}})
	}
	repoAt := map[int]int{}
	repoByPath := map[string]int{}
	for _, p := range projects {
		repoAt[p.ID] = len(inv.Repos)
		repoByPath[strings.ToLower(p.PathWithNamespace)] = len(inv.Repos)
		inv.Repos = append(inv.Repos, gitlabInventoryRepo{ID: p.ID, Name: p.Name, PathWithNamespace: p.PathWithNamespace,
			NamespaceID: p.Namespace.ID, WebURL: p.WebURL, DefaultBranch: p.DefaultBranch,
			Projetos: []gitlabRepoRef{}, Services: []gitlabRepoRef{}, DNS: []gitlabRepoRef{}, Hosts: []gitlabRepoRef{}})
	}
	// repoFor finds the listed repo, or the unlisted entry for path (created
	// on first use).
	unlistedAt := map[string]int{}
	repoFor := func(id int, path string) *gitlabInventoryRepo {
		if i, ok := repoAt[id]; ok && id != 0 {
			return &inv.Repos[i]
		}
		key := strings.ToLower(path)
		if i, ok := repoByPath[key]; ok {
			return &inv.Repos[i]
		}
		i, ok := unlistedAt[key]
		if !ok {
			i = len(inv.Unlisted)
			unlistedAt[key] = i
			name := path[strings.LastIndex(path, "/")+1:]
			inv.Unlisted = append(inv.Unlisted, gitlabInventoryRepo{Name: name, PathWithNamespace: path,
				WebURL:   strings.TrimRight(settings.BaseURL, "/") + "/" + path,
				Projetos: []gitlabRepoRef{}, Services: []gitlabRepoRef{}, DNS: []gitlabRepoRef{}, Hosts: []gitlabRepoRef{}})
		}
		return &inv.Unlisted[i]
	}

	// Projetos: links of either kind; a group link attaches to the group row.
	projs, err := store.NewProjectRepo(h.db.SQL).List(ctx)
	if err != nil {
		jsonServerError(w, r, "failed to list projects", err)
		return
	}
	projName := map[int64]string{}
	for _, p := range projs {
		projName[p.ID] = p.Name
	}
	links, err := store.NewProjectGitLabLinkRepo(h.db.SQL).ListAll(ctx)
	if err != nil {
		jsonServerError(w, r, "failed to list project gitlab links", err)
		return
	}
	for _, l := range links {
		name, live := projName[l.ProjectID]
		if !live || !sameGitLab(l.GitLabBaseURL, settings.BaseURL) {
			continue
		}
		ref := gitlabRepoRef{LinkID: l.ID, ID: l.ProjectID, Name: name}
		if l.Kind == models.GitLabLinkKindGroup {
			if i, ok := groupAt[l.GitLabProjectID]; ok {
				inv.Groups[i].Projetos = append(inv.Groups[i].Projetos, ref)
			}
			continue
		}
		rp := repoFor(l.GitLabProjectID, l.GitLabPath)
		rp.Projetos = append(rp.Projetos, ref)
	}

	// Services: the first field that points at a repo on this GitLab, once
	// per (service, repo).
	services, err := store.NewServiceRepo(h.db.SQL).List(ctx)
	if err != nil {
		jsonServerError(w, r, "failed to list services", err)
		return
	}
	for _, s := range services {
		seen := map[string]bool{}
		for _, f := range [][2]string{{"repository_url", s.RepositoryURL}, {"gitlab_url", s.GitlabURL}, {"git_repository", s.GitRepository}} {
			path, ok := gitlabRepoPath(f[1], settings.BaseURL)
			if !ok || seen[strings.ToLower(path)] {
				continue
			}
			seen[strings.ToLower(path)] = true
			rp := repoFor(0, path)
			rp.Services = append(rp.Services, gitlabRepoRef{ID: s.ID, Name: s.Nickname, Field: f[0]})
		}
	}

	// DNS records and hosts; a host's group link attaches to the group row.
	direct, err := store.NewGitLabRepoLinkRepo(h.db.SQL).List(ctx)
	if err != nil {
		jsonServerError(w, r, "failed to list repo links", err)
		return
	}
	for _, l := range direct {
		if !sameGitLab(l.GitLabBaseURL, settings.BaseURL) {
			continue
		}
		ref := gitlabRepoRef{LinkID: l.ID, ID: l.EntityID, Name: l.EntityName}
		if l.Kind == models.GitLabLinkKindGroup {
			if i, ok := groupAt[l.GitLabProjectID]; ok {
				inv.Groups[i].Hosts = append(inv.Groups[i].Hosts, ref)
			}
			continue
		}
		rp := repoFor(l.GitLabProjectID, l.GitLabPath)
		if l.EntityType == "dns" {
			rp.DNS = append(rp.DNS, ref)
		} else {
			rp.Hosts = append(rp.Hosts, ref)
		}
	}
	sort.Slice(inv.Unlisted, func(i, j int) bool { return inv.Unlisted[i].PathWithNamespace < inv.Unlisted[j].PathWithNamespace })
	jsonOK(w, inv)
}

type gitlabRepoLinkRequest struct {
	EntityType string `json:"entity_type" enums:"dns,host"`
	EntityID   int64  `json:"entity_id"`
	// Kind is "project" (default: one repo) or "group" (every repo under it,
	// subgroups included; hosts only).
	Kind string `json:"kind" enums:"project,group"`
	// Path is the repo's path with namespace ("seadpi/sgi") or the group's
	// full path ("seadpi/dl").
	Path string `json:"path"`
}

// handleCreateRepoLink links a GitLab repo straight to a DNS record or host,
// or a whole GitLab group to a host.
//
//	@Summary		Link a GitLab repo (or group) to a DNS record or host
//	@Description	Admin. The repo or group is resolved by path with the service token, so it must be readable by it. kind "group" links every repo under the group, subgroups included, and is for hosts only. Projetos and services are attached through their own endpoints.
//	@Tags			gitlab
//	@Accept			json
//	@Produce		json
//	@Param			body	body		gitlabRepoLinkRequest	true	"Entity and repo path"
//	@Success		201		{object}	models.GitLabRepoLink
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/settings/gitlab/repo-links [post]
func (h *projectGitLabHandlers) handleCreateRepoLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req gitlabRepoLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	req.Path = strings.Trim(strings.TrimSpace(req.Path), "/")
	if req.Path == "" {
		jsonError(w, http.StatusBadRequest, "path is required")
		return
	}
	if req.Kind == "" {
		req.Kind = models.GitLabLinkKindProject
	}
	if req.Kind != models.GitLabLinkKindProject && req.Kind != models.GitLabLinkKindGroup {
		jsonError(w, http.StatusBadRequest, "kind must be 'project' or 'group'")
		return
	}
	if req.Kind == models.GitLabLinkKindGroup && req.EntityType != "host" {
		jsonError(w, http.StatusBadRequest, "a GitLab group can be linked to a host or a projeto, not a "+req.EntityType)
		return
	}
	link := models.GitLabRepoLink{EntityType: req.EntityType, EntityID: req.EntityID, Kind: req.Kind}
	switch req.EntityType {
	case "dns":
		d, err := store.NewDNSRepo(h.db.SQL).Get(ctx, req.EntityID)
		if err != nil || d == nil {
			jsonError(w, http.StatusNotFound, "dns record not found")
			return
		}
		link.EntityName = d.Domain
	case "host":
		hst, err := store.NewHostRepo(h.db.SQL).GetByID(ctx, req.EntityID)
		if err != nil || hst == nil {
			jsonError(w, http.StatusNotFound, "host not found")
			return
		}
		link.EntityName = hst.Nickname
	default:
		jsonError(w, http.StatusBadRequest, "entity_type must be 'dns' or 'host'")
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
	if req.Kind == models.GitLabLinkKindGroup {
		g, err := client.SearchGroupByPath(req.Path)
		if err != nil {
			jsonError(w, http.StatusNotFound, "GitLab group not found: "+err.Error())
			return
		}
		link.GitLabProjectID, link.GitLabPath = g.ID, g.FullPath
	} else {
		p, err := client.SearchProjectByPath(req.Path)
		if err != nil {
			jsonError(w, http.StatusNotFound, "GitLab project not found: "+err.Error())
			return
		}
		link.GitLabProjectID, link.GitLabPath = p.ID, p.PathWithNamespace
	}
	link.GitLabBaseURL = settings.BaseURL
	var by *int64
	if u := auth.UserFromContext(ctx); u != nil {
		by = &u.ID
	}
	if err := store.NewGitLabRepoLinkRepo(h.db.SQL).Create(ctx, &link, by); err != nil {
		jsonError(w, http.StatusConflict, "link already exists or failed to create")
		return
	}
	jsonCreated(w, link)
}

// handleDeleteRepoLink removes a DNS/host repo link.
//
//	@Summary		Remove a DNS/host GitLab repo link
//	@Description	Admin.
//	@Tags			gitlab
//	@Produce		json
//	@Param			id	path		int	true	"Link ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/settings/gitlab/repo-links/{id} [delete]
func (h *projectGitLabHandlers) handleDeleteRepoLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid link id", err)
		return
	}
	ok, err := store.NewGitLabRepoLinkRepo(h.db.SQL).Delete(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to delete link", err)
		return
	}
	if !ok {
		jsonError(w, http.StatusNotFound, "link not found")
		return
	}
	jsonOK(w, StatusResponse{Status: "deleted"})
}
