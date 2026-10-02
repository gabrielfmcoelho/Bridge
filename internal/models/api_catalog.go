package models

import (
	"fmt"
	"strings"
	"time"
)

// APICatalog source-type domain. (The old projeto/avulso scope gave way to
// the api_service_links / api_project_links tables in v95.)
const (
	APIKeyManagementNone     = "none"     // no access keys tracked
	APIKeyManagementManual   = "manual"   // keys created elsewhere, registered in Bridge
	APIKeyManagementKeycloak = "keycloak" // one Keycloak client per key, managed through the keycloak_apis integration

	APICatalogSourceUpload = "upload"
	APICatalogSourceURL    = "url"
	// APICatalogSourceManual: registered without an OpenAPI spec (no operations).
	APICatalogSourceManual = "manual"
)

// APICatalog is a single imported REST API specification. SpecJSON is loaded
// only by the repo's GetSpec (it can be large); list/get omit it so payloads
// stay lean. Persistence lives in internal/store.APICatalogRepo — this file is
// the pure data types + validation only.
type APICatalog struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	UseCases       string         `json:"use_cases"`  // Markdown: who calls it and for what
	Origem         string         `json:"origem"`     // propria | terceiro | externa
	Fornecedor     string         `json:"fornecedor"` // who builds/provides it (vendor, other org)
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
	// ServiceIDs / ProjectIDs are the direct links (api_service_links,
	// api_project_links); always set, [] when none — an API with neither is
	// "avulso".
	ServiceIDs []int64 `json:"service_ids"`
	ProjectIDs []int64 `json:"project_ids"`
	// ConsumerServiceIDs are the services that call the API
	// (api_consumer_links); always set, [] when none.
	ConsumerServiceIDs []int64 `json:"consumer_service_ids"`
	// MainResponsavelName is the main internal responsável (list responses).
	MainResponsavelName string `json:"main_responsavel_name,omitempty"`
	// URLs are the API's other addresses (e.g. its origin when BaseURL is the
	// gateway), in display order; always set, [] when none.
	URLs []APICatalogURL `json:"urls"`
	// Responsaveis is loaded on detail responses only.
	Responsaveis []Responsavel `json:"responsaveis,omitempty"`

	// Key management. AdminBaseURL is the API's root, where its GET /escopos
	// and GET /admin/uso live (keycloak mode); ScopePrefix names its scopes
	// and Keycloak clients ("servidores" → "servidores:cadastro",
	// "servidores-<label>").
	KeyManagement string `json:"key_management"`
	AdminBaseURL  string `json:"admin_base_url,omitempty"`
	ScopePrefix   string `json:"scope_prefix"`
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
	ServiceID int64  // linked to this service
	ProjectID int64  // linked to this project, directly or through one of its services
	Query     string // matches name/title/description (case-insensitive)
}

// OperationSearchResult is a flattened endpoint hit carrying enough API context
// to link back to the owning catalog row.
type OperationSearchResult struct {
	APIID       int64    `json:"api_id"`
	APIName     string   `json:"api_name"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OpKey       string   `json:"op_key"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
}

// ValidKeyManagement reports whether m is a known key_management mode.
func ValidKeyManagement(m string) bool {
	switch m {
	case APIKeyManagementNone, APIKeyManagementManual, APIKeyManagementKeycloak:
		return true
	}
	return false
}

// Validate enforces the source_type invariant the DB CHECK also guards, so the
// model rejects bad input before any write.
// APICatalogURL is one extra address of an API, with a short label
// ("Gateway", "Origem").
type APICatalogURL struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// MaxAPICatalogURLs caps the extra addresses per API.
const MaxAPICatalogURLs = 10

// NormalizeAPIURLs trims, validates (http/https, max lengths, at most
// MaxAPICatalogURLs) and dedupes by URL, keeping the first label. Rows with
// neither label nor URL are dropped. The result is never nil.
func NormalizeAPIURLs(in []APICatalogURL) ([]APICatalogURL, error) {
	out := []APICatalogURL{}
	seen := map[string]bool{}
	for _, u := range in {
		label, raw := strings.TrimSpace(u.Label), strings.TrimSpace(u.URL)
		if label == "" && raw == "" {
			continue
		}
		if raw == "" {
			return nil, fmt.Errorf("url is required (label %q)", label)
		}
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			return nil, fmt.Errorf("url must be http(s): %q", raw)
		}
		if len(raw) > 2048 || len(label) > 80 {
			return nil, fmt.Errorf("url or label too long: %q", raw)
		}
		if seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, APICatalogURL{Label: label, URL: raw})
	}
	if len(out) > MaxAPICatalogURLs {
		return nil, fmt.Errorf("at most %d extra urls", MaxAPICatalogURLs)
	}
	return out, nil
}

func (a *APICatalog) Validate() error {
	switch a.SourceType {
	case APICatalogSourceUpload, APICatalogSourceURL, APICatalogSourceManual:
	default:
		return fmt.Errorf("invalid source_type %q", a.SourceType)
	}
	if !ValidAPIOrigem(a.Origem) {
		return fmt.Errorf("invalid origem %q", a.Origem)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if a.SourceType != APICatalogSourceManual && strings.TrimSpace(a.SpecJSON) == "" {
		return fmt.Errorf("spec_json is required")
	}
	return nil
}

// API origins: built and hosted by us, third-party software we host, or
// hosted elsewhere.
const (
	APIOrigemPropria  = "propria"
	APIOrigemTerceiro = "terceiro"
	APIOrigemExterna  = "externa"
)

// ValidAPIOrigem reports whether o is one of the API origins.
func ValidAPIOrigem(o string) bool {
	return o == APIOrigemPropria || o == APIOrigemTerceiro || o == APIOrigemExterna
}
