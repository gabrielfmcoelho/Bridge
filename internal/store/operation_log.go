package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// OperationLogRepo owns SQL for host_operation_logs (SSH/Coolify operation
// audit trail). Referenced by several handlers (sshconfig, coolify); built
// inline until the Phase 2 container hoists it.
type OperationLogRepo struct {
	db *sql.DB
}

// NewOperationLogRepo constructs an OperationLogRepo over the given DB handle.
func NewOperationLogRepo(db *sql.DB) *OperationLogRepo { return &OperationLogRepo{db: db} }

// Create inserts an operation-log row and sets log.ID.
func (r *OperationLogRepo) Create(ctx context.Context, log *models.OperationLog) error {
	id, err := database.InsertReturningID(r.db,
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

// ListByHost returns the most recent operation logs for a host (joined to the
// acting user's display name), newest first. limit <= 0 defaults to 50.
func (r *OperationLogRepo) ListByHost(ctx context.Context, hostID int64, limit int) ([]models.OperationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
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
	var logs []models.OperationLog
	for rows.Next() {
		var ol models.OperationLog
		if err := rows.Scan(&ol.ID, &ol.HostID, &ol.UserID, &ol.UserName, &ol.OperationType, &ol.AuthMethod, &ol.Status, &ol.Output, &ol.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, ol)
	}
	return logs, rows.Err()
}
