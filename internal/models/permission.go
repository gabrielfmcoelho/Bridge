package models

// Permission represents a granular permission code. Persistence (and all RBAC
// queries) lives in internal/store.PermissionRepo — this file is the pure data
// types only.
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
