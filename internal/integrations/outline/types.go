package outline

import "time"

// envelope is the standard response shape for Outline's JSON API.
// Every endpoint wraps its payload in {data, ok, pagination?}.
type envelope struct {
	Data       any        `json:"data"`
	OK         bool       `json:"ok"`
	Pagination *pageInfo  `json:"pagination,omitempty"`
	Status     int        `json:"status,omitempty"`
	Error      string     `json:"error,omitempty"`
	Message    string     `json:"message,omitempty"`
}

type pageInfo struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total,omitempty"`
}

// AuthInfo is the response from POST /api/auth.info.
type AuthInfo struct {
	User User      `json:"user"`
	Team Workspace `json:"team"`
}

// User represents an Outline user or service account.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Username string `json:"username"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// Workspace maps to an Outline "team" — the tenant/workspace.
type Workspace struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Subdomain  string `json:"subdomain"`
	URL        string `json:"url"`
	AvatarURL  string `json:"avatarUrl,omitempty"`
}

// Collection is a top-level grouping inside the workspace (sidebar section).
type Collection struct {
	ID          string    `json:"id"`
	URLID       string    `json:"urlId"`
	URL         string    `json:"url,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon,omitempty"`
	Permission  string    `json:"permission,omitempty"`
	Sort        any       `json:"sort,omitempty"`
	Index       string    `json:"index,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// BrowseURL builds the canonical Outline URL for this collection.
// Prefer the API's own `url` field (e.g. "/collection/engineering-aB3xYz")
// because Outline's path format sometimes includes a title slug that we'd
// otherwise have to recompute client-side. Fall back to urlId for older
// instances that omit `url`.
func (c *Collection) BrowseURL(base string) string {
	if c == nil {
		return ""
	}
	if c.URL != "" {
		return trimTrailingSlash(base) + c.URL
	}
	if c.URLID == "" {
		return ""
	}
	return trimTrailingSlash(base) + "/collection/" + c.URLID
}

// Document is a markdown page inside a collection.
type Document struct {
	ID              string    `json:"id"`
	URLID           string    `json:"urlId"`
	URL             string    `json:"url,omitempty"`
	Title           string    `json:"title"`
	Text            string    `json:"text"`
	CollectionID    string    `json:"collectionId"`
	ParentDocumentID string   `json:"parentDocumentId,omitempty"`
	Emoji           string    `json:"emoji,omitempty"`
	PublishedAt     *time.Time `json:"publishedAt,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	CreatedBy       *User     `json:"createdBy,omitempty"`
	UpdatedBy       *User     `json:"updatedBy,omitempty"`
}

// BrowseURL returns the canonical doc URL against a given base. Outline's real
// URL format is "/doc/<title-slug>-<urlId>" — we use the API's own `url` field
// when present so the link survives renames and matches what Outline itself
// serves. Reconstructing from urlId alone (e.g. "/doc/jnm6fS576A") 404s on
// newer Outline versions that require the slug prefix.
func (d *Document) BrowseURL(base string) string {
	if d == nil {
		return ""
	}
	if d.URL != "" {
		return trimTrailingSlash(base) + d.URL
	}
	if d.URLID == "" {
		return ""
	}
	return trimTrailingSlash(base) + "/doc/" + d.URLID
}

// DocumentNode is a single entry in Outline's nested document tree
// (response of POST /api/collections.documents). It intentionally carries
// just enough to drive a sidebar nav — full body fetched on-click via
// DocumentInfo.
type DocumentNode struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	URL      string         `json:"url,omitempty"`
	Emoji    string         `json:"emoji,omitempty"`
	Icon     string         `json:"icon,omitempty"`
	Color    string         `json:"color,omitempty"`
	Children []DocumentNode `json:"children,omitempty"`
}

// DocumentCreateInput is the body for POST /api/documents.create.
type DocumentCreateInput struct {
	Title            string `json:"title"`
	Text             string `json:"text,omitempty"`
	CollectionID     string `json:"collectionId"`
	ParentDocumentID string `json:"parentDocumentId,omitempty"`
	Publish          bool   `json:"publish,omitempty"`
}

// DocumentSearchResult is one hit from POST /api/documents.search.
type DocumentSearchResult struct {
	Ranking  float64  `json:"ranking"`
	Context  string   `json:"context"`
	Document Document `json:"document"`
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
