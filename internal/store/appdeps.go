package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// AppDepRepo owns SQL for app_dep_catalogs: each GitLab repository's
// catalog/dependencies.yml as last synced (service.AppDepsService).
type AppDepRepo struct {
	db *sql.DB
}

func NewAppDepRepo(db *sql.DB) *AppDepRepo { return &AppDepRepo{db: db} }

// List returns every synced catalog, parsed ones first by CI, then the
// files that failed to parse, by repository path.
func (r *AppDepRepo) List(ctx context.Context) ([]models.AppDepSource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT gitlab_project_id, project_path, web_url, ref, commit_id,
		catalog, parse_error, synced_at FROM app_dep_catalogs ORDER BY ci NULLS LAST, project_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.AppDepSource{}
	for rows.Next() {
		var s models.AppDepSource
		var raw []byte
		if err := rows.Scan(&s.GitLabProjectID, &s.ProjectPath, &s.WebURL, &s.Ref, &s.CommitID,
			&raw, &s.ParseError, &s.SyncedAt); err != nil {
			return nil, err
		}
		if raw != nil {
			if err := json.Unmarshal(raw, &s.Catalog); err != nil {
				return nil, fmt.Errorf("app_dep_catalogs %d: %w", s.GitLabProjectID, err)
			}
		}
		s.FileURL = models.AppDepFileURL(s.WebURL, s.Ref)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Apply writes one sync in a transaction: upserts replace their
// repository's row, remove deletes the rows of those GitLab project ids.
// Rows in neither list are kept.
func (r *AppDepRepo) Apply(ctx context.Context, upserts []models.AppDepSource, remove []int) (removed int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, s := range upserts {
		var ci, catalog any
		if s.Catalog != nil {
			b, mErr := json.Marshal(s.Catalog)
			if mErr != nil {
				return 0, fmt.Errorf("%s: %w", s.ProjectPath, mErr)
			}
			ci, catalog = s.Catalog.CI, string(b)
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO app_dep_catalogs (gitlab_project_id, project_path, web_url, ref, commit_id, ci, catalog, parse_error, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?)
			ON CONFLICT (gitlab_project_id) DO UPDATE SET project_path = EXCLUDED.project_path,
				web_url = EXCLUDED.web_url, ref = EXCLUDED.ref, commit_id = EXCLUDED.commit_id,
				ci = EXCLUDED.ci, catalog = EXCLUDED.catalog, parse_error = EXCLUDED.parse_error,
				synced_at = EXCLUDED.synced_at`,
			s.GitLabProjectID, s.ProjectPath, s.WebURL, s.Ref, s.CommitID, ci, catalog, s.ParseError, s.SyncedAt); err != nil {
			return 0, err
		}
	}
	if len(remove) > 0 {
		args := make([]any, len(remove))
		for i, id := range remove {
			args[i] = id
		}
		res, dErr := tx.ExecContext(ctx, `DELETE FROM app_dep_catalogs WHERE gitlab_project_id IN (`+placeholders(len(remove))+`)`, args...)
		if dErr != nil {
			err = dErr
			return 0, err
		}
		n, _ := res.RowsAffected()
		removed = int(n)
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return removed, nil
}

// IDs returns the GitLab project ids that have a row.
func (r *AppDepRepo) IDs(ctx context.Context) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT gitlab_project_id FROM app_dep_catalogs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// HasCI reports whether a parsed catalog declares ci.
func (r *AppDepRepo) HasCI(ctx context.Context, ci string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM app_dep_catalogs WHERE ci = ?)`, ci).Scan(&ok)
	return ok, err
}

// ManualLinks returns every stored link, whatever its asset's state.
func (r *AppDepRepo) ManualLinks(ctx context.Context) ([]models.AppDepManualLink, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT ci, entity_type, entity_id FROM app_dep_links ORDER BY ci, entity_type, entity_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.AppDepManualLink{}
	for rows.Next() {
		var l models.AppDepManualLink
		if err := rows.Scan(&l.CI, &l.EntityType, &l.EntityID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// AddLink stores a link; adding one that exists is a no-op. The caller has
// checked the CI exists and the asset is visible.
func (r *AppDepRepo) AddLink(ctx context.Context, l models.AppDepManualLink, createdBy int64) error {
	var by any
	if createdBy > 0 {
		by = createdBy
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO app_dep_links (ci, entity_type, entity_id, created_by)
		VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`, l.CI, l.EntityType, l.EntityID, by)
	return err
}

// RemoveLink deletes a stored link; false when there was none.
func (r *AppDepRepo) RemoveLink(ctx context.Context, l models.AppDepManualLink) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM app_dep_links WHERE ci = ? AND entity_type = ? AND entity_id = ?`,
		l.CI, l.EntityType, l.EntityID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ServiceRepoURL is a live service's repository fields (repository_url,
// gitlab_url, git_repository), the non-empty ones.
type ServiceRepoURL struct {
	ID   int64
	URLs []string
}

// ServiceRepoURLs returns every live service that names a repository.
// Unscoped: callers filter by visibility.
func (r *AppDepRepo) ServiceRepoURLs(ctx context.Context) ([]ServiceRepoURL, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, repository_url, gitlab_url, git_repository FROM services
		WHERE deleted_at IS NULL AND (repository_url <> '' OR gitlab_url <> '' OR git_repository <> '')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ServiceRepoURL{}
	for rows.Next() {
		var id int64
		var a, b, c string
		if err := rows.Scan(&id, &a, &b, &c); err != nil {
			return nil, err
		}
		s := ServiceRepoURL{ID: id}
		for _, u := range []string{a, b, c} {
			if u != "" {
				s.URLs = append(s.URLs, u)
			}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// appDepAssets maps an app link's entity type to its asset table.
var appDepAssets = map[string]struct {
	table string
	asset AssetType
}{
	"host":    {"hosts", AssetHost},
	"dns":     {"dns_records", AssetDNS},
	"service": {"services", AssetService},
	"project": {"projects", AssetProject},
}

// LiveVisibleIDs returns, per entity type an application links to, the ids
// of the live (not trashed) assets the caller can see.
func (r *AppDepRepo) LiveVisibleIDs(ctx context.Context) (map[string]map[int64]bool, error) {
	out := map[string]map[int64]bool{}
	for typ, a := range appDepAssets {
		q, args := liveVisibleIDs(ctx, a.table, a.asset)
		rows, err := r.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		ids, err := scanInt64s(rows)
		if err != nil {
			return nil, err
		}
		set := make(map[int64]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		out[typ] = set
	}
	return out, nil
}

// EntityLiveVisible reports whether an asset an application can link to
// exists, is live and is visible to the caller. Unknown types are false.
func (r *AppDepRepo) EntityLiveVisible(ctx context.Context, entityType string, id int64) (bool, error) {
	a, ok := appDepAssets[entityType]
	if !ok {
		return false, nil
	}
	q, args := liveVisibleIDs(ctx, a.table, a.asset)
	var found bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM (`+q+`) v WHERE v.id = ?)`, append(args, id)...).Scan(&found)
	return found, err
}
