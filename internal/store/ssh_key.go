package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// SSHKeyRepo owns SQL for ssh_keys (managed SSH credentials; legacy, gradually
// superseded by the secrets vault). Referenced by sshkey/sshconfig/host/coolify
// handlers; built inline until the Phase 2 container hoists it.
type SSHKeyRepo struct {
	db *sql.DB
}

// NewSSHKeyRepo constructs an SSHKeyRepo over the given DB handle.
func NewSSHKeyRepo(db *sql.DB) *SSHKeyRepo { return &SSHKeyRepo{db: db} }

const sshKeyCols = `id, name, credential_type, username, description,
	pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce,
	password_ciphertext, password_nonce, fingerprint, created_at`

func scanSSHKey(scanner interface{ Scan(...any) error }, k *models.SSHKey) error {
	return scanner.Scan(&k.ID, &k.Name, &k.CredentialType, &k.Username, &k.Description,
		&k.PubKeyCiphertext, &k.PubKeyNonce, &k.PrivKeyCiphertext, &k.PrivKeyNonce,
		&k.PasswordCiphertext, &k.PasswordNonce, &k.Fingerprint, &k.CreatedAt)
}

// List returns all ssh keys ordered by name.
func (r *SSHKeyRepo) List(ctx context.Context) ([]models.SSHKey, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sshKeyCols+` FROM ssh_keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []models.SSHKey
	for rows.Next() {
		var k models.SSHKey
		if err := scanSSHKey(rows, &k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// Get returns the ssh key with the given id, or (nil, nil) if absent.
func (r *SSHKeyRepo) Get(ctx context.Context, id int64) (*models.SSHKey, error) {
	k := &models.SSHKey{}
	err := scanSSHKey(r.db.QueryRowContext(ctx, `SELECT `+sshKeyCols+` FROM ssh_keys WHERE id = ?`, id), k)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// Create inserts an ssh key (defaulting credential_type to "key") and sets k.ID.
func (r *SSHKeyRepo) Create(ctx context.Context, k *models.SSHKey) error {
	if k.CredentialType == "" {
		k.CredentialType = "key"
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO ssh_keys (name, credential_type, username, description,
			pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce,
			password_ciphertext, password_nonce, fingerprint)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.Name, k.CredentialType, k.Username, k.Description,
		k.PubKeyCiphertext, k.PubKeyNonce, k.PrivKeyCiphertext, k.PrivKeyNonce,
		k.PasswordCiphertext, k.PasswordNonce, k.Fingerprint,
	)
	if err != nil {
		return err
	}
	k.ID = id
	return nil
}

// Update writes all fields of an existing ssh key by id.
func (r *SSHKeyRepo) Update(ctx context.Context, k *models.SSHKey) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ssh_keys SET name = ?, credential_type = ?, username = ?, description = ?,
			pub_key_ciphertext = ?, pub_key_nonce = ?, priv_key_ciphertext = ?, priv_key_nonce = ?,
			password_ciphertext = ?, password_nonce = ?, fingerprint = ?
		 WHERE id = ?`,
		k.Name, k.CredentialType, k.Username, k.Description,
		k.PubKeyCiphertext, k.PubKeyNonce, k.PrivKeyCiphertext, k.PrivKeyNonce,
		k.PasswordCiphertext, k.PasswordNonce, k.Fingerprint, k.ID,
	)
	return err
}

// Delete removes an ssh key by id.
func (r *SSHKeyRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id = ?`, id)
	return err
}
