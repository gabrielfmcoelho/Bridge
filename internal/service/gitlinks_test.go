package service_test

import (
	"context"
	"sort"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestRecordGitLinks(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	h := &models.Host{Nickname: "vm-git", OficialSlug: "vm-git", Hostname: "10.0.0.8", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	const base = "https://gl.example/"
	links := func() []string {
		all, err := store.NewGitLabRepoLinkRepo(d.SQL).List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, l := range all {
			out = append(out, l.GitLabPath)
		}
		sort.Strings(out)
		return out
	}
	record := func(info sshtest.VMInfo, baseURL string) *service.GitLinkSummary {
		sum, err := service.RecordGitLinks(ctx, d.SQL, h.ID, info, baseURL)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return sum
	}

	full := sshtest.VMInfo{GitRemotes: []sshtest.GitRemote{
		{Path: "/opt/app", Name: "origin", URL: "https://gl.example/g/app.git"},
		{Path: "/opt/tool", Name: "origin", URL: "git@gl.example:g/tool.git"},
		{Path: "/opt/x", Name: "origin", URL: "https://github.com/o/x"}, // another host: listed, never linked
		{Container: "api", Path: "/app", Name: "origin", URL: "https://gl.example/g/app"},
	}}
	if sum := record(full, base); sum == nil || sum.Added != 2 || sum.Removed != 0 {
		t.Fatalf("first scan = %+v, want 2 added", sum)
	}
	if got := links(); len(got) != 2 || got[0] != "g/app" || got[1] != "g/tool" {
		t.Fatalf("links = %v", got)
	}
	// Probe didn't run, or GitLab is off: nothing changes.
	if sum := record(sshtest.VMInfo{}, base); sum != nil {
		t.Fatalf("nil remotes = %+v, want no-op", sum)
	}
	if sum := record(sshtest.VMInfo{GitRemotes: []sshtest.GitRemote{}}, ""); sum != nil {
		t.Fatalf("no base URL = %+v, want no-op", sum)
	}
	// An incomplete list only adds; a complete one prunes.
	onlyApp := []sshtest.GitRemote{full.GitRemotes[0]}
	if sum := record(sshtest.VMInfo{GitRemotes: onlyApp, GitRemotesPartial: true}, base); sum.Removed != 0 {
		t.Fatalf("partial scan removed %d", sum.Removed)
	}
	if sum := record(sshtest.VMInfo{GitRemotes: onlyApp}, base); sum.Removed != 1 {
		t.Fatalf("complete scan = %+v, want 1 removed", sum)
	}
	if got := links(); len(got) != 1 || got[0] != "g/app" {
		t.Fatalf("links after prune = %v", got)
	}
}
