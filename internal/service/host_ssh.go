package service

import (
	"context"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// SSHConfigError is a host whose stored settings can't produce an SSH login
// (no credential, ambiguous method): the caller's to fix, not a fault.
type SSHConfigError struct{ Msg string }

func (e *SSHConfigError) Error() string { return e.Msg }

func sshConfigErr(msg string) error { return &SSHConfigError{msg} }

// ResolveSSHAuth builds the login for host. An empty method picks the host's
// only credential, or its preferred one when it has both.
func ResolveSSHAuth(ctx context.Context, db *database.DB, host *models.Host, method string) (string, sshtest.Auth, error) {
	if method == "" {
		switch {
		case host.HasPassword && host.HasKey:
			if host.PreferredAuth != "password" && host.PreferredAuth != "key" {
				return "", sshtest.Auth{}, sshConfigErr("host has both auth methods; set preferred auth or choose one")
			}
			method = host.PreferredAuth
		case host.HasKey:
			method = "key"
		case host.HasPassword:
			method = "password"
		default:
			return "", sshtest.Auth{}, sshConfigErr("host has no password or key configured")
		}
	}

	switch method {
	case "password":
		pw, ok, err := vault.HostGetPassword(ctx, db, host.ID)
		if err != nil {
			return "", sshtest.Auth{}, fmt.Errorf("failed to decrypt password: %w", err)
		}
		if !ok {
			return "", sshtest.Auth{}, sshConfigErr("no password stored for this host")
		}
		return "password", sshtest.PasswordAuth(pw), nil

	case "key":
		if !host.HasKey {
			return "", sshtest.Auth{}, sshConfigErr("no key configured for this host")
		}
		key, ok, err := vault.HostGetSSHKey(ctx, db, host.ID)
		if err != nil {
			return "", sshtest.Auth{}, sshConfigErr("load host key: " + err.Error())
		}
		if !ok || key.PrivateKeyPEM == "" {
			return "", sshtest.Auth{}, sshConfigErr("host has no stored private key — link an SSH key via the host editor")
		}
		// Carry the password for sudo operations even with key auth — but
		// only when the password actually works. If the last password test
		// failed (or the password was never tested) the stored credential
		// is likely stale, and feeding it to `sudo -S` makes every retry
		// wait on PAM's ~3-second faildelay before falling through to the
		// unprivileged path. Across the ~5 sudo sites in the capture
		// (key-enum, cron user crontabs, passwd -Sa, sshd -T, …) that's
		// enough cumulative slowdown to blow past the dev-proxy idle
		// timeout and surface as a "socket hang up" / 500 to the user.
		var pw string
		if host.HasPassword && host.PasswordTestStatus != nil && *host.PasswordTestStatus == "success" {
			pw, _, _ = vault.HostGetPassword(ctx, db, host.ID)
		}
		return "key", sshtest.KeyAuth([]byte(key.PrivateKeyPEM), pw), nil

	default:
		return "", sshtest.Auth{}, sshConfigErr("method must be 'password' or 'key'")
	}
}
