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
