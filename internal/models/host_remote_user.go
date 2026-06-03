package models

import "time"

// HostRemoteUser links a remote-user account (e.g. "coolify") that the
// create-remote-user wizard set up on a host, to the sshcm-managed ssh_keys
// row whose pubkey was installed in that user's authorized_keys. The Coolify
// integration uses this linkage to auto-pick the private key that matches the
// server's login user. Persistence lives in internal/store.HostRemoteUserRepo —
// this file is the pure data type only.
type HostRemoteUser struct {
	ID        int64     `json:"id"`
	HostID    int64     `json:"host_id"`
	Username  string    `json:"username"`
	SSHKeyID  *int64    `json:"ssh_key_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
