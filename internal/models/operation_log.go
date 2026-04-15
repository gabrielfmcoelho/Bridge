package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type OperationLog struct {
	ID            int64     `json:"id"`
	HostID        int64     `json:"host_id"`
	UserID        int64     `json:"user_id"`
	UserName      string    `json:"user_name,omitempty"`
	OperationType string    `json:"operation_type"`
	AuthMethod    *string   `json:"auth_method,omitempty"`
	Status        string    `json:"status"`
	Output        string    `json:"output"`
	CreatedAt     time.Time `json:"created_at"`
}

func CreateOperationLog(db *sql.DB, log *OperationLog) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO host_operation_logs (host_id, user_id, operation_type, auth_method, status, output)
		VALUES (?, ?, ?, ?, ?, ?)`,
		log.HostID, log.UserID, log.OperationType, log.AuthMethod, log.Status, log.Output,
	)
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

func ListOperationLogs(db *sql.DB, hostID int64, limit int) ([]OperationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(
		`SELECT l.id, l.host_id, l.user_id, u.display_name, l.operation_type, l.auth_method, l.status, l.output, l.created_at
		FROM host_operation_logs l
		JOIN users u ON l.user_id = u.id
		WHERE l.host_id = ?
		ORDER BY l.created_at DESC
		LIMIT ?`, hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []OperationLog
	for rows.Next() {
		var ol OperationLog
		if err := rows.Scan(&ol.ID, &ol.HostID, &ol.UserID, &ol.UserName, &ol.OperationType, &ol.AuthMethod, &ol.Status, &ol.Output, &ol.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, ol)
	}
	return logs, rows.Err()
}
