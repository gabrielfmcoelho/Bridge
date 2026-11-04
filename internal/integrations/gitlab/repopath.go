package gitlab

import (
	"net/url"
	"strings"
)

// RepoPath extracts "group/repo" from a clone or web URL (https, ssh://
// or scp-style git@host:path) when it points at the configured GitLab host.
func RepoPath(raw, baseURL string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" {
		return "", false
	}
	var host, path string
	if !strings.Contains(raw, "://") && strings.Contains(raw, "@") && strings.Contains(raw, ":") {
		// scp-style: git@host:group/repo.git
		at := raw[strings.Index(raw, "@")+1:]
		host, path, _ = strings.Cut(at, ":")
	} else {
		u, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		host, path = u.Host, u.Path
	}
	if !strings.EqualFold(strings.Split(host, ":")[0], base.Hostname()) {
		return "", false
	}
	path = strings.Trim(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	// Web URLs may point inside the repo: …/group/repo/-/tree/main
	if i := strings.Index(path, "/-/"); i >= 0 {
		path = path[:i]
	}
	if !strings.Contains(path, "/") {
		return "", false
	}
	return path, true
}
