package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ProjectEmbedRepo owns SQL for project_embeds. Callers check the project's
// visibility first; every query here is keyed by project id so an embed is
// only reachable through its project.
type ProjectEmbedRepo struct {
	db *sql.DB
}

// NewProjectEmbedRepo constructs the repo over the given DB handle.
func NewProjectEmbedRepo(db *sql.DB) *ProjectEmbedRepo { return &ProjectEmbedRepo{db: db} }

// List returns a project's embeds in display order.
func (r *ProjectEmbedRepo) List(ctx context.Context, projectID int64) ([]models.ProjectEmbed, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, title, url, height, sort_order, created_at, updated_at
		 FROM project_embeds WHERE project_id = ? ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProjectEmbed{}
	for rows.Next() {
		var e models.ProjectEmbed
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Title, &e.URL, &e.Height, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Create inserts an embed and sets e.ID.
func (r *ProjectEmbedRepo) Create(ctx context.Context, e *models.ProjectEmbed) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO project_embeds (project_id, title, url, height, sort_order) VALUES (?, ?, ?, ?, ?)`,
		e.ProjectID, e.Title, e.URL, e.Height, e.SortOrder)
	if err != nil {
		return err
	}
	e.ID = id
	return nil
}

// Update writes an embed's fields; false when no embed with that id belongs
// to the project.
func (r *ProjectEmbedRepo) Update(ctx context.Context, e *models.ProjectEmbed) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE project_embeds SET title = ?, url = ?, height = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND project_id = ?`,
		e.Title, e.URL, e.Height, e.SortOrder, e.ID, e.ProjectID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Delete removes one of the project's embeds.
func (r *ProjectEmbedRepo) Delete(ctx context.Context, projectID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM project_embeds WHERE id = ? AND project_id = ?`, id, projectID)
	return err
}
