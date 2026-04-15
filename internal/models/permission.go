package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// Permission represents a granular permission code.
type Permission struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// RolePermission maps a role to a permission.
type RolePermission struct {
	Role       string `json:"role"`
	Permission string `json:"permission"`
}

// AuthRoleMapping maps an external group to a local role.
type AuthRoleMapping struct {
	ID            int64  `json:"id"`
	ProviderName  string `json:"provider_name"`
	ExternalGroup string `json:"external_group"`
	LocalRole     string `json:"local_role"`
}

// ListPermissions returns all defined permissions ordered by category and code.
func ListPermissions(db *sql.DB) ([]Permission, error) {
	rows, err := db.Query(`SELECT code, description, category FROM permissions ORDER BY category, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Code, &p.Description, &p.Category); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ListPermissionsForRole returns all permission codes granted to a role.
func ListPermissionsForRole(db *sql.DB, role string) ([]string, error) {
	rows, err := db.Query(`SELECT permission FROM role_permissions WHERE role = ? ORDER BY permission`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// HasPermission checks whether a role has a specific permission.
func HasPermission(db *sql.DB, role, permission string) bool {
	if role == "admin" {
		return true
	}
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM role_permissions WHERE role = ? AND permission = ?`,
		role, permission,
	).Scan(&count)
	return err == nil && count > 0
}

// ListAllRolePermissions returns all role-permission mappings.
func ListAllRolePermissions(db *sql.DB) ([]RolePermission, error) {
	rows, err := db.Query(`SELECT role, permission FROM role_permissions ORDER BY role, permission`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rps []RolePermission
	for rows.Next() {
		var rp RolePermission
		if err := rows.Scan(&rp.Role, &rp.Permission); err != nil {
			return nil, err
		}
		rps = append(rps, rp)
	}
	return rps, rows.Err()
}

// SetRolePermissions replaces all permissions for a role.
func SetRolePermissions(db *sql.DB, role string, permissions []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM role_permissions WHERE role = ?`, role); err != nil {
		tx.Rollback()
		return err
	}
	for _, p := range permissions {
		if _, err := tx.Exec(`INSERT INTO role_permissions (role, permission) VALUES (?, ?)`, role, p); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListAuthRoleMappings returns all external group-to-role mappings.
func ListAuthRoleMappings(db *sql.DB) ([]AuthRoleMapping, error) {
	rows, err := db.Query(`SELECT id, provider_name, external_group, local_role FROM auth_role_mappings ORDER BY provider_name, external_group`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []AuthRoleMapping
	for rows.Next() {
		var m AuthRoleMapping
		if err := rows.Scan(&m.ID, &m.ProviderName, &m.ExternalGroup, &m.LocalRole); err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// CreateAuthRoleMapping creates a new external group-to-role mapping.
func CreateAuthRoleMapping(db *sql.DB, m *AuthRoleMapping) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO auth_role_mappings (provider_name, external_group, local_role) VALUES (?, ?, ?)`,
		m.ProviderName, m.ExternalGroup, m.LocalRole,
	)
	if err != nil {
		return err
	}
	m.ID = id
	return nil
}

// DeleteAuthRoleMapping deletes a role mapping by ID.
func DeleteAuthRoleMapping(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM auth_role_mappings WHERE id = ?`, id)
	return err
}

// ResolveRoleFromExternalGroups returns the highest-privilege local role
// that matches any of the given external groups for the given provider.
// Returns empty string if no mapping matches.
func ResolveRoleFromExternalGroups(db *sql.DB, providerName string, groups []string) string {
	levels := map[string]int{"viewer": 0, "editor": 1, "admin": 2}
	bestRole := ""
	bestLevel := -1

	for _, group := range groups {
		var localRole string
		err := db.QueryRow(
			`SELECT local_role FROM auth_role_mappings WHERE provider_name = ? AND external_group = ?`,
			providerName, group,
		).Scan(&localRole)
		if err == nil {
			if level, ok := levels[localRole]; ok && level > bestLevel {
				bestRole = localRole
				bestLevel = level
			}
		}
	}
	return bestRole
}
