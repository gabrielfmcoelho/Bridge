package vault

import (
	"context"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	gossh "golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// derived.go — columns computed from a secret's plaintext when it's written,
// so lists never decrypt: the login user, the SSH key fingerprint, and a
// keyed fingerprint of the value that groups identical credentials (the same
// password on 14 VMs). The value fingerprint is an HMAC under a key derived
// from the master key — a plain hash of a short password would be brute-
// forceable from a DB dump; this isn't without the master key.

const valueFPInfo = "bridge/secret-value-fp/v1"

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// derivedCols are what stampDerived writes next to a secret's payload.
type derivedCols struct {
	valueFP  []byte
	sshFP    string // "SHA256:…" for sshkey, else ""
	username string // from the JSON payload, else "" (column left as is)
}

// fpKey derives the fingerprint key from the master key. Cheap; not cached so
// a test encryptor per DB just works.
func fpKey(enc *database.Encryptor) ([]byte, error) {
	return hkdf.Key(sha256.New, enc.MasterKey(), nil, valueFPInfo, 32)
}

// canonicalValue is what two "identical" secrets share. An SSH key is its
// private key alone: copies made per host carry no username, the library
// original does, and they're still the same key.
func canonicalValue(typ models.SecretType, plaintext string) string {
	if typ == models.SecretTypePassword {
		return PasswordPlain(plaintext)
	}
	if typ == models.SecretTypeSSHKey {
		var k HostSSHKey
		if json.Unmarshal([]byte(plaintext), &k) == nil {
			if pem := strings.TrimSpace(k.PrivateKeyPEM); pem != "" {
				return pem
			}
			return strings.TrimSpace(k.PublicKey)
		}
	}
	return plaintext
}

func deriveCols(enc *database.Encryptor, typ models.SecretType, plaintext string) (derivedCols, error) {
	var d derivedCols
	key, err := fpKey(enc)
	if err != nil {
		return d, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(typ))
	mac.Write([]byte{0x1f})
	mac.Write([]byte(canonicalValue(typ, plaintext)))
	d.valueFP = mac.Sum(nil)

	switch typ {
	case models.SecretTypeSSHKey, models.SecretTypeCred, models.SecretTypeAppLogin:
		var p struct {
			Username      string `json:"username"`
			PrivateKeyPEM string `json:"private_key_pem"`
			PublicKey     string `json:"public_key"`
		}
		if json.Unmarshal([]byte(plaintext), &p) == nil {
			d.username = p.Username
			if typ == models.SecretTypeSSHKey {
				d.sshFP = sshFingerprint(p.PrivateKeyPEM, p.PublicKey)
			}
		}
	}
	return d, nil
}

// sshFingerprint is the SHA256 fingerprint of the key pair, from the private
// key when it parses (it can't be passphrase-protected here), else from the
// public key. "" when neither parses.
func sshFingerprint(privPEM, pub string) string {
	if privPEM != "" {
		if signer, err := gossh.ParsePrivateKey([]byte(privPEM)); err == nil {
			return gossh.FingerprintSHA256(signer.PublicKey())
		}
	}
	if pub != "" {
		if pk, _, _, _, err := gossh.ParseAuthorizedKey([]byte(pub)); err == nil {
			return gossh.FingerprintSHA256(pk)
		}
	}
	return ""
}

// NormalizeSSHKeyPayload fills a missing public key from the private one so
// every stored key can be installed on a server. Other payloads and keys that
// don't parse pass through unchanged.
func NormalizeSSHKeyPayload(plaintext string) string {
	var k HostSSHKey
	if json.Unmarshal([]byte(plaintext), &k) != nil || k.PrivateKeyPEM == "" || strings.TrimSpace(k.PublicKey) != "" {
		return plaintext
	}
	signer, err := gossh.ParsePrivateKey([]byte(k.PrivateKeyPEM))
	if err != nil {
		return plaintext
	}
	k.PublicKey = strings.TrimSpace(string(gossh.MarshalAuthorizedKey(signer.PublicKey())))
	out, err := json.Marshal(k)
	if err != nil {
		return plaintext
	}
	return string(out)
}

// stampDerived writes the derived columns of secret id. Called by every path
// that writes a payload, in the same transaction when there is one.
func stampDerived(ctx context.Context, ex execer, enc *database.Encryptor, id int64, typ models.SecretType, plaintext string) error {
	d, err := deriveCols(enc, typ, plaintext)
	if err != nil {
		return fmt.Errorf("derive secret %d: %w", id, err)
	}
	_, err = ex.ExecContext(ctx,
		`UPDATE secrets SET value_fingerprint = ?, ssh_fingerprint = NULLIF(?, ''),
		        username = COALESCE(NULLIF(?, ''), username)
		  WHERE id = ?`,
		d.valueFP, d.sshFP, d.username, id)
	return err
}

// BackfillDerived stamps live secrets written before v91 (or by a path that
// doesn't stamp). Idempotent: only rows without a value fingerprint. After a
// master-key rotation, NULL the column and this recomputes it.
func BackfillDerived(ctx context.Context, db *sql.DB, enc *database.Encryptor) (int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, type, payload_ciphertext, payload_nonce FROM secrets
		  WHERE value_fingerprint IS NULL AND deleted_at IS NULL`)
	if err != nil {
		return 0, err
	}
	type pending struct {
		id    int64
		typ   models.SecretType
		plain string
	}
	var todo []pending
	for rows.Next() {
		var p pending
		var typ string
		var ct, nonce []byte
		if err := rows.Scan(&p.id, &typ, &ct, &nonce); err != nil {
			rows.Close()
			return 0, err
		}
		plain, err := enc.Decrypt(ct, nonce)
		if err != nil {
			continue // undecryptable (other key): leave it ungrouped
		}
		p.typ, p.plain = models.SecretType(typ), plain
		todo = append(todo, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, p := range todo {
		if err := stampDerived(ctx, db, enc, p.id, p.typ, p.plain); err != nil {
			return 0, err
		}
	}
	return len(todo), nil
}
