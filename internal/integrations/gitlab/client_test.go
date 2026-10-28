package gitlab

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeGitLab(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "tok" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "tok")
}

func TestListTreePaginates(t *testing.T) {
	var pages []string
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/v4/projects/7/repository/tree" || q.Get("recursive") != "true" || q.Get("ref") != "main" {
			t.Errorf("unexpected request %s", r.URL)
		}
		pages = append(pages, q.Get("page"))
		n := 100
		if q.Get("page") == "2" {
			n = 3
		}
		items := make([]string, n)
		for i := range items {
			items[i] = fmt.Sprintf(`{"name":"f%d","type":"blob","path":"src/f%d"}`, i, i)
		}
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	})
	all, err := c.ListTree(7, "main", "", true)
	if err != nil || len(all) != 103 || strings.Join(pages, ",") != "1,2" {
		t.Fatalf("entries=%d pages=%v err=%v, want 103 over pages 1,2", len(all), pages, err)
	}
}

func TestGetRawFileEncodesPath(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/projects/7/repository/files/deploy%2Fnginx%2Fsite.conf/raw" || r.URL.Query().Get("ref") != "HEAD" {
			t.Errorf("unexpected request %s", r.URL.RequestURI())
		}
		_, _ = w.Write([]byte("server {}"))
	})
	b, err := c.GetRawFile(7, "deploy/nginx/site.conf", "")
	if err != nil || string(b) != "server {}" {
		t.Fatalf("got %q, %v", b, err)
	}
}

func TestSearchBlobsAndErrors(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "blobs" || r.URL.Query().Get("search") != "metabase" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`[{"path":"docker-compose.yml","startline":3,"data":"image: metabase/metabase:v0.46","ref":"main"}]`))
	})
	hits, err := c.SearchBlobs(7, "metabase", "")
	if err != nil || len(hits) != 1 || hits[0].Path != "docker-compose.yml" || hits[0].Startline != 3 {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
	if _, err := c.SearchBlobs(7, "other", ""); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want the 403 surfaced", err)
	}
}

func TestListProjectsPaginatesAndTruncates(t *testing.T) {
	page := func(n, from int) string {
		items := make([]string, n)
		for i := range items {
			items[i] = fmt.Sprintf(`{"id":%d,"path_with_namespace":"g/r%d","namespace":{"id":3,"full_path":"g"}}`, from+i, from+i)
		}
		return "[" + strings.Join(items, ",") + "]"
	}
	var calls int
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects" || r.URL.Query().Get("membership") != "true" {
			t.Errorf("unexpected request %s", r.URL)
		}
		calls++
		n := 100
		if r.URL.Query().Get("page") == "2" {
			n = 1
		}
		_, _ = w.Write([]byte(page(n, calls*100)))
	})
	all, truncated, err := c.ListProjects()
	if err != nil || len(all) != 101 || truncated || all[0].Namespace.ID != 3 || all[0].Namespace.FullPath != "g" {
		t.Fatalf("got %d projects truncated=%v err=%v first=%+v", len(all), truncated, err, all[0])
	}

	// Always a full page: stops at the cap and says so.
	calls = 0
	c = fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(page(100, calls*100)))
	})
	all, truncated, err = c.ListProjects()
	if err != nil || !truncated || calls != maxListPages || len(all) != 100*maxListPages {
		t.Fatalf("calls=%d len=%d truncated=%v err=%v, want the cap", calls, len(all), truncated, err)
	}
}

func TestListGroupsParent(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/groups" || r.URL.Query().Get("min_access_level") != "10" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"id":1,"full_path":"seadpi","parent_id":null},{"id":2,"full_path":"seadpi/dl","parent_id":1}]`))
	})
	gs, truncated, err := c.ListGroups()
	if err != nil || truncated || len(gs) != 2 || gs[0].ParentID != nil || gs[1].ParentID == nil || *gs[1].ParentID != 1 {
		t.Fatalf("got %+v truncated=%v err=%v", gs, truncated, err)
	}
}

func TestListGroupProjectsAllPaginaAteOFim(t *testing.T) {
	var pages []string
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/v4/groups/5/projects" || q.Get("include_subgroups") != "true" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if _, has := q["archived"]; has {
			t.Errorf("archived must be absent when includeArchived, got %s", r.URL)
		}
		pages = append(pages, q.Get("page"))
		n := 100
		if q.Get("page") == "3" {
			n = 7
		}
		items := make([]string, n)
		for i := range items {
			items[i] = fmt.Sprintf(`{"id":%d}`, i)
		}
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	})
	all, err := c.ListGroupProjectsAll("5", true)
	if err != nil || len(all) != 207 || strings.Join(pages, ",") != "1,2,3" {
		t.Fatalf("projects=%d pages=%v err=%v, want 207 over 1,2,3", len(all), pages, err)
	}
}

func TestListGroupProjectsAllPorCaminho(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/groups/seadpi%2Fgestor/projects" || r.URL.Query().Get("archived") != "false" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"id":1}]`))
	})
	all, err := c.ListGroupProjectsAll("seadpi/gestor", false)
	if err != nil || len(all) != 1 {
		t.Fatalf("all=%v err=%v", all, err)
	}
}

func TestListGroupProjectsAllTetoDeSeguranca(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		items := make([]string, 100)
		for i := range items {
			items[i] = `{"id":1}`
		}
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	})
	if _, err := c.ListGroupProjectsAll("5", true); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("want cap error, got %v", err)
	}
}

func TestListContributors(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/9/repository/contributors" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"name":"Ana","email":"Ana@x.gov.br","commits":4,"additions":1,"deletions":2}]`))
	})
	got, err := c.ListContributors(9)
	if err != nil || len(got) != 1 || got[0] != (Contributor{"Ana", "Ana@x.gov.br", 4}) {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestListContributorsRepoVazio(t *testing.T) {
	c := fakeGitLab(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/1/repository/contributors" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		http.Error(w, `{"message":"404 Not Found"}`, http.StatusNotFound)
	})
	for _, id := range []int{1, 2} {
		got, err := c.ListContributors(id)
		if err != nil || len(got) != 0 {
			t.Fatalf("id=%d got=%v err=%v, want empty", id, got, err)
		}
	}
}
