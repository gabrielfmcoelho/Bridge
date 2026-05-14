package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

// KeySource describes where the AES key came from on this process start.
// "generated" is the only value that signals possible data loss when
// pre-existing ciphertext is present in the database.
type KeySource string

const (
	KeySourceEnv       KeySource = "env"
	KeySourceFile      KeySource = "file"
	KeySourceGenerated KeySource = "generated"

	// secretKeyEnv, if set, overrides the on-disk key file. Value must be the
	// 32-byte AES-256 key encoded as base64 (standard or URL, padded or not).
	secretKeyEnv = "SSHCM_SECRET_KEY"
)

// Encryptor provides AES-256-GCM encryption for sensitive data.
type Encryptor struct {
	aead   cipher.AEAD
	source KeySource
}

// NewEncryptor builds an Encryptor. The key is resolved in this order:
//  1. SSHCM_SECRET_KEY env var (base64-encoded 32 bytes) — preferred in
//     containerised deployments so the key lives in a secrets manager.
//  2. keyPath on disk, if present and exactly 32 bytes.
//  3. Newly generated 32 random bytes, written to keyPath.
func NewEncryptor(keyPath string) (*Encryptor, error) {
	key, source, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("encryption key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Encryptor{aead: aead, source: source}, nil
}

// Source reports where the active key came from. Useful to detect the
// "key was just regenerated" failure mode after a redeploy.
func (e *Encryptor) Source() KeySource { return e.source }

// Encrypt returns (ciphertext, nonce) for the given plaintext.
func (e *Encryptor) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, e.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = e.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// Decrypt returns the plaintext for the given (ciphertext, nonce) pair.
func (e *Encryptor) Decrypt(ciphertext, nonce []byte) (string, error) {
	plain, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

func loadOrCreateKey(path string) ([]byte, KeySource, error) {
	if raw := strings.TrimSpace(os.Getenv(secretKeyEnv)); raw != "" {
		key, err := decodeBase64Key(raw)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", secretKeyEnv, err)
		}
		return key, KeySourceEnv, nil
	}

	data, err := os.ReadFile(path)
	if err == nil && len(data) == 32 {
		return data, KeySourceFile, nil
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, "", err
	}
	return key, KeySourceGenerated, nil
}

func decodeBase64Key(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if key, err := enc.DecodeString(s); err == nil {
			if len(key) != 32 {
				return nil, fmt.Errorf("must decode to 32 bytes, got %d", len(key))
			}
			return key, nil
		}
	}
	return nil, fmt.Errorf("not valid base64")
}
