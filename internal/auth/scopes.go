package auth

import (
	"slices"
	"strings"
)

// Token scopes: what a personal or service API token may call. A scope only
// ever narrows access — the owner's role, permissions and entidades still
// decide everything else — and browser sessions carry no scopes at all.
//
// Every non-public route resolves to exactly one scope (ScopeFor); the route
// registrar resolves it once at startup and refuses to start on a route with
// none, so a new route can't ship unscoped. Most scopes are derived: the
// route's domain (longest matching prefix below) plus read (GET/HEAD) or
// write (anything else). A few sensitive actions get their own scope so that
// e.g. hosts:read doesn't also read host passwords.

// ScopeAll is the wildcard: every route, current and future.
const ScopeAll = "*"

// domainRules maps route prefixes to their domain. Longest prefix wins.
var domainRules = map[string]string{
	"/api/hosts":         "hosts",
	"/api/dns":           "dns",
	"/api/services":      "services",
	"/api/orchestrators": "services",
	"/api/tools":         "services",
	"/api/projects":      "projects",
	"/api/releases":      "projects",
	"/api/issues":        "projects",
	"/api/canvases":      "projects",
	"/api/contacts":      "contacts",
	"/api/api-catalog":   "apis",
	"/api/secrets":       "vault",
	"/api/share-bundles": "vault",
	"/api/graph":         "inventory",
	"/api/relations":     "inventory",
	"/api/dashboard":     "inventory",
	"/api/tags":          "inventory",
	"/api/enums":         "inventory",
	"/api/entidades":     "inventory",
	"/api/assets":        "inventory",
	"/api/glpi":          "integrations",
	"/api/coolify":       "integrations",
	"/api/gitlab":        "integrations",
	"/api/grafana":       "integrations",
	"/api/telemetry":     "integrations",
	"/api/proxmox":       "integrations",
	"/api/wiki":          "integrations",
	"/api/ai":            "integrations",
	"/api/settings":      "settings",
	"/api/users":         "admin",
	"/api/import":        "admin",
	"/api/auth":          "auth",
}

// sensitive routes get a scope of their own, checked before the domain rules.
func sensitiveScope(method, pattern string) string {
	read := method == "GET" || method == "HEAD"
	switch {
	case pattern == "/api/secrets/{id}/reveal", pattern == "/api/hosts/{slug}/password":
		return "vault:reveal"
	case hasPrefix(pattern, "/api/ssh"):
		if read {
			return "ssh:read"
		}
		return "ssh:operate"
	case hasPrefix(pattern, "/api/backup"), hasPrefix(pattern, "/api/restore"):
		return "admin:backup"
	case hasPrefix(pattern, "/api/api-catalog/{id}/key-management"),
		hasPrefix(pattern, "/api/api-catalog/{id}/keys") && !read:
		return "apis:keys"
	}
	return ""
}

func hasPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// ScopeFor returns the scope a route needs (pattern in ServeMux spelling,
// e.g. "/api/hosts/{slug}"), or false when no rule covers it.
func ScopeFor(method, pattern string) (string, bool) {
	if s := sensitiveScope(method, pattern); s != "" {
		return s, true
	}
	best := ""
	for prefix := range domainRules {
		if hasPrefix(pattern, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return "", false
	}
	if method == "GET" || method == "HEAD" {
		return domainRules[best] + ":read", true
	}
	return domainRules[best] + ":write", true
}

// ScopeAllowed reports whether a token holding scopes may call a route that
// needs required.
func ScopeAllowed(scopes []string, required string) bool {
	return slices.Contains(scopes, ScopeAll) || slices.Contains(scopes, required)
}

// ScopeInfo is one catalogue entry — the same shape SEAD's
// GET /admin/keys/scopes answers, so one picker serves both.
type ScopeInfo struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // "wildcard" or "route"
	Description string   `json:"description"`
	Routes      []string `json:"routes"`
	// MinRole is the least role that can use it; Permission, when set, is a
	// permission code the owner needs instead of a role.
	MinRole    string `json:"min_role"`
	Permission string `json:"permission,omitempty"`
}

var domainDescriptions = map[string]string{
	"hosts":        "Hosts: inventory, specs, scans, alerts and chamados",
	"dns":          "DNS records and TLS certificates",
	"services":     "Services, orchestrators and external tools",
	"projects":     "Projects, releases and issues",
	"contacts":     "Contacts (responsáveis)",
	"apis":         "APIs (Atlas catalogue): specs, links and key metadata",
	"vault":        "Vault metadata and share links (not the secret values: see vault:reveal)",
	"inventory":    "Cross-inventory reads and org units: graph, relations, dashboard, tags, enums, entidades",
	"integrations": "GLPI, Coolify, GitLab, Grafana, Proxmox, wiki and AI",
	"settings":     "Application settings and integrations configuration",
	"admin":        "Users and bulk import",
	"auth":         "The token's own identity (/api/auth/me)",
}

// Catalogue lists every scope a token can carry, in a stable order: "*",
// then read/write per domain, then the sensitive scopes.
func Catalogue() []ScopeInfo {
	out := []ScopeInfo{{Name: ScopeAll, Kind: "wildcard", Description: "Full access: every route the owner can reach, including future ones", MinRole: "viewer", Routes: []string{}}}
	domains := map[string][]string{}
	for prefix, d := range domainRules {
		domains[d] = append(domains[d], prefix)
	}
	names := make([]string, 0, len(domains))
	for d := range domains {
		names = append(names, d)
	}
	slices.Sort(names)
	for _, d := range names {
		routes := domains[d]
		slices.Sort(routes)
		writeRole := "editor"
		if d == "admin" || d == "settings" {
			writeRole = "admin"
		}
		readRole := "viewer"
		if d == "admin" {
			readRole = "admin"
		}
		out = append(out,
			ScopeInfo{Name: d + ":read", Kind: "route", Description: domainDescriptions[d] + " — read", Routes: routes, MinRole: readRole},
			ScopeInfo{Name: d + ":write", Kind: "route", Description: domainDescriptions[d] + " — create, change, delete", Routes: routes, MinRole: writeRole})
	}
	out = append(out,
		ScopeInfo{Name: "vault:reveal", Kind: "route", Description: "Read secret values (vault reveal, host passwords). Each reveal is audited", Routes: []string{"=/api/secrets/{id}/reveal", "=/api/hosts/{slug}/password"}, MinRole: "viewer"},
		ScopeInfo{Name: "ssh:read", Kind: "route", Description: "SSH config, keys and operation logs", Routes: []string{"/api/ssh"}, MinRole: "viewer"},
		ScopeInfo{Name: "ssh:operate", Kind: "route", Description: "Run SSH operations on hosts (tests, key setup, docker, users)", Routes: []string{"/api/ssh"}, MinRole: "editor"},
		ScopeInfo{Name: "admin:backup", Kind: "route", Description: "Download a full backup or restore one", Routes: []string{"/api/backup", "/api/restore"}, MinRole: "admin"},
		ScopeInfo{Name: "apis:keys", Kind: "route", Description: "Issue, rotate and revoke access keys of catalogued APIs", Routes: []string{"/api/api-catalog/{id}/keys", "/api/api-catalog/{id}/key-management"}, MinRole: "viewer", Permission: "apis.keys.manage"},
	)
	return out
}

// ScopeUsable reports whether an owner with role (and hasPerm answering for
// permission codes) could use scope at all — a token can't be given more than
// its owner has. Unknown scopes are not usable.
func ScopeUsable(role string, hasPerm func(string) bool, scope string) bool {
	for _, s := range Catalogue() {
		if s.Name != scope {
			continue
		}
		if s.Permission != "" {
			return hasPerm(s.Permission)
		}
		return hasMinRole(role, s.MinRole)
	}
	return false
}

// KeycloakCatalogue is the catalogue as Keycloak client scopes: every scope
// with the "bridge:" prefix, minus the "*" wildcard (Keycloak clients carry
// explicit scopes only).
func KeycloakCatalogue() []ScopeInfo {
	out := []ScopeInfo{}
	for _, s := range Catalogue() {
		if s.Name == ScopeAll {
			continue
		}
		s.Name = BridgeScopePrefix + ":" + s.Name
		out = append(out, s)
	}
	return out
}

// LeastRoleFor is the lowest role that can use every scope (unprefixed),
// or "" when even admin can't (an unknown scope).
func LeastRoleFor(scopes []string, hasPerm func(role, code string) bool) string {
	for _, role := range []string{"viewer", "editor", "admin"} {
		ok := true
		for _, s := range scopes {
			if !ScopeUsable(role, func(code string) bool { return hasPerm(role, code) }, s) {
				ok = false
				break
			}
		}
		if ok {
			return role
		}
	}
	return ""
}
