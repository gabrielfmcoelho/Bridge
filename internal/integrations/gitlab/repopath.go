package gitlab

import (
	"net/url"
	"strings"
)

// RepoPath extracts "group/repo" from a clone or web URL (https, ssh://
// or scp-style git@host:path) when it points at the configured GitLab host.
func RepoPath(raw, baseURL string) (string, bool) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" {
		return "", false
	}
	host, path, ok := RemoteHostPath(raw)
	if !ok || !strings.EqualFold(host, base.Hostname()) || !strings.Contains(path, "/") {
		return "", false
	}
	return path, true
}

// RemoteHostPath splits a clone or web URL (https, ssh:// or scp-style
// git@host:path) into its lowercase host, port dropped, and repo path, with
// no ".git" and nothing from "/-/" on (web URLs inside a repo). ok is false
// for a local path or an unparseable URL.
func RemoteHostPath(raw string) (host, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") && strings.Contains(raw, "@") && strings.Contains(raw, ":") {
		// scp-style: git@host:group/repo.git
		at := raw[strings.Index(raw, "@")+1:]
		host, path, _ = strings.Cut(at, ":")
	} else {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", false
		}
		host, path = u.Host, u.Path
	}
	host = strings.ToLower(strings.Split(host, ":")[0])
	path = strings.Trim(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	if i := strings.Index(path, "/-/"); i >= 0 {
		path = path[:i]
	}
	return host, path, host != "" && path != ""
}
