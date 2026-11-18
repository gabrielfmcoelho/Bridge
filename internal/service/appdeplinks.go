package service

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// AppDepLinkFilter narrows AppDepLinks: by application (CI), or by asset
// (EntityType + EntityID). Zero value = every link.
type AppDepLinkFilter struct {
	CI         string
	EntityType string
	EntityID   int64
}

type appDepRef struct {
	id int
	ci string
}

// sourceRank orders a link's sources, most specific first.
var sourceRank = map[string]int{
	models.AppDepSourceManual: 0, models.AppDepSourceRepo: 1, models.AppDepSourceScan: 2, models.AppDepSourceGroup: 3,
}

// AppDepLinks returns the links between applications (the CIs of the synced
// catalogs) and hosts, DNS records, services and projetos the caller can see.
// They come from the CI's GitLab repository — whatever Bridge already links
// to it: a host or DNS record's repo links (one repo, a group holding it, or
// a checkout a host scan found), a projeto's GitLab links, a service's
// repository fields — plus the links editors add (app_dep_links). Nothing
// derived is stored: a new repo link or scan shows here on the next read.
// baseURL is the configured GitLab; links to another GitLab never match.
func AppDepLinks(ctx context.Context, db *sql.DB, baseURL string, f AppDepLinkFilter) ([]models.AppDepAssetLink, error) {
	repo := store.NewAppDepRepo(db)
	cats, err := repo.List(ctx)
	if err != nil {
		return nil, err
	}
	// The CI's id is its first repository by path (List's order), like the UI's index.
	byCI := map[string]appDepRef{}
	byPath := map[string]appDepRef{}
	byGID := map[int]appDepRef{}
	for _, s := range cats {
		if s.Catalog == nil {
			continue
		}
		ref, ok := byCI[s.Catalog.CI]
		if !ok {
			ref = appDepRef{s.GitLabProjectID, s.Catalog.CI}
			byCI[ref.ci] = ref
		}
		byPath[normRepoPath(s.ProjectPath)] = ref
		byGID[s.GitLabProjectID] = ref
	}
	if len(byCI) == 0 {
		return []models.AppDepAssetLink{}, nil
	}

	links := map[[3]string]*models.AppDepAssetLink{}
	add := func(ref appDepRef, typ string, id int64, source, via string) {
		if f.CI != "" && f.CI != ref.ci || f.EntityType != "" && (f.EntityType != typ || f.EntityID != id) {
			return
		}
		key := [3]string{ref.ci, typ, strconv.FormatInt(id, 10)}
		l := links[key]
		if l == nil {
			l = &models.AppDepAssetLink{CIID: ref.id, CI: ref.ci, EntityType: typ, EntityID: id}
			links[key] = l
		}
		for _, s := range l.Sources {
			if s == source {
				return
			}
		}
		l.Sources = append(l.Sources, source)
		if l.Via == "" {
			l.Via = via
		}
	}
	repoOf := func(gid int, path, base string) (appDepRef, bool) {
		if base != "" && !gitlab.SameBaseURL(base, baseURL) {
			return appDepRef{}, false
		}
		if ref, ok := byGID[gid]; gid != 0 && ok {
			return ref, true
		}
		ref, ok := byPath[normRepoPath(path)]
		return ref, ok
	}
	inGroup := func(group, base string) []appDepRef {
		if base != "" && !gitlab.SameBaseURL(base, baseURL) {
			return nil
		}
		prefix := normRepoPath(group) + "/"
		seen := map[string]bool{}
		var out []appDepRef
		for p, ref := range byPath {
			if strings.HasPrefix(p, prefix) && !seen[ref.ci] {
				seen[ref.ci] = true
				out = append(out, ref)
			}
		}
		return out
	}

	// Hosts and DNS records: their repo links (one repo, a group, a scan's checkout).
	repoLinks, err := store.NewGitLabRepoLinkRepo(db).List(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range repoLinks {
		if l.Kind == models.GitLabLinkKindGroup {
			for _, ref := range inGroup(l.GitLabPath, l.GitLabBaseURL) {
				add(ref, l.EntityType, l.EntityID, models.AppDepSourceGroup, l.GitLabPath)
			}
			continue
		}
		if ref, ok := repoOf(l.GitLabProjectID, l.GitLabPath, l.GitLabBaseURL); ok {
			src := models.AppDepSourceRepo
			if l.Source == "scan" {
				src = models.AppDepSourceScan
			}
			add(ref, l.EntityType, l.EntityID, src, l.GitLabPath)
		}
	}

	// Projetos: their GitLab links.
	projLinks, err := store.NewProjectGitLabLinkRepo(db).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range projLinks {
		if l.Kind == models.GitLabLinkKindGroup {
			for _, ref := range inGroup(l.GitLabPath, l.GitLabBaseURL) {
				add(ref, "project", l.ProjectID, models.AppDepSourceGroup, l.GitLabPath)
			}
			continue
		}
		if ref, ok := repoOf(l.GitLabProjectID, l.GitLabPath, l.GitLabBaseURL); ok {
			add(ref, "project", l.ProjectID, models.AppDepSourceRepo, l.GitLabPath)
		}
	}

	// Services: the repository they name (clone or web URL on this GitLab).
	svcs, err := repo.ServiceRepoURLs(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range svcs {
		for _, u := range s.URLs {
			if path, ok := gitlab.RepoPath(u, baseURL); ok {
				if ref, ok := byPath[normRepoPath(path)]; ok {
					add(ref, "service", s.ID, models.AppDepSourceRepo, path)
				}
			}
		}
	}

	// What editors added. A link to a CI no catalog declares anymore waits.
	manual, err := repo.ManualLinks(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range manual {
		if ref, ok := byCI[m.CI]; ok {
			add(ref, m.EntityType, m.EntityID, models.AppDepSourceManual, "")
		}
	}

	// Only live assets the caller can see.
	live, err := repo.LiveVisibleIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.AppDepAssetLink, 0, len(links))
	for _, l := range links {
		if !live[l.EntityType][l.EntityID] {
			continue
		}
		sort.Slice(l.Sources, func(i, j int) bool { return sourceRank[l.Sources[i]] < sourceRank[l.Sources[j]] })
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.CI != b.CI {
			return a.CI < b.CI
		}
		if a.EntityType != b.EntityType {
			return a.EntityType < b.EntityType
		}
		return a.EntityID < b.EntityID
	})
	return out, nil
}

// normRepoPath is a repository or group path as compared: lowercase, no
// surrounding slashes, no ".git".
func normRepoPath(p string) string {
	return strings.ToLower(strings.TrimSuffix(strings.Trim(strings.TrimSpace(p), "/"), ".git"))
}
