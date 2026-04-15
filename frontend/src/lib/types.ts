export interface User {
  id: number;
  username: string;
  display_name: string;
  role: "admin" | "editor" | "viewer";
  auth_provider: string;
  email: string;
  permissions: string[];
  external_identities: { provider: string; external_id: string }[];
  created_at: string;
  updated_at: string;
}

export interface AuthProviderInfo {
  name: string;
  type: "direct" | "oauth";
  label: string;
  icon: string;
  color: string;
}

export interface AuthStatus {
  setup_required: boolean;
  authenticated: boolean;
  providers: AuthProviderInfo[];
}

export interface Host {
  id: number;
  nickname: string;
  oficial_slug: string;
  hostname: string;
  hospedagem: string;
  tipo_maquina: string;
  user: string;
  has_password: boolean;
  has_key: boolean;
  key_path: string;
  port: string;
  identities_only: string;
  proxy_jump: string;
  forward_agent: string;
  description: string;
  setor_responsavel: string;
  responsavel_interno: string;
  contato_responsavel_interno: string;
  acesso_empresa_externa: boolean;
  empresa_responsavel: string;
  responsavel_externo: string;
  contato_responsavel_externo: string;
  recurso_cpu: string;
  recurso_ram: string;
  recurso_armazenamento: string;
  situacao: string;
  precisa_manutencao: boolean;
  preferred_auth: "password" | "key" | "";
  connections_failed?: number;
  password_test_status?: "success" | "failed" | null;
  key_test_status?: "success" | "failed" | null;
  docker_group_status?: "ok" | "fixed" | "needs_sudo" | "needs_relogin" | "not_installed" | "failed" | null;
  coolify_server_uuid?: string | null;
  observacoes: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  has_scan?: boolean;
  last_scan_at?: string;
  scan_resources?: {
    cpu?: string;
    cpu_usage?: string;
    ram?: string;
    ram_percent?: string;
    storage?: string;
    disk_percent?: string;
  };
  containers_count?: number;
  processes_count?: number;
  services_count?: number;
  dns_count?: number;
  issues_count?: number;
  projects_count?: number;
  can_compile?: boolean;
  alerts?: HostAlert[];
  responsaveis?: HostResponsavel[];
  chamados?: HostChamado[];
  main_responsavel_name?: string;
  chamados_count?: number;
}

export type AlertLevel = "critical" | "warning" | "info";

export interface HostAlert {
  id?: number;
  type: string;
  level: AlertLevel;
  message: string;
  description?: string;
  source: "auto" | "manual";
  status?: "active" | "resolved";
  host_id?: number;
  linked_issue_id?: number | null;
}

export interface AlertThresholds {
  resource_critical: number;
  resource_warning: number;
  resource_info_low: number;
}

export type SortField = "nickname" | "containers_count" | "resource_cpu" | "resource_ram" | "resource_disk" | "situacao";

export interface HostSortConfig {
  field: SortField;
  direction: "asc" | "desc";
}

export interface HostFilters {
  situacao: string;
  tag: string;
  entidade_responsavel: string;
  responsavel_interno: string;
  key_test_status: string;
  password_test_status: string;
  has_scan: string;
  alert_level: string;
}

export interface DNSRecord {
  id: number;
  domain: string;
  has_https: boolean;
  situacao: string;
  responsavel: string;
  observacoes: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  host_ids?: number[];
  main_responsavel_name?: string;
  responsaveis?: EntityResponsavel[];
}

export interface Project {
  id: number;
  name: string;
  description: string;
  situacao: string;
  setor_responsavel: string;
  responsavel: string;
  tem_empresa_externa_responsavel: boolean;
  contato_empresa_responsavel: string;
  is_directly_managed: boolean;
  is_responsible: boolean;
  gitlab_url: string;
  documentation_url: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  main_responsavel_name?: string;
}

// Legacy project responsavel — kept for backward compat with old data
export interface ProjectResponsavelLegacy {
  id: number;
  project_id: number;
  nome: string;
  contato: string;
}

// Contact-based project responsavel (same as EntityResponsavel)
export type ProjectResponsavel = EntityResponsavel;

export interface Service {
  id: number;
  nickname: string;
  project_id: number | null;
  description: string;
  service_type: string;
  service_subtype: string;
  technology_stack: string;
  deploy_approach: string;
  orchestrator_tool: string;
  environment: string;
  port: string;
  version: string;
  orchestrator_managed: boolean;
  is_directly_managed: boolean;
  is_responsible: boolean;
  developed_by: string;
  is_external_dependency: boolean;
  external_provider: string;
  external_url: string;
  external_contact: string;
  repository_url: string;
  gitlab_url: string;
  documentation_url: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  host_ids?: number[];
  dns_ids?: number[];
  depends_on_ids?: number[];
  main_responsavel_name?: string;
  responsaveis?: EntityResponsavel[];
}

export interface Orchestrator {
  id: number;
  host_id: number;
  type: string;
  version: string;
  observacoes: string;
  created_at: string;
  updated_at: string;
}

export interface ServiceCredential {
  id: number;
  role_name: string;
  credentials?: string;
}

export interface EnumOption {
  category: string;
  value: string;
  sort_order: number;
  color?: string;
}

export interface GraphNode {
  id: string;
  type: "host" | "service" | "dns" | "project";
  label: string;
  status?: string;
  data?: Record<string, unknown>;
}

export interface GraphEdge {
  source: string;
  target: string;
  label: string;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface Issue {
  id: number;
  project_id: number;
  service_id: number | null;
  entity_type: string;
  entity_id: number;
  title: string;
  description: string;
  status: string;
  priority: string;
  assignee: string;
  source: string;
  source_ref: string;
  created_by: number;
  position: number;
  created_at: string;
  updated_at: string;
  assignee_ids?: number[];
  expected_end_date?: string;
  start_date?: string;
  end_date?: string;
  alert_id?: number | null;
  alert_ids?: number[];
  archived?: boolean;
}

export interface Release {
  id: number;
  project_id: number | null;
  title: string;
  description: string;
  status: string;
  target_date: string;
  live_date: string;
  created_at: string;
  updated_at: string;
  issue_ids?: number[];
}

export interface ExternalTool {
  id: number;
  name: string;
  description: string;
  url: string;
  icon: string;
  embed_enabled: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface Contact {
  id: number;
  name: string;
  phone: string;
  role: string;
  entity: string;
}

// Generic entity responsavel — shared across hosts, DNS, services, projects
export interface EntityResponsavel {
  id?: number;
  contact_id?: number;
  is_main: boolean;
  is_externo: boolean;
  name: string;
  phone: string;
  role: string;
  entity: string;
}

// Alias for backward compatibility
export type HostResponsavel = EntityResponsavel;

export interface HostChamado {
  id?: number;
  host_id?: number;
  chamado_id: string;
  title: string;
  status: string;
  user_id: number;
  user_display_name?: string;
  date: string;
}

export interface SSHKeyRecord {
  id: number;
  name: string;
  credential_type: "key" | "password";
  username: string;
  description: string;
  fingerprint: string;
  has_public_key: boolean;
  has_private_key: boolean;
  has_password: boolean;
  created_at: string;
}

export interface DashboardStats {
  hosts: {
    total: number;
    by_situacao: Record<string, number>;
    by_hospedagem: Record<string, number>;
    with_scans: number;
    maintenance: number;
  };
  recent_scans: Array<{
    id: number;
    host_id: number;
    nickname: string;
    slug: string;
    scanned_at: string;
  }> | null;
  dns_records: number;
  projects: number;
  services: number;
  orchestrators: number;
  open_issues: number;
}

export interface AuthStatus {
  setup_required: boolean;
  authenticated: boolean;
}
