package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ExternalToolRepo owns SQL for external_tools. Soft-delete + cascade of child
// secrets is handled by the store cascade registry (DeleteParent/RestoreParent
// for the tool scope); this repo covers the remaining CRUD. The HasCredentials
// flag is computed per-row from the secrets table.
type ExternalToolRepo struct {
	db *sql.DB
}

// NewExternalToolRepo constructs an ExternalToolRepo over the given DB handle.
func NewExternalToolRepo(db *sql.DB) *ExternalToolRepo { return &ExternalToolRepo{db: db} }

// hasCredsExpr computes HasCredentials: true when the tool's linked service has
// at least one live secret. Shared by Get and List.
const hasCredsExpr = `(CASE WHEN t.service_id IS NOT NULL AND EXISTS (SELECT 1 FROM secrets s WHERE s.scope = 'service' AND s.parent_id = t.service_id AND s.deleted_at IS NULL) THEN 1 ELSE 0 END)`

func scanExternalTool(scanner interface{ Scan(...any) error }, t *models.ExternalTool) error {
	return scanner.Scan(&t.ID, &t.Name, &t.Description, &t.URL, &t.Icon, &t.EmbedEnabled, &t.SortOrder,
		&t.ServiceID, &t.DNSID, &t.Source, &t.HasCredentials, &t.CreatedAt, &t.UpdatedAt)
}

// Create inserts a tool (defaulting source to "manual") and sets t.ID.
func (r *ExternalToolRepo) Create(ctx context.Context, t *models.ExternalTool) error {
	if t.Source == "" {
		t.Source = "manual"
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO external_tools (name, description, url, icon, embed_enabled, sort_order, service_id, dns_id, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.Description, t.URL, t.Icon, t.EmbedEnabled, t.SortOrder, t.ServiceID, t.DNSID, t.Source,
	)
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

// Get returns a live (non-deleted) tool by id, or (nil, nil) if absent.
func (r *ExternalToolRepo) Get(ctx context.Context, id int64) (*models.ExternalTool, error) {
	t := &models.ExternalTool{}
	err := scanExternalTool(r.db.QueryRowContext(ctx,
		`SELECT t.id, t.name, t.description, t.url, t.icon, t.embed_enabled, t.sort_order,
			t.service_id, t.dns_id, t.source, `+hasCredsExpr+`, t.created_at, t.updated_at
		 FROM external_tools t WHERE t.id = ? AND t.deleted_at IS NULL`, id), t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// List returns all live tools ordered by sort_order, name.
func (r *ExternalToolRepo) List(ctx context.Context) ([]models.ExternalTool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.description, t.url, t.icon, t.embed_enabled, t.sort_order,
			t.service_id, t.dns_id, t.source, `+hasCredsExpr+`, t.created_at, t.updated_at
		 FROM external_tools t WHERE t.deleted_at IS NULL ORDER BY t.sort_order, t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tools []models.ExternalTool
	for rows.Next() {
		var t models.ExternalTool
		if err := scanExternalTool(rows, &t); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// Update writes the mutable fields of an existing tool by id.
func (r *ExternalToolRepo) Update(ctx context.Context, t *models.ExternalTool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE external_tools SET name = ?, description = ?, url = ?, icon = ?, embed_enabled = ?, sort_order = ?,
			service_id = ?, dns_id = ?, source = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		t.Name, t.Description, t.URL, t.Icon, t.EmbedEnabled, t.SortOrder,
		t.ServiceID, t.DNSID, t.Source, t.ID,
	)
	return err
}

// Delete hard-deletes a tool row by id (distinct from the soft-delete path that
// flows through the cascade registry).
func (r *ExternalToolRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM external_tools WHERE id = ?`, id)
	return err
}

// GetByServiceAndDNS looks up an existing live synced tool for a service+DNS
// pair (excludes soft-deleted so a sync re-creates rather than resurrects).
func (r *ExternalToolRepo) GetByServiceAndDNS(ctx context.Context, serviceID, dnsID int64) (*models.ExternalTool, error) {
	t := &models.ExternalTool{}
	err := scanExternalTool(r.db.QueryRowContext(ctx,
		`SELECT t.id, t.name, t.description, t.url, t.icon, t.embed_enabled, t.sort_order,
			t.service_id, t.dns_id, t.source, 0, t.created_at, t.updated_at
		 FROM external_tools t WHERE t.service_id = ? AND t.dns_id = ? AND t.deleted_at IS NULL`, serviceID, dnsID), t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}
