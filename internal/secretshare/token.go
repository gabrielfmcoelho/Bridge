// Package secretshare implements the guest share-link mechanism from spec
// §5.3 / §4.1: 256-bit URL-encoded tokens whose SHA-256 hash is the only
// persistent identity in the DB, plus an HKDF-derived per-link AES key
// (D4) so that DB compromise alone cannot decrypt the shared payload.
//
// The package is split into two responsibilities:
//
//   - token.go: generation + hashing. Tokens are short-lived strings that
//     live in URLs and HTTP requests. The hash is what the DB stores in
//     secret_share_links.token_hash.
//
//   - crypto.go: HKDF derivation + Encrypt/Decrypt under the per-link key.
//     The master key never gets near the persistence layer for shared
//     payloads; only the derived key briefly exists in memory during
//     create + redeem.
//
// External surface:
//
//	tok, hash, _ := secretshare.GenerateToken()
//	ct, nonce, _ := secretshare.Encrypt(master, tok, plaintext)
//	// store (hash, ct, nonce); raw tok goes into URL.
//
//	plaintext, _ := secretshare.Decrypt(master, tok, ct, nonce)
package secretshare

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// TokenByteSize is the raw entropy of a share token before encoding.
// 256 bits matches the AES-256 keyspace this derives into and makes
// collision probability negligible across any realistic deployment.
const TokenByteSize = 32

// GenerateToken returns a fresh URL-safe share token and its SHA-256 hash.
// The token is what goes into the URL the operator hands the guest; the
// hash is what the DB stores so a DB dump alone never yields a working
// token.
func GenerateToken() (token string, hash []byte, err error) {
	raw := make([]byte, TokenByteSize)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", nil, fmt.Errorf("share token: read random: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}

// HashToken returns sha256(token). Used at redemption time to look up the
// share_links row by the constant-time-comparable hash rather than the raw
// token. Returning a slice (not [32]byte) so callers can pass it straight
// to database/sql as a BLOB arg.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
