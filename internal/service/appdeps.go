package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// AppDepSyncResult sums up one app dependencies sync.
type AppDepSyncResult struct {
	Repos   int `json:"repos"`   // repositories looked at
	Found   int `json:"found"`   // with a catalog/dependencies.yml
	Parsed  int `json:"parsed"`  // found and read
	Failed  int `json:"failed"`  // found but failed to parse (stored with its error)
	Removed int `json:"removed"` // rows dropped: the file or the repository is gone
	// Truncated: GitLab's project list hit its cap, so rows of repositories
	// that weren't listed were kept rather than removed.
	Truncated bool     `json:"truncated"`
	Errors    []string `json:"errors"`
}

// maxAppDepErrors caps the error lines a result keeps.
const maxAppDepErrors = 50

// AppDepsService reads every repository's catalog/dependencies.yml from
// GitLab into app_dep_catalogs. It only reads and parses: linting and the
// vocabulary belong to catalog-parser and catalog-governance.
type AppDepsService struct {
	repo    *store.AppDepRepo
	load    func() (appDepsSource, error)
	workers int
}

type appDepsSource struct {
	gl     *gitlab.Client
	groups []string
}

// NewAppDepsService resolves the GitLab client and groups from settings on each sync.
func NewAppDepsService(db *sql.DB, enc *database.Encryptor) *AppDepsService {
	return &AppDepsService{repo: store.NewAppDepRepo(db), workers: 8, load: func() (appDepsSource, error) {
		gls, err := gitlab.LoadSettings(db, enc)
		if err != nil {
			return appDepsSource{}, err
		}
		if !gls.Enabled {
			return appDepsSource{}, fmt.Errorf("%w: integração desativada", ErrGitLabNaoConfigurado)
		}
		gl := gitlab.NewServiceClient(gls)
		if gl == nil {
			return appDepsSource{}, fmt.Errorf("%w: sem token de serviço", ErrGitLabNaoConfigurado)
		}
		return appDepsSource{gl: gl, groups: gls.CatalogGroups}, nil
	}}
}

// NewAppDepsServiceWith uses a fixed client and groups (tests).
func NewAppDepsServiceWith(db *sql.DB, gl *gitlab.Client, groups []string) *AppDepsService {
	return &AppDepsService{repo: store.NewAppDepRepo(db), workers: 4, load: func() (appDepsSource, error) {
		if gl == nil {
			return appDepsSource{}, ErrGitLabNaoConfigurado
		}
		return appDepsSource{gl: gl, groups: groups}, nil
	}}
}

// Ready reports why a sync can't start (ErrGitLabNaoConfigurado), or nil.
func (s *AppDepsService) Ready() error {
	_, err := s.load()
	return err
}

// projects lists the repositories to read: the configured groups (with
// subgroups), else every project the token is a member of. complete is
// false when the list may be short (cap reached, a group failed).
func (f appDepsSource) projects(res *AppDepSyncResult) (projs []gitlab.Project, complete bool, err error) {
	if len(f.groups) == 0 {
		ps, truncated, err := f.gl.ListProjects()
		if err != nil {
			return nil, false, fmt.Errorf("listar projetos: %w", err)
		}
		res.Truncated = truncated
		return ps, !truncated, nil
	}
	complete = true
	seen := map[int]bool{}
	failed := 0
	for _, g := range f.groups {
		ps, err := f.gl.ListGroupProjectsAll(g, false)
		if err != nil {
			complete = false
			failed++
			res.addError(fmt.Sprintf("grupo %s: %v", g, err))
			continue
		}
		for _, p := range ps {
			if !seen[p.ID] {
				seen[p.ID] = true
				projs = append(projs, p)
			}
		}
	}
	if failed == len(f.groups) {
		return nil, false, fmt.Errorf("nenhum grupo pôde ser listado: %s", res.Errors[0])
	}
	return projs, complete, nil
}

func (r *AppDepSyncResult) addError(msg string) {
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if len(r.Errors) < maxAppDepErrors {
		r.Errors = append(r.Errors, msg)
	} else if len(r.Errors) == maxAppDepErrors {
		r.Errors = append(r.Errors, "…")
	}
}

type appDepRead struct {
	p      gitlab.Project
	file   *gitlab.RepoFile
	absent bool // no default branch, or a 404: the repository has no catalog
	err    error
}

// Sync reads every listed repository's catalog at its default branch and
// stores what it found. A 404 removes the repository's row; any other
// error keeps it as it was. When the project list is complete, rows of
// repositories no longer listed are removed too.
func (s *AppDepsService) Sync(ctx context.Context) (AppDepSyncResult, error) {
	res := AppDepSyncResult{Errors: []string{}}
	f, err := s.load()
	if err != nil {
		return res, err
	}
	projs, complete, err := f.projects(&res)
	if err != nil {
		return res, err
	}
	res.Repos = len(projs)

	reads := make([]appDepRead, len(projs))
	sem := make(chan struct{}, max(s.workers, 1))
	var wg sync.WaitGroup
	for i, p := range projs {
		reads[i].p = p
		if p.DefaultBranch == "" { // empty repository
			reads[i].absent = true
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			file, err := f.gl.GetFile(p.ID, models.AppDepCatalogPath, p.DefaultBranch)
			var se *gitlab.StatusError
			switch {
			case errors.As(err, &se) && se.Code == http.StatusNotFound:
				reads[i].absent = true
			case err != nil:
				reads[i].err = err
			default:
				reads[i].file = file
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return res, err
	}

	now := time.Now().UTC()
	listed := make(map[int]bool, len(projs))
	var upserts []models.AppDepSource
	var remove []int
	for _, rd := range reads {
		listed[rd.p.ID] = true
		switch {
		case rd.absent:
			remove = append(remove, rd.p.ID)
		case rd.err != nil:
			res.addError(fmt.Sprintf("%s: %v", rd.p.PathWithNamespace, rd.err))
		case rd.file != nil:
			res.Found++
			commit := rd.file.LastCommitID
			if commit == "" {
				commit = rd.file.CommitID
			}
			src := models.AppDepSource{GitLabProjectID: rd.p.ID, ProjectPath: rd.p.PathWithNamespace, WebURL: rd.p.WebURL,
				Ref: rd.p.DefaultBranch, CommitID: commit, SyncedAt: now}
			if c, err := models.ParseAppDepCatalog([]byte(rd.file.Content)); err != nil {
				src.ParseError = err.Error()
				res.Failed++
			} else {
				src.Catalog = c
				res.Parsed++
			}
			upserts = append(upserts, src)
		}
	}
	if complete {
		ids, err := s.repo.IDs(ctx)
		if err != nil {
			return res, err
		}
		for _, id := range ids {
			if !listed[id] {
				remove = append(remove, id)
			}
		}
	}
	sort.Ints(remove)
	removed, err := s.repo.Apply(ctx, upserts, remove)
	if err != nil {
		return res, err
	}
	res.Removed = removed
	return res, nil
}
