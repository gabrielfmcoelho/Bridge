package secretshare

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// hkdfInfo is the fixed info parameter for the per-link HKDF derivation
// (D4 / spec §5.3). Bumping this string would invalidate every existing
// share link — version it as "secret-share-v1" so a future key-rotation
// scheme can introduce a v2 without conflict.
const hkdfInfo = "secret-share-v1"

// DeriveKey returns the per-link AES-256 key derived via
// HKDF-SHA256(master, salt=token, info="secret-share-v1"). The salt is the
// raw token (NOT its hash) — this is the property that prevents DB-only
// decryption: the DB has the hash but never the salt.
//
// Token must be the raw token string (the URL-encoded one), not its hash.
func DeriveKey(master []byte, token string) ([]byte, error) {
	if len(master) == 0 {
		return nil, fmt.Errorf("derive key: empty master")
	}
	if token == "" {
		return nil, fmt.Errorf("derive key: empty token")
	}
	r := hkdf.New(sha256.New, master, []byte(token), []byte(hkdfInfo))
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("hkdf read: %w", err)
	}
	return key, nil
}

// Encrypt seals plaintext under the per-link key derived from (master, token).
// Returns (ciphertext, nonce) for storage in secret_share_links rows or in
// a separate cipher-payload column. AES-256-GCM matches the rest of the
// codebase's at-rest encryption.
func Encrypt(master []byte, token string, plaintext string) (ciphertext, nonce []byte, err error) {
	key, err := DeriveKey(master, token)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("gcm: %w", err)
	}
	nonce = make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("nonce: %w", err)
	}
	ciphertext = aead.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// Decrypt is the inverse of Encrypt. Returns AEAD error if the token,
// master, ciphertext, or nonce don't match — the test suite asserts that
// a wrong token does NOT decrypt (the property D4 is designed to provide).
func Decrypt(master []byte, token string, ciphertext, nonce []byte) (string, error) {
	key, err := DeriveKey(master, token)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("share decrypt: %w", err)
	}
	return string(plain), nil
}
