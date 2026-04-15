package database

// TableCopyOrder lists every user-data table in a FK-safe order. Parents
// come first so children can reference them without deferring FK checks.
// Link and dependent tables follow after their referents. This drives the
// migrate-pg command and the portable backup/restore code path.
//
// schema_migrations is intentionally excluded — the target is expected to
// have been initialised to the same schema version before importing.
var TableCopyOrder = []string{
	// independent
	"users",
	"enum_options",
	"app_settings",
	"permissions",
	"contacts",
	"hosts",
	"dns_records",
	"projects",
	"external_tools",
	"ssh_keys",
	"tags",
	"oauth_states",
	"auth_role_mappings",
	// single-parent
	"orchestrators",
	"services",
	"sessions",
	"role_permissions",
	"user_external_identities",
	"user_gitlab_tokens",
	"project_gitlab_links",
	"host_scans",
	"host_responsaveis",
	"host_chamados",
	"host_alerts",
	"dns_responsaveis",
	"service_responsaveis",
	"project_responsaveis",
	// link/dependent
	"dns_host_links",
	"project_host_links",
	"service_host_links",
	"service_dns_links",
	"service_credentials",
	"service_dependencies",
	"issues",
	"issue_assignees",
	"issue_alert_links",
	"releases",
	"release_issues",
	"host_operation_logs",
}

// BoolColumns lists per-table the columns that are INTEGER 0/1 in SQLite
// but BOOLEAN in Postgres. The migrator and portable backup/restore code
// use this to coerce int64 ↔ bool as values move between dialects.
var BoolColumns = map[string]map[string]bool{
	"hosts": {
		"has_password":           true,
		"has_key":                true,
		"acesso_empresa_externa": true,
		"precisa_manutencao":     true,
	},
	"dns_records":          {"has_https": true},
	"projects":             {"tem_empresa_externa_responsavel": true, "is_directly_managed": true, "is_responsible": true},
	"services":             {"orchestrator_managed": true, "is_directly_managed": true, "is_responsible": true, "is_external_dependency": true},
	"external_tools":       {"embed_enabled": true},
	"issues":               {"archived": true},
	"host_responsaveis":    {"is_main": true, "is_externo": true},
	"dns_responsaveis":     {"is_main": true, "is_externo": true},
	"service_responsaveis": {"is_main": true, "is_externo": true},
	"project_responsaveis": {"is_main": true, "is_externo": true},
	"project_gitlab_links": {"sync_issues": true},
}

// BlobColumns lists per-table the columns stored as BLOB in SQLite and
// BYTEA in Postgres. The portable backup/restore code uses this to
// unconditionally base64-encode their values on dump and decode them on
// load, instead of relying on a content heuristic. A content heuristic
// misclassifies empty or short-printable payloads as text, which then
// fails to restore into Postgres with SQLSTATE 22P02.
var BlobColumns = map[string]map[string]bool{
	"hosts": {
		"password_ciphertext": true,
		"password_nonce":      true,
		"pub_key_ciphertext":  true,
		"pub_key_nonce":       true,
		"priv_key_ciphertext": true,
		"priv_key_nonce":      true,
	},
	"ssh_keys": {
		"pub_key_ciphertext":  true,
		"pub_key_nonce":       true,
		"priv_key_ciphertext": true,
		"priv_key_nonce":      true,
		"password_ciphertext": true,
		"password_nonce":      true,
	},
	"service_credentials": {
		"credentials_ciphertext": true,
		"credentials_nonce":      true,
	},
	"user_gitlab_tokens": {
		"access_token_cipher":  true,
		"access_token_nonce":   true,
		"refresh_token_cipher": true,
		"refresh_token_nonce":  true,
	},
}

// IntColumns lists per-table the columns that hold bigint-ish values Go
// reads into numeric fields. The portable restore uses this to coerce
// SQLite's weakly-typed values (e.g. a stray string in an int column)
// back to int64 or, failing that, 0 with a warning. Only columns that
// have been observed to contain bad data are listed; the safety net
// covers the rest via the generic "int64 destination" detection.
var IntColumns = map[string]map[string]bool{
	"issues": {"entity_id": true, "project_id": true, "service_id": true, "created_by": true, "alert_id": true},
}

// SerialTables are the tables whose `id` column is a BIGSERIAL in
// Postgres. After bulk-loading data with explicit ids, each of these
// sequences must be advanced past the maximum existing id so subsequent
// inserts from the app don't collide with imported rows.
var SerialTables = []string{
	"users", "hosts", "orchestrators", "dns_records", "projects",
	"project_responsaveis", "services", "service_credentials",
	"external_tools", "contacts", "ssh_keys", "host_scans",
	"host_responsaveis", "host_chamados", "dns_responsaveis",
	"service_responsaveis", "host_alerts", "issues", "releases",
	"host_operation_logs", "user_external_identities",
	"user_gitlab_tokens", "project_gitlab_links", "auth_role_mappings",
}

// ColumnRenames maps source → target column names for tables whose
// column naming differs across schema versions. Currently only records
// migration v48's hosts.user → hosts.ssh_user.
var ColumnRenames = map[string]map[string]string{
	"hosts": {"user": "ssh_user"},
}
