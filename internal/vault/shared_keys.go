package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// shared_keys.go — shared host credentials: avulso, shared vault secrets
// (an sshkey or a password) that hosts link to through host_remote_users
// instead of holding a copy. Server-side helpers (no ACL): the HTTP layer
// checks the caller may see the secret before linking or using it.

// ErrNotSharedCredential is returned when an id isn't a live shared host
// credential of the wanted type.
var ErrNotSharedCredential = errors.New("not a shared host credential")

// SharedKey is a shared SSH key loaded for server-side use.
type SharedKey struct {
	ID   int64
	Name string
	HostSSHKey
}

// LoadSharedKey returns the shared SSH key secretID, which must carry a
// private key.
func LoadSharedKey(ctx context.Context, db *database.DB, secretID int64) (SharedKey, error) {
	name, plain, err := loadShared(ctx, db, secretID, models.SecretTypeSSHKey)
	if err != nil {
		return SharedKey{}, err
	}
	k := SharedKey{ID: secretID, Name: name}
	if err := json.Unmarshal([]byte(plain), &k.HostSSHKey); err != nil {
		return SharedKey{}, fmt.Errorf("shared key %d payload: %w", secretID, err)
	}
	if k.PrivateKeyPEM == "" {
		return SharedKey{}, fmt.Errorf("credential %q has no private key — add one or pick a different key", name)
	}
	return k, nil
}

func loadShared(ctx context.Context, db *database.DB, id int64, typ models.SecretType) (string, string, error) {
	var name string
	var ct, nonce []byte
	err := db.SQL.QueryRowContext(ctx,
		`SELECT name, payload_ciphertext, payload_nonce FROM secrets
		  WHERE id = ? AND type = ? AND scope = 'avulso' AND visibility = 'shared' AND deleted_at IS NULL`,
		id, string(typ)).Scan(&name, &ct, &nonce)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotSharedCredential
	}
	if err != nil {
		return "", "", err
	}
	plain, err := db.Encryptor.Decrypt(ct, nonce)
	return name, plain, err
}

// KeySecretForLegacy maps an old ssh_keys id (clients before v91) to the
// vault key it was migrated to; 0 when there is none.
func KeySecretForLegacy(ctx context.Context, db *sql.DB, sshKeyID int64) int64 {
	var id int64
	_ = db.QueryRowContext(ctx,
		`SELECT id FROM secrets WHERE legacy_ssh_key_id = ? AND type = 'sshkey' AND deleted_at IS NULL`, sshKeyID).Scan(&id)
	return id
}

// LinkHostCredential points the host's login user (hosts.ssh_user) at the
// shared credential — key_secret_id for a key, secret_id for a password —
// creating the host_remote_users row when needed; secretID 0 unlinks.
func LinkHostCredential(ctx context.Context, ex execer, hostID, secretID int64, typ models.SecretType) error {
	return LinkRemoteUserCredential(ctx, ex, hostID, "", secretID, typ)
}

// LinkRemoteUserCredential is LinkHostCredential for a named remote user;
// "" means the host's login user.
func LinkRemoteUserCredential(ctx context.Context, ex execer, hostID int64, username string, secretID int64, typ models.SecretType) error {
	col := "secret_id"
	if typ == models.SecretTypeSSHKey {
		col = "key_secret_id"
	}
	var arg any
	if secretID > 0 {
		arg = secretID
	}
	userExpr := "?"
	args := []any{hostID, username, arg}
	if username == "" {
		userExpr = "(SELECT ssh_user FROM hosts WHERE id = ?)"
		args = []any{hostID, hostID, arg}
	}
	_, err := ex.ExecContext(ctx,
		`INSERT INTO host_remote_users (host_id, username, `+col+`, created_at, updated_at)
		 VALUES (?, `+userExpr+`, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (host_id, username) DO UPDATE SET `+col+` = excluded.`+col+`, updated_at = CURRENT_TIMESTAMP`,
		args...)
	return err
}

// Fingerprint is the key's SHA256 fingerprint ("SHA256:…"), "" if it doesn't parse.
func (k SharedKey) Fingerprint() string { return sshFingerprint(k.PrivateKeyPEM, k.PublicKey) }

// SharedKeyNames maps SSH fingerprint → name for the shared keys in the vault
// (what used to be the ssh_keys library): "is this installed key one we
// manage?" and the ~/.ssh/<name> an identity file is saved as.
func SharedKeyNames(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ssh_fingerprint, name FROM secrets
		  WHERE type = 'sshkey' AND scope = 'avulso' AND ssh_fingerprint IS NOT NULL AND deleted_at IS NULL
		  ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var fp, name string
		if err := rows.Scan(&fp, &name); err != nil {
			return nil, err
		}
		if _, seen := out[fp]; !seen {
			out[fp] = name
		}
	}
	return out, rows.Err()
}
