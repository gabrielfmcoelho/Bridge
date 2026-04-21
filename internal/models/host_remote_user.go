package models

import (
	"database/sql"
	"time"
)

// HostRemoteUser links a remote-user account (e.g. "coolify") that the
// create-remote-user wizard set up on a host, to the sshcm-managed ssh_keys
// row whose pubkey was installed in that user's authorized_keys. The
// Coolify integration uses this linkage to auto-pick the private key that
// matches the server's login user.
type HostRemoteUser struct {
	ID        int64     `json:"id"`
	HostID    int64     `json:"host_id"`
	Username  string    `json:"username"`
	SSHKeyID  *int64    `json:"ssh_key_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const hostRemoteUserCols = `id, host_id, username, ssh_key_id, created_at, updated_at`

func scanHostRemoteUser(scanner interface{ Scan(...any) error }, u *HostRemoteUser) error {
	var keyID sql.NullInt64
	if err := scanner.Scan(&u.ID, &u.HostID, &u.Username, &keyID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return err
	}
	if keyID.Valid {
		id := keyID.Int64
		u.SSHKeyID = &id
	}
	return nil
}

// CreateOrUpdateHostRemoteUser upserts the (host_id, username) row so repeated
// wizard runs (force=true, or re-running for the same user) keep a single row
// with the latest linked key.
func CreateOrUpdateHostRemoteUser(db *sql.DB, hostID int64, username string, sshKeyID *int64) error {
	var keyArg any
	if sshKeyID != nil {
		keyArg = *sshKeyID
	} else {
		keyArg = nil
	}
	_, err := db.Exec(
		`INSERT INTO host_remote_users (host_id, username, ssh_key_id, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(host_id, username) DO UPDATE SET
				ssh_key_id = excluded.ssh_key_id,
				updated_at = CURRENT_TIMESTAMP`,
		hostID, username, keyArg,
	)
	return err
}

func GetHostRemoteUserByUsername(db *sql.DB, hostID int64, username string) (*HostRemoteUser, error) {
	u := &HostRemoteUser{}
	err := scanHostRemoteUser(
		db.QueryRow(`SELECT `+hostRemoteUserCols+` FROM host_remote_users WHERE host_id = ? AND username = ?`, hostID, username),
		u,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func ListHostRemoteUsers(db *sql.DB, hostID int64) ([]HostRemoteUser, error) {
	rows, err := db.Query(`SELECT `+hostRemoteUserCols+` FROM host_remote_users WHERE host_id = ? ORDER BY username`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []HostRemoteUser
	for rows.Next() {
		var u HostRemoteUser
		if err := scanHostRemoteUser(rows, &u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func DeleteHostRemoteUser(db *sql.DB, hostID int64, username string) error {
	_, err := db.Exec(`DELETE FROM host_remote_users WHERE host_id = ? AND username = ?`, hostID, username)
	return err
}
