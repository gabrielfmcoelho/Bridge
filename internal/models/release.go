package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Release struct {
	ID          int64     `json:"id"`
	ProjectID   *int64    `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	TargetDate  string    `json:"target_date"`
	LiveDate    string    `json:"live_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func CreateRelease(db *sql.DB, r *Release) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO releases (project_id, title, description, status, target_date, live_date)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.ProjectID, r.Title, r.Description, r.Status, r.TargetDate, r.LiveDate,
	)
	if err != nil {
		return err
	}
	r.ID = id
	return nil
}

func GetRelease(db *sql.DB, id int64) (*Release, error) {
	r := &Release{}
	err := db.QueryRow(
		`SELECT id, project_id, title, description, status, target_date, live_date, created_at, updated_at
		FROM releases WHERE id = ?`, id,
	).Scan(&r.ID, &r.ProjectID, &r.Title, &r.Description, &r.Status, &r.TargetDate, &r.LiveDate, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func ListReleases(db *sql.DB) ([]Release, error) {
	rows, err := db.Query(
		`SELECT id, project_id, title, description, status, target_date, live_date, created_at, updated_at
		FROM releases ORDER BY
			CASE status
				WHEN 'live' THEN 1
				WHEN 'ready' THEN 2
				WHEN 'ongoing' THEN 3
				WHEN 'pending' THEN 4
				WHEN 'canceled' THEN 5
			END, created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Title, &r.Description, &r.Status, &r.TargetDate, &r.LiveDate, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func ListReleasesByProject(db *sql.DB, projectID int64) ([]Release, error) {
	rows, err := db.Query(
		`SELECT id, project_id, title, description, status, target_date, live_date, created_at, updated_at
		FROM releases WHERE project_id = ? ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Title, &r.Description, &r.Status, &r.TargetDate, &r.LiveDate, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func UpdateRelease(db *sql.DB, r *Release) error {
	_, err := db.Exec(
		`UPDATE releases SET project_id = ?, title = ?, description = ?, status = ?, target_date = ?, live_date = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		r.ProjectID, r.Title, r.Description, r.Status, r.TargetDate, r.LiveDate, r.ID,
	)
	return err
}

func DeleteRelease(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM releases WHERE id = ?`, id)
	return err
}

// Release-Issue links

func SetReleaseIssues(db *sql.DB, releaseID int64, issueIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM release_issues WHERE release_id = ?`, releaseID); err != nil {
		return err
	}
	for _, iid := range issueIDs {
		if _, err := tx.Exec(`INSERT INTO release_issues (release_id, issue_id) VALUES (?, ?)`, releaseID, iid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetReleaseIssueIDs(db *sql.DB, releaseID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT issue_id FROM release_issues WHERE release_id = ?`, releaseID)
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
