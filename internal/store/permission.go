package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// PermissionRepo owns SQL for permissions, role_permissions, and
// auth_role_mappings (the RBAC tables). Referenced by api handlers and the auth
// middleware; built inline until the Phase 2 container hoists it.
type PermissionRepo struct {
	db *sql.DB
}

// NewPermissionRepo constructs a PermissionRepo over the given DB handle.
func NewPermissionRepo(db *sql.DB) *PermissionRepo { return &PermissionRepo{db: db} }

// ListPermissions returns all defined permissions ordered by category, code.
func (r *PermissionRepo) ListPermissions(ctx context.Context) ([]models.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT code, description, category FROM permissions ORDER BY category, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.Code, &p.Description, &p.Category); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ListForRole returns all permission codes granted to a role.
func (r *PermissionRepo) ListForRole(ctx context.Context, role string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT permission FROM role_permissions WHERE role = ? ORDER BY permission`, role)
	if err != nil {
		return nil, err
	}
	return scanStrings(rows)
}

// Has reports whether a role has a specific permission. Admin always passes.
func (r *PermissionRepo) Has(ctx context.Context, role, permission string) bool {
	if role == "admin" {
		return true
	}
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM role_permissions WHERE role = ? AND permission = ?`, role, permission).Scan(&count)
	return err == nil && count > 0
}

// ListAllRolePermissions returns all role→permission mappings.
func (r *PermissionRepo) ListAllRolePermissions(ctx context.Context) ([]models.RolePermission, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT role, permission FROM role_permissions ORDER BY role, permission`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rps []models.RolePermission
	for rows.Next() {
		var rp models.RolePermission
		if err := rows.Scan(&rp.Role, &rp.Permission); err != nil {
			return nil, err
		}
		rps = append(rps, rp)
	}
	return rps, rows.Err()
}

// SetForRole replaces all permissions for a role (one tx).
func (r *PermissionRepo) SetForRole(ctx context.Context, role string, permissions []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role = ?`, role); err != nil {
		return err
	}
	for _, p := range permissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_permissions (role, permission) VALUES (?, ?)`, role, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListRoleMappings returns all external-group→local-role mappings.
func (r *PermissionRepo) ListRoleMappings(ctx context.Context) ([]models.AuthRoleMapping, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, provider_name, external_group, local_role FROM auth_role_mappings ORDER BY provider_name, external_group`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mappings []models.AuthRoleMapping
	for rows.Next() {
		var m models.AuthRoleMapping
		if err := rows.Scan(&m.ID, &m.ProviderName, &m.ExternalGroup, &m.LocalRole); err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// CreateRoleMapping inserts an external-group→role mapping and sets m.ID.
func (r *PermissionRepo) CreateRoleMapping(ctx context.Context, m *models.AuthRoleMapping) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO auth_role_mappings (provider_name, external_group, local_role) VALUES (?, ?, ?)`,
		m.ProviderName, m.ExternalGroup, m.LocalRole,
	)
	if err != nil {
		return err
	}
	m.ID = id
	return nil
}

// DeleteRoleMapping deletes a role mapping by id.
func (r *PermissionRepo) DeleteRoleMapping(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_role_mappings WHERE id = ?`, id)
	return err
}

// ResolveRoleFromExternalGroups returns the highest-privilege local role that
// matches any of the given external groups for the provider, or "" if none.
func (r *PermissionRepo) ResolveRoleFromExternalGroups(ctx context.Context, providerName string, groups []string) string {
	levels := map[string]int{"viewer": 0, "editor": 1, "admin": 2}
	bestRole := ""
	bestLevel := -1
	for _, group := range groups {
		var localRole string
		err := r.db.QueryRowContext(ctx,
			`SELECT local_role FROM auth_role_mappings WHERE provider_name = ? AND external_group = ?`,
			providerName, group).Scan(&localRole)
		if err == nil {
			if level, ok := levels[localRole]; ok && level > bestLevel {
				bestRole = localRole
				bestLevel = level
			}
		}
	}
	return bestRole
}
