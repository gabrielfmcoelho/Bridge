package store_test

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Scan links are keyed by path (GitLab id 0), only ever touch source='scan'
// rows of their host and base URL, and never duplicate a manual link.
func TestGitLabRepoLinkSyncScanLinks(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('vm1', 'vm1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	const base, other = "https://gl.example", "https://other.example"
	repo := store.NewGitLabRepoLinkRepo(d.SQL)
	if err := repo.Create(ctx, &models.GitLabRepoLink{EntityType: "host", EntityID: hostID, GitLabProjectID: 7,
		GitLabBaseURL: base, GitLabPath: "g/manual"}, nil); err != nil {
		t.Fatal(err)
	}
	paths := func() []string {
		all, err := repo.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, l := range all {
			out = append(out, l.Source+":"+l.GitLabBaseURL+"/"+l.GitLabPath)
		}
		sort.Strings(out)
		return out
	}
	sync := func(baseURL string, prune bool, p ...string) (int, int) {
		added, removed, err := repo.SyncScanLinks(ctx, hostID, baseURL, p, prune)
		if err != nil {
			t.Fatalf("sync %v: %v", p, err)
		}
		return added, removed
	}

	// Two id-0 repos on one host, a case twin collapsed, the manual one skipped.
	if a, r := sync(base, true, "g/a", "G/Manual", "g/b", "G/B"); a != 2 || r != 0 {
		t.Fatalf("first sync added=%d removed=%d, want 2, 0", a, r)
	}
	want := []string{"manual:" + base + "/g/manual", "scan:" + base + "/g/a", "scan:" + base + "/g/b"}
	if got := paths(); !slices.Equal(got, want) {
		t.Fatalf("links = %v, want %v", got, want)
	}
	if a, r := sync(base, true, "g/a", "g/b"); a != 0 || r != 0 {
		t.Fatalf("repeat sync added=%d removed=%d, want 0, 0", a, r)
	}
	// Incomplete scan: adds, never removes.
	if a, r := sync(base, false, "g/c"); a != 1 || r != 0 {
		t.Fatalf("add-only sync added=%d removed=%d, want 1, 0", a, r)
	}
	// Complete scan: what's gone goes, manual stays, other GitLabs untouched.
	sync(other, true, "x/y")
	if a, r := sync(base, true, "g/b"); a != 0 || r != 2 {
		t.Fatalf("prune sync added=%d removed=%d, want 0, 2", a, r)
	}
	want = []string{"manual:" + base + "/g/manual", "scan:" + base + "/g/b", "scan:" + other + "/x/y"}
	if got := paths(); !slices.Equal(got, want) {
		t.Fatalf("links = %v, want %v", got, want)
	}
	// The scan link has no GitLab id; manual ones keep theirs.
	for _, l := range mustList(t, repo) {
		if (l.Source == "scan") != (l.GitLabProjectID == 0) {
			t.Fatalf("link %+v: scan links have id 0, manual ones keep theirs", l)
		}
	}
}

func TestHostScanRepoLatestGitRemotes(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	host := func(name string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES (?, ?, ?) RETURNING id`, name, name+"-slug", name+".example").Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	scans := store.NewHostScanRepo(d.SQL)
	scan := func(id int64, data, at string) {
		if _, err := d.SQL.Exec(`INSERT INTO host_scans (host_id, data, scanned_at) VALUES (?, ?, ?)`, id, data, at); err != nil {
			t.Fatal(err)
		}
	}
	live, gone, broken, never := host("live"), host("gone"), host("broken"), host("never")
	scan(live, `{"git_remotes":[{"path":"/old","name":"origin","url":"https://gl.example/g/old.git"}]}`, "2026-01-01")
	scan(live, `{"git_remotes":[{"path":"/opt/app","name":"origin","url":"https://gl.example/g/app.git","commit":"0123456789abcdef0123456789abcdef01234567","branch":"main","pulled_at":1700001000,"pull_kind":"pull"},`+
		`{"container":"api","container_image":"api:1","path":"/app","name":"origin","url":"https://github.com/o/api","pulled_at":"soon"}]}`, "2026-01-02")
	scan(gone, `{"git_remotes":[{"path":"/x","name":"origin","url":"https://gl.example/g/x.git"}]}`, "2026-01-02")
	if _, err := d.SQL.Exec(`UPDATE hosts SET deleted_at = NOW() WHERE id = ?`, gone); err != nil {
		t.Fatal(err)
	}
	scan(broken, `{"git_remotes":[{"path":"/y","name":"origin","url":"https://gl.example/g/y.git"}]}`, "2026-01-01")
	scan(broken, `not json`, "2026-01-02")
	scan(never, `{"git_remotes":null}`, "2026-01-02")

	got, err := scans.LatestGitRemotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []models.HostGitRemote{
		{HostID: live, HostName: "live", HostSlug: "live-slug", HostAddress: "live.example", Path: "/opt/app", Name: "origin", URL: "https://gl.example/g/app.git", Commit: "0123456789abcdef0123456789abcdef01234567", Branch: "main", PulledAt: 1700001000, PullKind: "pull"},
		{HostID: live, HostName: "live", HostSlug: "live-slug", HostAddress: "live.example", Container: "api", ContainerImage: "api:1", Path: "/app", Name: "origin", URL: "https://github.com/o/api"},
	}
	if len(got) != len(want) {
		t.Fatalf("LatestGitRemotes = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func mustList(t *testing.T, repo *store.GitLabRepoLinkRepo) []models.GitLabRepoLink {
	t.Helper()
	all, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return all
}
