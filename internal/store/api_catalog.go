package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// APICatalogRepo owns SQL for api_catalog + api_operations (the Atlas REST API
// catalog). The model's Validate() invariants still live on the type; this repo
// calls them before writes.
type APICatalogRepo struct {
	db *sql.DB
}

// NewAPICatalogRepo constructs an APICatalogRepo over the given DB handle.
func NewAPICatalogRepo(db *sql.DB) *APICatalogRepo { return &APICatalogRepo{db: db} }

// Create inserts the catalog row and its operation index in one transaction.
// a.SpecJSON must be set (canonical JSON from apicatalog.Parse).
func (r *APICatalogRepo) Create(ctx context.Context, a *models.APICatalog, ops []models.APIOperation) error {
	if err := a.Validate(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
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
	if err := insertAPIOperations(ctx, tx, id, ops); err != nil {
		return err
	}
	return tx.Commit()
}

func insertAPIOperations(ctx context.Context, tx *sql.Tx, apiID int64, ops []models.APIOperation) error {
	for i, op := range ops {
		tags := op.Tags
		if tags == nil {
			tags = []string{}
		}
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
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

// Get loads a live catalog row with its operation index (no SpecJSON), or
// (nil, nil) when not found / soft-deleted.
func (r *APICatalogRepo) Get(ctx context.Context, id int64) (*models.APICatalog, error) {
	a := &models.APICatalog{}
	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	err := r.db.QueryRowContext(ctx,
		`SELECT id, scope, parent_id, name, description, source_type, source_url, external_url, base_url, docs_url,
			spec_version, spec_hash, title, version_label, owner_user_id, created_by, created_at, updated_at
		FROM api_catalog WHERE id = ? AND deleted_at IS NULL AND `+vis, append([]any{id}, vargs...)...,
	).Scan(&a.ID, &a.Scope, &a.ParentID, &a.Name, &a.Description, &a.SourceType, &a.SourceURL, &a.ExternalURL, &a.BaseURL, &a.DocsURL,
		&a.SpecVersion, &a.SpecHash, &a.Title, &a.VersionLabel, &a.OwnerUserID, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ops, err := r.listOperations(ctx, id)
	if err != nil {
		return nil, err
	}
	a.Operations = ops
	a.OperationCount = len(ops)
	return a, nil
}

// GetSpec returns the canonical spec JSON for a live catalog row, or ("", nil).
func (r *APICatalogRepo) GetSpec(ctx context.Context, id int64) (string, error) {
	var spec string
	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	err := r.db.QueryRowContext(ctx, `SELECT spec_json FROM api_catalog WHERE id = ? AND deleted_at IS NULL AND `+vis, append([]any{id}, vargs...)...).Scan(&spec)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return spec, err
}

func (r *APICatalogRepo) listOperations(ctx context.Context, apiID int64) ([]models.APIOperation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, api_id, method, path, operation_id, summary, description, tags, op_key, sort_order
		FROM api_operations WHERE api_id = ? ORDER BY sort_order, id`, apiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ops []models.APIOperation
	for rows.Next() {
		var op models.APIOperation
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

// List returns live catalog rows (no operations, no SpecJSON) with an
// OperationCount, filtered + searched per f.
func (r *APICatalogRepo) List(ctx context.Context, f models.APICatalogFilter) ([]models.APICatalog, error) {
	q := `SELECT c.id, c.scope, c.parent_id, c.name, c.description, c.source_type, c.source_url, c.external_url, c.base_url, c.docs_url,
			c.spec_version, c.spec_hash, c.title, c.version_label, c.owner_user_id, c.created_by, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM api_operations o WHERE o.api_id = c.id) AS op_count
		FROM api_catalog c WHERE c.deleted_at IS NULL`
	vis, args := VisibleExpr(ctx, AssetAPICatalog, "c.id")
	q += " AND " + vis
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

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.APICatalog
	for rows.Next() {
		var a models.APICatalog
		if err := rows.Scan(&a.ID, &a.Scope, &a.ParentID, &a.Name, &a.Description, &a.SourceType, &a.SourceURL,
			&a.ExternalURL, &a.BaseURL, &a.DocsURL, &a.SpecVersion, &a.SpecHash, &a.Title, &a.VersionLabel, &a.OwnerUserID, &a.CreatedBy,
			&a.CreatedAt, &a.UpdatedAt, &a.OperationCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SearchOperations finds endpoints across live APIs matching query against
// method/path/operation_id/summary/description/tags, optionally scoped.
func (r *APICatalogRepo) SearchOperations(ctx context.Context, query, scope string, parentID *int64) ([]models.OperationSearchResult, error) {
	q := `SELECT c.id, c.name, c.scope, o.method, o.path, o.op_key, o.summary, o.description, o.tags
		FROM api_operations o JOIN api_catalog c ON c.id = o.api_id
		WHERE c.deleted_at IS NULL`
	vis, args := VisibleExpr(ctx, AssetAPICatalog, "c.id")
	q += " AND " + vis
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

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OperationSearchResult
	for rows.Next() {
		var res models.OperationSearchResult
		var tagsJSON string
		if err := rows.Scan(&res.APIID, &res.APIName, &res.Scope, &res.Method, &res.Path, &res.OpKey, &res.Summary, &res.Description, &tagsJSON); err != nil {
			return nil, err
		}
		if tagsJSON != "" {
			_ = json.Unmarshal([]byte(tagsJSON), &res.Tags)
		}
		if res.Tags == nil {
			res.Tags = []string{}
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

// UpdateMeta renames / re-describes a catalog row and updates base_url + docs_url
// (the spec itself is untouched — use UpdateSpec for that).
func (r *APICatalogRepo) UpdateMeta(ctx context.Context, id int64, name, description, baseURL, docsURL string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_catalog SET name = ?, description = ?, base_url = ?, docs_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL AND `+vis, append([]any{name, description, baseURL, docsURL, id}, vargs...)...)
	return err
}

// UpdateSpec replaces the stored spec + operation index in one transaction
// (used on re-import / re-fetch).
func (r *APICatalogRepo) UpdateSpec(ctx context.Context, id int64, specJSON, specHash, specVersion, title, versionLabel, externalURL string, ops []models.APIOperation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	res, err := tx.ExecContext(ctx,
		`UPDATE api_catalog SET spec_json = ?, spec_hash = ?, spec_version = ?, title = ?,
			version_label = ?, external_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL AND `+vis,
		append([]any{specJSON, specHash, specVersion, title, versionLabel, externalURL, id}, vargs...)...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows // invisible or gone: don't touch its operations
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_operations WHERE api_id = ?`, id); err != nil {
		return err
	}
	if err := insertAPIOperations(ctx, tx, id, ops); err != nil {
		return err
	}
	return tx.Commit()
}

// SoftDelete marks a catalog row deleted. Its operations remain (FK CASCADE only
// fires on hard delete) but are unreachable via the live queries above.
func (r *APICatalogRepo) SoftDelete(ctx context.Context, id int64) error {
	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	_, err := r.db.ExecContext(ctx, `UPDATE api_catalog SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL AND `+vis, append([]any{id}, vargs...)...)
	return err
}
