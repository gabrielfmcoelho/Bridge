package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type HostAlert struct {
	ID          int64     `json:"id"`
	HostID      int64     `json:"host_id"`
	Type        string    `json:"type"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const hostAlertCols = `id, host_id, type, level, message, description, source, status, created_at, updated_at`

func scanHostAlert(scanner interface{ Scan(...any) error }, a *HostAlert) error {
	return scanner.Scan(&a.ID, &a.HostID, &a.Type, &a.Level, &a.Message, &a.Description, &a.Source, &a.Status, &a.CreatedAt, &a.UpdatedAt)
}

func CreateHostAlert(db *sql.DB, a *HostAlert) error {
	if a.Source == "" {
		a.Source = "manual"
	}
	id, err := database.InsertReturningID(db,
		`INSERT INTO host_alerts (host_id, type, level, message, description, source) VALUES (?, ?, ?, ?, ?, ?)`,
		a.HostID, a.Type, a.Level, a.Message, a.Description, a.Source,
	)
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

func GetHostAlert(db *sql.DB, id int64) (*HostAlert, error) {
	a := &HostAlert{}
	err := scanHostAlert(db.QueryRow(`SELECT `+hostAlertCols+` FROM host_alerts WHERE id = ?`, id), a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func ListHostAlerts(db *sql.DB, hostID int64) ([]HostAlert, error) {
	rows, err := db.Query(`SELECT `+hostAlertCols+` FROM host_alerts WHERE host_id = ? ORDER BY created_at DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []HostAlert
	for rows.Next() {
		var a HostAlert
		if err := scanHostAlert(rows, &a); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func ListHostAlertsBulk(db *sql.DB) (map[int64][]HostAlert, error) {
	rows, err := db.Query(`SELECT ` + hostAlertCols + ` FROM host_alerts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64][]HostAlert)
	for rows.Next() {
		var a HostAlert
		if err := scanHostAlert(rows, &a); err != nil {
			return nil, err
		}
		m[a.HostID] = append(m[a.HostID], a)
	}
	return m, rows.Err()
}

func UpdateHostAlert(db *sql.DB, a *HostAlert) error {
	_, err := db.Exec(
		`UPDATE host_alerts SET type = ?, level = ?, message = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		a.Type, a.Level, a.Message, a.Description, a.ID,
	)
	return err
}

func DeleteHostAlert(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM host_alerts WHERE id = ?`, id)
	return err
}

func ResolveHostAlert(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE host_alerts SET status = 'resolved', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// ResolveAlertsByIssueID resolves all alerts linked to a given issue.
func ResolveAlertsByIssueID(db *sql.DB, issueID int64) error {
	_, err := db.Exec(
		`UPDATE host_alerts SET status = 'resolved', updated_at = CURRENT_TIMESTAMP
		WHERE id IN (SELECT alert_id FROM issue_alert_links WHERE issue_id = ?)`, issueID)
	return err
}

// GetAlertLinkedIssueIDs returns a map of alert_id -> first issue_id for alerts that have linked issues.
func GetAlertLinkedIssueIDs(db *sql.DB, hostID int64) (map[int64]int64, error) {
	rows, err := db.Query(
		`SELECT l.alert_id, l.issue_id FROM issue_alert_links l
		JOIN host_alerts a ON a.id = l.alert_id
		WHERE a.host_id = ?`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]int64)
	for rows.Next() {
		var alertID, issueID int64
		if err := rows.Scan(&alertID, &issueID); err != nil {
			return nil, err
		}
		if _, exists := m[alertID]; !exists {
			m[alertID] = issueID
		}
	}
	return m, rows.Err()
}

// GetAlertLinkedIssueIDsBulk returns a map of alert_id -> first issue_id for ALL alerts.
func GetAlertLinkedIssueIDsBulk(db *sql.DB) (map[int64]int64, error) {
	rows, err := db.Query(`SELECT alert_id, issue_id FROM issue_alert_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]int64)
	for rows.Next() {
		var alertID, issueID int64
		if err := rows.Scan(&alertID, &issueID); err != nil {
			return nil, err
		}
		if _, exists := m[alertID]; !exists {
			m[alertID] = issueID
		}
	}
	return m, rows.Err()
}
