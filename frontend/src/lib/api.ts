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
