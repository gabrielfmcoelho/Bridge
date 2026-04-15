package models

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Issue struct {
	ID              int64     `json:"id"`
	ProjectID       *int64    `json:"project_id"`
	ServiceID       *int64    `json:"service_id"`
	EntityType      string    `json:"entity_type"`
	EntityID        int64     `json:"entity_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	Priority        string    `json:"priority"`
	Assignee        string    `json:"assignee"`
	Source          string    `json:"source"`
	SourceRef       string    `json:"source_ref"`
	CreatedBy       int64     `json:"created_by"`
	Position        float64   `json:"position"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ExpectedEndDate string    `json:"expected_end_date"`
	StartDate       string    `json:"start_date"`
	EndDate         string    `json:"end_date"`
	AlertID         *int64    `json:"alert_id"`
	Archived        bool      `json:"archived"`
}

const issueCols = `id, project_id, service_id, entity_type, entity_id, title, description, status, priority, assignee, source, source_ref, created_by, position, created_at, updated_at, expected_end_date, start_date, end_date, alert_id, archived`

func scanIssue(scanner interface{ Scan(...any) error }, i *Issue) error {
	return scanner.Scan(&i.ID, &i.ProjectID, &i.ServiceID, &i.EntityType, &i.EntityID,
		&i.Title, &i.Description, &i.Status, &i.Priority, &i.Assignee,
		&i.Source, &i.SourceRef, &i.CreatedBy, &i.Position, &i.CreatedAt, &i.UpdatedAt,
		&i.ExpectedEndDate, &i.StartDate, &i.EndDate, &i.AlertID, &i.Archived)
}

func CreateIssue(db *sql.DB, i *Issue) error {
	if i.EntityType == "" {
		i.EntityType = "project"
		if i.ProjectID != nil {
			i.EntityID = *i.ProjectID
		}
	}
	// Auto-assign position
	var maxPos sql.NullFloat64
	db.QueryRow(`SELECT MAX(position) FROM issues WHERE entity_type = ? AND entity_id = ? AND status = ?`,
		i.EntityType, i.EntityID, i.Status).Scan(&maxPos)
	if maxPos.Valid {
		i.Position = maxPos.Float64 + 1
	}

	// Auto-set start_date when created directly as in_progress
	if i.Status == "in_progress" && i.StartDate == "" {
		i.StartDate = time.Now().Format("2006-01-02")
	}

	newID, err := database.InsertReturningID(db,
		`INSERT INTO issues (project_id, service_id, entity_type, entity_id, title, description, status, priority, assignee, source, source_ref, created_by, position, expected_end_date, start_date, end_date, alert_id, archived)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ProjectID, i.ServiceID, i.EntityType, i.EntityID,
		i.Title, i.Description, i.Status, i.Priority, i.Assignee,
		i.Source, i.SourceRef, i.CreatedBy, i.Position,
		i.ExpectedEndDate, i.StartDate, i.EndDate, i.AlertID, i.Archived,
	)
	if err != nil {
		return err
	}
	i.ID = newID
	return nil
}

func GetIssue(db *sql.DB, id int64) (*Issue, error) {
	i := &Issue{}
	err := scanIssue(db.QueryRow(`SELECT `+issueCols+` FROM issues WHERE id = ?`, id), i)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return i, err
}

type IssueFilter struct {
	EntityType    string
	EntityID      int64
	ProjectID     int64
	ServiceID     int64
	Status        string
	Priority      string
	AssigneeID    int64
	Search        string
	ExcludeArchived bool
}

func ListIssues(db *sql.DB, f IssueFilter) ([]Issue, error) {
	query := `SELECT ` + issueCols + ` FROM issues`
	var args []any
	var where []string

	if f.EntityType != "" {
		where = append(where, "entity_type = ?")
		args = append(args, f.EntityType)
	}
	if f.EntityID > 0 {
		where = append(where, "entity_id = ?")
		args = append(args, f.EntityID)
	}
	if f.ProjectID > 0 {
		where = append(where, "project_id = ?")
		args = append(args, f.ProjectID)
	}
	if f.ServiceID > 0 {
		where = append(where, "service_id = ?")
		args = append(args, f.ServiceID)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Priority != "" {
		where = append(where, "priority = ?")
		args = append(args, f.Priority)
	}
	if f.AssigneeID > 0 {
		where = append(where, "id IN (SELECT issue_id FROM issue_assignees WHERE user_id = ?)")
		args = append(args, f.AssigneeID)
	}
	if f.Search != "" {
		op := database.LikeOp()
		where = append(where, "(title "+op+" ? OR description "+op+" ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s)
	}
	if f.ExcludeArchived {
		where = append(where, "NOT archived")
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY status, position"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var i Issue
		if err := scanIssue(rows, &i); err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

// Legacy functions for backward compat
func ListIssuesByProject(db *sql.DB, projectID int64, serviceID *int64) ([]Issue, error) {
	f := IssueFilter{ProjectID: projectID}
	if serviceID != nil {
		f.ServiceID = *serviceID
	}
	return ListIssues(db, f)
}

func ListIssuesByService(db *sql.DB, serviceID int64) ([]Issue, error) {
	return ListIssues(db, IssueFilter{ServiceID: serviceID})
}

func UpdateIssue(db *sql.DB, i *Issue) error {
	// Auto-set date fields based on status transitions
	today := time.Now().Format("2006-01-02")
	if i.Status == "in_progress" && i.StartDate == "" {
		i.StartDate = today
	}
	if i.Status == "done" && i.EndDate == "" {
		i.EndDate = today
	}
	// Clear end_date if reopened
	if i.Status != "done" && i.EndDate != "" {
		i.EndDate = ""
	}

	_, err := db.Exec(
		`UPDATE issues SET service_id = ?, entity_type = ?, entity_id = ?, title = ?, description = ?, status = ?, priority = ?, assignee = ?, source = ?, source_ref = ?, position = ?, expected_end_date = ?, start_date = ?, end_date = ?, alert_id = ?, archived = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		i.ServiceID, i.EntityType, i.EntityID, i.Title, i.Description, i.Status, i.Priority, i.Assignee, i.Source, i.SourceRef, i.Position, i.ExpectedEndDate, i.StartDate, i.EndDate, i.AlertID, i.Archived, i.ID,
	)
	return err
}

func MoveIssue(db *sql.DB, id int64, status string, position float64) error {
	today := time.Now().Format("2006-01-02")

	// Auto-set start_date when moving to in_progress (only if not already set)
	if status == "in_progress" {
		db.Exec(`UPDATE issues SET start_date = ? WHERE id = ? AND start_date = ''`, today, id)
	}
	// Auto-set end_date when moving to done
	if status == "done" {
		db.Exec(`UPDATE issues SET end_date = ? WHERE id = ? AND end_date = ''`, today, id)
	}
	// Clear end_date if reopened from done
	if status != "done" {
		db.Exec(`UPDATE issues SET end_date = '' WHERE id = ? AND end_date != ''`, id)
	}

	_, err := db.Exec(
		`UPDATE issues SET status = ?, position = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, position, id,
	)
	return err
}

func DeleteIssue(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM issues WHERE id = ?`, id)
	return err
}

// Assignees

func SetIssueAssignees(db *sql.DB, issueID int64, userIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM issue_assignees WHERE issue_id = ?`, issueID); err != nil {
		return err
	}
	for _, uid := range userIDs {
		if _, err := tx.Exec(`INSERT INTO issue_assignees (issue_id, user_id) VALUES (?, ?)`, issueID, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetIssueAssigneeIDs(db *sql.DB, issueID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT user_id FROM issue_assignees WHERE issue_id = ?`, issueID)
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

func GetIssueAssigneesBulk(db *sql.DB) (map[int64][]int64, error) {
	rows, err := db.Query(`SELECT issue_id, user_id FROM issue_assignees`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64][]int64)
	for rows.Next() {
		var issueID, userID int64
		if err := rows.Scan(&issueID, &userID); err != nil {
			return nil, err
		}
		m[issueID] = append(m[issueID], userID)
	}
	return m, rows.Err()
}

// Alert links (many-to-many)

func SetIssueAlertLinks(db *sql.DB, issueID int64, alertIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM issue_alert_links WHERE issue_id = ?`, issueID); err != nil {
		return err
	}
	for _, aid := range alertIDs {
		if _, err := tx.Exec(`INSERT INTO issue_alert_links (issue_id, alert_id) VALUES (?, ?)`, issueID, aid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetIssueAlertIDs(db *sql.DB, issueID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT alert_id FROM issue_alert_links WHERE issue_id = ?`, issueID)
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

func GetIssueAlertsBulk(db *sql.DB) (map[int64][]int64, error) {
	rows, err := db.Query(`SELECT issue_id, alert_id FROM issue_alert_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64][]int64)
	for rows.Next() {
		var issueID, alertID int64
		if err := rows.Scan(&issueID, &alertID); err != nil {
			return nil, err
		}
		m[issueID] = append(m[issueID], alertID)
	}
	return m, rows.Err()
}

// Counts

func IssueCountByProject(db *sql.DB, projectID int64) (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM issues WHERE project_id = ? GROUP BY status`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func OpenIssueCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM issues WHERE status != 'done'`).Scan(&n)
	return n, err
}

func GetIssueCountsByEntity(db *sql.DB, entityType string) (map[int64]int, error) {
	rows, err := db.Query(
		`SELECT entity_id, COUNT(*) FROM issues WHERE entity_type = ? AND status != 'done' GROUP BY entity_id`,
		entityType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[int64]int)
	for rows.Next() {
		var id int64
		var c int
		if err := rows.Scan(&id, &c); err != nil {
			return nil, err
		}
		counts[id] = c
	}
	return counts, rows.Err()
}
