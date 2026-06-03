package models

import "time"

// OperationLog records an SSH/Coolify operation performed against a host.
// Persistence lives in internal/store.OperationLogRepo — this file is the pure
// data type only.
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
