package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostRemoteUserRepo owns SQL for host_remote_users (links a remote-user account
// on a host to the shared vault credentials it logs in with). Referenced
// by sshconfig + coolify handlers; built inline until Phase 2 hoists it.
type HostRemoteUserRepo struct {
	db *sql.DB
}

// NewHostRemoteUserRepo constructs a HostRemoteUserRepo over the DB handle.
func NewHostRemoteUserRepo(db *sql.DB) *HostRemoteUserRepo { return &HostRemoteUserRepo{db: db} }

const hostRemoteUserCols = `id, host_id, username, key_secret_id, created_at, updated_at`

func scanHostRemoteUser(scanner interface{ Scan(...any) error }, u *models.HostRemoteUser) error {
	var keySecretID sql.NullInt64
	if err := scanner.Scan(&u.ID, &u.HostID, &u.Username, &keySecretID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return err
	}
	if keySecretID.Valid {
		id := keySecretID.Int64
		u.KeySecretID = &id
	}
	return nil
}

// CreateOrUpdate upserts the (host_id, username) row so repeated wizard runs
// keep a single row with the latest linked vault key (nil = no key).
func (r *HostRemoteUserRepo) CreateOrUpdate(ctx context.Context, hostID int64, username string, keySecretID *int64) error {
	var keyArg any
	if keySecretID != nil {
		keyArg = *keySecretID
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO host_remote_users (host_id, username, key_secret_id, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(host_id, username) DO UPDATE SET
				key_secret_id = excluded.key_secret_id,
				updated_at = CURRENT_TIMESTAMP`,
		hostID, username, keyArg,
	)
	return err
}

// GetByUsername returns the remote-user link for (host, username), or (nil, nil).
func (r *HostRemoteUserRepo) GetByUsername(ctx context.Context, hostID int64, username string) (*models.HostRemoteUser, error) {
	u := &models.HostRemoteUser{}
	err := scanHostRemoteUser(
		r.db.QueryRowContext(ctx, `SELECT `+hostRemoteUserCols+` FROM host_remote_users WHERE host_id = ? AND username = ?`, hostID, username),
		u,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// Delete removes the remote-user link for (host, username).
func (r *HostRemoteUserRepo) Delete(ctx context.Context, hostID int64, username string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM host_remote_users WHERE host_id = ? AND username = ?`, hostID, username)
	return err
}

// SetSecret links the (host, username) row to a shared password credential
// (secretID non-nil) or unlinks it (secretID nil → per-host resolution).
// Upserts so a host with no prior remote-user row still gets the link. The
// row's key_secret_id is preserved on update — a host can carry both a shared key
// and a shared password.
func (r *HostRemoteUserRepo) SetSecret(ctx context.Context, hostID int64, username string, secretID *int64) error {
	var secretArg any
	if secretID != nil {
		secretArg = *secretID
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO host_remote_users (host_id, username, secret_id, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(host_id, username) DO UPDATE SET
				secret_id = excluded.secret_id,
				updated_at = CURRENT_TIMESTAMP`,
		hostID, username, secretArg,
	)
	return err
}

// ListHostsBySecret returns the host ids currently linked to the given shared
// credential (secret_id or key_secret_id), for the credentials-library "used by" view.
func (r *HostRemoteUserRepo) ListHostsBySecret(ctx context.Context, secretID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT host_id FROM host_remote_users WHERE secret_id = ? OR key_secret_id = ? ORDER BY host_id`, secretID, secretID)
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
