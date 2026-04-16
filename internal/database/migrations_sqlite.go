package database

// migrationsSQLite is the ordered list of SQLite SQL statements. Each index is
// the version number. Once applied, a migration is never re-run.
var migrationsSQLite = []string{
	// Version 0: schema_migrations tracking table + users + sessions
	`CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name  TEXT NOT NULL DEFAULT '',
		role          TEXT NOT NULL DEFAULT 'viewer',
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);`,

	// Version 1: configurable enum options
	`CREATE TABLE IF NOT EXISTS enum_options (
		category   TEXT NOT NULL,
		value      TEXT NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (category, value)
	);

	INSERT OR IGNORE INTO enum_options (category, value, sort_order) VALUES
		('hospedagem', 'ETIPI', 0),
		('hospedagem', 'ETIPI/AWS', 1),
		('hospedagem', 'AWS', 2),
		('hospedagem', 'Hostinger', 3),
		('hospedagem', 'DigitalOcean', 4),
		('hospedagem', 'Azure', 5),
		('hospedagem', 'GCP', 6),
		('hospedagem', 'On-Premise', 7),
		('situacao', 'active', 0),
		('situacao', 'inactive', 1),
		('situacao', 'maintenance', 2),
		('tipo_maquina', 'VPS', 0),
		('tipo_maquina', 'Dedicated', 1),
		('tipo_maquina', 'Container', 2),
		('tipo_maquina', 'VM', 3),
		('tipo_maquina', 'Bare Metal', 4),
		('orchestrator_type', 'Docker', 0),
		('orchestrator_type', 'Docker Swarm', 1),
		('orchestrator_type', 'Kubernetes', 2),
		('orchestrator_type', 'Portainer', 3);`,

	// Version 2: hosts table
	`CREATE TABLE IF NOT EXISTS hosts (
		id                            INTEGER PRIMARY KEY AUTOINCREMENT,
		nickname                      TEXT NOT NULL,
		oficial_slug                  TEXT NOT NULL UNIQUE,
		hostname                      TEXT NOT NULL DEFAULT '',
		hospedagem                    TEXT NOT NULL DEFAULT '',
		tipo_maquina                  TEXT NOT NULL DEFAULT '',
		user                          TEXT NOT NULL DEFAULT '',
		has_password                  INTEGER NOT NULL DEFAULT 0,
		password_ciphertext           BLOB,
		password_nonce                BLOB,
		has_key                       INTEGER NOT NULL DEFAULT 0,
		key_path                      TEXT NOT NULL DEFAULT '',
		port                          TEXT NOT NULL DEFAULT '22',
		identities_only               TEXT NOT NULL DEFAULT '',
		proxy_jump                    TEXT NOT NULL DEFAULT '',
		forward_agent                 TEXT NOT NULL DEFAULT '',
		description                   TEXT NOT NULL DEFAULT '',
		setor_responsavel             TEXT NOT NULL DEFAULT '',
		responsavel_interno           TEXT NOT NULL DEFAULT '',
		contato_responsavel_interno   TEXT NOT NULL DEFAULT '',
		acesso_empresa_externa        INTEGER NOT NULL DEFAULT 0,
		empresa_responsavel           TEXT NOT NULL DEFAULT '',
		responsavel_externo           TEXT NOT NULL DEFAULT '',
		contato_responsavel_externo   TEXT NOT NULL DEFAULT '',
		recurso_cpu                   TEXT NOT NULL DEFAULT '',
		recurso_ram                   TEXT NOT NULL DEFAULT '',
		recurso_armazenamento         TEXT NOT NULL DEFAULT '',
		situacao                      TEXT NOT NULL DEFAULT 'active',
		precisa_manutencao            INTEGER NOT NULL DEFAULT 0,
		observacoes                   TEXT NOT NULL DEFAULT '',
		created_at                    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at                    DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_hosts_situacao ON hosts(situacao);
	CREATE INDEX IF NOT EXISTS idx_hosts_hospedagem ON hosts(hospedagem);`,

	// Version 3: orchestrators
	`CREATE TABLE IF NOT EXISTS orchestrators (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id     INTEGER NOT NULL UNIQUE REFERENCES hosts(id) ON DELETE CASCADE,
		type        TEXT NOT NULL DEFAULT '',
		version     TEXT NOT NULL DEFAULT '',
		observacoes TEXT NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 4: DNS records + host links
	`CREATE TABLE IF NOT EXISTS dns_records (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		domain      TEXT NOT NULL UNIQUE,
		has_https   INTEGER NOT NULL DEFAULT 0,
		situacao    TEXT NOT NULL DEFAULT 'active',
		responsavel TEXT NOT NULL DEFAULT '',
		observacoes TEXT NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dns_host_links (
		dns_id  INTEGER NOT NULL REFERENCES dns_records(id) ON DELETE CASCADE,
		host_id INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		PRIMARY KEY (dns_id, host_id)
	);`,

	// Version 5: projects + responsaveis
	`CREATE TABLE IF NOT EXISTS projects (
		id                              INTEGER PRIMARY KEY AUTOINCREMENT,
		name                            TEXT NOT NULL,
		description                     TEXT NOT NULL DEFAULT '',
		situacao                        TEXT NOT NULL DEFAULT 'active',
		setor_responsavel               TEXT NOT NULL DEFAULT '',
		responsavel                     TEXT NOT NULL DEFAULT '',
		tem_empresa_externa_responsavel INTEGER NOT NULL DEFAULT 0,
		contato_empresa_responsavel     TEXT NOT NULL DEFAULT '',
		is_directly_managed             INTEGER NOT NULL DEFAULT 1,
		is_responsible                  INTEGER NOT NULL DEFAULT 1,
		gitlab_url                      TEXT NOT NULL DEFAULT '',
		documentation_url               TEXT NOT NULL DEFAULT '',
		created_at                      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at                      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS project_responsaveis (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		nome       TEXT NOT NULL,
		contato    TEXT NOT NULL DEFAULT '',
		UNIQUE(project_id, nome)
	);`,

	// Version 6: services + link tables
	`CREATE TABLE IF NOT EXISTS services (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		nickname             TEXT NOT NULL,
		project_id           INTEGER REFERENCES projects(id) ON DELETE SET NULL,
		description          TEXT NOT NULL DEFAULT '',
		technology_stack     TEXT NOT NULL DEFAULT '',
		orchestrator_managed INTEGER NOT NULL DEFAULT 0,
		is_directly_managed  INTEGER NOT NULL DEFAULT 1,
		is_responsible       INTEGER NOT NULL DEFAULT 1,
		developed_by         TEXT NOT NULL DEFAULT 'internal',
		gitlab_url           TEXT NOT NULL DEFAULT '',
		documentation_url    TEXT NOT NULL DEFAULT '',
		created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS service_host_links (
		service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		host_id    INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, host_id)
	);

	CREATE TABLE IF NOT EXISTS service_dns_links (
		service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		dns_id     INTEGER NOT NULL REFERENCES dns_records(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, dns_id)
	);`,

	// Version 7: service credentials
	`CREATE TABLE IF NOT EXISTS service_credentials (
		id                     INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id             INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		role_name              TEXT NOT NULL,
		credentials_ciphertext BLOB,
		credentials_nonce      BLOB,
		created_at             DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at             DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(service_id, role_name)
	);`,

	// Version 8: polymorphic tags
	`CREATE TABLE IF NOT EXISTS tags (
		entity_type TEXT NOT NULL,
		entity_id   INTEGER NOT NULL,
		tag         TEXT NOT NULL,
		PRIMARY KEY (entity_type, entity_id, tag)
	);
	CREATE INDEX IF NOT EXISTS idx_tags_entity ON tags(entity_type, entity_id);
	CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);`,

	// Version 9: application settings (key-value store for appearance config etc.)
	`CREATE TABLE IF NOT EXISTS app_settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	INSERT OR IGNORE INTO app_settings (key, value) VALUES
		('app_name', 'SSHCM'),
		('app_color', '#06b6d4'),
		('app_logo', '');`,

	// Version 10: external dependency services + service-to-service dependencies
	`ALTER TABLE services ADD COLUMN is_external_dependency INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE services ADD COLUMN external_provider TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN external_url TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN external_contact TEXT NOT NULL DEFAULT '';

	CREATE TABLE IF NOT EXISTS service_dependencies (
		service_id    INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		depends_on_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, depends_on_id)
	);`,

	// Version 11: issue board per project
	`CREATE TABLE IF NOT EXISTS issues (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id  INTEGER REFERENCES services(id) ON DELETE SET NULL,
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'backlog',
		priority    TEXT NOT NULL DEFAULT 'medium',
		assignee    TEXT NOT NULL DEFAULT '',
		created_by  INTEGER NOT NULL REFERENCES users(id),
		position    REAL NOT NULL DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_issues_project ON issues(project_id);
	CREATE INDEX IF NOT EXISTS idx_issues_service ON issues(service_id);
	CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(project_id, status, position);

	INSERT OR IGNORE INTO enum_options (category, value, sort_order) VALUES
		('issue_status', 'backlog', 0),
		('issue_status', 'todo', 1),
		('issue_status', 'in_progress', 2),
		('issue_status', 'review', 3),
		('issue_status', 'done', 4),
		('issue_priority', 'low', 0),
		('issue_priority', 'medium', 1),
		('issue_priority', 'high', 2),
		('issue_priority', 'critical', 3);`,

	// Version 12: releases (lançamentos/metas) + release-issue links
	`CREATE TABLE IF NOT EXISTS releases (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  INTEGER REFERENCES projects(id) ON DELETE SET NULL,
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'pending',
		target_date TEXT NOT NULL DEFAULT '',
		live_date   TEXT NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_releases_project ON releases(project_id);
	CREATE INDEX IF NOT EXISTS idx_releases_status ON releases(status);

	CREATE TABLE IF NOT EXISTS release_issues (
		release_id INTEGER NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
		issue_id   INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
		PRIMARY KEY (release_id, issue_id)
	);`,

	// Version 13: external tools
	`CREATE TABLE IF NOT EXISTS external_tools (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		url           TEXT NOT NULL DEFAULT '',
		icon          TEXT NOT NULL DEFAULT '',
		embed_enabled INTEGER NOT NULL DEFAULT 0,
		sort_order    INTEGER NOT NULL DEFAULT 0,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 14: encrypted SSH key content storage in DB (backup alongside filesystem keys)
	`ALTER TABLE hosts ADD COLUMN pub_key_ciphertext BLOB;
	ALTER TABLE hosts ADD COLUMN pub_key_nonce BLOB;
	ALTER TABLE hosts ADD COLUMN priv_key_ciphertext BLOB;
	ALTER TABLE hosts ADD COLUMN priv_key_nonce BLOB;`,

	// Version 15: entidade_responsavel enum category
	`INSERT OR IGNORE INTO enum_options (category, value, sort_order) VALUES
		('entidade_responsavel', 'TI', 0),
		('entidade_responsavel', 'Infraestrutura', 1),
		('entidade_responsavel', 'Desenvolvimento', 2),
		('entidade_responsavel', 'Segurança', 3);`,

	// Version 16: reusable contacts + managed SSH keys (was v15)
	`CREATE TABLE IF NOT EXISTS contacts (
		id    INTEGER PRIMARY KEY AUTOINCREMENT,
		name  TEXT NOT NULL,
		phone TEXT NOT NULL DEFAULT '',
		UNIQUE(name, phone)
	);

	CREATE TABLE IF NOT EXISTS ssh_keys (
		id                    INTEGER PRIMARY KEY AUTOINCREMENT,
		name                  TEXT NOT NULL UNIQUE,
		pub_key_ciphertext    BLOB,
		pub_key_nonce         BLOB,
		priv_key_ciphertext   BLOB,
		priv_key_nonce        BLOB,
		fingerprint           TEXT NOT NULL DEFAULT '',
		created_at            DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 17: host scan snapshots (VM info captured via SSH)
	`CREATE TABLE IF NOT EXISTS host_scans (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id   INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		data      TEXT NOT NULL DEFAULT '{}',
		scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_scans_host ON host_scans(host_id);`,

	// Version 18: rename setor_responsavel -> entidade_responsavel enum category + ensure values exist
	`UPDATE enum_options SET category = 'entidade_responsavel' WHERE category = 'setor_responsavel';
	INSERT OR IGNORE INTO enum_options (category, value, sort_order) VALUES
		('entidade_responsavel', 'TI', 0),
		('entidade_responsavel', 'Infraestrutura', 1),
		('entidade_responsavel', 'Desenvolvimento', 2),
		('entidade_responsavel', 'Segurança', 3);`,

	// Version 19: ensure host_scans table exists (fix for skipped migration)
	`CREATE TABLE IF NOT EXISTS host_scans (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id   INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		data      TEXT NOT NULL DEFAULT '{}',
		scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_scans_host ON host_scans(host_id);`,

	// Version 20: preferred auth method for SSH operations
	`ALTER TABLE hosts ADD COLUMN preferred_auth TEXT NOT NULL DEFAULT '';`,

	// Version 21: optional color for enum options (used by situacao status badges)
	`ALTER TABLE enum_options ADD COLUMN color TEXT NOT NULL DEFAULT '';

	UPDATE enum_options SET color = '#10b981' WHERE category = 'situacao' AND value = 'active';
	UPDATE enum_options SET color = '#6b7280' WHERE category = 'situacao' AND value = 'inactive';
	UPDATE enum_options SET color = '#f59e0b' WHERE category = 'situacao' AND value = 'maintenance';`,

	// Version 22: track failed connection attempts
	`ALTER TABLE hosts ADD COLUMN connections_failed INTEGER NOT NULL DEFAULT 0;`,

	// Version 23: track individual test results for password and key methods
	`ALTER TABLE hosts ADD COLUMN password_test_status TEXT DEFAULT NULL;
	ALTER TABLE hosts ADD COLUMN key_test_status TEXT DEFAULT NULL;`,

	// Version 24: alert threshold settings
	`INSERT OR IGNORE INTO app_settings (key, value) VALUES
		('alert_resource_critical', '80'),
		('alert_resource_warning', '60'),
		('alert_resource_info_low', '5');`,

	// Version 25: enhanced service model
	`ALTER TABLE services ADD COLUMN service_type TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN service_subtype TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN deploy_approach TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN orchestrator_tool TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN environment TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN port TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN version TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN repository_url TEXT NOT NULL DEFAULT '';

	INSERT OR IGNORE INTO enum_options (category, value, sort_order) VALUES
		('service_type', 'database', 0),
		('service_type', 'infrastructure', 1),
		('service_type', 'api', 2),
		('service_type', 'frontend', 3),
		('service_type', 'fullstack', 4),
		('service_type', 'application', 5),
		('service_type', 'worker', 6),
		('service_type', 'monitoring', 7),

		('service_subtype', 'PostgreSQL', 0),
		('service_subtype', 'MySQL', 1),
		('service_subtype', 'MongoDB', 2),
		('service_subtype', 'Redis', 3),
		('service_subtype', 'SQLite', 4),
		('service_subtype', 'Elasticsearch', 5),
		('service_subtype', 'Nginx', 10),
		('service_subtype', 'Traefik', 11),
		('service_subtype', 'MinIO', 12),
		('service_subtype', 'Airflow', 13),
		('service_subtype', 'Trino', 14),
		('service_subtype', 'Apache', 15),
		('service_subtype', 'Keycloak', 16),
		('service_subtype', 'n8n', 17),
		('service_subtype', 'Metabase', 18),
		('service_subtype', 'Grafana', 19),
		('service_subtype', 'Prefect', 20),
		('service_subtype', 'Kong', 21),
		('service_subtype', 'Portainer', 22),
		('service_subtype', 'Coolify', 23),

		('technology_stack', 'Node.js', 0),
		('technology_stack', 'Python', 1),
		('technology_stack', 'Go', 2),
		('technology_stack', 'Java', 3),
		('technology_stack', 'PHP', 4),
		('technology_stack', 'Ruby', 5),
		('technology_stack', 'Rust', 6),
		('technology_stack', '.NET', 7),
		('technology_stack', 'React', 10),
		('technology_stack', 'Next.js', 11),
		('technology_stack', 'Vue', 12),
		('technology_stack', 'Angular', 13),
		('technology_stack', 'WordPress', 14),

		('deploy_approach', 'dockerfile', 0),
		('deploy_approach', 'docker-compose', 1),
		('deploy_approach', 'standalone', 2),
		('deploy_approach', 'helm', 3),
		('deploy_approach', 'binary', 4),
		('deploy_approach', 'script', 5),
		('deploy_approach', 'package-manager', 6),

		('orchestrator_tool', 'none', 0),
		('orchestrator_tool', 'coolify', 1),
		('orchestrator_tool', 'portainer', 2),
		('orchestrator_tool', 'gitlab-pipeline', 3),
		('orchestrator_tool', 'github-actions', 4),
		('orchestrator_tool', 'manual', 5),

		('environment', 'production', 0),
		('environment', 'staging', 1),
		('environment', 'development', 2),
		('environment', 'shared', 3);

	UPDATE services SET repository_url = gitlab_url WHERE gitlab_url != '';`,

	// Version 26: host credentials — ssh_keys table now supports passwords too
	`ALTER TABLE ssh_keys ADD COLUMN credential_type TEXT NOT NULL DEFAULT 'key';
	ALTER TABLE ssh_keys ADD COLUMN password_ciphertext BLOB;
	ALTER TABLE ssh_keys ADD COLUMN password_nonce BLOB;
	ALTER TABLE ssh_keys ADD COLUMN username TEXT NOT NULL DEFAULT '';
	ALTER TABLE ssh_keys ADD COLUMN description TEXT NOT NULL DEFAULT '';`,

	// Version 27: polymorphic issues + multi-assignee
	`ALTER TABLE issues ADD COLUMN entity_type TEXT NOT NULL DEFAULT 'project';
	ALTER TABLE issues ADD COLUMN entity_id INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE issues ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';
	ALTER TABLE issues ADD COLUMN source_ref TEXT NOT NULL DEFAULT '';

	UPDATE issues SET entity_type = 'project', entity_id = project_id;

	CREATE TABLE IF NOT EXISTS issue_assignees (
		issue_id  INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
		user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		PRIMARY KEY (issue_id, user_id)
	);
	CREATE INDEX IF NOT EXISTS idx_issues_entity ON issues(entity_type, entity_id);
	CREATE INDEX IF NOT EXISTS idx_issue_assignees_user ON issue_assignees(user_id);`,

	// Version 28: auth provider infrastructure + user external identities
	`CREATE TABLE IF NOT EXISTS user_external_identities (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		provider_name TEXT NOT NULL,
		external_id   TEXT NOT NULL,
		external_data TEXT NOT NULL DEFAULT '{}',
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(provider_name, external_id)
	);
	CREATE INDEX IF NOT EXISTS idx_uei_user ON user_external_identities(user_id);

	CREATE TABLE IF NOT EXISTS oauth_states (
		state      TEXT PRIMARY KEY,
		provider   TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL
	);

	ALTER TABLE users ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'local';
	ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';

	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_auto_provision', 'true');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_default_role', 'viewer');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_role_sync_enabled', 'false');`,

	// Version 29: granular permissions system
	`CREATE TABLE IF NOT EXISTS permissions (
		code        TEXT PRIMARY KEY,
		description TEXT NOT NULL DEFAULT '',
		category    TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS role_permissions (
		role       TEXT NOT NULL,
		permission TEXT NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
		PRIMARY KEY (role, permission)
	);

	CREATE TABLE IF NOT EXISTS auth_role_mappings (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_name  TEXT NOT NULL,
		external_group TEXT NOT NULL,
		local_role     TEXT NOT NULL,
		UNIQUE(provider_name, external_group)
	);

	INSERT OR IGNORE INTO permissions (code, description, category) VALUES
		('hosts.view', 'View hosts', 'hosts'),
		('hosts.edit', 'Create and edit hosts', 'hosts'),
		('hosts.delete', 'Delete hosts', 'hosts'),
		('hosts.passwords', 'View host passwords', 'hosts'),
		('dns.view', 'View DNS records', 'dns'),
		('dns.edit', 'Create and edit DNS records', 'dns'),
		('dns.delete', 'Delete DNS records', 'dns'),
		('projects.view', 'View projects', 'projects'),
		('projects.edit', 'Create and edit projects', 'projects'),
		('projects.delete', 'Delete projects', 'projects'),
		('services.view', 'View services', 'services'),
		('services.edit', 'Create and edit services', 'services'),
		('services.delete', 'Delete services', 'services'),
		('issues.view', 'View issues', 'issues'),
		('issues.edit', 'Create and edit issues', 'issues'),
		('issues.delete', 'Delete issues', 'issues'),
		('settings.view', 'View settings', 'settings'),
		('settings.edit', 'Edit settings', 'settings'),
		('users.manage', 'Manage users', 'users'),
		('ssh.operate', 'SSH operations', 'ssh'),
		('backup.manage', 'Backup and restore', 'backup'),
		('integrations.manage', 'Manage integrations', 'integrations'),
		('ai.use', 'Use AI features', 'ai');

	INSERT OR IGNORE INTO role_permissions (role, permission) VALUES
		('viewer', 'hosts.view'), ('viewer', 'dns.view'), ('viewer', 'projects.view'),
		('viewer', 'services.view'), ('viewer', 'issues.view'), ('viewer', 'settings.view');

	INSERT OR IGNORE INTO role_permissions (role, permission) VALUES
		('editor', 'hosts.view'), ('editor', 'hosts.edit'),
		('editor', 'dns.view'), ('editor', 'dns.edit'),
		('editor', 'projects.view'), ('editor', 'projects.edit'),
		('editor', 'services.view'), ('editor', 'services.edit'),
		('editor', 'issues.view'), ('editor', 'issues.edit'),
		('editor', 'settings.view'),
		('editor', 'ssh.operate'),
		('editor', 'ai.use');

	INSERT OR IGNORE INTO role_permissions (role, permission) VALUES
		('admin', 'hosts.view'), ('admin', 'hosts.edit'), ('admin', 'hosts.delete'), ('admin', 'hosts.passwords'),
		('admin', 'dns.view'), ('admin', 'dns.edit'), ('admin', 'dns.delete'),
		('admin', 'projects.view'), ('admin', 'projects.edit'), ('admin', 'projects.delete'),
		('admin', 'services.view'), ('admin', 'services.edit'), ('admin', 'services.delete'),
		('admin', 'issues.view'), ('admin', 'issues.edit'), ('admin', 'issues.delete'),
		('admin', 'settings.view'), ('admin', 'settings.edit'),
		('admin', 'users.manage'), ('admin', 'ssh.operate'),
		('admin', 'backup.manage'), ('admin', 'integrations.manage'), ('admin', 'ai.use');`,

	// Version 30: LDAP integration settings
	`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_enabled', 'false');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_host', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_port', '636');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_use_tls', 'true');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_skip_verify', 'false');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_base_dn', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_bind_dn', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_bind_password_cipher', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_bind_password_nonce', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_user_filter', '(mail=%s)');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_username_attr', 'uid');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_display_name_attr', 'cn');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_email_attr', 'mail');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_ldap_fallback_to_local', 'true');`,

	// Version 31: Keycloak SSO integration settings
	`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_keycloak_enabled', 'false');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_keycloak_base_url', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_keycloak_realm', 'pi');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_keycloak_client_id', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_keycloak_client_secret_cipher', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_keycloak_client_secret_nonce', '');`,

	// Version 32: GitLab integration (auth + code management)
	`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_gitlab_enabled', 'false');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_gitlab_base_url', 'https://gitlab.com');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_gitlab_client_id', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_gitlab_client_secret_cipher', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_gitlab_client_secret_nonce', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('gitlab_integration_enabled', 'false');

	CREATE TABLE IF NOT EXISTS user_gitlab_tokens (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id              INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		gitlab_base_url      TEXT NOT NULL DEFAULT 'https://gitlab.com',
		access_token_cipher  BLOB,
		access_token_nonce   BLOB,
		refresh_token_cipher BLOB,
		refresh_token_nonce  BLOB,
		gitlab_user_id       TEXT NOT NULL DEFAULT '',
		gitlab_username      TEXT NOT NULL DEFAULT '',
		expires_at           DATETIME,
		created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, gitlab_base_url)
	);

	CREATE TABLE IF NOT EXISTS project_gitlab_links (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id        INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		gitlab_project_id INTEGER NOT NULL,
		gitlab_base_url   TEXT NOT NULL DEFAULT 'https://gitlab.com',
		gitlab_path       TEXT NOT NULL DEFAULT '',
		sync_issues       INTEGER NOT NULL DEFAULT 0,
		last_synced_at    DATETIME,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, gitlab_project_id, gitlab_base_url)
	);`,

	// Version 33: LLM integration settings
	`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_enabled', 'false');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_base_url', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_api_key_cipher', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_api_key_nonce', '');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_model_text', 'Qwen/Qwen3-30B-A3B');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_model_vision', 'Qwen/Qwen3-VL-30B-A3B-Thinking');
	INSERT OR IGNORE INTO app_settings (key, value) VALUES ('llm_max_tokens', '2000');`,

	// Version 34: single active auth provider enforcement
	`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('auth_active_provider', 'local');`,

	// Version 35: host responsaveis + chamados + contact role/entity
	`ALTER TABLE contacts ADD COLUMN role TEXT NOT NULL DEFAULT '';
	ALTER TABLE contacts ADD COLUMN entity TEXT NOT NULL DEFAULT '';

	CREATE TABLE IF NOT EXISTS host_responsaveis (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		contact_id INTEGER NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		is_main INTEGER NOT NULL DEFAULT 0,
		is_externo INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_responsaveis_host ON host_responsaveis(host_id);

	CREATE TABLE IF NOT EXISTS host_chamados (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		chamado_id TEXT NOT NULL DEFAULT '',
		user_id INTEGER NOT NULL DEFAULT 0,
		date TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_chamados_host ON host_chamados(host_id);

	-- Migrate existing responsavel_interno data to contacts + host_responsaveis
	INSERT OR IGNORE INTO contacts (name, phone) SELECT responsavel_interno, contato_responsavel_interno FROM hosts WHERE responsavel_interno != '';
	INSERT INTO host_responsaveis (host_id, contact_id, is_main, is_externo)
		SELECT h.id, c.id, 1, 0
		FROM hosts h
		JOIN contacts c ON c.name = h.responsavel_interno AND c.phone = h.contato_responsavel_interno
		WHERE h.responsavel_interno != '';

	-- Migrate existing responsavel_externo data to contacts + host_responsaveis
	INSERT OR IGNORE INTO contacts (name, phone) SELECT responsavel_externo, contato_responsavel_externo FROM hosts WHERE responsavel_externo != '';
	INSERT INTO host_responsaveis (host_id, contact_id, is_main, is_externo)
		SELECT h.id, c.id, 1, 1
		FROM hosts h
		JOIN contacts c ON c.name = h.responsavel_externo AND c.phone = h.contato_responsavel_externo
		WHERE h.responsavel_externo != '';`,

	// Version 36: manual host alerts + issue date fields + alert-issue link
	`CREATE TABLE IF NOT EXISTS host_alerts (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id     INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		type        TEXT NOT NULL,
		level       TEXT NOT NULL DEFAULT 'info',
		message     TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		source      TEXT NOT NULL DEFAULT 'manual',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_alerts_host ON host_alerts(host_id);

	ALTER TABLE issues ADD COLUMN expected_end_date TEXT NOT NULL DEFAULT '';
	ALTER TABLE issues ADD COLUMN start_date TEXT NOT NULL DEFAULT '';
	ALTER TABLE issues ADD COLUMN end_date TEXT NOT NULL DEFAULT '';
	ALTER TABLE issues ADD COLUMN alert_id INTEGER REFERENCES host_alerts(id) ON DELETE SET NULL;`,

	// Version 37: issue-alert many-to-many link table
	`CREATE TABLE IF NOT EXISTS issue_alert_links (
		issue_id  INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
		alert_id  INTEGER NOT NULL REFERENCES host_alerts(id) ON DELETE CASCADE,
		PRIMARY KEY (issue_id, alert_id)
	);
	-- Migrate existing single alert_id to the join table
	INSERT OR IGNORE INTO issue_alert_links (issue_id, alert_id)
		SELECT id, alert_id FROM issues WHERE alert_id IS NOT NULL;`,

	// Version 38: make project_id nullable for non-project entity issues
	`CREATE TABLE issues_new (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id       INTEGER REFERENCES projects(id) ON DELETE SET NULL,
		service_id       INTEGER REFERENCES services(id) ON DELETE SET NULL,
		entity_type      TEXT NOT NULL DEFAULT 'project',
		entity_id        INTEGER NOT NULL DEFAULT 0,
		title            TEXT NOT NULL,
		description      TEXT NOT NULL DEFAULT '',
		status           TEXT NOT NULL DEFAULT 'backlog',
		priority         TEXT NOT NULL DEFAULT 'medium',
		assignee         TEXT NOT NULL DEFAULT '',
		source           TEXT NOT NULL DEFAULT 'manual',
		source_ref       TEXT NOT NULL DEFAULT '',
		created_by       INTEGER NOT NULL REFERENCES users(id),
		position         REAL NOT NULL DEFAULT 0,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		expected_end_date TEXT NOT NULL DEFAULT '',
		start_date       TEXT NOT NULL DEFAULT '',
		end_date         TEXT NOT NULL DEFAULT '',
		alert_id         INTEGER REFERENCES host_alerts(id) ON DELETE SET NULL
	);
	INSERT INTO issues_new SELECT * FROM issues;
	DROP TABLE issues;
	ALTER TABLE issues_new RENAME TO issues;
	CREATE INDEX IF NOT EXISTS idx_issues_project ON issues(project_id);
	CREATE INDEX IF NOT EXISTS idx_issues_service ON issues(service_id);
	CREATE INDEX IF NOT EXISTS idx_issues_entity ON issues(entity_type, entity_id);`,

	// Version 39: alert status (active / resolved)
	`ALTER TABLE host_alerts ADD COLUMN status TEXT NOT NULL DEFAULT 'active';`,

	// Version 40: chamado title and status
	`ALTER TABLE host_chamados ADD COLUMN title TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_chamados ADD COLUMN status TEXT NOT NULL DEFAULT 'in_execution';`,

	// Version 41: issue archive support
	`ALTER TABLE issues ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;`,

	// Version 42: responsaveis for DNS and Services (same pattern as host_responsaveis)
	`CREATE TABLE IF NOT EXISTS dns_responsaveis (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		dns_id     INTEGER NOT NULL REFERENCES dns_records(id) ON DELETE CASCADE,
		contact_id INTEGER NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		is_main    INTEGER NOT NULL DEFAULT 0,
		is_externo INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_dns_responsaveis_dns ON dns_responsaveis(dns_id);

	CREATE TABLE IF NOT EXISTS service_responsaveis (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		contact_id INTEGER NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		is_main    INTEGER NOT NULL DEFAULT 0,
		is_externo INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_service_responsaveis_service ON service_responsaveis(service_id);`,

	// Version 43: migrate project_responsaveis to contact-based pattern
	`ALTER TABLE project_responsaveis ADD COLUMN contact_id INTEGER NOT NULL DEFAULT 0 REFERENCES contacts(id) ON DELETE CASCADE;
	ALTER TABLE project_responsaveis ADD COLUMN is_main INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE project_responsaveis ADD COLUMN is_externo INTEGER NOT NULL DEFAULT 0;`,

	// Version 44: direct host-project links
	`CREATE TABLE IF NOT EXISTS project_host_links (
		project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		host_id    INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		PRIMARY KEY (project_id, host_id)
	);
	CREATE INDEX IF NOT EXISTS idx_project_host_links_host ON project_host_links(host_id);`,

	// Version 45: operation logs for SSH operations
	`CREATE TABLE IF NOT EXISTS host_operation_logs (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id        INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		operation_type TEXT NOT NULL,
		auth_method    TEXT,
		status         TEXT NOT NULL,
		output         TEXT NOT NULL DEFAULT '',
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_op_logs_host ON host_operation_logs(host_id);`,

	// Version 46: docker group status on hosts
	`ALTER TABLE hosts ADD COLUMN docker_group_status TEXT;`,

	// Version 47: Coolify integration — store linked server UUID per host
	`ALTER TABLE hosts ADD COLUMN coolify_server_uuid TEXT;`,

	// Version 48: rename hosts.user to hosts.ssh_user. `user` is a reserved
	// keyword in PostgreSQL and cannot be used unquoted, so we rename it
	// in both dialects to keep model SQL portable. SQLite 3.25+ supports
	// ALTER TABLE RENAME COLUMN; the embedded modernc/sqlite build ships
	// with a much newer release so this is safe.
	`ALTER TABLE hosts RENAME COLUMN user TO ssh_user;`,

	// Version 49: link external_tools to services and DNS for sync.
	`ALTER TABLE external_tools ADD COLUMN service_id INTEGER REFERENCES services(id) ON DELETE SET NULL;
	ALTER TABLE external_tools ADD COLUMN dns_id INTEGER REFERENCES dns_records(id) ON DELETE SET NULL;
	ALTER TABLE external_tools ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';`,
}
