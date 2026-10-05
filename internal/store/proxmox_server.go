package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ProxmoxServerRepo owns SQL for proxmox_servers (one row per independent
// Proxmox VE cluster, each with its own encrypted API token secret).
type ProxmoxServerRepo struct {
	db *sql.DB
}

// NewProxmoxServerRepo constructs a ProxmoxServerRepo over the given DB handle.
func NewProxmoxServerRepo(db *sql.DB) *ProxmoxServerRepo { return &ProxmoxServerRepo{db: db} }

const proxmoxServerCols = `id, name, base_url, token_id, token_cipher, token_nonce, skip_verify, enabled, created_at, updated_at`

func scanProxmoxServer(scanner interface{ Scan(...any) error }, s *models.ProxmoxServer) error {
	if err := scanner.Scan(&s.ID, &s.Name, &s.BaseURL, &s.TokenID, &s.TokenCipher, &s.TokenNonce, &s.SkipVerify, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return err
	}
	s.HasToken = len(s.TokenCipher) > 0 && len(s.TokenNonce) > 0
	return nil
}

// List returns every server ordered by name; enabledOnly keeps the enabled ones.
func (r *ProxmoxServerRepo) List(ctx context.Context, enabledOnly bool) ([]models.ProxmoxServer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+proxmoxServerCols+` FROM proxmox_servers WHERE enabled OR NOT ? ORDER BY name`, enabledOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProxmoxServer{}
	for rows.Next() {
		var s models.ProxmoxServer
		if err := scanProxmoxServer(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get returns one server (with the encrypted token), or (nil, nil).
func (r *ProxmoxServerRepo) Get(ctx context.Context, id int64) (*models.ProxmoxServer, error) {
	s := &models.ProxmoxServer{}
	err := scanProxmoxServer(r.db.QueryRowContext(ctx, `SELECT `+proxmoxServerCols+` FROM proxmox_servers WHERE id = ?`, id), s)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// Create inserts s (cipher/nonce already set) and sets s.ID.
func (r *ProxmoxServerRepo) Create(ctx context.Context, s *models.ProxmoxServer) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO proxmox_servers (name, base_url, token_id, token_cipher, token_nonce, skip_verify, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.Name, s.BaseURL, s.TokenID, s.TokenCipher, s.TokenNonce, s.SkipVerify, s.Enabled)
	s.ID = id
	return err
}

// Update rewrites every column of s; the caller merges the partial input
// over the stored row first, so the stored cipher is written back unchanged
// when no new secret came in.
func (r *ProxmoxServerRepo) Update(ctx context.Context, s *models.ProxmoxServer) error {
	_, err := r.db.ExecContext(ctx, `UPDATE proxmox_servers SET name = ?, base_url = ?, token_id = ?, token_cipher = ?, token_nonce = ?,
			skip_verify = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		s.Name, s.BaseURL, s.TokenID, s.TokenCipher, s.TokenNonce, s.SkipVerify, s.Enabled, s.ID)
	return err
}

// Delete removes a server. hosts.proxmox_server_id is ON DELETE SET NULL: its
// hosts stay, unlinked, and the next sync of any server may re-link them by
// IP or name.
func (r *ProxmoxServerRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM proxmox_servers WHERE id = ?`, id)
	return err
}
