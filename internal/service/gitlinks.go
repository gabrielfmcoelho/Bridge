package service

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// GitLinkSummary is what RecordGitLinks changed.
type GitLinkSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// RecordGitLinks links the host to every repo on the configured GitLab
// (baseURL) that a git remote in its scan points at, whether the token sees
// it or not: scan links are keyed by path. Remotes on other hosts are only
// listed (the scan JSON), never linked. A no-op (nil summary) when the probe
// didn't run (GitRemotes nil) or GitLab is off (baseURL ""); an incomplete
// list (GitRemotesPartial) only adds.
func RecordGitLinks(ctx context.Context, db *sql.DB, hostID int64, info sshtest.VMInfo, baseURL string) (*GitLinkSummary, error) {
	if info.GitRemotes == nil || baseURL == "" {
		return nil, nil
	}
	var paths []string
	for _, r := range info.GitRemotes {
		if p, ok := gitlab.RepoPath(r.URL, baseURL); ok {
			paths = append(paths, p)
		}
	}
	added, removed, err := store.NewGitLabRepoLinkRepo(db).SyncScanLinks(ctx, hostID, baseURL, paths, !info.GitRemotesPartial)
	if err != nil {
		return nil, err
	}
	return &GitLinkSummary{Added: added, Removed: removed}, nil
}
