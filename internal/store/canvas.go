package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// CanvasRepo owns all SQL for canvases. A canvas belongs to exactly one
// entidade and is visible iff that entidade is in the caller's visible set —
// no asset_entidades grants (see AssetEntidade).
type CanvasRepo struct {
	db *sql.DB
}

// NewCanvasRepo constructs a CanvasRepo over the given DB handle.
func NewCanvasRepo(db *sql.DB) *CanvasRepo { return &CanvasRepo{db: db} }

// ErrVersionConflict: the canvas changed since the caller loaded it.
var ErrVersionConflict = errors.New("canvas changed since it was loaded")

// canvasVisible is the visibility predicate on canvases.entidade_id.
func canvasVisible(ctx context.Context) (string, []any) {
	s, skip := unscoped(ctx)
	if skip {
		return "TRUE", nil
	}
	ids := s.EntidadeIDs
	if ids == nil {
		ids = []int64{}
	}
	return "c.entidade_id = ANY(?)", []any{ids}
}

const canvasCols = `c.id, c.entidade_id, e.name, c.title, c.version, c.created_by, c.created_at, c.updated_at`

func scanCanvas(s interface{ Scan(...any) error }, c *models.Canvas, extra ...any) error {
	return s.Scan(append([]any{&c.ID, &c.EntidadeID, &c.EntidadeName, &c.Title, &c.Version, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt}, extra...)...)
}

// List returns the visible canvases (without content), latest edit first;
// entidadeID > 0 narrows to one entidade.
func (r *CanvasRepo) List(ctx context.Context, entidadeID int64) ([]models.Canvas, error) {
	vis, args := canvasVisible(ctx)
	q := `SELECT ` + canvasCols + ` FROM canvases c JOIN entidades e ON e.id = c.entidade_id WHERE ` + vis
	if entidadeID > 0 {
		q += ` AND c.entidade_id = ?`
		args = append(args, entidadeID)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY c.updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Canvas{}
	for rows.Next() {
		var c models.Canvas
		if err := scanCanvas(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns a visible canvas with its content, or (nil, nil) if
// absent/invisible.
func (r *CanvasRepo) Get(ctx context.Context, id int64) (*models.Canvas, error) {
	vis, vargs := canvasVisible(ctx)
	c := &models.Canvas{}
	var content string
	err := scanCanvas(r.db.QueryRowContext(ctx,
		`SELECT `+canvasCols+`, c.content FROM canvases c JOIN entidades e ON e.id = c.entidade_id WHERE c.id = ? AND `+vis,
		append([]any{id}, vargs...)...), c, &content)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	c.Content = json.RawMessage(content)
	return c, err
}

// Create inserts an empty canvas and sets c.ID/c.Version. The caller checks
// the entidade is visible.
func (r *CanvasRepo) Create(ctx context.Context, c *models.Canvas) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO canvases (entidade_id, title, created_by) VALUES (?, ?, ?)`,
		c.EntidadeID, c.Title, c.CreatedBy)
	if err != nil {
		return err
	}
	c.ID, c.Version = id, 1
	return nil
}

// Update writes the non-nil fields iff the row is still at version and
// visible, and returns the new version. A visible row at another version is
// ErrVersionConflict; an invisible/absent one is sql.ErrNoRows.
func (r *CanvasRepo) Update(ctx context.Context, id int64, version int, title, content *string) (int, error) {
	vis, vargs := canvasVisible(ctx)
	var next int
	err := r.db.QueryRowContext(ctx,
		`UPDATE canvases c SET title = COALESCE(?, c.title), content = COALESCE(?, c.content),
			version = c.version + 1, updated_at = CURRENT_TIMESTAMP
		WHERE c.id = ? AND c.version = ? AND `+vis+` RETURNING c.version`,
		append([]any{title, content, id, version}, vargs...)...).Scan(&next)
	if err == sql.ErrNoRows {
		if cur, gerr := r.Get(ctx, id); gerr == nil && cur != nil {
			return 0, ErrVersionConflict
		}
	}
	return next, err
}

// Delete removes a visible canvas. Issues it sent to the backlog stay (they
// hang off the entidade).
func (r *CanvasRepo) Delete(ctx context.Context, id int64) error {
	vis, vargs := canvasVisible(ctx)
	_, err := r.db.ExecContext(ctx, `DELETE FROM canvases c WHERE c.id = ? AND `+vis, append([]any{id}, vargs...)...)
	return err
}
