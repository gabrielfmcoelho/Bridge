package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// GlpiToken is a named GLPI account profile — each one ties sshcm to a single
// GLPI user's personal API token. One app supports N of these (e.g. per team).
type GlpiToken struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	UserTokenCipher []byte    `json:"-"`
	UserTokenNonce  []byte    `json:"-"`
	HasToken        bool      `json:"has_token"`
	DefaultEntityID int       `json:"default_entity_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// GlpiTokenInput is the admin-supplied subset — user_token is plaintext on the
// way in and gets encrypted before persistence.
type GlpiTokenInput struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	UserToken       string `json:"user_token"`
	DefaultEntityID int    `json:"default_entity_id"`
}

const glpiTokenCols = `id, name, description, user_token_cipher, user_token_nonce, default_entity_id, created_at, updated_at`

func scanGlpiToken(scanner interface{ Scan(...any) error }, t *GlpiToken) error {
	if err := scanner.Scan(&t.ID, &t.Name, &t.Description, &t.UserTokenCipher, &t.UserTokenNonce, &t.DefaultEntityID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return err
	}
	t.HasToken = len(t.UserTokenCipher) > 0 && len(t.UserTokenNonce) > 0
	return nil
}

// ListGlpiTokens returns every profile. Ordered by name for stable UI listings.
func ListGlpiTokens(db *sql.DB) ([]GlpiToken, error) {
	rows, err := db.Query(`SELECT ` + glpiTokenCols + ` FROM glpi_tokens ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GlpiToken
	for rows.Next() {
		var t GlpiToken
		if err := scanGlpiToken(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetGlpiToken returns one profile by id (includes the encrypted token — callers
// that need the plaintext must decrypt via the Encryptor they hold).
func GetGlpiToken(db *sql.DB, id int64) (*GlpiToken, error) {
	t := &GlpiToken{}
	if err := scanGlpiToken(db.QueryRow(`SELECT `+glpiTokenCols+` FROM glpi_tokens WHERE id = ?`, id), t); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// CreateGlpiToken inserts a new profile. Caller is expected to have already
// encrypted the user token and populated the cipher/nonce fields.
func CreateGlpiToken(db *sql.DB, t *GlpiToken) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO glpi_tokens (name, description, user_token_cipher, user_token_nonce, default_entity_id)
		VALUES (?, ?, ?, ?, ?)`,
		t.Name, t.Description, t.UserTokenCipher, t.UserTokenNonce, t.DefaultEntityID,
	)
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

// UpdateGlpiToken rewrites a profile. An empty UserTokenCipher leaves the
// existing stored token alone — the CRUD handler decides whether to pass new
// bytes (if the admin typed a replacement) or reuse the existing ones.
func UpdateGlpiToken(db *sql.DB, t *GlpiToken) error {
	if t.UserTokenCipher != nil {
		_, err := db.Exec(
			`UPDATE glpi_tokens SET name = ?, description = ?, user_token_cipher = ?, user_token_nonce = ?, default_entity_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			t.Name, t.Description, t.UserTokenCipher, t.UserTokenNonce, t.DefaultEntityID, t.ID,
		)
		return err
	}
	// No new token — preserve existing cipher/nonce.
	_, err := db.Exec(
		`UPDATE glpi_tokens SET name = ?, description = ?, default_entity_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		t.Name, t.Description, t.DefaultEntityID, t.ID,
	)
	return err
}

// DeleteGlpiToken removes a profile by id. The ON DELETE SET NULL on
// projects.glpi_token_id means projects pointing here go unassigned, not
// deleted — admins can re-assign them later.
func DeleteGlpiToken(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM glpi_tokens WHERE id = ?`, id)
	return err
}
