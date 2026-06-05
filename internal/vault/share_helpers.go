package vault

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters for hashing optional public-link passphrases at rest.
// Shared by the share-bundle redemption path (originally defined for
// secret_share_links, retired in R3).
const (
	argon2Time    = 2
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// hashPassphrase returns salt||argon2id(passphrase, salt) for storage.
func hashPassphrase(passphrase string) ([]byte, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	hash := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	out := make([]byte, 0, argon2SaltLen+argon2KeyLen)
	out = append(out, salt...)
	out = append(out, hash...)
	return out, nil
}

// verifyPassphrase constant-time-checks a passphrase against a stored
// salt||hash blob.
func verifyPassphrase(passphrase string, stored []byte) bool {
	if len(stored) != argon2SaltLen+argon2KeyLen {
		return false
	}
	salt := stored[:argon2SaltLen]
	expected := stored[argon2SaltLen:]
	got := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(got, expected) == 1
}

// revealRaw decrypts a secret's payload by id (no ACL — callers that already
// resolved authorization, e.g. bundle redemption, use this).
func (r *SecretRepo) revealRaw(ctx context.Context, id int64) (string, error) {
	var ct, nonce []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT payload_ciphertext, payload_nonce FROM secrets WHERE id = ? AND deleted_at IS NULL`,
		id,
	).Scan(&ct, &nonce)
	if err != nil {
		return "", err
	}
	return r.enc.Decrypt(ct, nonce)
}

// parseTime accepts the handful of timestamp layouts SQLite/Postgres may hand
// back for DATETIME columns.
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q", s)
}
