package secretshare_test

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/secretshare"
)

// TestGenerateToken_NoCollisions exercises the entropy property required
// by Plans.md Task 3.1 DoD: 10k tokens, zero collisions. 256 bits of
// entropy makes the probability negligible across any realistic deploy.
func TestGenerateToken_NoCollisions(t *testing.T) {
	const N = 10_000
	seen := make(map[string]struct{}, N)
	for i := 0; i < N; i++ {
		tok, hash, err := secretshare.GenerateToken()
		if err != nil {
			t.Fatalf("generate %d: %v", i, err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("collision after %d iterations", i)
		}
		seen[tok] = struct{}{}
		if len(hash) != 32 {
			t.Errorf("hash size: got %d, want 32", len(hash))
		}
		if hash == nil {
			t.Error("hash nil")
		}
	}
}

func TestGenerateToken_URLSafe(t *testing.T) {
	tok, _, err := secretshare.GenerateToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(tok); err != nil {
		t.Errorf("token not URL-safe base64: %v (raw=%q)", err, tok)
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	tok := "fixed-test-token"
	a := secretshare.HashToken(tok)
	b := secretshare.HashToken(tok)
	if len(a) != 32 || len(b) != 32 {
		t.Fatalf("hash sizes: %d / %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("hash not deterministic at byte %d", i)
		}
	}
}

// TestEncryptDecrypt_RoundTrip — happy path: correct (master, token) pair
// decrypts the ciphertext back to plaintext.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	master := randomKey(t)
	tok := "test-token-abc"
	const plaintext = "the secret value"

	ct, nonce, err := secretshare.Encrypt(master, tok, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	out, err := secretshare.Decrypt(master, tok, ct, nonce)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if out != plaintext {
		t.Errorf("plaintext mismatch: got %q, want %q", out, plaintext)
	}
}

// TestDecrypt_WrongTokenFails encodes the D4 property: even with the same
// master key and the correct ciphertext+nonce, the wrong token must NOT
// decrypt. This is what makes a DB-only compromise insufficient.
func TestDecrypt_WrongTokenFails(t *testing.T) {
	master := randomKey(t)
	ct, nonce, err := secretshare.Encrypt(master, "real-token", "stuff")
	if err != nil {
		t.Fatalf("enc: %v", err)
	}
	if _, err := secretshare.Decrypt(master, "wrong-token", ct, nonce); err == nil {
		t.Error("decrypt with wrong token should fail (AEAD auth)")
	}
}

// TestDecrypt_WrongMasterFails the symmetric property: even with the
// correct token, a different master key fails. Protects against
// cross-deployment ciphertext replay.
func TestDecrypt_WrongMasterFails(t *testing.T) {
	master1 := randomKey(t)
	master2 := randomKey(t)
	ct, nonce, err := secretshare.Encrypt(master1, "tok", "stuff")
	if err != nil {
		t.Fatalf("enc: %v", err)
	}
	if _, err := secretshare.Decrypt(master2, "tok", ct, nonce); err == nil {
		t.Error("decrypt with wrong master should fail")
	}
}

func TestDeriveKey_RejectsEmpty(t *testing.T) {
	if _, err := secretshare.DeriveKey(nil, "tok"); err == nil {
		t.Error("empty master should fail")
	}
	master := randomKey(t)
	if _, err := secretshare.DeriveKey(master, ""); err == nil {
		t.Error("empty token should fail")
	}
}

func randomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}
