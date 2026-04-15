package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type SSHKey struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	CredentialType       string    `json:"credential_type"` // "key" or "password"
	Username             string    `json:"username"`
	Description          string    `json:"description"`
	PubKeyCiphertext     []byte    `json:"-"`
	PubKeyNonce          []byte    `json:"-"`
	PrivKeyCiphertext    []byte    `json:"-"`
	PrivKeyNonce         []byte    `json:"-"`
	PasswordCiphertext   []byte    `json:"-"`
	PasswordNonce        []byte    `json:"-"`
	Fingerprint          string    `json:"fingerprint"`
	CreatedAt            time.Time `json:"created_at"`
}

const sshKeyCols = `id, name, credential_type, username, description,
	pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce,
	password_ciphertext, password_nonce, fingerprint, created_at`

func scanSSHKey(scanner interface{ Scan(...any) error }, k *SSHKey) error {
	return scanner.Scan(&k.ID, &k.Name, &k.CredentialType, &k.Username, &k.Description,
		&k.PubKeyCiphertext, &k.PubKeyNonce, &k.PrivKeyCiphertext, &k.PrivKeyNonce,
		&k.PasswordCiphertext, &k.PasswordNonce, &k.Fingerprint, &k.CreatedAt)
}

func ListSSHKeys(db *sql.DB) ([]SSHKey, error) {
	rows, err := db.Query(`SELECT ` + sshKeyCols + ` FROM ssh_keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []SSHKey
	for rows.Next() {
		var k SSHKey
		if err := scanSSHKey(rows, &k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func GetSSHKey(db *sql.DB, id int64) (*SSHKey, error) {
	k := &SSHKey{}
	err := scanSSHKey(db.QueryRow(`SELECT `+sshKeyCols+` FROM ssh_keys WHERE id = ?`, id), k)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}

func CreateSSHKey(db *sql.DB, k *SSHKey) error {
	if k.CredentialType == "" {
		k.CredentialType = "key"
	}
	id, err := database.InsertReturningID(db,
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

func UpdateSSHKey(db *sql.DB, k *SSHKey) error {
	_, err := db.Exec(
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

func DeleteSSHKey(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM ssh_keys WHERE id = ?`, id)
	return err
}
