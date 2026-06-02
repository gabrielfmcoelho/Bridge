package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
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
// only by GetAPICatalogSpec (it can be large); list/get omit it so payloads
// stay lean.
type APICatalog struct {
	ID           int64          `json:"id"`
	Scope        string         `json:"scope"`
	ParentID     *int64         `json:"parent_id,omitempty"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	SourceType   string         `json:"source_type"`
	SourceURL    string         `json:"source_url,omitempty"`  // where the spec was fetched (json/yaml)
	ExternalURL  string         `json:"external_url,omitempty"` // server derived from the spec
	BaseURL      string         `json:"base_url,omitempty"`     // explicit API host (Scalar server override)
	DocsURL      string         `json:"docs_url,omitempty"`     // human docs page (open externally)
	SpecVersion  string         `json:"spec_version"`
	SpecHash     string         `json:"spec_hash"`
	Title        string         `json:"title"`
	VersionLabel string         `json:"version_label"`
	OwnerUserID  int64          `json:"owner_user_id"`
	CreatedBy    int64          `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	OperationCount int          `json:"operation_count"`
	Operations   []APIOperation `json:"operations,omitempty"`

	// SpecJSON is the canonical normalized spec. Carried on the struct so
	// Create can persist it, but excluded from JSON output — the raw spec is
	// served via the dedicated /spec endpoint, not inlined in list/get.
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

// APICatalogFilter narrows ListAPICatalog. Empty fields are ignored.
type APICatalogFilter struct {
	Scope    string
	ParentID *int64
	Query    string // matches name/title/description (case-insensitive)
}

// Validate enforces the scope/parent_id and source_type invariants that the
// DB CHECKs also guard, so the model rejects bad input before any write.
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

// --- CRUD -------------------------------------------------------------------

// CreateAPICatalog inserts the catalog row and its operation index in one
// transaction. a.SpecJSON must be set (canonical JSON from apicatalog.Parse).
func CreateAPICatalog(db *sql.DB, a *APICatalog, ops []APIOperation) error {
	if err := a.Validate(); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	id, err := database.InsertReturningID(tx,
		`INSERT INTO api_catalog
			(scope, parent_id, name, description, source_type, source_url, external_url, base_url, docs_url,
			 spec_version, spec_json, spec_hash, title, version_label, owner_user_id, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Scope, a.ParentID, a.Name, a.Description, a.SourceType, a.SourceURL, a.ExternalURL, a.BaseURL, a.DocsURL,
		a.SpecVersion, a.SpecJSON, a.SpecHash, a.Title, a.VersionLabel, a.OwnerUserID, a.CreatedBy,
	)
	if err != nil {
		return err
	}
	a.ID = id
	if err := insertOperations(tx, id, ops); err != nil {
		return err
	}
	return tx.Commit()
}

func insertOperations(tx *sql.Tx, apiID int64, ops []APIOperation) error {
	for i, op := range ops {
		tags := op.Tags
		if tags == nil {
			tags = []string{}
		}
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO api_operations
				(api_id, method, path, operation_id, summary, description, tags, op_key, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			apiID, op.Method, op.Path, op.OperationID, op.Summary, op.Description, string(tagsJSON), op.OpKey, i,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetAPICatalog loads a live catalog row with its operation index (no
// SpecJSON). Returns (nil, nil) when not found or soft-deleted.
func GetAPICatalog(db *sql.DB, id int64) (*APICatalog, error) {
	a := &APICatalog{}
	err := db.QueryRow(
		`SELECT id, scope, parent_id, name, description, source_type, source_url, external_url, base_url, docs_url,
			spec_version, spec_hash, title, version_label, owner_user_id, created_by, created_at, updated_at
		FROM api_catalog WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&a.ID, &a.Scope, &a.ParentID, &a.Name, &a.Description, &a.SourceType, &a.SourceURL, &a.ExternalURL, &a.BaseURL, &a.DocsURL,
		&a.SpecVersion, &a.SpecHash, &a.Title, &a.VersionLabel, &a.OwnerUserID, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ops, err := listOperations(db, id)
	if err != nil {
		return nil, err
	}
	a.Operations = ops
	a.OperationCount = len(ops)
	return a, nil
}

// GetAPICatalogSpec returns the canonical spec JSON for a live catalog row.
// Returns ("", nil) when not found.
func GetAPICatalogSpec(db *sql.DB, id int64) (string, error) {
	var spec string
	err := db.QueryRow(`SELECT spec_json FROM api_catalog WHERE id = ? AND deleted_at IS NULL`, id).Scan(&spec)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return spec, err
}

func listOperations(db *sql.DB, apiID int64) ([]APIOperation, error) {
	rows, err := db.Query(
		`SELECT id, api_id, method, path, operation_id, summary, description, tags, op_key, sort_order
		FROM api_operations WHERE api_id = ? ORDER BY sort_order, id`, apiID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []APIOperation
	for rows.Next() {
		var op APIOperation
		var tagsJSON string
		if err := rows.Scan(&op.ID, &op.APIID, &op.Method, &op.Path, &op.OperationID,
			&op.Summary, &op.Description, &tagsJSON, &op.OpKey, &op.SortOrder); err != nil {
			return nil, err
		}
		if tagsJSON != "" {
			_ = json.Unmarshal([]byte(tagsJSON), &op.Tags)
		}
		if op.Tags == nil {
			op.Tags = []string{}
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

// ListAPICatalog returns live catalog rows (no operations, no SpecJSON) with
// an OperationCount, filtered + searched per f.
func ListAPICatalog(db *sql.DB, f APICatalogFilter) ([]APICatalog, error) {
	q := `SELECT c.id, c.scope, c.parent_id, c.name, c.description, c.source_type, c.source_url, c.external_url, c.base_url, c.docs_url,
			c.spec_version, c.spec_hash, c.title, c.version_label, c.owner_user_id, c.created_by, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM api_operations o WHERE o.api_id = c.id) AS op_count
		FROM api_catalog c WHERE c.deleted_at IS NULL`
	var args []any
	if f.Scope != "" {
		q += " AND c.scope = ?"
		args = append(args, f.Scope)
	}
	if f.ParentID != nil {
		q += " AND c.parent_id = ?"
		args = append(args, *f.ParentID)
	}
	if f.Query != "" {
		like := "%" + strings.ToLower(f.Query) + "%"
		q += " AND (LOWER(c.name) LIKE ? OR LOWER(c.title) LIKE ? OR LOWER(c.description) LIKE ?)"
		args = append(args, like, like, like)
	}
	q += " ORDER BY c.name"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APICatalog
	for rows.Next() {
		var a APICatalog
		if err := rows.Scan(&a.ID, &a.Scope, &a.ParentID, &a.Name, &a.Description, &a.SourceType, &a.SourceURL,
			&a.ExternalURL, &a.BaseURL, &a.DocsURL, &a.SpecVersion, &a.SpecHash, &a.Title, &a.VersionLabel, &a.OwnerUserID, &a.CreatedBy,
			&a.CreatedAt, &a.UpdatedAt, &a.OperationCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// OperationSearchResult is a flattened endpoint hit carrying enough API
// context to link back to the owning catalog row.
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

// SearchOperations finds endpoints across live APIs matching query against
// method/path/operation_id/summary/description/tags, optionally scoped to a
// project. description is the operation's documentation text.
func SearchOperations(db *sql.DB, query, scope string, parentID *int64) ([]OperationSearchResult, error) {
	q := `SELECT c.id, c.name, c.scope, o.method, o.path, o.op_key, o.summary, o.description, o.tags
		FROM api_operations o JOIN api_catalog c ON c.id = o.api_id
		WHERE c.deleted_at IS NULL`
	var args []any
	if scope != "" {
		q += " AND c.scope = ?"
		args = append(args, scope)
	}
	if parentID != nil {
		q += " AND c.parent_id = ?"
		args = append(args, *parentID)
	}
	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		q += ` AND (LOWER(o.method) LIKE ? OR LOWER(o.path) LIKE ? OR LOWER(o.operation_id) LIKE ?
			OR LOWER(o.summary) LIKE ? OR LOWER(o.description) LIKE ? OR LOWER(o.tags) LIKE ?)`
		args = append(args, like, like, like, like, like, like)
	}
	q += " ORDER BY c.name, o.sort_order LIMIT 200"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OperationSearchResult
	for rows.Next() {
		var r OperationSearchResult
		var tagsJSON string
		if err := rows.Scan(&r.APIID, &r.APIName, &r.Scope, &r.Method, &r.Path, &r.OpKey, &r.Summary, &r.Description, &tagsJSON); err != nil {
			return nil, err
		}
		if tagsJSON != "" {
			_ = json.Unmarshal([]byte(tagsJSON), &r.Tags)
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateAPICatalogMeta renames / re-describes a catalog row and updates its
// base_url + docs_url (the spec itself is untouched — use UpdateAPICatalogSpec
// for that).
func UpdateAPICatalogMeta(db *sql.DB, id int64, name, description, baseURL, docsURL string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	_, err := db.Exec(
		`UPDATE api_catalog SET name = ?, description = ?, base_url = ?, docs_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL`, name, description, baseURL, docsURL, id)
	return err
}

// UpdateAPICatalogSpec replaces the stored spec + operation index in one
// transaction (used on re-import / re-fetch).
func UpdateAPICatalogSpec(db *sql.DB, id int64, specJSON, specHash, specVersion, title, versionLabel, externalURL string, ops []APIOperation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE api_catalog SET spec_json = ?, spec_hash = ?, spec_version = ?, title = ?,
			version_label = ?, external_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL`,
		specJSON, specHash, specVersion, title, versionLabel, externalURL, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM api_operations WHERE api_id = ?`, id); err != nil {
		return err
	}
	if err := insertOperations(tx, id, ops); err != nil {
		return err
	}
	return tx.Commit()
}

// SoftDeleteAPICatalog marks a catalog row deleted. Its operations remain
// (FK CASCADE only fires on hard delete) but are unreachable via the live
// queries above.
func SoftDeleteAPICatalog(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE api_catalog SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, id)
	return err
}
