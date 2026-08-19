package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ErrEntidadeInUse is returned by EntidadeRepo.Delete when the entidade still
// has children, members or asset grants. Handlers map it to 409.
var ErrEntidadeInUse = errors.New("entidade has children, members or grants")

// EntidadeRepo persists the entidade tree.
type EntidadeRepo struct{ db *sql.DB }

// NewEntidadeRepo constructs an EntidadeRepo over the given DB handle.
func NewEntidadeRepo(db *sql.DB) *EntidadeRepo { return &EntidadeRepo{db: db} }

const entidadeCols = `id, name, slug, parent_id, description, created_at, updated_at`

func scanEntidade(s interface{ Scan(...any) error }, e *models.Entidade) error {
	return s.Scan(&e.ID, &e.Name, &e.Slug, &e.ParentID, &e.Description, &e.CreatedAt, &e.UpdatedAt)
}

// List returns every entidade, parents before children (by depth, then name)
// so a UI can render the tree with a single pass.
func (r *EntidadeRepo) List(ctx context.Context) ([]models.Entidade, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE tree AS (
			SELECT e.*, 0 AS depth, lower(e.name)::text AS path FROM entidades e WHERE e.parent_id IS NULL
			UNION ALL
			SELECT c.*, t.depth + 1, t.path || '/' || lower(c.name) FROM entidades c JOIN tree t ON c.parent_id = t.id
		)
		SELECT `+entidadeCols+` FROM tree ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Entidade
	for rows.Next() {
		var e models.Entidade
		if err := scanEntidade(rows, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get returns one entidade or (nil, nil) when absent.
func (r *EntidadeRepo) Get(ctx context.Context, id int64) (*models.Entidade, error) {
	var e models.Entidade
	err := scanEntidade(r.db.QueryRowContext(ctx, `SELECT `+entidadeCols+` FROM entidades WHERE id = ?`, id), &e)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &e, err
}

// Create inserts e and fills ID/timestamps.
func (r *EntidadeRepo) Create(ctx context.Context, e *models.Entidade) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO entidades (name, slug, parent_id, description) VALUES (?, ?, ?, ?) RETURNING id, created_at, updated_at`,
		e.Name, e.Slug, e.ParentID, e.Description).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

// Update rewrites name/slug/parent/description. Re-parenting under one's own
// descendant would create a cycle; the check is done here with the same
// recursive walk the scope uses.
func (r *EntidadeRepo) Update(ctx context.Context, e *models.Entidade) error {
	if e.ParentID != nil {
		if *e.ParentID == e.ID {
			return fmt.Errorf("entidade cannot be its own parent")
		}
		var cycle bool
		if err := r.db.QueryRowContext(ctx, `
			WITH RECURSIVE sub AS (
				SELECT id FROM entidades WHERE id = ?
				UNION SELECT c.id FROM entidades c JOIN sub ON c.parent_id = sub.id)
			SELECT EXISTS (SELECT 1 FROM sub WHERE id = ?)`, e.ID, *e.ParentID).Scan(&cycle); err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("entidade cannot be moved under its own descendant")
		}
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE entidades SET name = ?, slug = ?, parent_id = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		e.Name, e.Slug, e.ParentID, e.Description, e.ID)
	return err
}

// Delete removes an entidade that has no children, members or grants;
// otherwise returns ErrEntidadeInUse (checked up-front so we never depend on
// parsing FK-violation errors).
func (r *EntidadeRepo) Delete(ctx context.Context, id int64) error {
	var inUse bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM entidades WHERE parent_id = ?)
		    OR EXISTS (SELECT 1 FROM user_entidades WHERE entidade_id = ?)
		    OR EXISTS (SELECT 1 FROM asset_entidades WHERE entidade_id = ?)`, id, id, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrEntidadeInUse
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM entidades WHERE id = ?`, id)
	return err
}

// ScopeForUser computes the caller's visible set: every entidade the user
// belongs to plus all descendants, and the primary entidade. Admin is NOT
// set here — the caller decides that from the role.
func (r *EntidadeRepo) ScopeForUser(ctx context.Context, userID int64) (Scope, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE tree AS (
			SELECT ue.entidade_id AS id, ue.is_primary FROM user_entidades ue WHERE ue.user_id = ?
			UNION
			SELECT c.id, FALSE FROM entidades c JOIN tree t ON c.parent_id = t.id)
		SELECT id, bool_or(is_primary) FROM tree GROUP BY id`, userID)
	if err != nil {
		return Scope{}, err
	}
	defer rows.Close()
	sc := Scope{EntidadeIDs: []int64{}}
	for rows.Next() {
		var id int64
		var primary bool
		if err := rows.Scan(&id, &primary); err != nil {
			return Scope{}, err
		}
		sc.EntidadeIDs = append(sc.EntidadeIDs, id)
		if primary {
			sc.PrimaryEntidadeID = id
		}
	}
	return sc, rows.Err()
}

// UserEntidadeRepo persists user ↔ entidade membership.
type UserEntidadeRepo struct{ db *sql.DB }

// NewUserEntidadeRepo constructs a UserEntidadeRepo over the given DB handle.
func NewUserEntidadeRepo(db *sql.DB) *UserEntidadeRepo { return &UserEntidadeRepo{db: db} }

const userEntidadeSelect = `SELECT ue.user_id, e.id, e.name, e.slug, ue.is_primary
	FROM user_entidades ue JOIN entidades e ON e.id = ue.entidade_id`

// ListForUser returns the user's memberships, primary first.
func (r *UserEntidadeRepo) ListForUser(ctx context.Context, userID int64) ([]models.UserEntidade, error) {
	m, err := r.list(ctx, ` WHERE ue.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	out := m[userID]
	if out == nil {
		out = []models.UserEntidade{}
	}
	return out, nil
}

// ListBulk returns memberships for every user, keyed by user id.
func (r *UserEntidadeRepo) ListBulk(ctx context.Context) (map[int64][]models.UserEntidade, error) {
	return r.list(ctx, "")
}

func (r *UserEntidadeRepo) list(ctx context.Context, where string, args ...any) (map[int64][]models.UserEntidade, error) {
	rows, err := r.db.QueryContext(ctx, userEntidadeSelect+where+` ORDER BY ue.is_primary DESC, e.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]models.UserEntidade{}
	for rows.Next() {
		var uid int64
		var ue models.UserEntidade
		if err := rows.Scan(&uid, &ue.ID, &ue.Name, &ue.Slug, &ue.IsPrimary); err != nil {
			return nil, err
		}
		out[uid] = append(out[uid], ue)
	}
	return out, rows.Err()
}

// Replace sets the user's memberships to exactly ids, marking primaryID as
// primary (must be one of ids; 0 = no primary, or the first id when only one
// is given). Delete+insert in one tx.
func (r *UserEntidadeRepo) Replace(ctx context.Context, userID int64, ids []int64, primaryID int64) error {
	if len(ids) == 1 && primaryID == 0 {
		primaryID = ids[0]
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_entidades WHERE user_id = ?`, userID); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_entidades (user_id, entidade_id, is_primary) VALUES (?, ?, ?)`,
			userID, id, id == primaryID); err != nil {
			return err
		}
	}
	if primaryID != 0 && !seen[primaryID] {
		return fmt.Errorf("primary entidade %d is not among the memberships [%s]", primaryID, joinInt64(ids))
	}
	return tx.Commit()
}

func joinInt64(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return strings.Join(parts, ",")
}
