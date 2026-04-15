package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// Encryptor provides AES-256-GCM encryption for sensitive data.
type Encryptor struct {
	aead cipher.AEAD
}

// NewEncryptor creates an Encryptor by loading or generating a 32-byte key at keyPath.
func NewEncryptor(keyPath string) (*Encryptor, error) {
	key, err := loadOrCreateKey(keyPath)
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

	return &Encryptor{aead: aead}, nil
}

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

func loadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil && len(data) == 32 {
		return data, nil
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}
