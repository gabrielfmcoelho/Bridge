package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AppDepCatalogPath is where each repository declares its dependency
// catalog (catalog-parser's layout.catalog_dir + "dependencies.yml", as set
// by catalog-governance).
const AppDepCatalogPath = "catalog/dependencies.yml"

// AppDepCatalog is one repository's catalog/dependencies.yml, in the schema
// of catalog-parser (models.py): the system (CI), its components and the
// CIs it links to in each direction. Bridge only reads it: the linting and
// the vocabulary belong to catalog-parser and catalog-governance.
type AppDepCatalog struct {
	CI           string            `yaml:"ci" json:"ci"`
	Description  string            `yaml:"description" json:"description"`
	Owners       []AppDepOwner     `yaml:"owners" json:"owners"`
	Entity       []AppDepEntity    `yaml:"entity" json:"entity"`
	Git          *AppDepGit        `yaml:"git" json:"git"`
	Metadata     map[string]any    `yaml:"metadata" json:"metadata"`
	Components   []AppDepComponent `yaml:"components" json:"components"`
	Dependencies AppDepLinks       `yaml:"dependencies" json:"dependencies"`
}

type AppDepOwner struct {
	Name     string `yaml:"name" json:"name"`
	Email    string `yaml:"email" json:"email"`
	Username string `yaml:"username" json:"username"`
}

type AppDepEntity struct {
	Name string `yaml:"name" json:"name"`
	Slug string `yaml:"slug" json:"slug"`
	Type string `yaml:"type" json:"type"`
}

type AppDepGit struct {
	Type string `yaml:"type" json:"type"`
	Host string `yaml:"host" json:"host"`
	Path string `yaml:"path" json:"path"`
}

// AppDepComponent is a part of the CI itself; its traits live here.
type AppDepComponent struct {
	Name        string         `yaml:"name" json:"name"`
	Category    string         `yaml:"category" json:"category"`
	Technology  string         `yaml:"technology" json:"technology"`
	Protocol    string         `yaml:"protocol" json:"protocol,omitempty"`
	Port        int            `yaml:"port" json:"port,omitempty"`
	Criticality string         `yaml:"criticality" json:"criticality"`
	Description string         `yaml:"description" json:"description"`
	Metadata    map[string]any `yaml:"metadata" json:"metadata,omitempty"`
}

type AppDepLinks struct {
	Upstream   []AppDepLink `yaml:"upstream" json:"upstream"`
	Downstream []AppDepLink `yaml:"downstream" json:"downstream"`
}

// AppDepLink is one end of a link: CI + Component name the OTHER end,
// LocalComponent this CI's end. An external system (no catalog of its own)
// has no Component and carries its traits inline.
type AppDepLink struct {
	CI             string         `yaml:"ci" json:"ci"`
	Component      string         `yaml:"component" json:"component,omitempty"`
	LocalComponent string         `yaml:"local_component" json:"local_component,omitempty"`
	Via            string         `yaml:"via" json:"via,omitempty"`
	Category       string         `yaml:"category" json:"category,omitempty"`
	Technology     string         `yaml:"technology" json:"technology,omitempty"`
	Protocol       string         `yaml:"protocol" json:"protocol,omitempty"`
	Port           int            `yaml:"port" json:"port,omitempty"`
	Criticality    string         `yaml:"criticality" json:"criticality,omitempty"`
	Description    string         `yaml:"description" json:"description,omitempty"`
	Metadata       map[string]any `yaml:"metadata" json:"metadata,omitempty"`
}

// AppDepSource is one GitLab repository's catalog as last synced. Catalog is
// nil when the file failed to parse (ParseError says why).
type AppDepSource struct {
	GitLabProjectID int            `json:"gitlab_project_id"`
	ProjectPath     string         `json:"project_path"`
	WebURL          string         `json:"web_url"`
	Ref             string         `json:"ref"`
	CommitID        string         `json:"commit_id"`
	FileURL         string         `json:"file_url"`
	Catalog         *AppDepCatalog `json:"catalog"`
	ParseError      string         `json:"parse_error,omitempty"`
	SyncedAt        time.Time      `json:"synced_at"`
}

// AppDepLinkTypes are the assets an application links to.
var AppDepLinkTypes = []string{"host", "dns", "service", "project"}

// AppDepAssetLink sources, most specific first.
const (
	AppDepSourceManual = "manual" // an editor added it in Bridge
	AppDepSourceRepo   = "repo"   // the asset is linked to the CI's repository
	AppDepSourceScan   = "scan"   // a host scan found a checkout of the repository
	AppDepSourceGroup  = "group"  // the asset is linked to a GitLab group holding the repository
)

// AppDepAssetLink is one application (CI) ↔ asset link, with every source
// it comes from. CIID is the GitLab project id of the CI's catalog: the
// number lists and graphs key the application by.
type AppDepAssetLink struct {
	CIID       int      `json:"ci_id"`
	CI         string   `json:"ci"`
	EntityType string   `json:"entity_type"`
	EntityID   int64    `json:"entity_id"`
	Sources    []string `json:"sources"`
	// Via is the repository or group path a derived link goes through.
	Via string `json:"via,omitempty"`
}

// AppDepManualLink is a stored link (app_dep_links).
type AppDepManualLink struct {
	CI         string `json:"ci"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
}

// AppDepFileURL is the browser link of a repository's catalog at ref.
func AppDepFileURL(webURL, ref string) string {
	if webURL == "" {
		return ""
	}
	return strings.TrimRight(webURL, "/") + "/-/blob/" + ref + "/" + AppDepCatalogPath
}

// ParseAppDepCatalog reads a catalog/dependencies.yml. It is not a linter:
// unknown keys are ignored and values are not checked against any
// vocabulary. It fails only on invalid YAML, a top level that is not a
// mapping, a value of the wrong type (port: abc) or a missing ci.
func ParseAppDepCatalog(data []byte) (*AppDepCatalog, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("top level is not a mapping")
	}
	var c AppDepCatalog
	if err := root.Content[0].Decode(&c); err != nil {
		return nil, err
	}
	c.CI = strings.TrimSpace(c.CI)
	if c.CI == "" {
		return nil, errors.New("ci is missing")
	}
	c.normalize()
	return &c, nil
}

// normalize turns absent lists into empty ones (the JSON carries no null
// arrays) and metadata into JSON-encodable values.
func (c *AppDepCatalog) normalize() {
	if c.Owners == nil {
		c.Owners = []AppDepOwner{}
	}
	if c.Entity == nil {
		c.Entity = []AppDepEntity{}
	}
	if c.Components == nil {
		c.Components = []AppDepComponent{}
	}
	if c.Dependencies.Upstream == nil {
		c.Dependencies.Upstream = []AppDepLink{}
	}
	if c.Dependencies.Downstream == nil {
		c.Dependencies.Downstream = []AppDepLink{}
	}
	c.Metadata = jsonSafeMap(c.Metadata)
	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}
	for i := range c.Components {
		c.Components[i].Metadata = jsonSafeMap(c.Components[i].Metadata)
	}
	for _, l := range [][]AppDepLink{c.Dependencies.Upstream, c.Dependencies.Downstream} {
		for i := range l {
			l[i].Metadata = jsonSafeMap(l[i].Metadata)
		}
	}
}

func jsonSafeMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = jsonSafe(v)
	}
	return out
}

// jsonSafe rewrites what YAML allows and JSON doesn't: a map with
// non-string keys (1: x) becomes a map keyed by their text.
func jsonSafe(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return jsonSafeMap(t)
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, v := range t {
			out[fmt.Sprint(k)] = jsonSafe(v)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, v := range t {
			out[i] = jsonSafe(v)
		}
		return out
	}
	return v
}
