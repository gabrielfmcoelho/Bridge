package database

// migrationsPostgres is the ordered list of PostgreSQL SQL statements. Each
// index is the version number and must stay aligned 1:1 with migrationsSQLite
// so the same schema_migrations table tracks progress across both backends.
//
// Translation rules applied:
//   - INTEGER PRIMARY KEY AUTOINCREMENT -> BIGSERIAL PRIMARY KEY
//   - BLOB                              -> BYTEA
//   - DATETIME                          -> TIMESTAMPTZ
//   - REAL                              -> DOUBLE PRECISION
//   - INSERT OR IGNORE                  -> INSERT ... ON CONFLICT DO NOTHING
//   - Boolean 0/1 INTEGER columns that Go reads into `bool` fields are
//     expressed as BOOLEAN so the driver coerces them natively. Go code
//     passes Go bools through `?` placeholders unchanged.
//   - PRAGMAs and other SQLite-only directives are dropped.
//   - Version 38's rebuild-via-temp-table trick is replaced with a
//     straight ALTER TABLE since Postgres supports nullable column changes
//     in place. The resulting schema is equivalent.
var migrationsPostgres = []string{
	// Version 0
	`CREATE TABLE IF NOT EXISTS users (
		id            BIGSERIAL PRIMARY KEY,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name  TEXT NOT NULL DEFAULT '',
		role          TEXT NOT NULL DEFAULT 'viewer',
		created_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);`,

	// Version 1
	`CREATE TABLE IF NOT EXISTS enum_options (
		category   TEXT NOT NULL,
		value      TEXT NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (category, value)
	);
	INSERT INTO enum_options (category, value, sort_order) VALUES
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
		('orchestrator_type', 'Portainer', 3)
	ON CONFLICT DO NOTHING;`,

	// Version 2: hosts
	`CREATE TABLE IF NOT EXISTS hosts (
		id                            BIGSERIAL PRIMARY KEY,
		nickname                      TEXT NOT NULL,
		oficial_slug                  TEXT NOT NULL UNIQUE,
		hostname                      TEXT NOT NULL DEFAULT '',
		hospedagem                    TEXT NOT NULL DEFAULT '',
		tipo_maquina                  TEXT NOT NULL DEFAULT '',
		"user"                        TEXT NOT NULL DEFAULT '',
		has_password                  BOOLEAN NOT NULL DEFAULT FALSE,
		password_ciphertext           BYTEA,
		password_nonce                BYTEA,
		has_key                       BOOLEAN NOT NULL DEFAULT FALSE,
		key_path                      TEXT NOT NULL DEFAULT '',
		port                          TEXT NOT NULL DEFAULT '22',
		identities_only               TEXT NOT NULL DEFAULT '',
		proxy_jump                    TEXT NOT NULL DEFAULT '',
		forward_agent                 TEXT NOT NULL DEFAULT '',
		description                   TEXT NOT NULL DEFAULT '',
		setor_responsavel             TEXT NOT NULL DEFAULT '',
		responsavel_interno           TEXT NOT NULL DEFAULT '',
		contato_responsavel_interno   TEXT NOT NULL DEFAULT '',
		acesso_empresa_externa        BOOLEAN NOT NULL DEFAULT FALSE,
		empresa_responsavel           TEXT NOT NULL DEFAULT '',
		responsavel_externo           TEXT NOT NULL DEFAULT '',
		contato_responsavel_externo   TEXT NOT NULL DEFAULT '',
		recurso_cpu                   TEXT NOT NULL DEFAULT '',
		recurso_ram                   TEXT NOT NULL DEFAULT '',
		recurso_armazenamento         TEXT NOT NULL DEFAULT '',
		situacao                      TEXT NOT NULL DEFAULT 'active',
		precisa_manutencao            BOOLEAN NOT NULL DEFAULT FALSE,
		observacoes                   TEXT NOT NULL DEFAULT '',
		created_at                    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at                    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_hosts_situacao ON hosts(situacao);
	CREATE INDEX IF NOT EXISTS idx_hosts_hospedagem ON hosts(hospedagem);`,

	// Version 3: orchestrators
	`CREATE TABLE IF NOT EXISTS orchestrators (
		id          BIGSERIAL PRIMARY KEY,
		host_id     BIGINT NOT NULL UNIQUE REFERENCES hosts(id) ON DELETE CASCADE,
		type        TEXT NOT NULL DEFAULT '',
		version     TEXT NOT NULL DEFAULT '',
		observacoes TEXT NOT NULL DEFAULT '',
		created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 4: DNS
	`CREATE TABLE IF NOT EXISTS dns_records (
		id          BIGSERIAL PRIMARY KEY,
		domain      TEXT NOT NULL UNIQUE,
		has_https   BOOLEAN NOT NULL DEFAULT FALSE,
		situacao    TEXT NOT NULL DEFAULT 'active',
		responsavel TEXT NOT NULL DEFAULT '',
		observacoes TEXT NOT NULL DEFAULT '',
		created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS dns_host_links (
		dns_id  BIGINT NOT NULL REFERENCES dns_records(id) ON DELETE CASCADE,
		host_id BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		PRIMARY KEY (dns_id, host_id)
	);`,

	// Version 5: projects
	`CREATE TABLE IF NOT EXISTS projects (
		id                              BIGSERIAL PRIMARY KEY,
		name                            TEXT NOT NULL,
		description                     TEXT NOT NULL DEFAULT '',
		situacao                        TEXT NOT NULL DEFAULT 'active',
		setor_responsavel               TEXT NOT NULL DEFAULT '',
		responsavel                     TEXT NOT NULL DEFAULT '',
		tem_empresa_externa_responsavel BOOLEAN NOT NULL DEFAULT FALSE,
		contato_empresa_responsavel     TEXT NOT NULL DEFAULT '',
		is_directly_managed             BOOLEAN NOT NULL DEFAULT TRUE,
		is_responsible                  BOOLEAN NOT NULL DEFAULT TRUE,
		gitlab_url                      TEXT NOT NULL DEFAULT '',
		documentation_url               TEXT NOT NULL DEFAULT '',
		created_at                      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at                      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS project_responsaveis (
		id         BIGSERIAL PRIMARY KEY,
		project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		nome       TEXT NOT NULL,
		contato    TEXT NOT NULL DEFAULT '',
		UNIQUE(project_id, nome)
	);`,

	// Version 6: services
	`CREATE TABLE IF NOT EXISTS services (
		id                   BIGSERIAL PRIMARY KEY,
		nickname             TEXT NOT NULL,
		project_id           BIGINT REFERENCES projects(id) ON DELETE SET NULL,
		description          TEXT NOT NULL DEFAULT '',
		technology_stack     TEXT NOT NULL DEFAULT '',
		orchestrator_managed BOOLEAN NOT NULL DEFAULT FALSE,
		is_directly_managed  BOOLEAN NOT NULL DEFAULT TRUE,
		is_responsible       BOOLEAN NOT NULL DEFAULT TRUE,
		developed_by         TEXT NOT NULL DEFAULT 'internal',
		gitlab_url           TEXT NOT NULL DEFAULT '',
		documentation_url    TEXT NOT NULL DEFAULT '',
		created_at           TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at           TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS service_host_links (
		service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		host_id    BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, host_id)
	);
	CREATE TABLE IF NOT EXISTS service_dns_links (
		service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		dns_id     BIGINT NOT NULL REFERENCES dns_records(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, dns_id)
	);`,

	// Version 7: service credentials
	`CREATE TABLE IF NOT EXISTS service_credentials (
		id                     BIGSERIAL PRIMARY KEY,
		service_id             BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		role_name              TEXT NOT NULL,
		credentials_ciphertext BYTEA,
		credentials_nonce      BYTEA,
		created_at             TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at             TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(service_id, role_name)
	);`,

	// Version 8: tags
	`CREATE TABLE IF NOT EXISTS tags (
		entity_type TEXT NOT NULL,
		entity_id   BIGINT NOT NULL,
		tag         TEXT NOT NULL,
		PRIMARY KEY (entity_type, entity_id, tag)
	);
	CREATE INDEX IF NOT EXISTS idx_tags_entity ON tags(entity_type, entity_id);
	CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);`,

	// Version 9: app settings
	`CREATE TABLE IF NOT EXISTS app_settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO app_settings (key, value) VALUES
		('app_name', 'SSHCM'),
		('app_color', '#06b6d4'),
		('app_logo', '')
	ON CONFLICT DO NOTHING;`,

	// Version 10: external dependency services
	`ALTER TABLE services ADD COLUMN IF NOT EXISTS is_external_dependency BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE services ADD COLUMN IF NOT EXISTS external_provider TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS external_url TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS external_contact TEXT NOT NULL DEFAULT '';
	CREATE TABLE IF NOT EXISTS service_dependencies (
		service_id    BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		depends_on_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, depends_on_id)
	);`,

	// Version 11: issues
	`CREATE TABLE IF NOT EXISTS issues (
		id          BIGSERIAL PRIMARY KEY,
		project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id  BIGINT REFERENCES services(id) ON DELETE SET NULL,
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'backlog',
		priority    TEXT NOT NULL DEFAULT 'medium',
		assignee    TEXT NOT NULL DEFAULT '',
		created_by  BIGINT NOT NULL REFERENCES users(id),
		position    DOUBLE PRECISION NOT NULL DEFAULT 0,
		created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_issues_project ON issues(project_id);
	CREATE INDEX IF NOT EXISTS idx_issues_service ON issues(service_id);
	CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(project_id, status, position);
	INSERT INTO enum_options (category, value, sort_order) VALUES
		('issue_status', 'backlog', 0),
		('issue_status', 'todo', 1),
		('issue_status', 'in_progress', 2),
		('issue_status', 'review', 3),
		('issue_status', 'done', 4),
		('issue_priority', 'low', 0),
		('issue_priority', 'medium', 1),
		('issue_priority', 'high', 2),
		('issue_priority', 'critical', 3)
	ON CONFLICT DO NOTHING;`,

	// Version 12: releases
	`CREATE TABLE IF NOT EXISTS releases (
		id          BIGSERIAL PRIMARY KEY,
		project_id  BIGINT REFERENCES projects(id) ON DELETE SET NULL,
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'pending',
		target_date TEXT NOT NULL DEFAULT '',
		live_date   TEXT NOT NULL DEFAULT '',
		created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_releases_project ON releases(project_id);
	CREATE INDEX IF NOT EXISTS idx_releases_status ON releases(status);
	CREATE TABLE IF NOT EXISTS release_issues (
		release_id BIGINT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
		issue_id   BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
		PRIMARY KEY (release_id, issue_id)
	);`,

	// Version 13: external tools
	`CREATE TABLE IF NOT EXISTS external_tools (
		id            BIGSERIAL PRIMARY KEY,
		name          TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		url           TEXT NOT NULL DEFAULT '',
		icon          TEXT NOT NULL DEFAULT '',
		embed_enabled BOOLEAN NOT NULL DEFAULT FALSE,
		sort_order    INTEGER NOT NULL DEFAULT 0,
		created_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 14: host SSH key BYTEA columns
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS pub_key_ciphertext BYTEA;
	ALTER TABLE hosts ADD COLUMN IF NOT EXISTS pub_key_nonce BYTEA;
	ALTER TABLE hosts ADD COLUMN IF NOT EXISTS priv_key_ciphertext BYTEA;
	ALTER TABLE hosts ADD COLUMN IF NOT EXISTS priv_key_nonce BYTEA;`,

	// Version 15: entidade_responsavel enum
	`INSERT INTO enum_options (category, value, sort_order) VALUES
		('entidade_responsavel', 'TI', 0),
		('entidade_responsavel', 'Infraestrutura', 1),
		('entidade_responsavel', 'Desenvolvimento', 2),
		('entidade_responsavel', 'Segurança', 3)
	ON CONFLICT DO NOTHING;`,

	// Version 16: contacts + ssh_keys
	`CREATE TABLE IF NOT EXISTS contacts (
		id    BIGSERIAL PRIMARY KEY,
		name  TEXT NOT NULL,
		phone TEXT NOT NULL DEFAULT '',
		UNIQUE(name, phone)
	);
	CREATE TABLE IF NOT EXISTS ssh_keys (
		id                    BIGSERIAL PRIMARY KEY,
		name                  TEXT NOT NULL UNIQUE,
		pub_key_ciphertext    BYTEA,
		pub_key_nonce         BYTEA,
		priv_key_ciphertext   BYTEA,
		priv_key_nonce        BYTEA,
		fingerprint           TEXT NOT NULL DEFAULT '',
		created_at            TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 17: host_scans
	`CREATE TABLE IF NOT EXISTS host_scans (
		id         BIGSERIAL PRIMARY KEY,
		host_id    BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		data       TEXT NOT NULL DEFAULT '{}',
		scanned_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_scans_host ON host_scans(host_id);`,

	// Version 18: rename enum category
	`UPDATE enum_options SET category = 'entidade_responsavel' WHERE category = 'setor_responsavel';
	INSERT INTO enum_options (category, value, sort_order) VALUES
		('entidade_responsavel', 'TI', 0),
		('entidade_responsavel', 'Infraestrutura', 1),
		('entidade_responsavel', 'Desenvolvimento', 2),
		('entidade_responsavel', 'Segurança', 3)
	ON CONFLICT DO NOTHING;`,

	// Version 19: idempotent host_scans
	`CREATE TABLE IF NOT EXISTS host_scans (
		id         BIGSERIAL PRIMARY KEY,
		host_id    BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		data       TEXT NOT NULL DEFAULT '{}',
		scanned_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_scans_host ON host_scans(host_id);`,

	// Version 20
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS preferred_auth TEXT NOT NULL DEFAULT '';`,

	// Version 21
	`ALTER TABLE enum_options ADD COLUMN IF NOT EXISTS color TEXT NOT NULL DEFAULT '';
	UPDATE enum_options SET color = '#10b981' WHERE category = 'situacao' AND value = 'active';
	UPDATE enum_options SET color = '#6b7280' WHERE category = 'situacao' AND value = 'inactive';
	UPDATE enum_options SET color = '#f59e0b' WHERE category = 'situacao' AND value = 'maintenance';`,

	// Version 22
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS connections_failed INTEGER NOT NULL DEFAULT 0;`,

	// Version 23
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS password_test_status TEXT DEFAULT NULL;
	ALTER TABLE hosts ADD COLUMN IF NOT EXISTS key_test_status TEXT DEFAULT NULL;`,

	// Version 24
	`INSERT INTO app_settings (key, value) VALUES
		('alert_resource_critical', '80'),
		('alert_resource_warning', '60'),
		('alert_resource_info_low', '5')
	ON CONFLICT DO NOTHING;`,

	// Version 25: enhanced service model
	`ALTER TABLE services ADD COLUMN IF NOT EXISTS service_type TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS service_subtype TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS deploy_approach TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS orchestrator_tool TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS port TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS repository_url TEXT NOT NULL DEFAULT '';
	INSERT INTO enum_options (category, value, sort_order) VALUES
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
		('environment', 'shared', 3)
	ON CONFLICT DO NOTHING;
	UPDATE services SET repository_url = gitlab_url WHERE gitlab_url != '';`,

	// Version 26: ssh_keys for passwords
	`ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS credential_type TEXT NOT NULL DEFAULT 'key';
	ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS password_ciphertext BYTEA;
	ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS password_nonce BYTEA;
	ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS username TEXT NOT NULL DEFAULT '';
	ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';`,

	// Version 27: polymorphic issues
	`ALTER TABLE issues ADD COLUMN IF NOT EXISTS entity_type TEXT NOT NULL DEFAULT 'project';
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS entity_id BIGINT NOT NULL DEFAULT 0;
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'manual';
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS source_ref TEXT NOT NULL DEFAULT '';
	UPDATE issues SET entity_type = 'project', entity_id = project_id;
	CREATE TABLE IF NOT EXISTS issue_assignees (
		issue_id BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
		user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		PRIMARY KEY (issue_id, user_id)
	);
	CREATE INDEX IF NOT EXISTS idx_issues_entity ON issues(entity_type, entity_id);
	CREATE INDEX IF NOT EXISTS idx_issue_assignees_user ON issue_assignees(user_id);`,

	// Version 28: auth provider infrastructure
	`CREATE TABLE IF NOT EXISTS user_external_identities (
		id            BIGSERIAL PRIMARY KEY,
		user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		provider_name TEXT NOT NULL,
		external_id   TEXT NOT NULL,
		external_data TEXT NOT NULL DEFAULT '{}',
		created_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(provider_name, external_id)
	);
	CREATE INDEX IF NOT EXISTS idx_uei_user ON user_external_identities(user_id);
	CREATE TABLE IF NOT EXISTS oauth_states (
		state      TEXT PRIMARY KEY,
		provider   TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMPTZ NOT NULL
	);
	ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider TEXT NOT NULL DEFAULT 'local';
	ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';
	INSERT INTO app_settings (key, value) VALUES ('auth_auto_provision', 'true') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('auth_default_role', 'viewer') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('auth_role_sync_enabled', 'false') ON CONFLICT DO NOTHING;`,

	// Version 29: permissions
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
		id             BIGSERIAL PRIMARY KEY,
		provider_name  TEXT NOT NULL,
		external_group TEXT NOT NULL,
		local_role     TEXT NOT NULL,
		UNIQUE(provider_name, external_group)
	);
	INSERT INTO permissions (code, description, category) VALUES
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
		('ai.use', 'Use AI features', 'ai')
	ON CONFLICT DO NOTHING;
	INSERT INTO role_permissions (role, permission) VALUES
		('viewer', 'hosts.view'), ('viewer', 'dns.view'), ('viewer', 'projects.view'),
		('viewer', 'services.view'), ('viewer', 'issues.view'), ('viewer', 'settings.view'),
		('editor', 'hosts.view'), ('editor', 'hosts.edit'),
		('editor', 'dns.view'), ('editor', 'dns.edit'),
		('editor', 'projects.view'), ('editor', 'projects.edit'),
		('editor', 'services.view'), ('editor', 'services.edit'),
		('editor', 'issues.view'), ('editor', 'issues.edit'),
		('editor', 'settings.view'),
		('editor', 'ssh.operate'),
		('editor', 'ai.use'),
		('admin', 'hosts.view'), ('admin', 'hosts.edit'), ('admin', 'hosts.delete'), ('admin', 'hosts.passwords'),
		('admin', 'dns.view'), ('admin', 'dns.edit'), ('admin', 'dns.delete'),
		('admin', 'projects.view'), ('admin', 'projects.edit'), ('admin', 'projects.delete'),
		('admin', 'services.view'), ('admin', 'services.edit'), ('admin', 'services.delete'),
		('admin', 'issues.view'), ('admin', 'issues.edit'), ('admin', 'issues.delete'),
		('admin', 'settings.view'), ('admin', 'settings.edit'),
		('admin', 'users.manage'), ('admin', 'ssh.operate'),
		('admin', 'backup.manage'), ('admin', 'integrations.manage'), ('admin', 'ai.use')
	ON CONFLICT DO NOTHING;`,

	// Version 30: LDAP
	`INSERT INTO app_settings (key, value) VALUES
		('auth_ldap_enabled', 'false'),
		('auth_ldap_host', ''),
		('auth_ldap_port', '636'),
		('auth_ldap_use_tls', 'true'),
		('auth_ldap_skip_verify', 'false'),
		('auth_ldap_base_dn', ''),
		('auth_ldap_bind_dn', ''),
		('auth_ldap_bind_password_cipher', ''),
		('auth_ldap_bind_password_nonce', ''),
		('auth_ldap_user_filter', '(mail=%s)'),
		('auth_ldap_username_attr', 'uid'),
		('auth_ldap_display_name_attr', 'cn'),
		('auth_ldap_email_attr', 'mail'),
		('auth_ldap_fallback_to_local', 'true')
	ON CONFLICT DO NOTHING;`,

	// Version 31: Keycloak
	`INSERT INTO app_settings (key, value) VALUES
		('auth_keycloak_enabled', 'false'),
		('auth_keycloak_base_url', ''),
		('auth_keycloak_realm', 'pi'),
		('auth_keycloak_client_id', ''),
		('auth_keycloak_client_secret_cipher', ''),
		('auth_keycloak_client_secret_nonce', '')
	ON CONFLICT DO NOTHING;`,

	// Version 32: GitLab
	`INSERT INTO app_settings (key, value) VALUES
		('auth_gitlab_enabled', 'false'),
		('auth_gitlab_base_url', 'https://gitlab.com'),
		('auth_gitlab_client_id', ''),
		('auth_gitlab_client_secret_cipher', ''),
		('auth_gitlab_client_secret_nonce', ''),
		('gitlab_integration_enabled', 'false')
	ON CONFLICT DO NOTHING;
	CREATE TABLE IF NOT EXISTS user_gitlab_tokens (
		id                   BIGSERIAL PRIMARY KEY,
		user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		gitlab_base_url      TEXT NOT NULL DEFAULT 'https://gitlab.com',
		access_token_cipher  BYTEA,
		access_token_nonce   BYTEA,
		refresh_token_cipher BYTEA,
		refresh_token_nonce  BYTEA,
		gitlab_user_id       TEXT NOT NULL DEFAULT '',
		gitlab_username      TEXT NOT NULL DEFAULT '',
		expires_at           TIMESTAMPTZ,
		created_at           TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at           TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, gitlab_base_url)
	);
	CREATE TABLE IF NOT EXISTS project_gitlab_links (
		id                BIGSERIAL PRIMARY KEY,
		project_id        BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		gitlab_project_id BIGINT NOT NULL,
		gitlab_base_url   TEXT NOT NULL DEFAULT 'https://gitlab.com',
		gitlab_path       TEXT NOT NULL DEFAULT '',
		sync_issues       BOOLEAN NOT NULL DEFAULT FALSE,
		last_synced_at    TIMESTAMPTZ,
		created_at        TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, gitlab_project_id, gitlab_base_url)
	);`,

	// Version 33: LLM
	`INSERT INTO app_settings (key, value) VALUES
		('llm_enabled', 'false'),
		('llm_base_url', ''),
		('llm_api_key_cipher', ''),
		('llm_api_key_nonce', ''),
		('llm_model_text', 'Qwen/Qwen3-30B-A3B'),
		('llm_model_vision', 'Qwen/Qwen3-VL-30B-A3B-Thinking'),
		('llm_max_tokens', '2000')
	ON CONFLICT DO NOTHING;`,

	// Version 34
	`INSERT INTO app_settings (key, value) VALUES ('auth_active_provider', 'local') ON CONFLICT DO NOTHING;`,

	// Version 35
	`ALTER TABLE contacts ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT '';
	ALTER TABLE contacts ADD COLUMN IF NOT EXISTS entity TEXT NOT NULL DEFAULT '';
	CREATE TABLE IF NOT EXISTS host_responsaveis (
		id         BIGSERIAL PRIMARY KEY,
		host_id    BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		is_main    BOOLEAN NOT NULL DEFAULT FALSE,
		is_externo BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_responsaveis_host ON host_responsaveis(host_id);
	CREATE TABLE IF NOT EXISTS host_chamados (
		id         BIGSERIAL PRIMARY KEY,
		host_id    BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		chamado_id TEXT NOT NULL DEFAULT '',
		user_id    BIGINT NOT NULL DEFAULT 0,
		date       TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_chamados_host ON host_chamados(host_id);
	INSERT INTO contacts (name, phone) SELECT responsavel_interno, contato_responsavel_interno FROM hosts WHERE responsavel_interno != '' ON CONFLICT DO NOTHING;
	INSERT INTO host_responsaveis (host_id, contact_id, is_main, is_externo)
		SELECT h.id, c.id, TRUE, FALSE
		FROM hosts h
		JOIN contacts c ON c.name = h.responsavel_interno AND c.phone = h.contato_responsavel_interno
		WHERE h.responsavel_interno != '';
	INSERT INTO contacts (name, phone) SELECT responsavel_externo, contato_responsavel_externo FROM hosts WHERE responsavel_externo != '' ON CONFLICT DO NOTHING;
	INSERT INTO host_responsaveis (host_id, contact_id, is_main, is_externo)
		SELECT h.id, c.id, TRUE, TRUE
		FROM hosts h
		JOIN contacts c ON c.name = h.responsavel_externo AND c.phone = h.contato_responsavel_externo
		WHERE h.responsavel_externo != '';`,

	// Version 36
	`CREATE TABLE IF NOT EXISTS host_alerts (
		id          BIGSERIAL PRIMARY KEY,
		host_id     BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		type        TEXT NOT NULL,
		level       TEXT NOT NULL DEFAULT 'info',
		message     TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		source      TEXT NOT NULL DEFAULT 'manual',
		created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_host_alerts_host ON host_alerts(host_id);
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS expected_end_date TEXT NOT NULL DEFAULT '';
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS start_date TEXT NOT NULL DEFAULT '';
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS end_date TEXT NOT NULL DEFAULT '';
	ALTER TABLE issues ADD COLUMN IF NOT EXISTS alert_id BIGINT REFERENCES host_alerts(id) ON DELETE SET NULL;`,

	// Version 37
	`CREATE TABLE IF NOT EXISTS issue_alert_links (
		issue_id BIGINT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
		alert_id BIGINT NOT NULL REFERENCES host_alerts(id) ON DELETE CASCADE,
		PRIMARY KEY (issue_id, alert_id)
	);
	INSERT INTO issue_alert_links (issue_id, alert_id)
		SELECT id, alert_id FROM issues WHERE alert_id IS NOT NULL
	ON CONFLICT DO NOTHING;`,

	// Version 38: make project_id nullable (Postgres supports it in-place)
	`ALTER TABLE issues ALTER COLUMN project_id DROP NOT NULL;`,

	// Version 39
	`ALTER TABLE host_alerts ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';`,

	// Version 40
	`ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'in_execution';`,

	// Version 41
	`ALTER TABLE issues ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT FALSE;`,

	// Version 42
	`CREATE TABLE IF NOT EXISTS dns_responsaveis (
		id         BIGSERIAL PRIMARY KEY,
		dns_id     BIGINT NOT NULL REFERENCES dns_records(id) ON DELETE CASCADE,
		contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		is_main    BOOLEAN NOT NULL DEFAULT FALSE,
		is_externo BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_dns_responsaveis_dns ON dns_responsaveis(dns_id);
	CREATE TABLE IF NOT EXISTS service_responsaveis (
		id         BIGSERIAL PRIMARY KEY,
		service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		is_main    BOOLEAN NOT NULL DEFAULT FALSE,
		is_externo BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_service_responsaveis_service ON service_responsaveis(service_id);`,

	// Version 43
	`ALTER TABLE project_responsaveis ADD COLUMN IF NOT EXISTS contact_id BIGINT NOT NULL DEFAULT 0 REFERENCES contacts(id) ON DELETE CASCADE;
	ALTER TABLE project_responsaveis ADD COLUMN IF NOT EXISTS is_main BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE project_responsaveis ADD COLUMN IF NOT EXISTS is_externo BOOLEAN NOT NULL DEFAULT FALSE;`,

	// Version 44
	`CREATE TABLE IF NOT EXISTS project_host_links (
		project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		host_id    BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		PRIMARY KEY (project_id, host_id)
	);
	CREATE INDEX IF NOT EXISTS idx_project_host_links_host ON project_host_links(host_id);`,

	// Version 45
	`CREATE TABLE IF NOT EXISTS host_operation_logs (
		id             BIGSERIAL PRIMARY KEY,
		host_id        BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		operation_type TEXT NOT NULL,
		auth_method    TEXT,
		status         TEXT NOT NULL,
		output         TEXT NOT NULL DEFAULT '',
		created_at     TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_op_logs_host ON host_operation_logs(host_id);`,

	// Version 46
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS docker_group_status TEXT;`,

	// Version 47
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS coolify_server_uuid TEXT;`,

	// Version 48: rename hosts."user" to hosts.ssh_user so model SQL can
	// stop quoting the reserved word. Aligns with SQLite v48.
	`ALTER TABLE hosts RENAME COLUMN "user" TO ssh_user;`,

	// Version 49: link external_tools to services and DNS for sync.
	`ALTER TABLE external_tools ADD COLUMN IF NOT EXISTS service_id BIGINT REFERENCES services(id) ON DELETE SET NULL;
	ALTER TABLE external_tools ADD COLUMN IF NOT EXISTS dns_id BIGINT REFERENCES dns_records(id) ON DELETE SET NULL;
	ALTER TABLE external_tools ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'manual';`,

	// Version 50: service evolution — container discovery, service modes (manual/auto/fixed).
	`ALTER TABLE services ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'manual';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS container_status TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS container_id TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS container_name TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS container_image TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS container_ports TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS discovered_at TIMESTAMPTZ;
	ALTER TABLE services ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

	UPDATE services SET service_type = 'app-fullstack' WHERE service_type = 'fullstack';
	UPDATE services SET service_type = 'app-frontend' WHERE service_type = 'frontend';
	UPDATE services SET service_type = 'app-api' WHERE service_type = 'api';
	UPDATE enum_options SET value = 'app-fullstack' WHERE category = 'service_type' AND value = 'fullstack';
	UPDATE enum_options SET value = 'app-frontend' WHERE category = 'service_type' AND value = 'frontend';
	UPDATE enum_options SET value = 'app-api' WHERE category = 'service_type' AND value = 'api';

	INSERT INTO enum_options (category, value, sort_order) VALUES
		('service_type', 'nginx', 8),
		('service_type', 'agents', 9),
		('service_type', 'others', 10)
	ON CONFLICT DO NOTHING;

	CREATE INDEX IF NOT EXISTS idx_services_container ON services(container_name, source);`,

	// Version 51: host_remote_users — links a remote-user account (e.g. "coolify")
	// created on a host to the sshcm-managed ssh_keys row whose pubkey was
	// installed in that user's authorized_keys. Used by the Coolify integration
	// to auto-pick the right private key when registering the server.
	`CREATE TABLE IF NOT EXISTS host_remote_users (
		id          BIGSERIAL PRIMARY KEY,
		host_id     BIGINT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		username    TEXT NOT NULL,
		ssh_key_id  BIGINT REFERENCES ssh_keys(id) ON DELETE SET NULL,
		created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(host_id, username)
	);
	CREATE INDEX IF NOT EXISTS idx_host_remote_users_host ON host_remote_users(host_id);`,

	// Version 52: GitLab Code Management — see migrations_sqlite.go for rationale.
	`ALTER TABLE project_gitlab_links ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'project';
	ALTER TABLE project_gitlab_links ADD COLUMN IF NOT EXISTS ref_name TEXT NOT NULL DEFAULT '';
	ALTER TABLE project_gitlab_links ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';

	INSERT INTO app_settings (key, value) VALUES ('gitlab_code_service_token_cipher', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('gitlab_code_service_token_nonce', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('gitlab_code_default_ref', '') ON CONFLICT DO NOTHING;`,

	// Version 53: cache per-project AI analyses — see migrations_sqlite.go for rationale.
	`CREATE TABLE IF NOT EXISTS project_ai_analyses (
		project_id    BIGINT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
		content       TEXT NOT NULL DEFAULT '',
		locale        TEXT NOT NULL DEFAULT '',
		commits_used  INTEGER NOT NULL DEFAULT 0,
		repos_used    INTEGER NOT NULL DEFAULT 0,
		generated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,

	// Version 54: Grafana integration scaffolding — see migrations_sqlite.go for details.
	`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS grafana_dashboard_uid TEXT NOT NULL DEFAULT '';
	ALTER TABLE services ADD COLUMN IF NOT EXISTS grafana_dashboard_uid TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_alerts ADD COLUMN IF NOT EXISTS external_id TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_alerts ADD COLUMN IF NOT EXISTS external_source TEXT NOT NULL DEFAULT '';
	CREATE INDEX IF NOT EXISTS idx_host_alerts_external ON host_alerts(external_source, external_id);

	INSERT INTO app_settings (key, value) VALUES ('grafana_enabled', 'false') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_base_url', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_api_token_cipher', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_api_token_nonce', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_webhook_secret_cipher', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_webhook_secret_nonce', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_host_default_dashboard_uid', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_service_default_dashboard_uid', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_prom_remote_write_url', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_prom_remote_write_username', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_prom_remote_write_password_cipher', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_prom_remote_write_password_nonce', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('grafana_datasource_uid', '') ON CONFLICT DO NOTHING;`,

	// Version 55: Outline integration — see migrations_sqlite.go for rationale.
	`ALTER TABLE projects ADD COLUMN IF NOT EXISTS outline_collection_id TEXT NOT NULL DEFAULT '';

	INSERT INTO app_settings (key, value) VALUES ('outline_enabled', 'false') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('outline_base_url', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('outline_api_token_cipher', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('outline_api_token_nonce', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('outline_common_collection_id', '') ON CONFLICT DO NOTHING;`,

	// Version 56: GLPI integration — see migrations_sqlite.go for rationale.
	`CREATE TABLE IF NOT EXISTS glpi_tokens (
		id                BIGSERIAL PRIMARY KEY,
		name              TEXT NOT NULL UNIQUE,
		description       TEXT NOT NULL DEFAULT '',
		user_token_cipher BYTEA,
		user_token_nonce  BYTEA,
		default_entity_id INTEGER NOT NULL DEFAULT 0,
		created_at        TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at        TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS external_source TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS external_url TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS cached_title TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS cached_status TEXT NOT NULL DEFAULT '';
	ALTER TABLE host_chamados ADD COLUMN IF NOT EXISTS cached_at TIMESTAMPTZ;

	ALTER TABLE projects ADD COLUMN IF NOT EXISTS glpi_token_id BIGINT DEFAULT NULL REFERENCES glpi_tokens(id) ON DELETE SET NULL;
	ALTER TABLE projects ADD COLUMN IF NOT EXISTS glpi_entity_id INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE projects ADD COLUMN IF NOT EXISTS glpi_category_id INTEGER NOT NULL DEFAULT 0;

	CREATE TABLE IF NOT EXISTS alert_chamado_links (
		alert_id   BIGINT NOT NULL REFERENCES host_alerts(id) ON DELETE CASCADE,
		chamado_id BIGINT NOT NULL REFERENCES host_chamados(id) ON DELETE CASCADE,
		PRIMARY KEY(alert_id, chamado_id)
	);

	INSERT INTO app_settings (key, value) VALUES ('glpi_enabled', 'false') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('glpi_base_url', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('glpi_app_token_cipher', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('glpi_app_token_nonce', '') ON CONFLICT DO NOTHING;
	INSERT INTO app_settings (key, value) VALUES ('glpi_default_entity_id', '0') ON CONFLICT DO NOTHING;`,

	// Version 57: GLPI dropdown catalogue — see migrations_sqlite.go for rationale.
	`CREATE TABLE IF NOT EXISTS glpi_dropdown_catalogues (
		id           BIGSERIAL PRIMARY KEY,
		itemtype     TEXT NOT NULL UNIQUE,
		options      TEXT NOT NULL DEFAULT '[]',
		option_count INTEGER NOT NULL DEFAULT 0,
		updated_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_by   BIGINT
	);`,
}
