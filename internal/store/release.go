package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ReleaseRepo owns SQL for releases and the release_issues junction.
type ReleaseRepo struct {
	db *sql.DB
}

// NewReleaseRepo constructs a ReleaseRepo over the given DB handle.
func NewReleaseRepo(db *sql.DB) *ReleaseRepo { return &ReleaseRepo{db: db} }

const releaseCols = `id, project_id, title, description, status, target_date, live_date, created_at, updated_at`

func scanRelease(scanner interface{ Scan(...any) error }, r *models.Release) error {
	return scanner.Scan(&r.ID, &r.ProjectID, &r.Title, &r.Description, &r.Status, &r.TargetDate, &r.LiveDate, &r.CreatedAt, &r.UpdatedAt)
}

// Create inserts a release and sets rel.ID.
func (r *ReleaseRepo) Create(ctx context.Context, rel *models.Release) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO releases (project_id, title, description, status, target_date, live_date)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		rel.ProjectID, rel.Title, rel.Description, rel.Status, rel.TargetDate, rel.LiveDate,
	)
	if err != nil {
		return err
	}
	rel.ID = id
	return nil
}

// Get returns a release by id, or (nil, nil) if absent.
func (r *ReleaseRepo) Get(ctx context.Context, id int64) (*models.Release, error) {
	rel := &models.Release{}
	err := scanRelease(r.db.QueryRowContext(ctx, `SELECT `+releaseCols+` FROM releases WHERE id = ?`, id), rel)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rel, err
}

// List returns all releases ordered by lifecycle status then recency.
func (r *ReleaseRepo) List(ctx context.Context) ([]models.Release, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+releaseCols+` FROM releases ORDER BY
		CASE status
			WHEN 'live' THEN 1
			WHEN 'ready' THEN 2
			WHEN 'ongoing' THEN 3
			WHEN 'pending' THEN 4
			WHEN 'canceled' THEN 5
		END, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Release
	for rows.Next() {
		var rel models.Release
		if err := scanRelease(rows, &rel); err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

// Update writes the mutable fields of an existing release by id.
func (r *ReleaseRepo) Update(ctx context.Context, rel *models.Release) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE releases SET project_id = ?, title = ?, description = ?, status = ?, target_date = ?, live_date = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		rel.ProjectID, rel.Title, rel.Description, rel.Status, rel.TargetDate, rel.LiveDate, rel.ID,
	)
	return err
}

// Delete removes a release by id.
func (r *ReleaseRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM releases WHERE id = ?`, id)
	return err
}

// SetIssues replaces the release's linked issues with issueIDs (one tx).
func (r *ReleaseRepo) SetIssues(ctx context.Context, releaseID int64, issueIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM release_issues WHERE release_id = ?`, releaseID); err != nil {
		return err
	}
	for _, iid := range issueIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO release_issues (release_id, issue_id) VALUES (?, ?)`, releaseID, iid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IssueIDs returns the issue ids linked to a release.
func (r *ReleaseRepo) IssueIDs(ctx context.Context, releaseID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT issue_id FROM release_issues WHERE release_id = ?`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
