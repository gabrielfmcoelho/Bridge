package sshsetup

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// CopyPublicKey connects to a remote host via password auth and appends the
// public key to ~/.ssh/authorized_keys (like ssh-copy-id).
// It is idempotent: skips if the key is already present.
func CopyPublicKey(hostname, port, user, password, pubKeyPath string) error {
	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	pubKeyStr := strings.TrimSpace(string(pubKeyData))

	if port == "" {
		port = "22"
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(hostname, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("SSH connect to %s: %w", addr, err)
	}
	defer client.Close()

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
