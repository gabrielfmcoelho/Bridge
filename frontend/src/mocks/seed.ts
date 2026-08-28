// Fixtures for the mock transport layer. Deterministic on purpose — no
// Math.random(), no Date.now() — so every server restart/reset produces the
// exact same data and a scoping bug is reproducible, not "worked on reload".
import type {
  Entidade,
  User,
  EnumOption,
  Host,
  Service,
  DNSRecord,
  Project,
  ApiCatalog,
  ExternalTool,
  Offering,
  ServiceRequest,
  RequestEvent,
} from "@/lib/types";
import type { Seed } from "./db";

// ── Entidades — verbatim from internal/spec/entidades.md:11 ────────────────
// GovPI → { ETIPI, SEAD-PI, Parceiros }, SEAD-PI → { NTGD, SGA, SGP, SGI },
// Parceiros → { Trin, Syslae, Vobys }. Parents before children, like the API.

function entidades(): Entidade[] {
  const at = "2025-09-01T09:00:00Z";
  const e = (id: number, name: string, slug: string, parent_id: number | null): Entidade => ({
    id, name, slug, parent_id, description: "", created_at: at, updated_at: at,
  });
  return [
    e(1, "GovPI", "govpi", null),
    e(2, "ETIPI", "etipi", 1),
    e(3, "SEAD-PI", "sead-pi", 1),
    e(4, "Parceiros", "parceiros", 1),
    e(5, "NTGD", "ntgd", 3),
    e(6, "SGA", "sga", 3),
    e(7, "SGP", "sgp", 3),
    e(8, "SGI", "sgi", 3),
    e(9, "Trin", "trin", 4),
    e(10, "Syslae", "syslae", 4),
    e(11, "Vobys", "vobys", 4),
  ];
}

// Entidade ids for readability below.
const GOVPI = 1, ETIPI = 2, NTGD = 5, SGA = 6, SGP = 7, SGI = 8, TRIN = 9;

// ── Users. id 1 is the acting persona (__user mutates its role/entidades in
// place); ids 2-9 are static historical requesters/approvers/commenters. ──

function users(): User[] {
  const mk = (
    id: number, display_name: string, username: string, role: User["role"],
    entidadeId: number, entidadeName: string, entidadeSlug: string,
  ): User => ({
    id, username, display_name, role,
    auth_provider: "local",
    email: `${username}@sead.pi.gov.br`,
    permissions: [],
    external_identities: [],
    entidades: [{ id: entidadeId, name: entidadeName, slug: entidadeSlug, is_primary: true }],
    created_at: "2025-09-15T09:00:00Z",
    updated_at: "2025-09-15T09:00:00Z",
  });
  return [
    mk(1, "Marcos Vinícius Rêgo Sousa", "marcos.sousa", "editor", SGA, "SGA", "sga"),
    mk(2, "Francisca das Chagas Lima Ferraz", "francisca.ferraz", "viewer", SGP, "SGP", "sgp"),
    mk(3, "Antônio Carlos Bezerra Melo", "antonio.melo", "editor", ETIPI, "ETIPI", "etipi"),
    mk(4, "Raimunda Nonata Ferreira Castro", "raimunda.castro", "editor", GOVPI, "GovPI", "govpi"),
    mk(5, "José Ribamar Coelho Aguiar", "jose.aguiar", "viewer", SGA, "SGA", "sga"),
    mk(6, "Luciana Maria Pereira Rocha", "luciana.rocha", "editor", TRIN, "Trin", "trin"),
    mk(7, "Paulo Sérgio Nunes Cavalcante", "paulo.cavalcante", "editor", NTGD, "NTGD", "ntgd"),
    mk(8, "Socorro Helena Dias Brito", "socorro.brito", "editor", SGI, "SGI", "sgi"),
    mk(9, "Wellington Régis Carvalho Nunes", "wellington.nunes", "admin", GOVPI, "GovPI", "govpi"),
  ];
}

function enums(): Record<string, EnumOption[]> {
  return {
    situacao: [
      { category: "situacao", value: "active", sort_order: 1, color: "#10b981" },
      { category: "situacao", value: "maintenance", sort_order: 2, color: "#f59e0b" },
      { category: "situacao", value: "inactive", sort_order: 3, color: "#6b7280" },
    ],
    issue_priority: [
      { category: "issue_priority", value: "low", sort_order: 1, color: "#6b7280" },
      { category: "issue_priority", value: "medium", sort_order: 2, color: "#3b82f6" },
      { category: "issue_priority", value: "high", sort_order: 3, color: "#f59e0b" },
      { category: "issue_priority", value: "critical", sort_order: 4, color: "#ef4444" },
    ],
  };
}

// ── Discovery fixtures (~20 rows: hosts, services, dns, projects, apis, tools) ─

function mkHost(o: Partial<Host> & Pick<Host, "id" | "nickname" | "oficial_slug" | "hostname" | "description">): Host {
  return {
    hospedagem: "on-premise",
    tipo_maquina: "vm",
    user: "deploy",
    has_password: false,
    has_key: true,
    key_path: "~/.ssh/id_ed25519",
    port: "22",
    identities_only: "yes",
    proxy_jump: "",
    forward_agent: "no",
    setor_responsavel: "ETIPI",
    responsavel_interno: "Antônio Carlos Bezerra Melo",
    contato_responsavel_interno: "antonio.melo@sead.pi.gov.br",
    acesso_empresa_externa: false,
    empresa_responsavel: "",
    responsavel_externo: "",
    contato_responsavel_externo: "",
    recurso_cpu: "4 vCPU",
    recurso_ram: "8 GB",
    recurso_armazenamento: "100 GB SSD",
    situacao: "active",
    precisa_manutencao: false,
    preferred_auth: "key",
    observacoes: "",
    created_at: "2025-11-04T10:00:00Z",
    updated_at: "2026-06-01T10:00:00Z",
    ...o,
  };
}

function hosts(): Host[] {
  return [
    mkHost({
      id: 1, nickname: "vm-postgres-02", oficial_slug: "vm-postgres-02",
      hostname: "vm-postgres-02.sead.pi.gov.br",
      description: "Servidor dedicado ao PostgreSQL de homologação.",
    }),
    mkHost({
      id: 2, nickname: "srv-sga-web01", oficial_slug: "srv-sga-web01",
      hostname: "srv-sga-web01.sead.pi.gov.br",
      description: "Servidor web do Portal do Servidor Estadual (produção).",
      setor_responsavel: "SGA", responsavel_interno: "Marcos Vinícius Rêgo Sousa",
      contato_responsavel_interno: "marcos.sousa@sead.pi.gov.br",
    }),
    mkHost({
      id: 3, nickname: "vm-etipi-mon01", oficial_slug: "vm-etipi-mon01",
      hostname: "mon01.etipi.pi.gov.br",
      description: "Servidor de monitoramento Grafana/Prometheus da ETIPI.",
    }),
    mkHost({
      id: 4, nickname: "srv-ntgd-backup01", oficial_slug: "srv-ntgd-backup01",
      hostname: "backup01.ntgd.pi.gov.br",
      description: "Servidor de backups incrementais do NTGD.",
      setor_responsavel: "NTGD", responsavel_interno: "Paulo Sérgio Nunes Cavalcante",
      contato_responsavel_interno: "paulo.cavalcante@sead.pi.gov.br",
      situacao: "maintenance", precisa_manutencao: true,
    }),
    mkHost({
      id: 5, nickname: "vm-sgi-chamados01", oficial_slug: "vm-sgi-chamados01",
      hostname: "chamados01.sgi.pi.gov.br",
      description: "Hospeda a API e o worker do sistema de chamados da SGI.",
      setor_responsavel: "SGI", responsavel_interno: "Socorro Helena Dias Brito",
      contato_responsavel_interno: "socorro.brito@sead.pi.gov.br",
    }),
  ];
}

function mkService(o: Partial<Service> & Pick<Service, "id" | "nickname" | "description" | "service_type" | "technology_stack" | "port">): Service {
  return {
    project_id: null,
    service_subtype: "",
    deploy_approach: "docker-compose",
    orchestrator_tool: "docker compose",
    environment: "production",
    version: "",
    orchestrator_managed: true,
    is_directly_managed: true,
    is_responsible: true,
    developed_by: "",
    is_external_dependency: false,
    external_provider: "",
    external_url: "",
    external_contact: "",
    repository_url: "",
    gitlab_url: "",
    documentation_url: "",
    source: "manual",
    discovery_kind: "",
    discovery_key: "",
    container_status: "online",
    container_id: "",
    container_name: "",
    container_image: "",
    container_ports: "",
    discovered_at: null,
    last_seen_at: null,
    created_at: "2025-11-10T10:00:00Z",
    updated_at: "2026-06-01T10:00:00Z",
    ...o,
  };
}

function services(): Service[] {
  return [
    mkService({
      id: 1, nickname: "pg-prod-01", project_id: 1,
      description: "Cluster PostgreSQL de produção da SEAD-PI, usado pelo Portal do Servidor.",
      service_type: "database", service_subtype: "relational", technology_stack: "PostgreSQL 16", port: "5432", version: "16.4",
    }),
    mkService({
      id: 2, nickname: "portal-servidor-web", project_id: 1,
      description: "Frontend do Portal do Servidor Estadual.",
      service_type: "web", technology_stack: "Next.js 15", port: "3000", version: "15.1.0",
    }),
    mkService({
      id: 3, nickname: "chamados-api", project_id: 2,
      description: "API de abertura e acompanhamento de chamados técnicos da SGI.",
      service_type: "api", technology_stack: "Go 1.23", port: "8080",
    }),
    mkService({
      id: 4, nickname: "redis-cache-etipi", project_id: 3,
      description: "Cache compartilhado dos serviços da ETIPI.",
      service_type: "cache", technology_stack: "Redis 7", port: "6379",
    }),
    mkService({
      id: 5, nickname: "keycloak-sso", project_id: 3,
      description: "Provedor de identidade único (SSO) do governo do Piauí.",
      service_type: "auth", technology_stack: "Keycloak 25", port: "8443",
    }),
  ];
}

function dns(): DNSRecord[] {
  const mk = (id: number, domain: string, has_https: boolean, situacao: string, responsavel: string): DNSRecord => ({
    id, domain, has_https, situacao, responsavel, observacoes: "",
    created_at: "2025-10-01T09:00:00Z", updated_at: "2026-05-01T09:00:00Z",
  });
  return [
    mk(1, "portal.sead.pi.gov.br", true, "active", "Antônio Carlos Bezerra Melo"),
    mk(2, "chamados.sead.pi.gov.br", true, "active", "Socorro Helena Dias Brito"),
    mk(3, "sso.pi.gov.br", true, "active", "Antônio Carlos Bezerra Melo"),
    mk(4, "homolog.sead.pi.gov.br", false, "maintenance", "Paulo Sérgio Nunes Cavalcante"),
  ];
}

function projects(): Project[] {
  const mk = (id: number, name: string, description: string, setor_responsavel: string, responsavel: string): Project => ({
    id, name, description, situacao: "active", setor_responsavel, responsavel,
    tem_empresa_externa_responsavel: false, contato_empresa_responsavel: "",
    is_directly_managed: true, is_responsible: true,
    gitlab_url: "", documentation_url: "",
    created_at: "2025-09-20T09:00:00Z", updated_at: "2026-05-01T09:00:00Z",
  });
  return [
    mk(1, "Portal do Servidor Estadual", "Portal unificado de serviços para servidores públicos do Piauí.", "SGA", "Marcos Vinícius Rêgo Sousa"),
    mk(2, "Modernização do Sistema de Chamados", "Reescrita do sistema de chamados técnicos da SGI.", "SGI", "Socorro Helena Dias Brito"),
    mk(3, "Migração de Infraestrutura para Contêineres", "Migração gradual dos serviços legados da ETIPI para orquestração em contêineres.", "ETIPI", "Antônio Carlos Bezerra Melo"),
  ];
}

function apiCatalogs(): ApiCatalog[] {
  const mk = (o: Partial<ApiCatalog> & Pick<ApiCatalog, "id" | "name" | "description" | "title">): ApiCatalog => ({
    scope: "avulso", parent_id: null, source_type: "upload", spec_version: "3.0.0", spec_hash: "",
    version_label: "v1.0.0", owner_user_id: 3, created_by: 3,
    created_at: "2025-12-01T09:00:00Z", updated_at: "2026-04-01T09:00:00Z", operation_count: 0,
    ...o,
  });
  return [
    mk({ id: 1, scope: "projeto", parent_id: 1, name: "API do Portal do Servidor", title: "Portal do Servidor API", description: "Endpoints REST usados pelo frontend do Portal do Servidor.", version_label: "v1.3.0" }),
    mk({ id: 2, name: "API de Chamados SGI", title: "Chamados SGI API", description: "Integração de abertura e consulta de chamados técnicos.", version_label: "v2.0.0" }),
  ];
}

function tools(): ExternalTool[] {
  return [
    {
      id: 1, name: "Grafana ETIPI", description: "Dashboards de observabilidade da infraestrutura da ETIPI.",
      url: "https://mon01.etipi.pi.gov.br/grafana", icon: "chart-bar", embed_enabled: true, sort_order: 1,
      service_id: null, dns_id: null, source: "manual", has_credentials: false,
      created_at: "2025-12-05T09:00:00Z", updated_at: "2026-03-01T09:00:00Z",
    },
  ];
}

// ── The new module: offerings ───────────────────────────────────────────

function offerings(): Offering[] {
  const at = (d: string) => d;
  return [
    {
      id: 1, slug: "provisionamento-vm", name: "Provisionamento de Máquina Virtual",
      category: "Infraestrutura", description: "Solicita uma nova máquina virtual dimensionada conforme a necessidade do projeto.",
      request_type: "vm", approver_entidade_id: ETIPI, glpi_mode: "always", is_active: true, sort_order: 1,
      created_at: at("2026-01-05T09:00:00Z"), updated_at: at("2026-01-05T09:00:00Z"),
      form_schema: { fields: [
        { key: "ambiente", type: "select", label_pt: "Ambiente", label_en: "Environment", required: true, options: ["Produção", "Homologação", "Desenvolvimento"] },
        { key: "vcpu", type: "number", label_pt: "vCPUs", label_en: "vCPUs", required: true },
        { key: "ram_gb", type: "number", label_pt: "RAM (GB)", label_en: "RAM (GB)", required: true },
        { key: "sistema_operacional", type: "select", label_pt: "Sistema operacional", label_en: "Operating system", required: true, options: ["Ubuntu 24.04 LTS", "Rocky Linux 9", "Debian 12"] },
      ] },
    },
    {
      id: 2, slug: "registro-subdominio", name: "Registro de Subdomínio",
      category: "Rede", description: "Solicita o registro de um novo subdomínio sob os domínios do governo.",
      request_type: "dns", approver_entidade_id: ETIPI, glpi_mode: "inherit", is_active: true, sort_order: 2,
      created_at: at("2026-01-06T09:00:00Z"), updated_at: at("2026-01-06T09:00:00Z"),
      form_schema: { fields: [
        { key: "subdominio", type: "text", label_pt: "Subdomínio desejado", label_en: "Desired subdomain", required: true, max_length: 80, help_pt: "Ex.: portal.sead.pi.gov.br" },
        { key: "tipo_registro", type: "select", label_pt: "Tipo de registro", label_en: "Record type", required: true, options: ["A", "CNAME", "TXT"] },
      ] },
    },
    {
      id: 3, slug: "conta-sistema-corporativo", name: "Conta de Acesso ao Sistema Corporativo",
      category: "Acessos", description: "Solicita uma conta de acesso a um sistema corporativo já em produção.",
      request_type: "account", approver_entidade_id: NTGD, glpi_mode: "never", is_active: true, sort_order: 3,
      created_at: at("2026-01-08T09:00:00Z"), updated_at: at("2026-01-08T09:00:00Z"),
      form_schema: { fields: [
        { key: "justificativa", type: "textarea", label_pt: "Justificativa de acesso", label_en: "Access justification", required: true, max_length: 500 },
        { key: "acesso_administrativo", type: "checkbox", label_pt: "Necessita acesso administrativo?", label_en: "Needs administrative access?", required: false },
      ] },
    },
    {
      id: 4, slug: "vinculo-servico-host", name: "Vínculo de Serviço a Host Existente",
      category: "Infraestrutura", description: "Vincula um serviço já catalogado a um host existente no inventário.",
      request_type: "service", approver_entidade_id: ETIPI, glpi_mode: "inherit", is_active: true, sort_order: 4,
      created_at: at("2026-01-10T09:00:00Z"), updated_at: at("2026-01-10T09:00:00Z"),
      form_schema: { fields: [
        { key: "host_alvo", type: "asset_ref", asset_type: "host", label_pt: "Host de destino", label_en: "Target host", required: true },
        { key: "porta", type: "text", label_pt: "Porta do serviço", label_en: "Service port", required: false },
      ] },
    },
    {
      id: 5, slug: "melhoria-sistema", name: "Solicitação de Melhoria em Sistema",
      category: "Desenvolvimento", description: "Solicita uma melhoria ou nova funcionalidade em um sistema já existente.",
      request_type: "feature", approver_entidade_id: SGP, glpi_mode: "never", is_active: true, sort_order: 5,
      created_at: at("2026-01-12T09:00:00Z"), updated_at: at("2026-01-12T09:00:00Z"),
      form_schema: { fields: [
        { key: "descricao", type: "textarea", label_pt: "Descrição da melhoria", label_en: "Improvement description", required: true, max_length: 1000 },
        { key: "data_desejada", type: "date", label_pt: "Data desejada para entrega", label_en: "Desired delivery date", required: false },
        { key: "modulos_afetados", type: "tags", label_pt: "Módulos afetados", label_en: "Affected modules", required: false },
      ] },
    },
    {
      id: 6, slug: "token-integracao-api", name: "Emissão de Token de Integração API",
      category: "Integrações", description: "Solicita a emissão de um token para consumo de APIs internas.",
      request_type: "api_token", approver_entidade_id: ETIPI, glpi_mode: "always", is_active: true, sort_order: 6,
      created_at: at("2026-01-14T09:00:00Z"), updated_at: at("2026-01-14T09:00:00Z"),
      form_schema: { fields: [
        { key: "nome_integracao", type: "text", label_pt: "Nome da integração", label_en: "Integration name", required: true, max_length: 120 },
        { key: "escopo", type: "select", label_pt: "Escopo de acesso", label_en: "Access scope", required: true, options: ["Leitura", "Leitura e escrita", "Administrativo"] },
      ] },
    },
    {
      id: 7, slug: "suporte-tecnico-especializado", name: "Suporte Técnico Especializado",
      category: "Suporte", description: "Abre uma solicitação de suporte técnico especializado para um sistema ou serviço.",
      request_type: "support", approver_entidade_id: SGI, glpi_mode: "always", is_active: true, sort_order: 7,
      created_at: at("2026-01-16T09:00:00Z"), updated_at: at("2026-01-16T09:00:00Z"),
      form_schema: { fields: [
        { key: "descricao_problema", type: "textarea", label_pt: "Descrição do problema", label_en: "Problem description", required: true, max_length: 800 },
        { key: "urgencia", type: "select", label_pt: "Urgência", label_en: "Urgency", required: true, options: ["Baixa", "Média", "Alta", "Crítica"] },
      ] },
    },
    // Deliberate near-collision with service "pg-prod-01" (technology_stack
    // "PostgreSQL 16") and host "vm-postgres-02" — searching "postgres" must
    // return hits from both the offering group and the asset group.
    {
      id: 8, slug: "managed-postgres", name: "Managed Postgres",
      category: "Infraestrutura", description: "Provisionamento de um banco de dados PostgreSQL gerenciado, com backups automáticos.",
      request_type: "service", approver_entidade_id: NTGD, glpi_mode: "inherit", is_active: true, sort_order: 8,
      created_at: at("2026-01-18T09:00:00Z"), updated_at: at("2026-01-18T09:00:00Z"),
      form_schema: { fields: [
        { key: "plano", type: "select", label_pt: "Plano do banco", label_en: "Database plan", required: true, options: ["Standard - 2 vCPU / 8GB", "Performance - 4 vCPU / 16GB"] },
        { key: "armazenamento_gb", type: "number", label_pt: "Armazenamento (GB)", label_en: "Storage (GB)", required: true },
        { key: "nome_banco", type: "text", label_pt: "Nome do banco de dados", label_en: "Database name", required: true, max_length: 60 },
      ] },
    },
  ];
}

// ── Service requests + events ───────────────────────────────────────────
// Row shape kept close to the DB: only id, offering_id, title, status,
// requester_user_id/entidade_id and the GLPI/fulfillment fields vary per
// row; everything else defaults sanely. offering_name/requester_name/
// entidade_name are display-only joins — computed on read (see
// db.ts#hydrateRequest), never stored here.

type Req = Partial<ServiceRequest> &
  Pick<ServiceRequest, "id" | "offering_id" | "title" | "status" | "requester_user_id" | "created_at" | "updated_at">;

function mkRequest(offeringsBySlug: Map<number, Offering>, r: Req): ServiceRequest {
  const offering = offeringsBySlug.get(r.offering_id)!;
  return {
    priority: "medium",
    form_data: {},
    form_schema_snapshot: offering.form_schema,
    requester_entidade_id: null,
    assignee_user_id: null,
    decided_by_user_id: null,
    decided_at: null,
    delivered_by_user_id: null,
    delivered_at: null,
    fulfilled_asset_type: "",
    fulfilled_asset_id: null,
    external_source: "",
    external_ref: "",
    external_url: "",
    cached_title: "",
    cached_status: "",
    cached_at: null,
    ...r,
  };
}

function requests(offeringList: Offering[]): ServiceRequest[] {
  const byId = new Map(offeringList.map((o) => [o.id, o]));
  const R = (r: Req) => mkRequest(byId, r);
  return [
    // The cross-branch star: created by SGA, offering's approver is ETIPI —
    // ETIPI is a sibling of SEAD-PI, not an ancestor of SGA.
    R({
      id: 1, offering_id: 1, title: "Provisionamento de VM para homologação do Portal do Servidor",
      status: "under_review", priority: "high", requester_user_id: 5, requester_entidade_id: SGA,
      form_data: { ambiente: "Homologação", vcpu: 4, ram_gb: 8, sistema_operacional: "Ubuntu 24.04 LTS" },
      created_at: "2026-08-10T13:20:00Z", updated_at: "2026-08-11T09:00:00Z",
    }),
    R({
      id: 2, offering_id: 2, title: "Registro de subdomínio homolog.sead.pi.gov.br",
      status: "delivered", priority: "medium", requester_user_id: 1, requester_entidade_id: SGA,
      form_data: { subdominio: "homolog.sead.pi.gov.br", tipo_registro: "A" },
      external_source: "glpi", external_ref: "GLPI-15820", external_url: "https://glpi.pi.gov.br/tickets/15820",
      cached_title: "Registro DNS - homolog.sead", cached_status: "Solucionado", cached_at: "2026-02-06T16:00:00Z",
      delivered_by_user_id: 3, delivered_at: "2026-02-06T16:00:00Z",
      decided_by_user_id: 3, decided_at: "2026-02-04T10:00:00Z",
      fulfilled_asset_type: "dns", fulfilled_asset_id: 4,
      created_at: "2026-02-03T08:30:00Z", updated_at: "2026-02-06T16:00:00Z",
    }),
    R({
      id: 3, offering_id: 3, title: "Acesso administrativo ao sistema de chamados",
      status: "approved", priority: "medium", requester_user_id: 8, requester_entidade_id: SGI,
      form_data: { justificativa: "Atuação como administradora do módulo de chamados da SGI.", acesso_administrativo: true },
      decided_by_user_id: 7, decided_at: "2026-05-13T11:00:00Z",
      created_at: "2026-05-11T14:00:00Z", updated_at: "2026-05-13T11:00:00Z",
    }),
    R({
      id: 4, offering_id: 4, title: "Vincular novo serviço de cache ao host mon01.etipi",
      status: "in_progress", priority: "low", requester_user_id: 3, requester_entidade_id: ETIPI,
      form_data: { host_alvo: 3, porta: "6380" },
      assignee_user_id: 3, decided_by_user_id: 3, decided_at: "2026-06-03T09:00:00Z",
      created_at: "2026-06-01T09:10:00Z", updated_at: "2026-06-04T10:00:00Z",
    }),
    R({
      id: 5, offering_id: 5, title: "Melhoria: exportação de relatórios em PDF no Portal do Servidor",
      status: "submitted", priority: "medium", requester_user_id: 1, requester_entidade_id: SGA,
      form_data: { descricao: "Permitir exportar o extrato funcional em PDF diretamente do Portal.", modulos_afetados: ["portal", "relatorios"] },
      created_at: "2026-08-20T10:15:00Z", updated_at: "2026-08-20T10:15:00Z",
    }),
    R({
      id: 6, offering_id: 6, title: "Token de integração com o barramento de dados do Estado",
      status: "delivered", priority: "high", requester_user_id: 3, requester_entidade_id: ETIPI,
      form_data: { nome_integracao: "barramento-estadual", escopo: "Leitura e escrita" },
      external_source: "glpi", external_ref: "GLPI-14290", external_url: "https://glpi.pi.gov.br/tickets/14290",
      cached_title: "Emissão token API barramento", cached_status: "Fechado", cached_at: "2026-01-20T15:00:00Z",
      delivered_by_user_id: 3, delivered_at: "2026-01-20T15:00:00Z",
      decided_by_user_id: 3, decided_at: "2026-01-17T09:00:00Z",
      created_at: "2026-01-15T08:00:00Z", updated_at: "2026-01-20T15:00:00Z",
    }),
    R({
      id: 7, offering_id: 7, title: "Instabilidade no sistema de chamados durante pico de acesso",
      status: "rejected", priority: "critical", requester_user_id: 2, requester_entidade_id: SGP,
      form_data: { descricao_problema: "Sistema de chamados apresenta lentidão extrema entre 8h e 9h.", urgencia: "Crítica" },
      decided_by_user_id: 8, decided_at: "2026-03-04T09:00:00Z",
      created_at: "2026-03-02T08:00:00Z", updated_at: "2026-03-04T09:00:00Z",
    }),
    R({
      id: 8, offering_id: 8, title: "Provisionamento de banco gerenciado para o módulo de Folha de Pagamento",
      status: "needs_info", priority: "high", requester_user_id: 7, requester_entidade_id: NTGD,
      form_data: { plano: "Performance - 4 vCPU / 16GB", armazenamento_gb: 200, nome_banco: "folha_pagamento" },
      created_at: "2026-07-22T09:00:00Z", updated_at: "2026-07-24T11:00:00Z",
    }),
    R({
      id: 9, offering_id: 1, title: "VM para testes de carga do sistema de chamados",
      status: "cancelled", priority: "low", requester_user_id: 8, requester_entidade_id: SGI,
      form_data: { ambiente: "Desenvolvimento", vcpu: 2, ram_gb: 4, sistema_operacional: "Debian 12" },
      created_at: "2026-04-09T10:00:00Z", updated_at: "2026-04-10T09:00:00Z",
    }),
    R({
      id: 10, offering_id: 2, title: "Registro de subdomínio sso.pi.gov.br",
      status: "delivered", priority: "medium", requester_user_id: 3, requester_entidade_id: ETIPI,
      form_data: { subdominio: "sso.pi.gov.br", tipo_registro: "A" },
      delivered_by_user_id: 3, delivered_at: "2026-01-08T14:00:00Z",
      decided_by_user_id: 3, decided_at: "2026-01-06T10:00:00Z",
      fulfilled_asset_type: "dns", fulfilled_asset_id: 3,
      created_at: "2026-01-05T09:00:00Z", updated_at: "2026-01-08T14:00:00Z",
    }),
    R({
      id: 11, offering_id: 3, title: "Conta de acesso ao sistema corporativo para nova servidora da SGP",
      status: "submitted", priority: "medium", requester_user_id: 2, requester_entidade_id: SGP,
      form_data: { justificativa: "Nova servidora lotada na SGP precisa de acesso ao sistema de protocolo.", acesso_administrativo: false },
      created_at: "2026-08-24T09:30:00Z", updated_at: "2026-08-24T09:30:00Z",
    }),
    R({
      id: 12, offering_id: 4, title: "Vincular serviço Keycloak ao host srv-sga-web01",
      status: "approved", priority: "medium", requester_user_id: 1, requester_entidade_id: SGA,
      form_data: { host_alvo: 2, porta: "8443" },
      decided_by_user_id: 3, decided_at: "2026-06-17T09:00:00Z",
      created_at: "2026-06-15T09:00:00Z", updated_at: "2026-06-17T09:00:00Z",
    }),
    R({
      id: 13, offering_id: 5, title: "Melhoria: autenticação via SSO no sistema de chamados",
      status: "in_progress", priority: "high", requester_user_id: 8, requester_entidade_id: SGI,
      form_data: { descricao: "Integrar o login do sistema de chamados ao Keycloak corporativo.", data_desejada: "2026-10-01" },
      assignee_user_id: 8, decided_by_user_id: 7, decided_at: "2026-05-06T09:00:00Z",
      created_at: "2026-05-02T09:00:00Z", updated_at: "2026-05-08T10:00:00Z",
    }),
    R({
      id: 14, offering_id: 6, title: "Token de leitura para dashboard de indicadores da SGA",
      status: "approved", priority: "low", requester_user_id: 5, requester_entidade_id: SGA,
      form_data: { nome_integracao: "dashboard-indicadores-sga", escopo: "Leitura" },
      decided_by_user_id: 3, decided_at: "2026-07-03T09:00:00Z",
      created_at: "2026-07-01T09:00:00Z", updated_at: "2026-07-03T09:00:00Z",
    }),
    R({
      id: 15, offering_id: 7, title: "Falha recorrente no backup automático do NTGD",
      status: "delivered", priority: "critical", requester_user_id: 7, requester_entidade_id: NTGD,
      form_data: { descricao_problema: "Job de backup noturno falha desde a última atualização do sistema.", urgencia: "Alta" },
      external_source: "glpi", external_ref: "GLPI-13655", external_url: "https://glpi.pi.gov.br/tickets/13655",
      cached_title: "Falha backup NTGD", cached_status: "Fechado", cached_at: "2026-02-24T17:00:00Z",
      delivered_by_user_id: 8, delivered_at: "2026-02-24T17:00:00Z",
      decided_by_user_id: 8, decided_at: "2026-02-21T09:00:00Z",
      fulfilled_asset_type: "host", fulfilled_asset_id: 4,
      created_at: "2026-02-20T08:00:00Z", updated_at: "2026-02-24T17:00:00Z",
    }),
    R({
      id: 16, offering_id: 8, title: "Banco gerenciado para ambiente de homologação do Portal do Servidor",
      status: "submitted", priority: "medium", requester_user_id: 1, requester_entidade_id: SGA,
      form_data: { plano: "Standard - 2 vCPU / 8GB", armazenamento_gb: 50, nome_banco: "portal_homolog" },
      created_at: "2026-08-26T11:00:00Z", updated_at: "2026-08-26T11:00:00Z",
    }),
    R({
      id: 17, offering_id: 1, title: "VM adicional para o cluster de monitoramento da ETIPI",
      status: "rejected", priority: "medium", requester_user_id: 3, requester_entidade_id: ETIPI,
      form_data: { ambiente: "Produção", vcpu: 4, ram_gb: 16, sistema_operacional: "Rocky Linux 9" },
      decided_by_user_id: 3, decided_at: "2026-03-20T09:00:00Z",
      created_at: "2026-03-18T09:00:00Z", updated_at: "2026-03-20T09:00:00Z",
    }),
    R({
      id: 18, offering_id: 2, title: "Registro CNAME para redirecionamento de chamados.sead",
      status: "needs_info", priority: "low", requester_user_id: 8, requester_entidade_id: SGI,
      form_data: { subdominio: "suporte.sead.pi.gov.br", tipo_registro: "CNAME" },
      created_at: "2026-08-05T09:00:00Z", updated_at: "2026-08-06T10:00:00Z",
    }),
    R({
      id: 19, offering_id: 3, title: "Acesso administrativo temporário para migração de dados",
      status: "cancelled", priority: "medium", requester_user_id: 6, requester_entidade_id: TRIN,
      form_data: { justificativa: "Migração pontual de dados do parceiro Trin para o novo sistema.", acesso_administrativo: true },
      created_at: "2026-04-27T09:00:00Z", updated_at: "2026-04-29T09:00:00Z",
    }),
    R({
      id: 20, offering_id: 4, title: "Vincular serviço de cache Redis ao host de produção",
      status: "delivered", priority: "medium", requester_user_id: 3, requester_entidade_id: ETIPI,
      form_data: { host_alvo: 3, porta: "6379" },
      external_source: "glpi", external_ref: "GLPI-12980", external_url: "https://glpi.pi.gov.br/tickets/12980",
      cached_title: "Vínculo Redis", cached_status: "Fechado", cached_at: "2026-01-30T15:00:00Z",
      delivered_by_user_id: 3, delivered_at: "2026-01-30T15:00:00Z",
      decided_by_user_id: 3, decided_at: "2026-01-29T09:00:00Z",
      fulfilled_asset_type: "service", fulfilled_asset_id: 4,
      created_at: "2026-01-28T09:00:00Z", updated_at: "2026-01-30T15:00:00Z",
    }),
    R({
      id: 21, offering_id: 5, title: "Melhoria: notificações por e-mail no sistema de chamados",
      status: "under_review", priority: "medium", requester_user_id: 2, requester_entidade_id: SGP,
      form_data: { descricao: "Enviar e-mail ao solicitante a cada mudança de status do chamado.", modulos_afetados: ["chamados", "notificacoes"] },
      created_at: "2026-08-15T09:00:00Z", updated_at: "2026-08-16T09:00:00Z",
    }),
    R({
      id: 22, offering_id: 6, title: "Token administrativo para integração com fornecedor Syslae",
      status: "cancelled", priority: "high", requester_user_id: 7, requester_entidade_id: NTGD,
      form_data: { nome_integracao: "syslae-faturamento", escopo: "Administrativo" },
      created_at: "2026-05-30T09:00:00Z", updated_at: "2026-06-01T09:00:00Z",
    }),
    R({
      id: 23, offering_id: 7, title: "Suporte para configuração de HTTPS no ambiente de homologação",
      status: "submitted", priority: "low", requester_user_id: 1, requester_entidade_id: SGA,
      form_data: { descricao_problema: "Certificado TLS do ambiente de homologação expirou.", urgencia: "Média" },
      created_at: "2026-08-27T10:00:00Z", updated_at: "2026-08-27T10:00:00Z",
    }),
    R({
      id: 24, offering_id: 8, title: "Aumento de armazenamento do cluster PostgreSQL de produção",
      status: "under_review", priority: "high", requester_user_id: 3, requester_entidade_id: ETIPI,
      form_data: { plano: "Performance - 4 vCPU / 16GB", armazenamento_gb: 500, nome_banco: "pg-prod-01" },
      created_at: "2026-08-18T09:00:00Z", updated_at: "2026-08-19T09:00:00Z",
    }),
    R({
      id: 25, offering_id: 1, title: "VM para ambiente de desenvolvimento do módulo de Folha de Pagamento",
      status: "approved", priority: "medium", requester_user_id: 7, requester_entidade_id: NTGD,
      form_data: { ambiente: "Desenvolvimento", vcpu: 2, ram_gb: 8, sistema_operacional: "Ubuntu 24.04 LTS" },
      decided_by_user_id: 3, decided_at: "2026-07-12T09:00:00Z",
      created_at: "2026-07-10T09:00:00Z", updated_at: "2026-07-12T09:00:00Z",
    }),
  ];
}

function events(userList: User[]): RequestEvent[] {
  let seq = 1;
  const name = (id: number) => userList.find((u) => u.id === id)?.display_name ?? "";
  const status = (requestId: number, from: string, to: string, userId: number, at: string, body = ""): RequestEvent => ({
    id: seq++, request_id: requestId, user_id: userId, user_name: name(userId), kind: "status", body, from_status: from, to_status: to, created_at: at,
  });
  const comment = (requestId: number, userId: number, body: string, at: string): RequestEvent => ({
    id: seq++, request_id: requestId, user_id: userId, user_name: name(userId), kind: "comment", body, from_status: "", to_status: "", created_at: at,
  });

  const out: RequestEvent[] = [];

  // request 1 (under_review, cross-branch star)
  out.push(status(1, "", "submitted", 5, "2026-08-10T13:20:00Z"));
  out.push(status(1, "submitted", "under_review", 3, "2026-08-11T09:00:00Z"));
  out.push(comment(1, 3, "Precisamos confirmar o ambiente de destino antes de aprovar o provisionamento.", "2026-08-11T09:05:00Z"));
  // request 2 (delivered)
  out.push(status(2, "", "submitted", 1, "2026-02-03T08:30:00Z"));
  out.push(comment(2, 1, "Registro solicitado com urgência para o lançamento da nova versão do Portal.", "2026-02-03T08:35:00Z"));
  out.push(status(2, "under_review", "approved", 3, "2026-02-04T10:00:00Z"));
  out.push(status(2, "in_progress", "delivered", 3, "2026-02-06T16:00:00Z"));
  // request 3 (approved)
  out.push(status(3, "", "submitted", 8, "2026-05-11T14:00:00Z"));
  out.push(comment(3, 7, "Justificativa aceita, aguardando liberação do time de segurança.", "2026-05-12T09:00:00Z"));
  out.push(status(3, "under_review", "approved", 7, "2026-05-13T11:00:00Z"));
  // request 4 (in_progress)
  out.push(status(4, "", "submitted", 3, "2026-06-01T09:10:00Z"));
  out.push(status(4, "under_review", "approved", 3, "2026-06-03T09:00:00Z"));
  out.push(comment(4, 3, "Serviço de cache validado em ambiente de testes antes do vínculo.", "2026-06-04T09:30:00Z"));
  out.push(status(4, "approved", "in_progress", 3, "2026-06-04T10:00:00Z"));
  // request 5 (submitted)
  out.push(status(5, "", "submitted", 1, "2026-08-20T10:15:00Z"));
  // request 6 (delivered)
  out.push(status(6, "", "submitted", 3, "2026-01-15T08:00:00Z"));
  out.push(status(6, "under_review", "approved", 3, "2026-01-17T09:00:00Z"));
  out.push(status(6, "in_progress", "delivered", 3, "2026-01-20T15:00:00Z"));
  out.push(comment(6, 3, "Token de integração emitido e compartilhado com a equipe pelo canal seguro.", "2026-01-20T15:05:00Z"));
  // request 7 (rejected)
  out.push(status(7, "", "submitted", 2, "2026-03-02T08:00:00Z"));
  out.push(status(7, "under_review", "rejected", 8, "2026-03-04T09:00:00Z", "Instabilidade já tratada pela correção de performance do último release."));
  // request 8 (needs_info)
  out.push(status(8, "", "submitted", 7, "2026-07-22T09:00:00Z"));
  out.push(comment(8, 7, "Favor detalhar o volume estimado de dados para dimensionar o armazenamento.", "2026-07-23T09:00:00Z"));
  out.push(status(8, "under_review", "needs_info", 7, "2026-07-24T11:00:00Z"));
  // request 9 (cancelled)
  out.push(status(9, "", "submitted", 8, "2026-04-09T10:00:00Z"));
  out.push(status(9, "submitted", "cancelled", 8, "2026-04-10T09:00:00Z"));
  // request 10 (delivered)
  out.push(status(10, "", "submitted", 3, "2026-01-05T09:00:00Z"));
  out.push(status(10, "under_review", "approved", 3, "2026-01-06T10:00:00Z"));
  out.push(status(10, "in_progress", "delivered", 3, "2026-01-08T14:00:00Z"));
  // request 11 (submitted)
  out.push(status(11, "", "submitted", 2, "2026-08-24T09:30:00Z"));
  // request 12 (approved)
  out.push(status(12, "", "submitted", 1, "2026-06-15T09:00:00Z"));
  out.push(comment(12, 3, "Aprovado. Aguardando janela de manutenção para o vínculo do serviço.", "2026-06-17T09:05:00Z"));
  out.push(status(12, "under_review", "approved", 3, "2026-06-17T09:00:00Z"));
  // request 13 (in_progress)
  out.push(status(13, "", "submitted", 8, "2026-05-02T09:00:00Z"));
  out.push(status(13, "under_review", "approved", 7, "2026-05-06T09:00:00Z"));
  out.push(status(13, "approved", "in_progress", 7, "2026-05-08T10:00:00Z"));
  // request 14 (approved)
  out.push(status(14, "", "submitted", 5, "2026-07-01T09:00:00Z"));
  out.push(status(14, "under_review", "approved", 3, "2026-07-03T09:00:00Z"));
  // request 15 (delivered)
  out.push(status(15, "", "submitted", 7, "2026-02-20T08:00:00Z"));
  out.push(status(15, "under_review", "approved", 8, "2026-02-21T09:00:00Z"));
  out.push(status(15, "in_progress", "delivered", 8, "2026-02-24T17:00:00Z"));
  out.push(comment(15, 8, "Backup restabelecido, monitoramento reforçado por 72 horas.", "2026-02-24T17:10:00Z"));
  // request 16 (submitted)
  out.push(status(16, "", "submitted", 1, "2026-08-26T11:00:00Z"));
  // request 17 (rejected)
  out.push(status(17, "", "submitted", 3, "2026-03-18T09:00:00Z"));
  out.push(status(17, "under_review", "rejected", 3, "2026-03-20T09:00:00Z", "Cluster de monitoramento já possui capacidade suficiente neste ciclo."));
  // request 18 (needs_info)
  out.push(status(18, "", "submitted", 8, "2026-08-05T09:00:00Z"));
  out.push(comment(18, 8, "Poderia informar qual subdomínio deve apontar para o CNAME?", "2026-08-06T09:30:00Z"));
  out.push(status(18, "under_review", "needs_info", 8, "2026-08-06T10:00:00Z"));
  // request 19 (cancelled)
  out.push(status(19, "", "submitted", 6, "2026-04-27T09:00:00Z"));
  out.push(status(19, "submitted", "under_review", 7, "2026-04-28T09:00:00Z"));
  out.push(status(19, "under_review", "cancelled", 6, "2026-04-29T09:00:00Z", "Migração adiada para o próximo trimestre."));
  // request 20 (delivered)
  out.push(status(20, "", "submitted", 3, "2026-01-28T09:00:00Z"));
  out.push(status(20, "under_review", "approved", 3, "2026-01-29T09:00:00Z"));
  out.push(status(20, "in_progress", "delivered", 3, "2026-01-30T15:00:00Z"));
  // request 21 (under_review)
  out.push(status(21, "", "submitted", 2, "2026-08-15T09:00:00Z"));
  out.push(status(21, "submitted", "under_review", 7, "2026-08-16T09:00:00Z"));
  // request 22 (cancelled)
  out.push(status(22, "", "submitted", 7, "2026-05-30T09:00:00Z"));
  out.push(status(22, "submitted", "cancelled", 7, "2026-06-01T09:00:00Z"));
  // request 23 (submitted)
  out.push(status(23, "", "submitted", 1, "2026-08-27T10:00:00Z"));
  // request 24 (under_review)
  out.push(status(24, "", "submitted", 3, "2026-08-18T09:00:00Z"));
  out.push(comment(24, 3, "Analisando impacto do aumento de armazenamento no cluster de produção.", "2026-08-19T08:30:00Z"));
  out.push(status(24, "submitted", "under_review", 3, "2026-08-19T09:00:00Z"));
  // request 25 (approved)
  out.push(status(25, "", "submitted", 7, "2026-07-10T09:00:00Z"));
  out.push(status(25, "under_review", "approved", 3, "2026-07-12T09:00:00Z"));

  return out;
}

export function seed(): Seed {
  const entidadeList = entidades();
  const userList = users();
  const offeringList = offerings();
  return {
    entidades: entidadeList,
    users: userList,
    actorId: 1,
    enums: enums(),
    hosts: hosts(),
    services: services(),
    dns: dns(),
    projects: projects(),
    apiCatalogs: apiCatalogs(),
    tools: tools(),
    offerings: offeringList,
    requests: requests(offeringList),
    events: events(userList),
  };
}
