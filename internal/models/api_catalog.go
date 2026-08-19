package models

import (
	"fmt"
	"strings"
	"time"
)

// APICatalog scope + source-type domains. Scope mirrors the secrets pattern:
// 'projeto' attaches the API to a project; 'avulso' is standalone.
const (
	APICatalogScopeProjeto = "projeto"
	APICatalogScopeAvulso  = "avulso"

	APICatalogSourceUpload = "upload"
	APICatalogSourceURL    = "url"
)

// APICatalog is a single imported REST API specification. SpecJSON is loaded
// only by the repo's GetSpec (it can be large); list/get omit it so payloads
// stay lean. Persistence lives in internal/store.APICatalogRepo — this file is
// the pure data types + validation only.
type APICatalog struct {
	ID             int64          `json:"id"`
	Scope          string         `json:"scope"`
	ParentID       *int64         `json:"parent_id,omitempty"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	SourceType     string         `json:"source_type"`
	SourceURL      string         `json:"source_url,omitempty"`   // where the spec was fetched (json/yaml)
	ExternalURL    string         `json:"external_url,omitempty"` // server derived from the spec
	BaseURL        string         `json:"base_url,omitempty"`     // explicit API host (Scalar server override)
	DocsURL        string         `json:"docs_url,omitempty"`     // human docs page (open externally)
	SpecVersion    string         `json:"spec_version"`
	SpecHash       string         `json:"spec_hash"`
	Title          string         `json:"title"`
	VersionLabel   string         `json:"version_label"`
	OwnerUserID    int64          `json:"owner_user_id"`
	CreatedBy      int64          `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	OperationCount int            `json:"operation_count"`
	Operations     []APIOperation `json:"operations,omitempty"`
	// Entidades carries the entidade grants on detail responses (edit-form
	// prefill); nil on list rows.
	Entidades *AssetGrants `json:"entidades,omitempty"`

	// SpecJSON is the canonical normalized spec. Carried on the struct so the
	// repo's Create can persist it, but excluded from JSON output — the raw
	// spec is served via the dedicated /spec endpoint, not inlined in list/get.
	SpecJSON string `json:"-"`
}

// APIOperation is one endpoint in the derived operation index.
type APIOperation struct {
	ID          int64    `json:"id"`
	APIID       int64    `json:"api_id"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
	OpKey       string   `json:"op_key"`
	SortOrder   int      `json:"sort_order"`
}

// APICatalogFilter narrows the repo's List. Empty fields are ignored.
type APICatalogFilter struct {
	Scope    string
	ParentID *int64
	Query    string // matches name/title/description (case-insensitive)
}

// OperationSearchResult is a flattened endpoint hit carrying enough API context
// to link back to the owning catalog row.
type OperationSearchResult struct {
	APIID       int64    `json:"api_id"`
	APIName     string   `json:"api_name"`
	Scope       string   `json:"scope"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OpKey       string   `json:"op_key"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
}

// Validate enforces the scope/parent_id and source_type invariants that the DB
// CHECKs also guard, so the model rejects bad input before any write.
func (a *APICatalog) Validate() error {
	switch a.Scope {
	case APICatalogScopeProjeto, APICatalogScopeAvulso:
	default:
		return fmt.Errorf("invalid scope %q", a.Scope)
	}
	switch a.SourceType {
	case APICatalogSourceUpload, APICatalogSourceURL:
	default:
		return fmt.Errorf("invalid source_type %q", a.SourceType)
	}
	if a.Scope == APICatalogScopeAvulso && a.ParentID != nil {
		return fmt.Errorf("avulso api must not have a parent_id")
	}
	if a.Scope == APICatalogScopeProjeto && a.ParentID == nil {
		return fmt.Errorf("projeto api requires a parent_id")
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(a.SpecJSON) == "" {
		return fmt.Errorf("spec_json is required")
	}
	return nil
}
