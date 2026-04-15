package sshsetup

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_.\-]`)

// sanitizeHost replaces non-alphanumeric characters (except _ . -) with underscores.
func sanitizeHost(host string) string {
	return nonAlphanumeric.ReplaceAllString(host, "_")
}

// GenerateKeyPair creates an ed25519 keypair saved to ~/.ssh/id_ed25519_<host>.
// Returns the private and public key file paths.
// Refuses to overwrite existing files.
func GenerateKeyPair(host string) (privPath, pubPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("get home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", "", fmt.Errorf("create .ssh directory: %w", err)
	}

	base := "id_ed25519_" + sanitizeHost(host)
	privPath = filepath.Join(sshDir, base)
	pubPath = privPath + ".pub"

	// Refuse to overwrite
	if _, err := os.Stat(privPath); err == nil {
		return "", "", fmt.Errorf("private key already exists: %s", privPath)
	}
	if _, err := os.Stat(pubPath); err == nil {
		return "", "", fmt.Errorf("public key already exists: %s", pubPath)
	}

	// Generate ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	// Marshal private key in OpenSSH format
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return "", "", fmt.Errorf("create signer: %w", err)
	}
	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}

	if err := os.WriteFile(privPath, privPEM.Bytes, 0600); err != nil {
		return "", "", fmt.Errorf("write private key: %w", err)
	}

	// Write public key in authorized_keys format
	pubKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	_ = pubKey // pubKey used indirectly via signer
	pubLine := pubKeyStr + " " + base + "\n"
	if err := os.WriteFile(pubPath, []byte(pubLine), 0644); err != nil {
		os.Remove(privPath)
		return "", "", fmt.Errorf("write public key: %w", err)
	}

	return privPath, pubPath, nil
}
