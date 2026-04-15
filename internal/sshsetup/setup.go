package sshsetup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Request holds the parameters for an SSH key setup operation.
type Request struct {
	Host            string // SSH config Host alias
	HostName        string // Actual hostname/IP to connect to
	Port            string // SSH port (default "22")
	User            string // SSH username
	Password        string // Password for initial connection
	Mode            string // "generate" or "existing"
	ExistingKeyPath string // Path to existing private key (when Mode="existing")
}

// Result holds the outcome of an SSH key setup operation.
type Result struct {
	PrivateKeyPath string
	PublicKeyPath  string
	Generated      bool
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}

// Execute orchestrates the SSH key setup: optionally generates a keypair,
// then copies the public key to the remote host.
func Execute(req Request) (*Result, error) {
	if req.User == "" {
		return nil, fmt.Errorf("user is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if req.HostName == "" {
		return nil, fmt.Errorf("hostname is required")
	}

	var result Result

	switch req.Mode {
	case "generate":
		privPath, pubPath, err := GenerateKeyPair(req.Host)
		if err != nil {
			return nil, fmt.Errorf("generate keypair: %w", err)
		}
		result.PrivateKeyPath = privPath
		result.PublicKeyPath = pubPath
		result.Generated = true

	case "existing":
		privPath, err := expandTilde(req.ExistingKeyPath)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(privPath); err != nil {
			return nil, fmt.Errorf("private key not found: %s", privPath)
		}
		pubPath := privPath + ".pub"
		if _, err := os.Stat(pubPath); err != nil {
			return nil, fmt.Errorf("public key not found: %s (expected alongside private key)", pubPath)
		}
		result.PrivateKeyPath = privPath
		result.PublicKeyPath = pubPath
		result.Generated = false

	default:
		return nil, fmt.Errorf("invalid mode: %q (must be \"generate\" or \"existing\")", req.Mode)
	}

	if err := CopyPublicKey(req.HostName, req.Port, req.User, req.Password, result.PublicKeyPath); err != nil {
		return nil, fmt.Errorf("copy public key: %w", err)
	}

	return &result, nil
}
