export interface User {
  id: number;
  username: string;
  display_name: string;
  role: "admin" | "editor" | "viewer";
  auth_provider: string;
  email: string;
  /** "service": owns API tokens for an integration, never signs in. */
  kind: "person" | "service";
  permissions: string[];
  external_identities: { provider: string; external_id: string }[];
  /** Entidade memberships (primary first). Present on /api/auth/me and /api/users. */
  entidades?: UserEntidade[];
  created_at: string;
  updated_at: string;
}

// ── Entidades (org units; hierarchical visibility) ─────────────────────────

export interface Entidade {
  id: number;
  name: string;
  slug: string;
  parent_id: number | null;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface UserEntidade {
  id: number;
  name: string;
  slug: string;
  is_primary: boolean;
}

/** Resolved grants of one asset (GET /api/assets/{type}/{id}/entidades). */
export interface AssetGrants {
  creator_entidade_id: number | null;
  responsible_entidade_ids: number[];
  is_global: boolean;
}

/** Write-side grants; every field optional = "leave untouched". Embedded in
 *  create/update bodies of root assets and PUT .../entidades. */
export interface AssetGrantsInput {
  creator_entidade_id?: number | null;
  responsible_entidade_ids?: number[];
  is_global?: boolean;
}

export type AssetType =
  | "host" | "dns" | "service" | "project" | "contact"
  | "tool" | "api_catalog" | "secret" | "offering";

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
  grafana_dashboard_uid?: string;
  proxmox_id?: string | null;
  parent_host_id?: number | null;
  parent_host_slug?: string | null;
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
  // Idle-VM heuristic (computed during scan). idle=true means every rule
  // (R1..R4) passed; idle_reasons lists which rules passed,
  // idle_counterfacts lists which rules failed (with brief explanations).
  idle?: boolean;
  idle_reasons?: string[];
  idle_counterfacts?: string[];
  containers_count?: number;
  processes_count?: number;
  services_count?: number;
  dns_count?: number;
  issues_count?: number;
  projects_count?: number;
  can_compile?: boolean;
  alerts?: HostAlert[];
  /** Latest Coolify Sentinel sync, kept apart from the scan (list only). */
  sentinel?: SentinelHostSnapshot | null;
  responsaveis?: HostResponsavel[];
  chamados?: HostChamado[];
  main_responsavel_name?: string;
  main_entidade?: string;
  chamados_count?: number;
}


export type AlertLevel = "critical" | "warning" | "info";

export interface HostAlert {
  id?: number;
  type: string;
  level: AlertLevel;
  message: string;
  description?: string;
  source: "auto" | "manual" | "grafana";
  status?: "active" | "resolved";
  host_id?: number;
  linked_issue_id?: number | null;
  external_id?: string;
  external_source?: string;
}

export interface AlertThresholds {
  resource_critical: number;
  resource_warning: number;
  resource_info_low: number;
  /** Sentinel: share of requests answered 5xx in 24h (percent). */
  traffic_5xx_warning: number;
  traffic_5xx_critical: number;
  /** Sentinel: p95 latency in 24h (ms). */
  latency_p95_warning: number;
  latency_p95_critical: number;
}

export type SortField =
  | "nickname" | "hostname" | "hospedagem" | "containers_count" | "resource_cpu" | "resource_ram" | "resource_disk" | "situacao"
  | "sentinel_cpu" | "sentinel_ram" | "sentinel_disk";

export interface HostSortConfig {
  field: SortField;
  direction: "asc" | "desc";
}

export interface HostFilters {
  situacao: string;
  tag: string;
  entidade_id: string;
  responsavel_interno: string;
  key_test_status: string;
  password_test_status: string;
  scan_result: string;
  has_scan: string;
  alert_level: string;
  // "idle"   → only hosts the scan flagged as idle (R1..R4 all passed)
  // "active" → only hosts with at least one workload signal
  // ""       → no filter
  idle: string;
}

export interface DNSRecord {
  id: number;
  /** Set while the record is in the trash. */
  deleted_at?: string | null;
  /** Creator entidade name (list endpoint), like Host.main_entidade. */
  main_entidade?: string;
  domain: string;
  has_https: boolean;
  situacao: string;
  responsavel: string;
  observacoes: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  host_ids?: number[];
  service_ids?: number[];
  project_ids?: number[];
  /** List endpoint: linked services/projects and open issues. */
  services_count?: number;
  projects_count?: number;
  issues_count?: number;
  main_responsavel_name?: string;
  responsaveis?: EntityResponsavel[];
  // TLS certificate from the latest scan. cert_checked_at null = never scanned;
  // cert_expires_at null after a scan = the handshake failed (see cert_error).
  cert_not_before?: string | null;
  cert_expires_at?: string | null;
  cert_issuer?: string;
  cert_subject?: string;
  cert_sans?: string;
  cert_error?: string;
  cert_checked_at?: string | null;
  // DNS answer and HTTP status from the latest scan. observed_at null = never
  // probed. obs_target is usually the proxy's public address, not the host's.
  // Status codes are 0 when the port gave no HTTP answer.
  obs_record_type?: string;
  obs_target?: string;
  obs_http_status?: number;
  obs_https_status?: number;
  obs_status?: "" | "online" | "no_content" | "error" | "offline";
  observed_at?: string | null;
}

export interface Project {
  id: number;
  /** Creator entidade name (list endpoint). */
  main_entidade?: string;
  /** List endpoint: hosts and DNS are direct links plus those of the project's services. */
  hosts_count?: number;
  services_count?: number;
  dns_count?: number;
  /** Open issues; issues_total also counts done ones (archived excluded). */
  issues_count?: number;
  issues_total?: number;
  /** Linked GitLab repositories/groups. */
  repos_count?: number;
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
  outline_collection_id?: string;
  glpi_token_id?: number | null;
  glpi_entity_id?: number;
  glpi_category_id?: number;
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
  /** Category (database, cache, queue, web, proxy, …); "" = unclassified. */
  service_kind?: string;
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
  grafana_dashboard_uid?: string;
  source: "manual" | "auto" | "fixed" | "coolify";
  /** How the scan found this service; "" for manual services. */
  discovery_kind: "container" | "host" | "";
  /** Per-host identity the scan reconciles on: container name or catalog name. */
  discovery_key: string;
  container_status: "online" | "offline" | "";
  container_id: string;
  container_name: string;
  container_image: string;
  container_ports: string;
  discovered_at: string | null;
  last_seen_at: string | null;
  /** Written by the Coolify sync only. */
  coolify_resource_uuid?: string;
  coolify_resource_type?: string;
  coolify_project?: string;
  coolify_environment?: string;
  coolify_stack?: string;
  git_repository?: string;
  git_branch?: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  host_ids?: number[];
  dns_ids?: number[];
  depends_on_ids?: number[];
  main_responsavel_name?: string;
  /** List endpoint: open issues. */
  issues_count?: number;
  /** List endpoint: latest Coolify Sentinel sync (traffic + container). */
  sentinel?: SentinelServiceSnapshot | null;
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

// Secret mirrors the backend vault.SecretView metadata shape returned by
// /api/secrets — metadata only, never plaintext. Plaintext arrives via
// /api/secrets/{id}/reveal which returns { payload: string } in SecretReveal.
export interface Secret {
  id: number;
  type: "cred" | "sshkey" | "password" | "app_login" | "env_var" | "api_key";
  scope: "service" | "host" | "tool" | "projeto" | "avulso" | "api_catalog";
  visibility: "personal" | "shared";
  parent_id?: number | null;
  owner_user_id: number;
  name: string;
  group_label?: string | null;
  description?: string | null;
  key_version: number;
  created_by: number;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
  // List context (GET /api/secrets).
  username?: string | null;
  ssh_fingerprint?: string | null;
  parent_name?: string | null;
  owner_name?: string | null;
  /** Hosts logging in with this shared credential. */
  linked_hosts?: number;
  /** >1: this many visible secrets hold the same value; dup_group ties them. */
  dup_count?: number;
  dup_group?: string;
}

/** Identical per-host copies of a password/key the admin can consolidate. */
export interface ConsolidationGroup {
  key: string;
  type: "password" | "sshkey";
  username: string;
  host_ids: number[];
  host_names: string[];
  target_id?: number;
  target_name?: string;
}

export interface SecretReveal {
  payload: string;
}

export interface EnumOption {
  category: string;
  value: string;
  sort_order: number;
  color?: string;
  /** situação only: the meaning code relies on ("active" | "inactive" |
   *  "maintenance"), kept when the value is renamed; "" = none. */
  role?: string;
}

export interface GraphNode {
  id: string;
  type: "host" | "service" | "dns" | "project" | "api";
  label: string;
  status?: string;
  /** Service nodes may carry container_name, source, coolify_project,
   *  coolify_environment, coolify_stack, project_id (all optional). */
  data?: Record<string, unknown>;
}

export interface GraphEdge {
  source: string;
  target: string;
  label: string;
  /** Inferred (project → host/DNS through its services), not a stored link. */
  derived?: boolean;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
  /** Node in focus on a detail page ("service-12"); none = the whole map. */
  focus?: string;
}

/** An entidade's idea board. content (xyflow {nodes, edges}) only on GET by id. */
export interface Canvas {
  id: number;
  entidade_id: number;
  entidade_name: string;
  title: string;
  content?: { nodes: unknown[]; edges: unknown[] };
  version: number;
  created_by: number | null;
  created_at: string;
  updated_at: string;
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
  project_id: number;
  title: string;
  description: string;
  status: string;
  target_date: string;
  live_date: string;
  created_at: string;
  updated_at: string;
  issue_ids?: number[];
}

/** A BI / observability tool a project shows in an iframe. */
export interface ProjectEmbed {
  id: number;
  project_id: number;
  title: string;
  url: string;
  height: number;
  sort_order: number;
}

export interface ExternalTool {
  id: number;
  name: string;
  description: string;
  url: string;
  icon: string;
  embed_enabled: boolean;
  sort_order: number;
  service_id: number | null;
  dns_id: number | null;
  source: "manual" | "service";
  has_credentials: boolean;
  created_at: string;
  updated_at: string;
}

export interface Contact {
  id: number;
  name: string;
  phone: string;
  role: string;
  entity: string;
  notes: string;
  is_external: boolean;
  email?: string;
  /** List only: asset type (host, dns, service, project) → how many it's responsável for. */
  usage?: Partial<Record<"host" | "dns" | "service" | "project" | "api_catalog", number>>;
  /** Set while the contact is in the trash. */
  deleted_at?: string | null;
}

// Generic entity responsavel — shared across hosts, DNS, services, projects.
// Hosts/DNS/services/projects can only LINK to an existing contact (contact_id
// is required). Display fields (name/phone/role/entity/notes/is_external) are
// joined from the contact for read responses; the backend ignores them on write.
export interface EntityResponsavel {
  id?: number;
  contact_id: number;
  is_main: boolean;
  // Joined from the contact:
  name: string;
  phone: string;
  role: string;
  entity: string;
  notes: string;
  is_external: boolean;
}

// Write-side payload for syncing responsaveis. Only contact_id + is_main are
// persisted; everything else on EntityResponsavel is read-side metadata joined
// from contacts.
export interface EntityResponsavelInput {
  contact_id: number;
  is_main: boolean;
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
  external_source?: string;
  external_url?: string;
  cached_title?: string;
  cached_status?: string;
  cached_at?: string;
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


// --- Atlas REST API catalog (Phase A–C) ---

export interface ApiOperation {
  id: number;
  api_id: number;
  method: string;
  path: string;
  operation_id?: string;
  summary?: string;
  description?: string;
  tags: string[];
  op_key: string;
  sort_order: number;
}

export type ApiOrigem = "propria" | "terceiro" | "externa";

export interface ApiCatalog {
  id: number;
  name: string;
  description: string;
  /** Markdown: who calls the API and for what. */
  use_cases?: string;
  /** propria: we build and host it · terceiro: third-party software we host · externa: hosted elsewhere. */
  origem?: ApiOrigem;
  /** Who builds/provides it (vendor, another organization). */
  fornecedor?: string;
  /** Services that call this API (service_ids are the ones that serve it). */
  consumer_service_ids?: number[];
  /** Main internal responsável (list responses). */
  main_responsavel_name?: string;
  source_type: string; // "upload" | "url"
  source_url?: string;
  external_url?: string;
  base_url?: string;
  docs_url?: string;
  /** The API's other addresses (e.g. its origin when base_url is the gateway), in order; [] when none. */
  urls: ApiCatalogURL[];
  spec_version: string;
  spec_hash: string;
  title: string;
  version_label: string;
  owner_user_id: number;
  created_by: number;
  created_at: string;
  updated_at: string;
  operation_count: number;
  operations?: ApiOperation[];
  /** Direct links; [] when none. An API with neither is "avulso". */
  service_ids: number[];
  project_ids: number[];
  /** Detail only. */
  responsaveis?: EntityResponsavel[];
  /** Detail only: entidade grants, for the edit form. */
  entidades?: AssetGrants;
  /** How the API's access keys are handled. */
  key_management: ApiKeyManagement;
  /** keycloak mode: the API root, where GET /escopos and GET /admin/uso live. */
  admin_base_url?: string;
  /** Names the API's scopes ("<prefix>:…") and Keycloak clients ("<prefix>-<label>"); "bridge" is Bridge itself. */
  scope_prefix: string;
}

export type ApiKeyManagement = "none" | "manual" | "keycloak";

export interface ApiCatalogURL {
  label: string;
  url: string;
}

/** One access key of a catalogued API. The plaintext lives in the vault (secret_id). */
export interface ApiKey {
  id: number;
  api_id: number;
  label: string;
  source: "manual" | "keycloak";
  /** keycloak: the client id */
  external_label?: string;
  secret_id?: number;
  owner: string;
  owner_contact_id?: number;
  owner_contact_name?: string;
  notes: string;
  scopes: string[];
  rate_limit_per_minute?: number;
  expires_at: string | null;
  revoked_at: string | null;
  grace_until: string | null;
  last_used_at: string | null;
  lifetime_uses: number;
  synced_at: string | null;
  created_at: string;
  updated_at: string;
  status: "active" | "grace" | "expired" | "revoked";
}

/** One scope-catalogue entry: a catalogued API's (GET /escopos) or Bridge's own tokens. */
export interface ApiKeyScope {
  name: string;
  kind: "wildcard" | "route" | "modifier";
  /** Bridge tokens: the least role an owner needs to use it. */
  min_role?: "viewer" | "editor" | "admin";
  /** Bridge tokens: a permission code needed instead of a role. */
  permission?: string;
  description: string;
  /** Route patterns: a prefix, or "=/path" for that exact path. */
  routes: string[];
}

export interface ApiKeyUsage {
  cliente: string;
  lifetime: number;
  last_used_at?: string | null;
  /** "YYYY-MM-DD" → requests */
  daily: Record<string, number>;
}

/** POST /api/api-catalog/{id}/keys/{keyId}/test: one key's verdict. */
export interface ApiKeyTestResult {
  key_id: number;
  label: string;
  client_id: string;
  ok: boolean;
  token_ok: boolean;
  token_error?: string;
  granted: string[];
  /** in Bridge's list, absent from the token */
  missing: string[];
  rate_limit_per_minute?: number;
  expires_in?: number;
  url: string;
  status?: number;
  answered_by?: "gateway" | "api";
  /** start of an error answer (never of a 2xx one) */
  body?: string;
  error?: string;
}

export interface ApiKeyTestBatch {
  results: ApiKeyTestResult[];
  total: number;
  ok: number;
}

export interface OperationSearchResult {
  api_id: number;
  api_name: string;
  method: string;
  path: string;
  op_key: string;
  summary?: string;
  description?: string;
  tags: string[];
}

// ApiDocSelector describes which slice of an API doc a share bundle exposes
// (Phase D). Defined here so the share UI and partial-spec preview agree.
export interface ApiDocSelector {
  mode: "all" | "operations" | "tags";
  op_keys?: string[];
  tags?: string[];
}

// --- Share bundles (Phase D–E) ---

export interface ShareBundleItemView {
  type: "secret" | "api_doc" | "wiki_doc" | "wiki_collection";
  ref_id: number;
  ref_key?: string; // string UUID for wiki items (ref_id is 0)
  label: string;
  selector?: string; // raw JSON selector for api_doc items
}

export interface ShareBundleView {
  id: number;
  title: string;
  description?: string;
  expires_at: string | null; // null = never expires
  max_views?: number | null;
  view_count: number;
  created_by: number;
  created_at: string;
  revoked_at?: string | null;
  deleted_at?: string | null;
  has_passphrase: boolean;
  items: ShareBundleItemView[];
  /** Who the link is for: a contact (recipient_name is its name) and/or free text. */
  recipient_contact_id?: number | null;
  recipient_name?: string;
  /** The recipient contact's email — the default address for a send. */
  recipient_email?: string;
  recipient_label?: string;
  created_by_name?: string;
  /** Last guest redemption (admin reveals don't count). */
  last_access_at?: string | null;
  /** The link and passphrase are stored encrypted, so an admin can reveal them. */
  recoverable?: boolean;
}

/** What an admin gets back from POST /api/share-bundles/{id}/reveal. */
export interface ShareBundleReveal {
  token: string;
  url: string;
  passphrase: string;
}

// BundlePayload is the resolved, guest-facing content returned by the public
// GET /api/share-bundle/{token} endpoint.
export interface BundleSecretItem {
  name: string;
  type: string;
  payload: string;
  /** Set when the secret is an API access key's: what the guest needs to use it. */
  key?: BundleKeyInfo;
}

export interface BundleKeyInfo {
  client_id?: string;
  api_name: string;
  api_base_url?: string;
  scopes: BundleScopeInfo[];
  rate_limit_per_minute?: number;
  token_url?: string;
}

/** kind/description are empty when the API's /escopos couldn't be read. */
export interface BundleScopeInfo {
  name: string;
  kind?: "route" | "modifier";
  description?: string;
}

export interface BundleApiDocItem {
  name: string;
  title: string;
  version?: string;
  external_url?: string;
  spec: Record<string, unknown>;
}

// BundleWikiDoc is one resolved Outline document (raw markdown + nested child
// tree for collections). Markdown is rendered through a sanitizing pipeline on
// the public redeem page.
export interface BundleWikiDoc {
  id: string;
  title: string;
  emoji?: string;
  markdown: string;
  browse_url?: string;
  updated_at?: string;
  updated_by?: string;
  children?: BundleWikiDoc[];
}

// BundleWikiItem is one resolved wiki bundle item: a single document
// (kind "doc", one root in documents) or a whole collection (kind "collection",
// the collection's document tree in documents). truncated is set when a large
// collection's fan-out was capped.
export interface BundleWikiItem {
  kind: "doc" | "collection";
  title: string;
  collection_name?: string;
  description?: string;
  browse_url?: string;
  documents: BundleWikiDoc[];
  truncated?: boolean;
}

export interface BundlePayload {
  title: string;
  description?: string;
  secrets: BundleSecretItem[];
  api_docs: BundleApiDocItem[];
  wiki: BundleWikiItem[];
  /** Last 30 days of each shared Keycloak key's own requests (absent when SigNoz is off). */
  usage?: BundleKeyUsage[];
}

/** One shared key's usage; secret is the shared secret's name. Latencies in ms. */
export interface BundleKeyUsage {
  secret: string;
  api_name: string;
  summary: { count: number; errors: number; p50: number; p95: number; p99: number };
  series: { t: string; count: number; errors: number; p50: number; p95: number; p99: number }[];
  top: { key: string; count: number; errors: number; p50: number; p95: number; p99: number }[];
}

// ShareBundleAccessEntry is one access-log row returned by
// GET /api/share-bundles/{id}/access-log: an anonymous guest redemption
// (network metadata only), an admin reveal or an email send (actor_name says
// who; sent_to, for a send, the address).
export interface ShareBundleAccessEntry {
  accessed_at: string;
  remote_ip: string;
  user_agent: string;
  used_passphrase: boolean;
  action?: "redeem" | "reveal" | "send";
  actor_name?: string;
  sent_to?: string;
}

/** One address's outcome from POST /api/share-bundles/{id}/send. */
export interface ShareSendResult {
  email: string;
  sent: boolean;
  error?: string;
}

// ── Service Catalog & Requests ──────────────────────────────────────────────

export type RequestStatus =
  | "submitted" | "under_review" | "approved"
  | "in_progress" | "delivered" | "rejected" | "cancelled" | "needs_info";

export type RequestType =
  | "vm" | "service" | "dns" | "api_token" | "feature" | "account" | "support";

export type FormFieldType =
  | "text" | "textarea" | "number" | "select"
  | "checkbox" | "date" | "tags" | "asset_ref";

/** One field descriptor in an offering's form schema. Labels are bilingual
 *  because offerings are user-authored data, not code, and the UI is bilingual. */
export interface FormField {
  key: string;
  type: FormFieldType;
  label_en: string;
  label_pt: string;
  required: boolean;
  options?: string[]; // for "select"
  help_en?: string;
  help_pt?: string;
  /** Shown inside the empty control. A placeholder is an example, never a
   *  label — the label always sits above the input. */
  placeholder_en?: string;
  placeholder_pt?: string;
  max_length?: number;
  asset_type?: AssetType; // for "asset_ref"

  // ── Rules the offering declares; the form adapts to whatever is present ──
  /** Inclusive bounds for "number". Without these a user can ask for 9999 vCPUs
   *  and nothing objects until a human reads it. */
  min?: number;
  max?: number;
  /** Anchored regex source for "text"/"textarea", e.g. a hostname or a database
   *  identifier. Stored as a string because form_schema is authored data (JSONB),
   *  not code. Invalid patterns are ignored rather than throwing — a typo in an
   *  admin's regex must not break the whole form. */
  pattern?: string;
  pattern_hint_en?: string;
  pattern_hint_pt?: string;
  /** Conditional visibility: render this field only while another field's value
   *  is one of `equals`. A hidden field is not validated and its answer is
   *  dropped at submit, so form_data never carries an answer the user could not
   *  see themselves giving. */
  depends_on?: { key: string; equals: string[] };
}

export interface FormSchema {
  fields: FormField[];
}

/** A named preset over one offering's form_schema. Lets an offering answer
 *  "what size do I need?" with worked examples instead of making every
 *  requester invent numbers. Authored data, like the schema itself — the UI
 *  renders whatever templates an offering declares, and none is a valid
 *  answer (the "customised" path). */
export interface OfferingTemplate {
  key: string;
  name_en: string;
  name_pt: string;
  summary_en: string;
  summary_pt: string;
  /** Partial form_data. Keys not present are left for the user to fill. */
  values: Record<string, unknown>;
}

export interface Offering {
  id: number;
  slug: string;
  name: string;
  category: string;
  description: string;
  request_type: RequestType;
  templates?: OfferingTemplate[];
  /** Short, concrete situations this offering is meant for, shown on the
   *  catalog card so a user can tell "is this the thing I need?" without
   *  opening the request form. Authored content like `name`/`description`,
   *  so it is stored in the offering's own language, not the i18n bundle.
   *  ponytail: needs a `use_cases` JSONB column in B1's migration. */
  use_cases?: string[];
  form_schema: FormSchema;
  approver_entidade_id?: number | null;
  glpi_mode: "inherit" | "never" | "always";
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
  entidades?: AssetGrants; // detail endpoint returns grants alongside
}

export interface ServiceRequest {
  id: number;
  offering_id: number;
  title: string;
  status: RequestStatus;
  priority: string;
  form_data: Record<string, unknown>;
  form_schema_snapshot: FormSchema;
  requester_user_id: number;
  /** Who to contact about this request, and how. Defaults to the signed-in
   *  user's name but stays editable: the person who files a request is often
   *  not the person the fulfilling team needs to call, and `users` carries no
   *  phone at all (only Contact/Responsavel do). Kept separate from
   *  requester_user_id on purpose — editing a contact detail must never
   *  silently reassign who owns the request.
   *  ponytail: needs two columns in B1's migration. */
  contact_name?: string;
  contact_phone?: string;
  requester_entidade_id?: number | null;
  assignee_user_id?: number | null;
  decided_by_user_id?: number | null;
  decided_at?: string | null;
  delivered_by_user_id?: number | null;
  delivered_at?: string | null;
  fulfilled_asset_type: string;
  fulfilled_asset_id?: number | null;
  external_source: string;
  external_ref: string;
  external_url: string;
  cached_title: string;
  cached_status: string;
  cached_at?: string | null;
  created_at: string;
  updated_at: string;
  // Display-only joined fields the list endpoint returns.
  offering_name?: string;
  requester_name?: string;
  entidade_name?: string;
}

export interface RequestEvent {
  id: number;
  request_id: number;
  user_id?: number | null;
  user_name?: string;
  kind: "comment" | "status" | "assign" | "link" | "delivery" | "integration";
  body: string;
  from_status: string;
  to_status: string;
  created_at: string;
}

/** The server is the single source of truth for what the current user may do
 *  with a request; the UI must never re-derive these rules. */
export interface RequestAbilities {
  approve: boolean;
  fulfill: boolean;
  cancel: boolean;
  edit: boolean;
}

export interface RequestDetail {
  request: ServiceRequest;
  offering: Offering;
  events: RequestEvent[];
  can: RequestAbilities;
}

/** Unified discovery result (GET /api/catalog/search) — one flat shape so the
 *  UI renders a single list of existing assets alongside requestable offerings. */
export interface CatalogHit {
  kind: "offering" | "asset";
  asset_type: string; // empty for offerings
  id: number;
  name: string;
  description: string;
  /** One short, type-specific fact — a host's FQDN, a service's stack and
   *  port, an API's spec version. The discovery table is polymorphic, so this
   *  is the column that lets one row shape say something true about six
   *  different asset types without six sets of columns. */
  detail?: string;
  href: string;
  entidade_name?: string;
  slug?: string;
}

/** A Bridge project mapped to a Coolify project (and environment; "" = all). */
export interface ProjectCoolifyLink {
  coolify_project: string;
  coolify_environment: string;
}

// ── Assistant (POST /api/ai/chat) ──────────────────────────────────────────

/** A Bridge page a tool result pointed to; cited in the answer as a markdown link. */
export interface AiRef { type: string; id: string; label: string; href: string }

/** One tool the model ran while answering (shown as a step in the thread). */
export interface AiStep { tool: string; query?: string; count: number; error?: string }

/** An action the model drafted; nothing happens until the user confirms it. */
export interface AiProposal {
  id: string;
  kind: "issue" | "chamado";
  title: string;
  description: string;
  priority?: string;
  parent?: AiRef;
  /** The issue's entity_type / entity_id for POST /api/issues. */
  parent_type?: string;
  parent_id?: number;
  profile?: { id: number; name: string };
  host_slug?: string;
}

export interface AiChatResponse {
  response: string;
  /** false: the model gateway rejected tools, so the answer didn't look at the data. */
  tools?: boolean;
  steps?: AiStep[];
  refs?: AiRef[];
  proposals?: AiProposal[];
}

/* ─── Coolify Sentinel ─── */

/** Why Sentinel can't be read (i18n: sentinel.reason.<reason>). */
export type SentinelReason =
  | "ok" | "disabled" | "coolify_off" | "not_linked" | "ssh_config" | "ssh_failed" | "no_docker_access"
  | "no_sentinel" | "no_tcp_forwarding" | "no_token" | "metrics_off" | "traffic_off" | "error";

export interface SentinelHostSnapshot {
  host_id: number;
  state: SentinelReason;
  detail?: string;
  metrics_enabled: boolean;
  traffic_enabled: boolean;
  push_ok: boolean | null;
  last_push_at: string | null;
  cpu_avg_24h: number | null;
  cpu_p95_24h: number | null;
  mem_p95_24h: number | null;
  disk_pct: number | null;
  load1: number | null;
  synced_at: string;
}

export interface SentinelServiceSnapshot {
  service_id: number;
  host_id: number;
  req_24h: number | null;
  s4xx_24h: number | null;
  s5xx_24h: number | null;
  p50_ms: number | null;
  p95_ms: number | null;
  p99_ms: number | null;
  visitors_24h: number | null;
  container_state: string;
  container_health: string;
  restart_count: number | null;
  prev_restart_count: number | null;
  /** Percent of one core: can pass 100. */
  cpu_pct: number | null;
  mem_pct: number | null;
  traefik_bypass: boolean;
  synced_at: string;
  service_name: string;
}

/** Every Sentinel read: the data, or why there is none. */
export interface SentinelResponse<T> {
  available: boolean;
  reason?: SentinelReason;
  detail?: string;
  data?: T;
}

export interface SentinelPushStatus {
  last_attempt_at: string | null;
  last_success_at: string | null;
  last_error: string | null;
  consecutive_failures: number;
}

export interface SentinelStatus {
  available: boolean;
  reason?: SentinelReason;
  detail?: string;
  metrics: boolean;
  traffic: boolean;
  version?: string;
  push?: SentinelPushStatus;
}

export interface SentinelHostView {
  status: SentinelStatus;
  snapshot: SentinelHostSnapshot | null;
  services: Record<string, SentinelServiceSnapshot>;
  alerts: HostAlert[];
}

/** t = Unix ms. */
export interface SentinelPoint {
  t: number;
  v: number;
}

export interface SentinelHostMetrics {
  range: string;
  cpu: SentinelPoint[];
  memory: SentinelPoint[];
  load: SentinelPoint[];
  current: { cpu: number; memory: number; disk: number; load1: number; uptime_s: number; sample_at: number };
}

export interface SentinelContainer {
  name: string;
  state: string;
  health: string;
  restart_count: number;
  cpu_pct: number | null;
  mem_pct: number | null;
  mem_used_bytes: number | null;
  service_id?: number;
  service_name?: string;
}

export interface SentinelStatusCounts {
  s2xx: number;
  s3xx: number;
  s4xx: number;
  s5xx: number;
}

export interface SentinelOverview {
  requests: number;
  bytes_in: number;
  bytes_out: number;
  status: SentinelStatusCounts;
  latency: { p50: number; p95: number; p99: number };
  unique_visitors: number;
}

export interface SentinelBucket extends SentinelStatusCounts {
  bucket: number;
  requests: number;
  unique_visitors: number;
  p95: number;
}

export interface SentinelPath {
  path: string;
  app: string;
  requests: number;
  bytes_out: number;
  s4xx: number;
  s5xx: number;
  p50: number;
  p95: number;
}

export interface SentinelServiceTraffic {
  range: string;
  /** Sentinel app keys (Traefik routers) matched; empty = no traffic through the proxy. */
  keys: string[];
  overview: SentinelOverview;
  series: SentinelBucket[];
  paths: SentinelPath[];
}

export interface SentinelBreakdownRow {
  value: string;
  requests: number;
  bytes_out: number;
}

export interface SentinelServiceResources {
  range: string;
  /** "" = no live container matched. */
  container: string;
  cpu: SentinelPoint[];
  memory: SentinelPoint[];
}

export interface SentinelSyncReport {
  hosts: { host_id: number; slug: string; nickname: string; state: SentinelReason; detail?: string; services: number }[];
  ok: number;
}

export interface SentinelHostRow {
  id: number;
  slug: string;
  nickname: string;
  hostname: string;
  snapshot: SentinelHostSnapshot | null;
}

// Vulnerability findings (ETIPI report imports, Wazuh). Visible through the
// assets they link to; via is set on entity-tab rows reached through a
// related host / DNS record ("dns:portal.sead.pi.gov.br").
export type VulnSeverity = "critical" | "high" | "medium" | "low" | "info";
export type VulnStatus = "open" | "accepted" | "false_positive" | "resolved";
export type VulnEntityType = "host" | "dns" | "service" | "project";

export interface VulnLink {
  entity_type: VulnEntityType;
  entity_id: number;
  origin: "auto" | "manual";
  name: string;
  slug?: string; // host URL key
}

export interface VulnFinding {
  id: number;
  source: "etipi" | "wazuh";
  title: string;
  severity: VulnSeverity;
  cve: string;
  cvss: number | null;
  cwe: string;
  scanner: string;
  target: string;
  vm_hint: string;
  evidence: string;
  risk: string;
  remediation: string;
  reference: string;
  known_exploited: boolean;
  status: VulnStatus;
  status_note: string;
  status_by: number | null;
  issue_id: number | null;
  glpi_ticket_id: number | null;
  glpi_ticket_url: string;
  first_seen_at: string;
  last_seen_at: string;
  resolved_at: string | null;
  via?: string;
  links?: VulnLink[];
}

export interface VulnImportResult {
  parsed: number;
  new: number;
  updated: number;
  reopened: number;
  resolved: number;
  unmatched: number;
  dry_run: boolean;
  unmatched_targets: string[];
}

/** One CVE's public NVD text plus a generic analysis in pt-BR and en,
 *  shared by every finding with that CVE. Catalog list rows leave the
 *  analysis fields empty. open_findings counts the caller's visible ones. */
export interface CveCatalogEntry {
  cve_id: string;
  description: string;
  cvss: number | null;
  cvss_vector: string;
  cwe: string;
  published_at: string | null;
  references: string[];
  analysis_pt: string;
  analysis_en: string;
  model: string;
  analyzed_by: number | null;
  analyzed_at: string;
  open_findings: number;
}

/** One tool call the fix plan's model made while exploring. */
export interface AiStep {
  tool: string;
  query?: string;
  count: number;
  error?: string;
}

/** A finding's case-specific fix plan (pt-BR markdown). repos: what the
 *  model was allowed to read; steps: what it actually read. */
export interface VulnFixPlan {
  finding_id: number;
  content: string;
  steps: AiStep[];
  repos: string[];
  model: string;
  generated_by: number | null;
  generated_at: string;
}
