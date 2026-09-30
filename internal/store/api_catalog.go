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
			(name, description, source_type, source_url, external_url, base_url, docs_url,
			 spec_version, spec_json, spec_hash, title, version_label, owner_user_id, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Name, a.Description, a.SourceType, a.SourceURL, a.ExternalURL, a.BaseURL, a.DocsURL,
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
		`SELECT id, name, description, source_type, source_url, external_url, base_url, docs_url,
			spec_version, spec_hash, title, version_label, owner_user_id, created_by, created_at, updated_at
		FROM api_catalog WHERE id = ? AND deleted_at IS NULL AND `+vis, append([]any{id}, vargs...)...,
	).Scan(&a.ID, &a.Name, &a.Description, &a.SourceType, &a.SourceURL, &a.ExternalURL, &a.BaseURL, &a.DocsURL,
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
	links, err := r.LinksBulk(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	a.ServiceIDs, a.ProjectIDs = links.Services[id], links.Projects[id]
	normalizeAPILinks(a)
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
	q := `SELECT c.id, c.name, c.description, c.source_type, c.source_url, c.external_url, c.base_url, c.docs_url,
			c.spec_version, c.spec_hash, c.title, c.version_label, c.owner_user_id, c.created_by, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM api_operations o WHERE o.api_id = c.id) AS op_count
		FROM api_catalog c WHERE c.deleted_at IS NULL`
	vis, args := VisibleExpr(ctx, AssetAPICatalog, "c.id")
	q += " AND " + vis
	q, args = apiLinkFilter(q, args, f.ServiceID, f.ProjectID)
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
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.SourceType, &a.SourceURL,
			&a.ExternalURL, &a.BaseURL, &a.DocsURL, &a.SpecVersion, &a.SpecHash, &a.Title, &a.VersionLabel, &a.OwnerUserID, &a.CreatedBy,
			&a.CreatedAt, &a.UpdatedAt, &a.OperationCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.withLinks(ctx, out)
}

// withLinks fills ServiceIDs/ProjectIDs on a page of rows in one query each.
func (r *APICatalogRepo) withLinks(ctx context.Context, apis []models.APICatalog) ([]models.APICatalog, error) {
	ids := make([]int64, len(apis))
	for i := range apis {
		ids[i] = apis[i].ID
	}
	links, err := r.LinksBulk(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range apis {
		apis[i].ServiceIDs, apis[i].ProjectIDs = links.Services[apis[i].ID], links.Projects[apis[i].ID]
		normalizeAPILinks(&apis[i])
	}
	return apis, nil
}

func normalizeAPILinks(a *models.APICatalog) {
	if a.ServiceIDs == nil {
		a.ServiceIDs = []int64{}
	}
	if a.ProjectIDs == nil {
		a.ProjectIDs = []int64{}
	}
}

// apiLinkFilter narrows a query over api_catalog c to APIs linked to a
// service, and/or to a project — directly or through one of its live
// services (the same indirection the relations use for hosts and DNS).
func apiLinkFilter(q string, args []any, serviceID, projectID int64) (string, []any) {
	if serviceID != 0 {
		q += " AND c.id IN (SELECT api_id FROM api_service_links WHERE service_id = ?)"
		args = append(args, serviceID)
	}
	if projectID != 0 {
		q += ` AND c.id IN (SELECT api_id FROM api_project_links WHERE project_id = ?
			UNION SELECT l.api_id FROM api_service_links l JOIN services s ON s.id = l.service_id
			 WHERE s.project_id = ? AND s.deleted_at IS NULL)`
		args = append(args, projectID, projectID)
	}
	return q, args
}

// APILinks maps api id → linked service / project ids.
type APILinks struct {
	Services map[int64][]int64
	Projects map[int64][]int64
}

// LinksBulk loads the direct links of the given APIs. Links to trashed
// services or projects are left out.
func (r *APICatalogRepo) LinksBulk(ctx context.Context, apiIDs []int64) (APILinks, error) {
	out := APILinks{Services: map[int64][]int64{}, Projects: map[int64][]int64{}}
	if len(apiIDs) == 0 {
		return out, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(apiIDs)), ",")
	args := make([]any, len(apiIDs))
	for i, id := range apiIDs {
		args[i] = id
	}
	for _, q := range []struct {
		dst map[int64][]int64
		sql string
	}{
		{out.Services, `SELECT l.api_id, l.service_id FROM api_service_links l JOIN services s ON s.id = l.service_id
			WHERE s.deleted_at IS NULL AND l.api_id IN (` + ph + `) ORDER BY l.service_id`},
		{out.Projects, `SELECT l.api_id, l.project_id FROM api_project_links l JOIN projects p ON p.id = l.project_id
			WHERE p.deleted_at IS NULL AND l.api_id IN (` + ph + `) ORDER BY l.project_id`},
	} {
		rows, err := r.db.QueryContext(ctx, q.sql, args...)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var apiID, otherID int64
			if err := rows.Scan(&apiID, &otherID); err != nil {
				rows.Close()
				return out, err
			}
			q.dst[apiID] = append(q.dst[apiID], otherID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}
	return out, nil
}

// SetLinks replaces an API's service and project links. A nil slice leaves
// that side untouched; an empty one clears it.
func (r *APICatalogRepo) SetLinks(ctx context.Context, apiID int64, serviceIDs, projectIDs []int64) error {
	if serviceIDs != nil {
		if err := replaceLinks(ctx, r.db, `api_service_links`, `api_id`, `service_id`, apiID, dedupeIDs(serviceIDs)); err != nil {
			return err
		}
	}
	if projectIDs != nil {
		if err := replaceLinks(ctx, r.db, `api_project_links`, `api_id`, `project_id`, apiID, dedupeIDs(projectIDs)); err != nil {
			return err
		}
	}
	return nil
}

func dedupeIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// SearchOperations finds endpoints across live APIs matching query against
// method/path/operation_id/summary/description/tags, optionally scoped.
func (r *APICatalogRepo) SearchOperations(ctx context.Context, query string, serviceID, projectID int64) ([]models.OperationSearchResult, error) {
	q := `SELECT c.id, c.name, o.method, o.path, o.op_key, o.summary, o.description, o.tags
		FROM api_operations o JOIN api_catalog c ON c.id = o.api_id
		WHERE c.deleted_at IS NULL`
	vis, args := VisibleExpr(ctx, AssetAPICatalog, "c.id")
	q += " AND " + vis
	q, args = apiLinkFilter(q, args, serviceID, projectID)
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
		if err := rows.Scan(&res.APIID, &res.APIName, &res.Method, &res.Path, &res.OpKey, &res.Summary, &res.Description, &tagsJSON); err != nil {
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

// SoftDelete marks a catalog row deleted. Its operations and links remain (FK
// CASCADE only fires on hard delete) but are unreachable via the live queries
// above, so Restore brings the API back whole. found is false when no live,
// visible row matched.
func (r *APICatalogRepo) SoftDelete(ctx context.Context, id int64) (found bool, err error) {
	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	res, err := r.db.ExecContext(ctx, `UPDATE api_catalog SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL AND `+vis, append([]any{id}, vargs...)...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Restore clears deleted_at. found is false when no trashed, visible row matched.
func (r *APICatalogRepo) Restore(ctx context.Context, id int64) (found bool, err error) {
	vis, vargs := VisibleExpr(ctx, AssetAPICatalog, "api_catalog.id")
	res, err := r.db.ExecContext(ctx, `UPDATE api_catalog SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NOT NULL AND `+vis, append([]any{id}, vargs...)...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListTrash returns soft-deleted, visible APIs (no operations), newest first.
func (r *APICatalogRepo) ListTrash(ctx context.Context) ([]models.APICatalog, error) {
	vis, args := VisibleExpr(ctx, AssetAPICatalog, "c.id")
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.id, c.name, c.description, c.source_type, c.title, c.version_label, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM api_operations o WHERE o.api_id = c.id)
		FROM api_catalog c WHERE c.deleted_at IS NOT NULL AND `+vis+` ORDER BY c.deleted_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.APICatalog{}
	for rows.Next() {
		var a models.APICatalog
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.SourceType, &a.Title, &a.VersionLabel, &a.CreatedAt, &a.UpdatedAt, &a.OperationCount); err != nil {
			return nil, err
		}
		normalizeAPILinks(&a)
		out = append(out, a)
	}
	return out, rows.Err()
}
