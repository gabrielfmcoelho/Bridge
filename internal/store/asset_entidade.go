package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ErrNoEntidadeInScope: a non-admin tried to create an asset with no creator
// entidade (none given, no primary) and not global — the asset would be
// invisible to its own creator. Handlers map it to 400.
var ErrNoEntidadeInScope = errors.New("no creator entidade: pick one of your entidades or set a primary")

// ErrEntidadeForbidden: a non-admin tried to set a creator entidade outside
// their visible set. Handlers map it to 403.
var ErrEntidadeForbidden = errors.New("creator entidade is outside your scope")

// ResolveGrants merges a request's AssetGrantsInput over the existing grants
// (nil for create) and applies the caller's scope rules:
//   - creator defaults to the caller's primary entidade on create;
//   - a non-admin's creator must be inside their visible set;
//   - responsibles are unrestricted (any entidade may be made responsible —
//     widening who sees your asset is the owner's call, same as is_global);
//   - a non-admin may not end up with no creator AND not global.
//
// Admin / system / absent scope skips the restrictions (bulk imports,
// startup code, tests).
func ResolveGrants(ctx context.Context, in models.AssetGrantsInput, existing *models.AssetGrants) (models.AssetGrants, error) {
	var g models.AssetGrants
	if existing != nil {
		g = *existing
	}
	sc, scoped := ScopeFrom(ctx)
	if in.CreatorEntidadeID != nil {
		g.CreatorEntidadeID = in.CreatorEntidadeID
	} else if existing == nil && scoped && sc.PrimaryEntidadeID != 0 {
		p := sc.PrimaryEntidadeID
		g.CreatorEntidadeID = &p
	}
	if in.ResponsibleEntidadeIDs != nil {
		g.ResponsibleEntidadeIDs = *in.ResponsibleEntidadeIDs
	}
	if in.IsGlobal != nil {
		g.IsGlobal = *in.IsGlobal
	}
	if g.ResponsibleEntidadeIDs == nil {
		g.ResponsibleEntidadeIDs = []int64{}
	}
	if !scoped || sc.Admin {
		return g, nil
	}
	if g.CreatorEntidadeID != nil && !sc.Allows(*g.CreatorEntidadeID) {
		return g, ErrEntidadeForbidden
	}
	if g.CreatorEntidadeID == nil && !g.IsGlobal && len(g.ResponsibleEntidadeIDs) == 0 {
		return g, ErrNoEntidadeInScope
	}
	return g, nil
}

// AssetEntidadeRepo persists per-asset entidade grants.
type AssetEntidadeRepo struct{ db *sql.DB }

// NewAssetEntidadeRepo constructs an AssetEntidadeRepo over the given DB handle.
func NewAssetEntidadeRepo(db *sql.DB) *AssetEntidadeRepo { return &AssetEntidadeRepo{db: db} }

// Replace sets the asset's grants to exactly g (delete + insert). exec may be
// the repo's DB or a caller-owned tx so the grants land atomically with the
// asset insert. A creator id that also appears in responsibles is kept once.
func (r *AssetEntidadeRepo) Replace(ctx context.Context, exec Execer, t AssetType, assetID int64, g models.AssetGrants) error {
	if _, err := exec.ExecContext(ctx, `DELETE FROM asset_entidades WHERE asset_type = ? AND asset_id = ?`, string(t), assetID); err != nil {
		return err
	}
	ins := func(ent *int64, rel string) error {
		_, err := exec.ExecContext(ctx,
			`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES (?, ?, ?, ?)`,
			string(t), assetID, ent, rel)
		return err
	}
	if g.CreatorEntidadeID != nil {
		if err := ins(g.CreatorEntidadeID, "creator"); err != nil {
			return err
		}
	}
	seen := map[int64]bool{}
	for _, id := range g.ResponsibleEntidadeIDs {
		if seen[id] || (g.CreatorEntidadeID != nil && id == *g.CreatorEntidadeID) {
			continue
		}
		seen[id] = true
		id := id
		if err := ins(&id, "responsible"); err != nil {
			return err
		}
	}
	if g.IsGlobal {
		if err := ins(nil, "global"); err != nil {
			return err
		}
	}
	return nil
}

// CopyFrom duplicates the grants of (fromType, fromID) onto (toType, toID) —
// used when a child asset is born from a parent (scan-discovered services
// from their host, tools synced from a service).
func (r *AssetEntidadeRepo) CopyFrom(ctx context.Context, exec Execer, fromType AssetType, fromID int64, toType AssetType, toID int64) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation)
		SELECT ?, ?, entidade_id, relation FROM asset_entidades WHERE asset_type = ? AND asset_id = ?
		ON CONFLICT DO NOTHING`, string(toType), toID, string(fromType), fromID)
	return err
}

// Get returns the asset's grants (zero value, never nil slices, when none).
func (r *AssetEntidadeRepo) Get(ctx context.Context, t AssetType, assetID int64) (models.AssetGrants, error) {
	m, err := r.GetBulk(ctx, t, []int64{assetID})
	if err != nil {
		return models.AssetGrants{}, err
	}
	g, ok := m[assetID]
	if !ok {
		g = models.AssetGrants{ResponsibleEntidadeIDs: []int64{}}
	}
	return g, nil
}

// GetBulk returns grants for many assets of one type, keyed by asset id.
// Assets without rows are absent from the map.
func (r *AssetEntidadeRepo) GetBulk(ctx context.Context, t AssetType, ids []int64) (map[int64]models.AssetGrants, error) {
	out := map[int64]models.AssetGrants{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT asset_id, entidade_id, relation FROM asset_entidades WHERE asset_type = ? AND asset_id = ANY(?) ORDER BY asset_id, relation, entidade_id`,
		string(t), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var aid int64
		var ent *int64
		var rel string
		if err := rows.Scan(&aid, &ent, &rel); err != nil {
			return nil, err
		}
		g := out[aid]
		if g.ResponsibleEntidadeIDs == nil {
			g.ResponsibleEntidadeIDs = []int64{}
		}
		switch rel {
		case "creator":
			g.CreatorEntidadeID = ent
		case "responsible":
			g.ResponsibleEntidadeIDs = append(g.ResponsibleEntidadeIDs, *ent)
		case "global":
			g.IsGlobal = true
		}
		out[aid] = g
	}
	return out, rows.Err()
}

// CreatorNamesBulk returns asset id → creator entidade name for one asset
// type (card/list decoration; replaces the old host "main entidade" label).
func (r *AssetEntidadeRepo) CreatorNamesBulk(ctx context.Context, t AssetType) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ae.asset_id, e.name FROM asset_entidades ae JOIN entidades e ON e.id = ae.entidade_id
		WHERE ae.asset_type = ? AND ae.relation = 'creator'`, string(t))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// BulkReplace applies the same grants to many assets in one tx (admin triage).
func (r *AssetEntidadeRepo) BulkReplace(ctx context.Context, t AssetType, ids []int64, g models.AssetGrants) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, id := range ids {
		if err := r.Replace(ctx, tx, t, id, g); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UnassignedRow is one triage-list entry.
type UnassignedRow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ListUnassigned pages through live rows of an asset type that have no grant
// row at all (admin-only today), for the Settings triage screen.
func (r *AssetEntidadeRepo) ListUnassigned(ctx context.Context, t AssetType, page, perPage int) ([]UnassignedRow, int, error) {
	spec, ok := AssetOf(t)
	if !ok {
		return nil, 0, fmt.Errorf("unknown asset type %q", t)
	}
	where := []string{`NOT EXISTS (SELECT 1 FROM asset_entidades ae WHERE ae.asset_type = ? AND ae.asset_id = x.id)`}
	args := []any{string(t)}
	if spec.SoftDelete {
		where = append(where, "x.deleted_at IS NULL")
	}
	if spec.Extra != "" {
		where = append(where, "("+spec.Extra+")")
	}
	from := fmt.Sprintf(` FROM %s x WHERE %s`, spec.Table, strings.Join(where, " AND "))
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT x.id, x.%s`, spec.NameColumn)+from+` ORDER BY 2, 1 LIMIT ? OFFSET ?`,
		append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []UnassignedRow{}
	for rows.Next() {
		var u UnassignedRow
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}
