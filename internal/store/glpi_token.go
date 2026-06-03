package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// GlpiTokenRepo owns SQL for glpi_tokens (named GLPI account profiles, each
// holding one encrypted personal API token).
type GlpiTokenRepo struct {
	db *sql.DB
}

// NewGlpiTokenRepo constructs a GlpiTokenRepo over the given DB handle.
func NewGlpiTokenRepo(db *sql.DB) *GlpiTokenRepo { return &GlpiTokenRepo{db: db} }

const glpiTokenCols = `id, name, description, user_token_cipher, user_token_nonce, default_entity_id, created_at, updated_at`

func scanGlpiToken(scanner interface{ Scan(...any) error }, t *models.GlpiToken) error {
	if err := scanner.Scan(&t.ID, &t.Name, &t.Description, &t.UserTokenCipher, &t.UserTokenNonce, &t.DefaultEntityID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return err
	}
	t.HasToken = len(t.UserTokenCipher) > 0 && len(t.UserTokenNonce) > 0
	return nil
}

// List returns every profile, ordered by name.
func (r *GlpiTokenRepo) List(ctx context.Context) ([]models.GlpiToken, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+glpiTokenCols+` FROM glpi_tokens ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.GlpiToken
	for rows.Next() {
		var t models.GlpiToken
		if err := scanGlpiToken(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one profile by id (including the encrypted token), or (nil, nil).
func (r *GlpiTokenRepo) Get(ctx context.Context, id int64) (*models.GlpiToken, error) {
	t := &models.GlpiToken{}
	if err := scanGlpiToken(r.db.QueryRowContext(ctx, `SELECT `+glpiTokenCols+` FROM glpi_tokens WHERE id = ?`, id), t); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// Create inserts a new profile (cipher/nonce already populated) and sets t.ID.
func (r *GlpiTokenRepo) Create(ctx context.Context, t *models.GlpiToken) error {
	id, err := database.InsertReturningID(r.db,
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

// Update rewrites a profile. A nil UserTokenCipher preserves the existing
// stored token (the handler decides whether to pass new bytes).
func (r *GlpiTokenRepo) Update(ctx context.Context, t *models.GlpiToken) error {
	if t.UserTokenCipher != nil {
		_, err := r.db.ExecContext(ctx,
			`UPDATE glpi_tokens SET name = ?, description = ?, user_token_cipher = ?, user_token_nonce = ?, default_entity_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			t.Name, t.Description, t.UserTokenCipher, t.UserTokenNonce, t.DefaultEntityID, t.ID,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE glpi_tokens SET name = ?, description = ?, default_entity_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		t.Name, t.Description, t.DefaultEntityID, t.ID,
	)
	return err
}

// Delete removes a profile by id. projects.glpi_token_id is ON DELETE SET NULL,
// so dependent projects go unassigned rather than being deleted.
func (r *GlpiTokenRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM glpi_tokens WHERE id = ?`, id)
	return err
}
