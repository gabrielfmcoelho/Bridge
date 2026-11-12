package sshsetup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Request holds the parameters for an SSH key setup operation.
type Request struct {
	Host            string // SSH config Host alias
	Mode            string // "generate" or "existing"
	ExistingKeyPath string // Path to existing private key (when Mode="existing")
	Key             []byte // existing private key PEM (the host's vault key); wins over ExistingKeyPath
}

// Result holds the outcome of an SSH key setup operation.
type Result struct {
	PrivKeyPEM []byte // PEM-encoded private key material
	PubKeyLine string // authorized_keys-formatted public key
	Generated  bool
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
// then copies the public key to the remote host via the provided SSH client.
func Execute(client *ssh.Client, req Request) (*Result, error) {
	result, err := resolveKey(req)
	if err != nil {
		return nil, err
	}
	if err := CopyPublicKey(client, result.PubKeyLine); err != nil {
		return nil, fmt.Errorf("copy public key: %w", err)
	}
	return result, nil
}

// resolveKey is the key a setup installs: a new pair, the given PEM (its
// public half derived from it, so what's installed is what logs in), or a
// key pair on disk.
func resolveKey(req Request) (*Result, error) {
	var result Result

	switch req.Mode {
	case "generate":
		privPEM, pubLine, err := GenerateKeyPair(req.Host)
		if err != nil {
			return nil, fmt.Errorf("generate keypair: %w", err)
		}
		result.PrivKeyPEM = privPEM
		result.PubKeyLine = pubLine
		result.Generated = true

	case "existing":
		if len(req.Key) > 0 {
			signer, err := ssh.ParsePrivateKey(req.Key)
			if err != nil {
				return nil, fmt.Errorf("the host's key can't be used to log in: %w", err)
			}
			result.PrivKeyPEM = req.Key
			result.PubKeyLine = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
			return &result, nil
		}
		if req.ExistingKeyPath == "" {
			return nil, fmt.Errorf("no existing key: link a key to the host or generate one")
		}
		privPath, err := expandTilde(req.ExistingKeyPath)
		if err != nil {
			return nil, err
		}
		privPEM, err := os.ReadFile(privPath)
		if err != nil {
			return nil, fmt.Errorf("read private key %s: %w", privPath, err)
		}
		pubBytes, err := os.ReadFile(privPath + ".pub")
		if err != nil {
			return nil, fmt.Errorf("read public key %s.pub: %w", privPath, err)
		}
		result.PrivKeyPEM = privPEM
		result.PubKeyLine = strings.TrimSpace(string(pubBytes))
		result.Generated = false

	default:
		return nil, fmt.Errorf("invalid mode: %q (must be \"generate\" or \"existing\")", req.Mode)
	}
	return &result, nil
}
