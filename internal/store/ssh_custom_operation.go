package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// SSHCustomOperationRepo owns SQL for ssh_custom_operations, the global
// library of admin-written scripts run from a host's Operações tab.
type SSHCustomOperationRepo struct {
	db *sql.DB
}

// NewSSHCustomOperationRepo constructs an SSHCustomOperationRepo over the given DB handle.
func NewSSHCustomOperationRepo(db *sql.DB) *SSHCustomOperationRepo {
	return &SSHCustomOperationRepo{db: db}
}

const sshCustomOpCols = `id, name, description, script, use_sudo, created_by, created_at, updated_at`

func scanSSHCustomOp(scanner interface{ Scan(...any) error }, o *models.SSHCustomOperation) error {
	return scanner.Scan(&o.ID, &o.Name, &o.Description, &o.Script, &o.UseSudo, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt)
}

// List returns every operation ordered by name.
func (r *SSHCustomOperationRepo) List(ctx context.Context) ([]models.SSHCustomOperation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sshCustomOpCols+` FROM ssh_custom_operations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SSHCustomOperation{}
	for rows.Next() {
		var o models.SSHCustomOperation
		if err := scanSSHCustomOp(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Get returns one operation, or (nil, nil).
func (r *SSHCustomOperationRepo) Get(ctx context.Context, id int64) (*models.SSHCustomOperation, error) {
	o := &models.SSHCustomOperation{}
	err := scanSSHCustomOp(r.db.QueryRowContext(ctx, `SELECT `+sshCustomOpCols+` FROM ssh_custom_operations WHERE id = ?`, id), o)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

// Create inserts o and sets o.ID. A duplicate name is a unique violation.
func (r *SSHCustomOperationRepo) Create(ctx context.Context, o *models.SSHCustomOperation) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO ssh_custom_operations (name, description, script, use_sudo, created_by) VALUES (?, ?, ?, ?, ?)`,
		o.Name, o.Description, o.Script, o.UseSudo, o.CreatedBy)
	o.ID = id
	return err
}

// Update rewrites the editable columns of o; the caller merges the partial
// input over the stored row first.
func (r *SSHCustomOperationRepo) Update(ctx context.Context, o *models.SSHCustomOperation) error {
	_, err := r.db.ExecContext(ctx, `UPDATE ssh_custom_operations SET name = ?, description = ?, script = ?, use_sudo = ?,
			updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		o.Name, o.Description, o.Script, o.UseSudo, o.ID)
	return err
}

// Delete removes an operation. Its past runs stay in host_operation_logs.
func (r *SSHCustomOperationRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ssh_custom_operations WHERE id = ?`, id)
	return err
}
