package store

import (
	"context"
	"database/sql"
	"encoding/hex"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// AppSecretRepo owns the app_secrets table — the structured home (R3) for
// encrypted integration secrets that used to sprawl across app_settings as
// hex-encoded <key>_cipher / <key>_nonce row pairs. Values are stored as hex of
// the AEAD ciphertext/nonce (unchanged encoding); this repo encapsulates the
// hex+crypto so callers deal only in plaintext.
type AppSecretRepo struct {
	db *sql.DB
}

// NewAppSecretRepo constructs an AppSecretRepo over the given DB handle.
func NewAppSecretRepo(db *sql.DB) *AppSecretRepo { return &AppSecretRepo{db: db} }

// getHex returns the stored hex cipher/nonce for a key, or empty strings if no
// secret is stored.
func (r *AppSecretRepo) getHex(ctx context.Context, key string) (cipherHex, nonceHex string, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT cipher, nonce FROM app_secrets WHERE key = ?`, key).Scan(&cipherHex, &nonceHex)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return cipherHex, nonceHex, err
}

// Configured reports whether a non-empty secret is stored for key.
func (r *AppSecretRepo) Configured(ctx context.Context, key string) bool {
	cipherHex, _, _ := r.getHex(ctx, key)
	return cipherHex != ""
}

// Reveal decrypts the secret stored under key. Returns ("", false, nil) when no
// secret is configured; an error only on a genuine decode/decrypt failure.
func (r *AppSecretRepo) Reveal(ctx context.Context, enc *database.Encryptor, key string) (string, bool, error) {
	cipherHex, nonceHex, err := r.getHex(ctx, key)
	if err != nil {
		return "", false, err
	}
	if cipherHex == "" || nonceHex == "" {
		return "", false, nil
	}
	cipher, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", false, err
	}
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return "", false, err
	}
	plain, err := enc.Decrypt(cipher, nonce)
	if err != nil {
		return "", false, err
	}
	return plain, true, nil
}

// Store encrypts plaintext and upserts it under key.
func (r *AppSecretRepo) Store(ctx context.Context, enc *database.Encryptor, key, plaintext string) error {
	cipher, nonce, err := enc.Encrypt(plaintext)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO app_secrets (key, cipher, nonce) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET cipher = excluded.cipher, nonce = excluded.nonce`,
		key, hex.EncodeToString(cipher), hex.EncodeToString(nonce))
	return err
}

// Clear removes the secret stored under key.
func (r *AppSecretRepo) Clear(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM app_secrets WHERE key = ?`, key)
	return err
}
