package sshsetup

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_.\-]`)

// sanitizeHost replaces non-alphanumeric characters (except _ . -) with underscores.
func sanitizeHost(host string) string {
	return nonAlphanumeric.ReplaceAllString(host, "_")
}

// GenerateKeyPair creates an ed25519 keypair in memory.
// Returns the PEM-encoded private key bytes and the authorized_keys-formatted
// public key string. Nothing is written to the filesystem — the caller is
// responsible for encrypting and storing the material in the database.
func GenerateKeyPair(host string) (privPEM []byte, pubKeyLine string, err error) {
	base := "id_ed25519_" + sanitizeHost(host)

	// Generate ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}

	// Marshal private key in OpenSSH format
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, "", fmt.Errorf("create signer: %w", err)
	}
	privBlock, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, "", fmt.Errorf("marshal private key: %w", err)
	}

	// Public key in authorized_keys format
	pubKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	_ = pubKey // pubKey used indirectly via signer
	pubKeyLine = pubKeyStr + " " + base

	return privBlock.Bytes, pubKeyLine, nil
}
