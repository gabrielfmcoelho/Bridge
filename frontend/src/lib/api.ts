const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

export type OperationLog = {
  id: number;
  host_id: number;
  user_id: number;
  user_name: string;
  operation_type: string;
  auth_method?: string;
  status: string;
  output: string;
  created_at: string;
};

export type RemoteKeyInfo = {
  name: string;
  type: string;
  fingerprint: string;
  source: string;
};

export type DockerStatusType = {
  installed: boolean;
  docker_version?: string;
  compose_version?: string;
  user_in_group: boolean;
  needs_sudo: boolean;
  group_fix_applied?: boolean;
  message: string;
};

export type SystemdServiceType = {
  unit: string;
  description?: string;
  is_native: boolean;
};

export type InstalledPackageType = {
  name: string;
  version: string;
  source: string;
};

export type NginxCleanupStepType = {
  name: string;
  status: string;
  output?: string;
};

export type NginxCleanupStatusType = {
  found: boolean;
  is_native: boolean;
  is_container: boolean;
  backup_path?: string;
  steps: NginxCleanupStepType[];
  package_manager?: string;
  message: string;
};

export type VMInfoType = {
  cpu: string; cpu_usage: string;
  ram: string; ram_used: string; ram_percent: string;
  storage: string; storage_used: string; disk_percent: string;
  os: string; kernel: string; uptime: string; hostname_remote: string;
  load_avg: string; logged_users: string; public_ip: string;
  swap_total: string; swap_used: string;
  last_logins: string[]; services: string[]; service_details: string[];
  containers: string[]; container_stats: string[]; ports: string[];
  warnings?: string[];
  process_details?: ProcessDetail[];
  ssh_keys?: SSHKeyInfoScan[];
  systemd_services?: SystemdServiceType[];
  installed_packages?: InstalledPackageType[];
  cron_jobs?: string[];
  firewall_status?: string;
  remote_users?: RemoteUserInfo[];
  port_owners?: PortOwner[];
  parsed_containers?: ParsedContainer[];
};

export type ParsedContainer = {
  id: string;
  name: string;
  image: string;
  status: string;
  ports: string;
};

export type PortOwner = {
  port: number;
  process?: string;
  owner_type?: "container" | "nginx" | "process" | "docker" | string;
  owner_name?: string;
  target?: string;
};

export type RemoteUserInfo = {
  name: string;
  uid: number;
  shell?: string;
  home?: string;
  has_login: boolean;
  is_current?: boolean;
};

export type SSHKeyInfoScan = {
  user?: string; // owning account; absent on legacy scans
  name: string;
  type: string;
  fingerprint: string;
  source: string; // "authorized_keys" or "private_key"
  managed?: boolean;
  managed_name?: string;
};

export type CoolifyServer = {
  uuid: string;
  name: string;
  description: string;
  ip: string;
  user: string;
  port: number;
  is_reachable: boolean;
  is_usable: boolean;
};

export type ProcessDetail = {
  pid: string;
  user: string;
  cpu: string;
  mem: string;
  command: string;
  cwd: string;
  started_via: string;
  venv?: string;
  ports?: string;
};

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }

  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data?: unknown) =>
    request<T>(path, { method: "POST", body: data ? JSON.stringify(data) : undefined }),
  put: <T>(path: string, data: unknown) =>
    request<T>(path, { method: "PUT", body: JSON.stringify(data) }),
  patch: <T>(path: string, data: unknown) =>
    request<T>(path, { method: "PATCH", body: JSON.stringify(data) }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};

// Auth
export const authAPI = {
  status: () => api.get<import("./types").AuthStatus>("/api/auth/status"),
  setup: (data: { username: string; password: string; display_name: string }) =>
    api.post("/api/auth/setup", data),
  login: (data: { username: string; password: string; provider?: string }) =>
    api.post<{ user: unknown; token: string }>("/api/auth/login", data),
  logout: () => api.post("/api/auth/logout"),
  me: () => api.get<import("./types").User>("/api/auth/me"),
};

// Pagination envelope
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

// Hosts
export const hostsAPI = {
  list: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return api.get<import("./types").Host[]>(`/api/hosts${qs}`);
  },
  listPaginated: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return api.get<PaginatedResponse<import("./types").Host>>(`/api/hosts${qs}`);
  },
  get: (slug: string) =>
    api.get<{
      host: import("./types").Host;
      tags: string[];
      orchestrator: import("./types").Orchestrator | null;
      dns_records: import("./types").DNSRecord[];
      services: import("./types").Service[];
      projects: import("./types").Project[];
      last_scan: { id: number; data: string; scanned_at: string } | null;
      responsaveis: import("./types").HostResponsavel[];
      chamados: import("./types").HostChamado[];
    }>(`/api/hosts/${slug}`),
  create: (data: Partial<import("./types").Host> & { password?: string; tags?: string[]; dns_ids?: number[]; service_ids?: number[]; project_ids?: number[] }) =>
    api.post<import("./types").Host>("/api/hosts", data),
  update: (slug: string, data: Partial<import("./types").Host> & { password?: string; tags?: string[]; dns_ids?: number[]; service_ids?: number[]; project_ids?: number[] }) =>
    api.put<import("./types").Host>(`/api/hosts/${slug}`, data),
  delete: (slug: string) => api.delete(`/api/hosts/${slug}`),
  getPassword: (slug: string) => api.get<{ password: string }>(`/api/hosts/${slug}/password`),
};

// Host Alerts (manual)
export const hostAlertsAPI = {
  list: (slug: string) =>
    api.get<import("./types").HostAlert[]>(`/api/hosts/${slug}/alerts`),
  create: (slug: string, data: { type: string; level: string; message: string; description?: string; source?: string }) =>
    api.post<import("./types").HostAlert>(`/api/hosts/${slug}/alerts`, data),
  update: (slug: string, id: number, data: { type: string; level: string; message: string; description?: string }) =>
    api.put<import("./types").HostAlert>(`/api/hosts/${slug}/alerts/${id}`, data),
  conclude: (slug: string, id: number) =>
    api.post(`/api/hosts/${slug}/alerts/${id}/conclude`, {}),
  delete: (slug: string, id: number) =>
    api.delete(`/api/hosts/${slug}/alerts/${id}`),
};

// Host Chamados
export const hostChamadosAPI = {
  list: (slug: string) =>
    api.get<import("./types").HostChamado[]>(`/api/hosts/${slug}/chamados`),
  create: (slug: string, data: { chamado_id: string; title: string; status: string; user_id: number; date: string }) =>
    api.post<import("./types").HostChamado>(`/api/hosts/${slug}/chamados`, data),
  update: (slug: string, id: number, data: { chamado_id: string; title: string; status: string; user_id: number; date: string }) =>
    api.put<import("./types").HostChamado>(`/api/hosts/${slug}/chamados/${id}`, data),
  delete: (slug: string, id: number) =>
    api.delete(`/api/hosts/${slug}/chamados/${id}`),
};

// DNS
export const dnsAPI = {
  list: () => api.get<import("./types").DNSRecord[]>("/api/dns"),
  get: (id: number) => api.get<{ dns_record: import("./types").DNSRecord; tags: string[]; host_ids: number[]; responsaveis: import("./types").EntityResponsavel[] }>(`/api/dns/${id}`),
  create: (data: Partial<import("./types").DNSRecord> & { tags?: string[]; host_ids?: number[]; responsaveis?: import("./types").EntityResponsavel[] }) =>
    api.post<import("./types").DNSRecord>("/api/dns", data),
  update: (id: number, data: Partial<import("./types").DNSRecord> & { tags?: string[]; host_ids?: number[]; responsaveis?: import("./types").EntityResponsavel[] }) =>
    api.put<import("./types").DNSRecord>(`/api/dns/${id}`, data),
  delete: (id: number) => api.delete(`/api/dns/${id}`),
};

// Bulk Import
export type ImportResult = {
  created: number;
  skipped: number;
  failed: number;
  errors?: { index: number; name: string; error: string }[];
};

export const importAPI = {
  hosts: (data: Record<string, unknown>[]) =>
    api.post<ImportResult>("/api/import/hosts", data),
  dns: (data: Record<string, unknown>[]) =>
    api.post<ImportResult>("/api/import/dns", data),
  auto: (payload: { type: string; data: Record<string, unknown>[] }) =>
    api.post<ImportResult>("/api/import", payload),
};

// Database Backup/Restore
export const backupAPI = {
  download: async () => {
    const res = await fetch(`${API_BASE}/api/backup`, { credentials: "include" });
    if (!res.ok) throw new Error("Backup failed");
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const disposition = res.headers.get("Content-Disposition") || "";
    const match = disposition.match(/filename="(.+?)"/);
    a.download = match ? match[1] : `sshcm_backup_${new Date().toISOString().slice(0, 10)}.sshcmbak`;
    a.click();
    URL.revokeObjectURL(url);
  },
  restore: async (file: File) => {
    const form = new FormData();
    form.append("backup", file);
    const res = await fetch(`${API_BASE}/api/restore`, {
      method: "POST",
      credentials: "include",
      body: form,
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || "Restore failed");
    }
    return res.json() as Promise<{
      status: string;
      message: string;
      source_dialect?: "sqlite" | "postgres";
      target_dialect?: "sqlite" | "postgres";
      cross_dialect?: boolean;
      schema_version?: number;
      row_count?: number;
    }>;
  },
};

// Alert Settings
export const alertSettingsAPI = {
  get: () => api.get<import("./types").AlertThresholds>("/api/settings/alerts"),
  update: (data: import("./types").AlertThresholds) =>
    api.put<import("./types").AlertThresholds>("/api/settings/alerts", data),
};

// Projects
export const projectsAPI = {
  list: () => api.get<import("./types").Project[]>("/api/projects"),
  get: (id: number) =>
    api.get<{
      project: import("./types").Project;
      tags: string[];
      responsaveis: import("./types").EntityResponsavel[];
      services: import("./types").Service[];
      host_ids: number[];
      dns_ids: number[];
    }>(`/api/projects/${id}`),
  create: (data: Partial<import("./types").Project> & { tags?: string[]; responsaveis?: import("./types").EntityResponsavel[] }) =>
    api.post<import("./types").Project>("/api/projects", data),
  update: (id: number, data: Partial<import("./types").Project> & { tags?: string[]; responsaveis?: import("./types").EntityResponsavel[] }) =>
    api.put<import("./types").Project>(`/api/projects/${id}`, data),
  delete: (id: number) => api.delete(`/api/projects/${id}`),
};

// Services
export const servicesAPI = {
  list: () => api.get<import("./types").Service[]>("/api/services"),
  get: (id: number) =>
    api.get<{
      service: import("./types").Service;
      tags: string[];
      host_ids: number[];
      dns_ids: number[];
      depends_on_ids: number[];
      dependent_ids: number[];
      credentials: import("./types").ServiceCredential[];
      responsaveis: import("./types").EntityResponsavel[];
    }>(`/api/services/${id}`),
  create: (data: Partial<import("./types").Service> & { tags?: string[]; host_ids?: number[]; dns_ids?: number[]; depends_on_ids?: number[]; responsaveis?: import("./types").EntityResponsavel[] }) =>
    api.post<import("./types").Service>("/api/services", data),
  update: (id: number, data: Partial<import("./types").Service> & { tags?: string[]; host_ids?: number[]; dns_ids?: number[]; depends_on_ids?: number[]; responsaveis?: import("./types").EntityResponsavel[] }) =>
    api.put<import("./types").Service>(`/api/services/${id}`, data),
  delete: (id: number) => api.delete(`/api/services/${id}`),
  createCredential: (serviceId: number, data: { role_name: string; credentials: string }) =>
    api.post(`/api/services/${serviceId}/credentials`, data),
  getCredential: (serviceId: number, credId: number) =>
    api.get<import("./types").ServiceCredential>(`/api/services/${serviceId}/credentials/${credId}`),
  deleteCredential: (serviceId: number, credId: number) =>
    api.delete(`/api/services/${serviceId}/credentials/${credId}`),
  fixate: (id: number) =>
    api.post<import("./types").Service>(`/api/services/${id}/fixate`),
  updateContainer: (id: number, data: { container_name: string; container_id: string }) =>
    api.put<import("./types").Service>(`/api/services/${id}/container`, data),
};

// Orchestrators
export const orchestratorsAPI = {
  list: () => api.get<import("./types").Orchestrator[]>("/api/orchestrators"),
  create: (data: Partial<import("./types").Orchestrator>) =>
    api.post<import("./types").Orchestrator>("/api/orchestrators", data),
  update: (id: number, data: Partial<import("./types").Orchestrator>) =>
    api.put<import("./types").Orchestrator>(`/api/orchestrators/${id}`, data),
  delete: (id: number) => api.delete(`/api/orchestrators/${id}`),
};

// SSH
export const sshAPI = {
  previewConfig: () => api.get<{ content: string }>("/api/ssh/preview-config"),
  generateConfig: () => api.post<{ status: string; host_count: number; path: string }>("/api/ssh/generate-config"),
  testConnection: (slug: string, method: "password" | "key", capture = false) =>
    api.post<{ success: boolean; error?: string; vm_info?: VMInfoType }>(`/api/ssh/test/${slug}`, { method, capture }),
  fixDevNull: (slug: string, method: "password" | "key") =>
    api.post<{ success: boolean; method?: string; message?: string; output?: string; error?: string }>(`/api/ssh/fix-dev-null/${slug}`, { method }),
  setupSudoNopasswd: (slug: string) =>
    api.post<{ success: boolean; message?: string; output?: string; error?: string }>(`/api/ssh/setup-sudo-nopasswd/${slug}`),
  createRemoteUser: (slug: string, username: string, pubKey: string, force = false, sshKeyId?: number) =>
    api.post<{ success: boolean; message?: string; output?: string; error?: string; user_exists?: boolean }>(`/api/ssh/create-remote-user/${slug}`, { username, pub_key: pubKey, force, ssh_key_id: sshKeyId }),
  deleteRemoteUser: (slug: string, username: string, removeHome = false) =>
    api.post<{ success: boolean; message?: string; output?: string; error?: string; user_missing?: boolean; user_protected?: boolean }>(`/api/ssh/delete-remote-user/${slug}`, { username, remove_home: removeHome }),
  setupKey: (slug: string, data: {
    user?: string; password?: string; use_saved_password?: boolean;
    mode: "generate" | "existing"; existing_key_path?: string;
  }) => api.post<{ success: boolean; generated: boolean; public_key: string }>(`/api/ssh/setup-key/${slug}`, data),
  listKeys: () => api.get<{ PrivatePath: string }[]>("/api/ssh/keys"),
  downloadConfigURL: () => `${API_BASE}/api/ssh/download-config`,
  serverInfo: () => api.get<{
    hostname: string;
    user: string;
    home: string;
    config_path: string;
    is_local: boolean;
    message: string;
  }>("/api/ssh/server-info"),
  operationLogs: (slug: string, limit = 50) =>
    api.get<OperationLog[]>(`/api/ssh/operation-logs/${slug}?limit=${limit}`),
  listRemoteKeys: (slug: string) =>
    api.post<{ success: boolean; error?: string; keys?: RemoteKeyInfo[] }>(`/api/ssh/list-remote-keys/${slug}`),
  hostConfig: (slug: string, includeKey: boolean) =>
    api.get<{ config: string }>(`/api/ssh/host-config/${slug}?include_key=${includeKey}`),
  dockerSetup: (slug: string, fix: boolean) =>
    api.post<{ success: boolean; error?: string; status?: DockerStatusType }>(`/api/ssh/docker-setup/${slug}`, { fix }),
  nginxCleanup: (slug: string, purge: boolean) =>
    api.post<{ success: boolean; error?: string; status?: NginxCleanupStatusType }>(`/api/ssh/nginx-cleanup/${slug}`, { purge }),
  grafanaAgentSetup: (slug: string) =>
    api.post<{ success: boolean; error?: string; output?: string; message?: string }>(`/api/ssh/grafana-agent-setup/${slug}`),
};

// SSH Keys
export const sshKeysAPI = {
  list: async () => {
    const data = await api.get<unknown>("/api/ssh-keys");
    return Array.isArray(data) ? (data as import("./types").SSHKeyRecord[]) : [];
  },
  get: (id: number) => api.get<{
    id: number;
    name: string;
    credential_type: string;
    username?: string;
    description?: string;
    public_key?: string;
    private_key?: string;
    password?: string;
    fingerprint: string;
    created_at: string;
  }>(`/api/ssh-keys/${id}`),
  create: (data: { name: string; credential_type?: string; username?: string; description?: string; public_key?: string; private_key?: string; password?: string }) =>
    api.post<{ id: number; name: string; fingerprint: string; created_at: string }>("/api/ssh-keys", data),
  update: (id: number, data: { name?: string; credential_type?: string; username?: string; description?: string; public_key?: string; private_key?: string; password?: string }) =>
    api.put<{ id: number; name: string; fingerprint: string; created_at: string }>(`/api/ssh-keys/${id}`, data),
  delete: (id: number) => api.delete(`/api/ssh-keys/${id}`),
};

// Graph & Dashboard
export const graphAPI = {
  get: async () => {
    const data = await api.get<Partial<import("./types").GraphData> | null>("/api/graph");
    return {
      nodes: Array.isArray(data?.nodes) ? data.nodes : [],
      edges: Array.isArray(data?.edges) ? data.edges : [],
    } satisfies import("./types").GraphData;
  },
};

export const dashboardAPI = {
  get: () => api.get<import("./types").DashboardStats>("/api/dashboard"),
};

// Enums
export const enumsAPI = {
  list: (category: string) => api.get<import("./types").EnumOption[]>(`/api/enums/${category}`),
  listAll: () => api.get<Record<string, import("./types").EnumOption[]>>("/api/enums"),
  create: (category: string, value: string, color?: string) => api.post(`/api/enums/${category}`, { value, color: color || "" }),
  update: (category: string, oldValue: string, newValue: string, color?: string) =>
    api.put(`/api/enums/${category}/${encodeURIComponent(oldValue)}`, { value: newValue, color: color || "" }),
  delete: (category: string, value: string) => api.delete(`/api/enums/${category}/${encodeURIComponent(value)}`),
};

// Tags
export const tagsAPI = {
  list: async (type?: string) => {
    const qs = type ? `?type=${type}` : "";
    const data = await api.get<unknown>(`/api/tags${qs}`);
    return Array.isArray(data) ? data : [];
  },
};

// Contacts
export const contactsAPI = {
  list: async () => {
    const data = await api.get<unknown>("/api/contacts");
    return Array.isArray(data) ? (data as import("./types").Contact[]) : [];
  },
  create: (data: { name: string; phone?: string; role?: string; entity?: string }) =>
    api.post<import("./types").Contact>("/api/contacts", data),
  update: (id: number, data: { name: string; phone?: string; role?: string; entity?: string }) =>
    api.put<import("./types").Contact>(`/api/contacts/${id}`, data),
  delete: (id: number) => api.delete(`/api/contacts/${id}`),
};

// Issues
export const issuesAPI = {
  listByProject: async (projectId: number, serviceId?: number) => {
    const qs = serviceId ? `?service_id=${serviceId}` : "";
    const data = await api.get<unknown>(`/api/projects/${projectId}/issues${qs}`);
    return Array.isArray(data) ? (data as import("./types").Issue[]) : [];
  },
  listByService: async (serviceId: number) => {
    const data = await api.get<unknown>(`/api/services/${serviceId}/issues`);
    return Array.isArray(data) ? (data as import("./types").Issue[]) : [];
  },
  create: (projectId: number, data: Partial<import("./types").Issue>) =>
    api.post<import("./types").Issue>(`/api/projects/${projectId}/issues`, data),
  update: (projectId: number, issueId: number, data: Partial<import("./types").Issue>) =>
    api.put<import("./types").Issue>(`/api/projects/${projectId}/issues/${issueId}`, data),
  move: (projectId: number, issueId: number, status: string, position: number) =>
    api.patch<{ status: string }>(`/api/projects/${projectId}/issues/${issueId}/move`, { status, position }),
  delete: (projectId: number, issueId: number) =>
    api.delete(`/api/projects/${projectId}/issues/${issueId}`),
};

// Global Issues
export const globalIssuesAPI = {
  list: async (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    const data = await api.get<unknown>(`/api/issues${qs}`);
    return Array.isArray(data) ? (data as import("./types").Issue[]) : [];
  },
  create: (data: Partial<import("./types").Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) =>
    api.post<import("./types").Issue>("/api/issues", data),
  update: (id: number, data: Partial<import("./types").Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) =>
    api.put<import("./types").Issue>(`/api/issues/${id}`, data),
  move: (id: number, status: string, position: number) =>
    api.patch<{ status: string }>(`/api/issues/${id}/move`, { status, position }),
  archive: (id: number) =>
    api.patch<import("./types").Issue>(`/api/issues/${id}/archive`, {}),
  delete: (id: number) => api.delete(`/api/issues/${id}`),
};

// Releases
export const releasesAPI = {
  list: () => api.get<(import("./types").Release & { issue_ids: number[] })[]>("/api/releases"),
  get: (id: number) => api.get<{ release: import("./types").Release; issue_ids: number[] }>(`/api/releases/${id}`),
  create: (data: Partial<import("./types").Release> & { issue_ids?: number[] }) =>
    api.post<import("./types").Release>("/api/releases", data),
  update: (id: number, data: Partial<import("./types").Release> & { issue_ids?: number[] }) =>
    api.put<import("./types").Release>(`/api/releases/${id}`, data),
  delete: (id: number) => api.delete(`/api/releases/${id}`),
};

// External tools
export const toolsAPI = {
  list: async () => {
    const data = await api.get<unknown>("/api/tools");
    return Array.isArray(data) ? (data as import("./types").ExternalTool[]) : [];
  },
  get: (id: number) => api.get<import("./types").ExternalTool>(`/api/tools/${id}`),
  create: (data: Partial<import("./types").ExternalTool>) =>
    api.post<import("./types").ExternalTool>("/api/tools", data),
  update: (id: number, data: Partial<import("./types").ExternalTool>) =>
    api.put<import("./types").ExternalTool>(`/api/tools/${id}`, data),
  delete: (id: number) => api.delete(`/api/tools/${id}`),
  syncFromService: (data: { service_id: number; dns_id: number; embed_enabled?: boolean; icon?: string; sort_order?: number }) =>
    api.post<import("./types").ExternalTool>("/api/tools/sync-service", data),
  unsyncService: (id: number) => api.delete(`/api/tools/sync-service/${id}`),
  listToolCredentials: (toolId: number) =>
    api.get<import("./types").ServiceCredential[]>(`/api/tools/${toolId}/credentials`),
  getToolCredential: (toolId: number, credId: number) =>
    api.get<import("./types").ServiceCredential>(`/api/tools/${toolId}/credentials/${credId}`),
};

// Service credentials (all services)
export const serviceCredentialsAPI = {
  listAll: () => api.get<import("./types").ServiceWithCredentials[]>("/api/services/credentials/all"),
};

// Appearance settings
export const appearanceAPI = {
  get: () => api.get<{ app_name: string; app_color: string; app_logo: string }>("/api/settings/appearance"),
  update: (data: { app_name: string; app_color: string; app_logo: string }) =>
    api.put<{ app_name: string; app_color: string; app_logo: string }>("/api/settings/appearance", data),
  uploadLogo: async (file: File): Promise<{ logo: string }> => {
    const form = new FormData();
    form.append("logo", file);
    const res = await fetch(`${API_BASE}/api/settings/appearance/logo`, {
      method: "POST",
      credentials: "include",
      body: form,
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `Upload failed: ${res.status}`);
    }
    return res.json();
  },
  deleteLogo: () => api.delete<{ status: string }>("/api/settings/appearance/logo"),
};

// Integration settings (admin)
export const integrationsAPI = {
  get: () => api.get<Record<string, Record<string, string>>>("/api/settings/integrations"),
  update: (group: string, data: Record<string, string>) =>
    api.put<{ status: string }>(`/api/settings/integrations/${group}`, data),
  testLDAP: () => api.post<{ success: boolean; error?: string }>("/api/settings/integrations/test/ldap"),
  testGitLabCode: (data?: { base_url?: string; token?: string }) =>
    api.post<{ success: boolean; error?: string; username?: string; name?: string }>(
      "/api/settings/integrations/test/gitlab-code",
      data ?? {}
    ),
  testLLM: (data?: { base_url?: string; api_key?: string; model?: string }) =>
    api.post<{
      success: boolean;
      error?: string;
      stage?: "models" | "chat";
      warning?: string;
      models_count?: number;
      model?: string;
      model_available?: boolean;
      chat_ok?: boolean;
      chat_reply?: string;
    }>("/api/settings/integrations/test/llm", data ?? {}),
  testGrafana: (data?: { base_url?: string; token?: string }) =>
    api.post<{
      success: boolean;
      error?: string;
      stage?: "health" | "auth";
      version?: string;
      database?: string;
      user?: string;
      name?: string;
      org_id?: number;
    }>("/api/settings/integrations/test/grafana", data ?? {}),
  testOutline: (data?: { base_url?: string; token?: string }) =>
    api.post<{
      success: boolean;
      error?: string;
      user?: string;
      user_email?: string;
      workspace?: string;
      workspace_url?: string;
    }>("/api/settings/integrations/test/outline", data ?? {}),
  clearSecret: (group: string, key: string) =>
    api.delete<{ status: string }>(`/api/settings/integrations/${group}/secret/${key}`),
};

// GitLab Code Management — per-project links + aggregated commits (uses shared service PAT)
export type ProjectGitLabLink = {
  id: number;
  project_id: number;
  gitlab_project_id: number;
  gitlab_base_url: string;
  gitlab_path: string;
  kind: "project" | "group";
  ref_name: string;
  display_name: string;
  sync_issues: boolean;
  last_synced_at: string | null;
  created_at: string;
  reachable?: boolean;        // set by the list endpoint after verifying against GitLab
  health_error?: string;
};

export type ProjectGitLabLinksResponse = {
  enabled: boolean;
  configured: boolean;
  links: ProjectGitLabLink[];
};

export type ProjectGitLabCommit = {
  id: string;
  short_id: string;
  title: string;
  message: string;
  author_name: string;
  author_email: string;
  committed_date: string;
  web_url: string;
  source_project_id: number;
  source_project_name: string;
  source_project_path: string;
  branches?: string[];
};

export type ProjectGitLabCommitsResponse = {
  enabled: boolean;
  configured: boolean;
  commits: ProjectGitLabCommit[];
  warnings?: string[];
  error?: string;
};

export const projectGitlabAPI = {
  listLinks: (projectId: number) =>
    api.get<ProjectGitLabLinksResponse>(`/api/projects/${projectId}/gitlab/links`),
  addLink: (projectId: number, data: { kind: "project" | "group"; path: string; ref_name?: string }) =>
    api.post<ProjectGitLabLink>(`/api/projects/${projectId}/gitlab/links`, data),
  deleteLink: (projectId: number, linkId: number) =>
    api.delete(`/api/projects/${projectId}/gitlab/links/${linkId}`),
  listCommits: (projectId: number) =>
    api.get<ProjectGitLabCommitsResponse>(`/api/projects/${projectId}/gitlab/commits`),
};

// Permissions management (admin)
export const permissionsAPI = {
  get: () =>
    api.get<{
      permissions: { code: string; description: string; category: string }[];
      matrix: Record<string, string[]>;
    }>("/api/settings/permissions"),
  update: (role: string, permissions: string[]) =>
    api.put<{ status: string }>("/api/settings/permissions", { role, permissions }),
};

// Role mappings (admin)
export const roleMappingsAPI = {
  list: () =>
    api.get<{ id: number; provider_name: string; external_group: string; local_role: string }[]>(
      "/api/settings/role-mappings"
    ),
  create: (data: { provider_name: string; external_group: string; local_role: string }) =>
    api.post<{ id: number; provider_name: string; external_group: string; local_role: string }>(
      "/api/settings/role-mappings",
      data
    ),
  delete: (id: number) => api.delete(`/api/settings/role-mappings/${id}`),
};

// AI / LLM integration
export const aiAPI = {
  status: () => api.get<{ enabled: boolean; configured: boolean; model: string }>("/api/ai/status"),
  assistIssue: (summary: string, context?: string) =>
    api.post<{ description: string }>("/api/ai/assist/issue", { summary, context }),
  assistHostDoc: (hostSlug: string) =>
    api.post<{ documentation: string }>("/api/ai/assist/host-doc", { host_slug: hostSlug }),
  chat: (message: string) =>
    api.post<{ response: string }>("/api/ai/chat", { message }),
  analyzeProject: (projectId: number, locale: string) =>
    api.post<ProjectAIAnalysisRecord>(`/api/projects/${projectId}/ai/analyze`, { locale }),
  getProjectAnalysis: (projectId: number) =>
    api.get<ProjectAIAnalysisRecord | null>(`/api/projects/${projectId}/ai/analyze`),
};

export type ProjectAIAnalysisRecord = {
  project_id: number;
  content: string;
  locale: string;
  commits_used: number;
  repos_used: number;
  generated_at: string;
};

// GitLab integration
export const gitlabAPI = {
  status: () => api.get<{ connected: boolean; username?: string; name?: string; error?: string }>("/api/gitlab/status"),
  saveToken: (data: { token: string; base_url?: string }) =>
    api.post<{ status: string; username: string }>("/api/gitlab/token", data),
  deleteToken: () => api.delete("/api/gitlab/token"),
  listCommits: (projectId: number) =>
    api.get<{ id: string; short_id: string; title: string; author_name: string; committed_date: string; web_url: string }[]>(`/api/gitlab/projects/${projectId}/commits`),
  listIssues: (projectId: number, state?: string) =>
    api.get<{ iid: number; title: string; state: string; web_url: string; labels: string[]; author: { username: string } }[]>(
      `/api/gitlab/projects/${projectId}/issues${state ? `?state=${state}` : ""}`
    ),
  linkProject: (projectId: number, gitlabPath: string) =>
    api.post(`/api/gitlab/projects/${projectId}/link`, { gitlab_path: gitlabPath }),
};

// GLPI integration
export type GlpiTokenProfile = {
  id: number;
  name: string;
  description: string;
  has_token: boolean;
  default_entity_id: number;
  created_at: string;
  updated_at: string;
};

export type GlpiTicketSummary = {
  id: number;
  name: string;
  status: number;
  status_label: string;
  status_slug: "new" | "assigned" | "planned" | "waiting" | "solved" | "closed" | "unknown";
  priority: number;
  entities_id: number;
  date?: string;
  url: string;
};

export type GlpiTicketEvent = {
  type: "followup" | "task" | "solution";
  id: number;
  content: string;
  date: string;
  user_id: number;
  user_name?: string;
  is_private?: boolean;
  state?: number;   // tasks: 0=info 1=todo 2=done
  status?: number;  // solutions: 1=proposed 2=accepted 3=refused
};

export type GlpiTicketDetails = {
  ticket: GlpiTicketSummary & { content?: string; date_mod?: string };
  glpi_base_url: string;
  requester: { id: number; name: string };
  events: GlpiTicketEvent[];
  event_counts: { followup: number; task: number; solution: number };
  warnings?: string[];
};

// ─── Formcreator types ──────────────────────────────────────────────────────
export type FormcreatorForm = {
  id: number;
  name: string;
  description?: string;
  content?: string;
  is_active?: number;
  language?: string;
  entities_id?: number;
  plugin_formcreator_categories_id?: number;
  access_rights?: number;
  icon?: string;
  icon_color?: string;
  background_color?: string;
};

export type FormcreatorSection = {
  id: number;
  name: string;
  order: number;
  plugin_formcreator_forms_id: number;
};

export type FormcreatorQuestion = {
  id: number;
  name: string;
  fieldtype: string;
  required: number;
  description?: string;
  default_values?: string;
  values?: string;
  order: number;
  row?: number;
  col?: number;
  width?: number;
  plugin_formcreator_sections_id: number;
  regex?: string;
};

export type FormcreatorCondition = {
  id: number;
  itemtype: "PluginFormcreatorQuestion" | "PluginFormcreatorSection";
  items_id: number;
  plugin_formcreator_questions_id: number;
  show_logic: "AND" | "OR" | string;
  show_condition: "eq" | "neq" | "lt" | "le" | "gt" | "ge" | "regex" | string;
  show_value: string;
  order: number;
};

export type FormcreatorBundle = {
  form: FormcreatorForm;
  sections: FormcreatorSection[];
  questions: FormcreatorQuestion[];
  conditions: FormcreatorCondition[];
  glpi_base_url: string;
  warnings?: string[];
};

export type FormcreatorSubmitResult = {
  form_answer_id: number;
  status: number;
  url?: string;
  created_tickets?: { id: number; url: string }[];
  created_counts?: Record<string, number>;
};

// ── Dropdown catalogue (admin-curated option lists) ──
export type GlpiCatalogueOption = {
  id: number;
  name: string;
  completename?: string;
  parent_id?: number;
};

export type GlpiDropdownCatalogueSummary = {
  itemtype: string;
  option_count: number;
  updated_at: string;
  updated_by?: number | null;
};

export type GlpiDropdownCatalogue = {
  id?: number;
  itemtype: string;
  options: GlpiCatalogueOption[];
  option_count: number;
  updated_at?: string;
  updated_by?: number | null;
};

export const glpiAPI = {
  // Admin profile CRUD
  listProfiles: () => api.get<GlpiTokenProfile[]>("/api/settings/integrations/glpi/tokens"),
  createProfile: (data: { name: string; description?: string; user_token: string; default_entity_id?: number }) =>
    api.post<GlpiTokenProfile>("/api/settings/integrations/glpi/tokens", data),
  updateProfile: (id: number, data: { name: string; description?: string; user_token?: string; default_entity_id?: number }) =>
    api.put<{ status: string }>(`/api/settings/integrations/glpi/tokens/${id}`, data),
  deleteProfile: (id: number) =>
    api.delete<{ status: string }>(`/api/settings/integrations/glpi/tokens/${id}`),
  testProfile: (id: number) =>
    api.post<{ success: boolean; error?: string; profiles?: string[] }>(`/api/settings/integrations/glpi/tokens/${id}/test`),

  // Ticket ops
  createTicket: (data: {
    profile_id: number;
    title: string;
    description?: string;
    entity_id?: number;
    category_id?: number;
    host_slug?: string;
    alert_id?: number;
    link_computer?: boolean;
  }) =>
    api.post<{
      ticket_id: number;
      ticket_url: string;
      chamado_id?: number;
      computer_linked?: boolean;
      warning?: string;
    }>("/api/glpi/tickets", data),
  getTicket: (ticketID: number, profileID: number) =>
    api.get<GlpiTicketSummary>(`/api/glpi/tickets/${ticketID}?profile_id=${profileID}`),
  ticketDetails: (ticketID: number, profileID: number) =>
    api.get<GlpiTicketDetails>(
      `/api/glpi/tickets/${ticketID}/details?profile_id=${profileID}`
    ),
  listForms: (profileID: number, q?: string) => {
    const params = new URLSearchParams({ profile_id: String(profileID) });
    if (q) params.set("q", q);
    return api.get<{ forms: FormcreatorForm[]; count: number }>(
      `/api/glpi/forms?${params.toString()}`
    );
  },
  getFormBundle: (formID: number, profileID: number) =>
    api.get<FormcreatorBundle>(`/api/glpi/forms/${formID}?profile_id=${profileID}`),
  submitForm: (formID: number, profileID: number, answers: Record<string, unknown>) =>
    api.post<FormcreatorSubmitResult>(
      `/api/glpi/forms/${formID}/submit?profile_id=${profileID}`,
      { answers }
    ),
  searchDropdown: (itemtype: string, profileID: number, q: string) => {
    const params = new URLSearchParams({ profile_id: String(profileID) });
    if (q) params.set("q", q);
    return api.get<{
      items: { id: number; name: string; completename?: string }[];
      count: number;
      source?: "catalogue" | "rest";
    }>(`/api/glpi/dropdowns/${encodeURIComponent(itemtype)}/search?${params.toString()}`);
  },
  // ── Admin: dropdown catalogues (manually-mapped option lists) ──
  listDropdownCatalogues: () =>
    api.get<{
      catalogues: GlpiDropdownCatalogueSummary[];
      allowed_itemtypes: string[];
    }>("/api/settings/integrations/glpi/dropdowns"),
  getDropdownCatalogue: (itemtype: string) =>
    api.get<GlpiDropdownCatalogue>(
      `/api/settings/integrations/glpi/dropdowns/${encodeURIComponent(itemtype)}`
    ),
  upsertDropdownCatalogue: (itemtype: string, options: GlpiCatalogueOption[]) =>
    api.put<{ itemtype: string; option_count: number }>(
      `/api/settings/integrations/glpi/dropdowns/${encodeURIComponent(itemtype)}`,
      { options }
    ),
  deleteDropdownCatalogue: (itemtype: string) =>
    api.delete<{ status: string }>(
      `/api/settings/integrations/glpi/dropdowns/${encodeURIComponent(itemtype)}`
    ),
  searchFormcreatorTags: (profileID: number, q: string) => {
    const params = new URLSearchParams({ profile_id: String(profileID) });
    if (q) params.set("q", q);
    return api.get<{
      tags: { id: number; name: string; color?: string }[];
      count: number;
    }>(`/api/glpi/formcreator/tags/search?${params.toString()}`);
  },
  searchUsers: (profileID: number, q: string) => {
    const params = new URLSearchParams({ profile_id: String(profileID) });
    if (q) params.set("q", q);
    return api.get<{
      users: { id: number; login: string; display: string; email?: string }[];
      count: number;
    }>(`/api/glpi/users/search?${params.toString()}`);
  },
  uploadFormFile: async (profileID: number, file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(
      `${API_BASE}/api/glpi/forms/uploads?profile_id=${profileID}`,
      { method: "POST", credentials: "include", body: form }
    );
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `Upload failed: ${res.status}`);
    }
    return res.json() as Promise<{ id: number; filename: string; mime?: string; size: number }>;
  },
  projectTickets: (projectID: number) =>
    api.get<{ tickets: GlpiTicketSummary[]; warning?: string }>(`/api/projects/${projectID}/glpi/tickets`),
  profileTickets: (profileID: number, opts?: { includeClosed?: boolean; range?: string }) => {
    const params = new URLSearchParams();
    if (opts?.includeClosed) params.set("status", "all");
    if (opts?.range) params.set("range", opts.range);
    const qs = params.toString();
    return api.get<{ tickets: GlpiTicketSummary[]; count: number; range: string }>(
      `/api/glpi/profiles/${profileID}/tickets${qs ? `?${qs}` : ""}`
    );
  },
  hostTickets: (slug: string, profileID: number) =>
    api.get<{ tickets: GlpiTicketSummary[]; computer?: { id: number; name: string } | null; warning?: string }>(
      `/api/hosts/${encodeURIComponent(slug)}/glpi/tickets?profile_id=${profileID}`
    ),
  refreshChamadoCache: (slug: string, chamadoID: number, profileID: number) =>
    api.post<GlpiTicketSummary>(`/api/hosts/${encodeURIComponent(slug)}/chamados/${chamadoID}/glpi/refresh?profile_id=${profileID}`),
};

// Outline (wiki) integration
export type OutlineDocumentSummary = {
  id: string;
  url_id: string;
  title: string;
  emoji?: string;
  excerpt: string;
  updated_at: string;
  updated_by?: string;
  browse_url: string;
};

export type OutlineCollectionSummary = {
  id: string;
  urlId: string;
  name: string;
  description: string;
  color: string;
  icon?: string;
};

export type OutlineWikiEnvelope = {
  enabled: boolean;
  configured: boolean;
  collection: OutlineCollectionSummary | null;
  collection_browse_url?: string;
  documents: OutlineDocumentSummary[];
  warning?: string;
};

// One configured common collection + its recent documents. Failures are soft —
// `collection` is null and `warning` carries the reason.
export type OutlineCommonWikiSection = {
  collection_id: string;
  collection: OutlineCollectionSummary | null;
  collection_browse_url?: string;
  documents: OutlineDocumentSummary[];
  warning?: string;
};

export type OutlineCommonWikiEnvelope = {
  enabled: boolean;
  configured: boolean;
  sections: OutlineCommonWikiSection[];
  warning?: string;
};

export type OutlineSearchHit = {
  context: string;
  id: string;
  url_id: string;
  title: string;
  collection_id: string;
  updated_at: string;
  browse_url: string;
};

// Workspace collection picker row (Settings multi-select).
export type OutlineWorkspaceCollection = {
  id: string;
  url_id: string;
  name: string;
  description?: string;
  color?: string;
  icon?: string;
};

// Recursive tree node for the /wiki left nav.
export type OutlineDocumentNode = {
  id: string;
  title: string;
  url?: string;
  emoji?: string;
  icon?: string;
  color?: string;
  children?: OutlineDocumentNode[];
};

export type OutlineCommonWikiTreeSection = {
  collection_id: string;
  collection: OutlineCollectionSummary | null;
  collection_browse_url?: string;
  nodes: OutlineDocumentNode[];
  warning?: string;
};

export type OutlineCommonWikiTreeEnvelope = {
  enabled: boolean;
  configured: boolean;
  base_url?: string;
  sections: OutlineCommonWikiTreeSection[];
  warning?: string;
};

// Single-doc fetch used by the in-app viewer.
export type OutlineFullDocument = {
  id: string;
  url_id: string;
  title: string;
  emoji?: string;
  text: string;
  collection_id: string;
  updated_at: string;
  updated_by?: string;
  browse_url: string;
};

export const outlineAPI = {
  projectWiki: (projectId: number) =>
    api.get<OutlineWikiEnvelope>(`/api/projects/${projectId}/wiki`),
  createProjectDocument: (projectId: number, title: string) =>
    api.post<{ id: string; url_id: string; title: string; browse_url: string }>(
      `/api/projects/${projectId}/wiki/documents`,
      { title }
    ),
  searchProjectWiki: (projectId: number, query: string) =>
    api.get<{ results: OutlineSearchHit[] }>(
      `/api/projects/${projectId}/wiki/search?q=${encodeURIComponent(query)}`
    ),
  commonWiki: () => api.get<OutlineCommonWikiEnvelope>("/api/wiki/documents"),
  createCommonDocument: (title: string, collectionID?: string) =>
    api.post<{ id: string; url_id: string; title: string; browse_url: string; collection_id: string }>(
      "/api/wiki/documents",
      collectionID ? { title, collection_id: collectionID } : { title }
    ),
  searchCommonWiki: (query: string) =>
    api.get<{ results: OutlineSearchHit[] }>(`/api/wiki/search?q=${encodeURIComponent(query)}`),
  listWorkspaceCollections: () =>
    api.get<{ collections: OutlineWorkspaceCollection[] }>("/api/wiki/collections"),
  commonWikiTree: () => api.get<OutlineCommonWikiTreeEnvelope>("/api/wiki/tree"),
  getDocument: (id: string) =>
    api.get<OutlineFullDocument>(`/api/wiki/documents/${encodeURIComponent(id)}`),
};

// Grafana integration
export type HostLiveMetrics = {
  enabled: boolean;
  configured: boolean;
  host_up: boolean | null;
  cpu_pct: number | null;
  ram_pct: number | null;
  disk_pct: number | null;
  load_1m: number | null;
  uptime_seconds: number | null;
  fetched_at: string;
  warnings?: string[];
};

export const grafanaAPI = {
  embedURL: (entity: "host" | "service", id: string | number) =>
    api.get<{
      configured: boolean;
      url?: string;
      dashboard_uid?: string;
      variable?: string;
      value?: string;
    }>(`/api/grafana/embed-url?entity=${entity}&id=${encodeURIComponent(String(id))}`),
  hostLiveMetrics: (slug: string) =>
    api.get<HostLiveMetrics>(`/api/hosts/${encodeURIComponent(slug)}/metrics/live`),
  provisionHostDashboard: (slug: string) =>
    api.post<{ uid: string; message: string }>(`/api/hosts/${encodeURIComponent(slug)}/grafana/provision`),
  provisionServiceDashboard: (serviceId: number) =>
    api.post<{ uid: string; message: string }>(`/api/services/${serviceId}/grafana/provision`),
};

// Coolify integration
export const coolifyAPI = {
  status: () => api.get<{ enabled: boolean; configured: boolean }>("/api/coolify/status"),
  testConnection: () => api.post<{ success: boolean; error?: string }>("/api/coolify/test"),
  getServerStatus: (slug: string) =>
    api.get<{ server: CoolifyServer }>(`/api/coolify/server-status/${slug}`),
  checkHost: (slug: string) =>
    api.post<{ found: boolean; server?: CoolifyServer }>(`/api/coolify/check/${slug}`),
  registerHost: (slug: string, sshKeyId?: number) =>
    api.post<{ uuid: string }>(`/api/coolify/register/${slug}`, sshKeyId ? { ssh_key_id: sshKeyId } : {}),
  updateServerKey: (slug: string, sshKeyId: number) =>
    api.post<{ success: boolean; private_key_uuid: string }>(`/api/coolify/server/${slug}/key`, { ssh_key_id: sshKeyId }),
  validateHost: (slug: string) =>
    api.post<{ message: string }>(`/api/coolify/validate/${slug}`),
  syncHost: (slug: string) =>
    api.post<{ success: boolean }>(`/api/coolify/sync/${slug}`),
  deleteHost: (slug: string) =>
    api.delete(`/api/coolify/server/${slug}`),
  checkKey: (id: number) =>
    api.get<{ found: boolean; coolify_uuid?: string; coolify_name?: string }>(`/api/coolify/keys/${id}/check`),
  syncKey: (id: number) =>
    api.post<{ uuid: string; name: string; already_existed: boolean }>(`/api/coolify/keys/${id}/sync`),
};

// Users (admin)
export const usersAPI = {
  list: () => api.get<import("./types").User[]>("/api/users"),
  create: (data: { username: string; password: string; display_name: string; role: string }) =>
    api.post<import("./types").User>("/api/users", data),
  update: (id: number, data: Partial<import("./types").User> & { password?: string }) =>
    api.put<import("./types").User>(`/api/users/${id}`, data),
  delete: (id: number) => api.delete(`/api/users/${id}`),
};
