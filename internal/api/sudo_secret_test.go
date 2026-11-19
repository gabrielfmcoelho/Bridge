package api

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// A root operation on a host with no stored password runs only when the
// host's scan found that root needs none; a stored password always wins.
func TestSudoSecret(t *testing.T) {
	keyOnly := &models.Host{HasKey: true, SudoNoPasswd: true}
	needsPassword := &models.Host{HasKey: true}

	if got := sudoSecret(keyOnly, ""); got != sshtest.NoPassword {
		t.Fatalf("key-only host with passwordless root: got %q", got)
	}
	if got := sudoSecret(needsPassword, ""); got != "" {
		t.Fatalf("no password and root needs one: got %q, the operation must be refused", got)
	}
	if got := sudoSecret(keyOnly, "s3cret"); got != "s3cret" {
		t.Fatalf("a stored password must be used as is: got %q", got)
	}
}
