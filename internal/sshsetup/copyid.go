package sshsetup

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// CopyPublicKey appends the public key to ~/.ssh/authorized_keys on the remote
// host using an already-established SSH connection.
// pubKey is the authorized_keys-formatted public key string.
// It is idempotent: skips if the key is already present.
func CopyPublicKey(client *ssh.Client, pubKey string) error {
	pubKeyStr := strings.TrimSpace(pubKey)

	// Check if key already exists, then append if not
	cmd := fmt.Sprintf(
		`mkdir -p ~/.ssh && chmod 700 ~/.ssh && `+
			`if ! grep -qF %q ~/.ssh/authorized_keys 2>/dev/null; then `+
			`echo %q >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && echo "ADDED"; `+
			`else echo "EXISTS"; fi`,
		pubKeyStr, pubKeyStr,
	)

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("remote command failed: %w\nOutput: %s", err, string(output))
	}

	result := strings.TrimSpace(string(output))
	if result != "ADDED" && result != "EXISTS" {
		return fmt.Errorf("unexpected remote output: %s", result)
	}

	return nil
}
