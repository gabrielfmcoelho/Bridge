// Mock transport layer — catch-all dispatcher for every /mock/api/* request.
//
// Mounted at /mock (not /api/mock): next.config.ts rewrites "/api/:path*" to
// the Go backend BEFORE dynamic routes resolve, so anything under /api/mock
// would be silently proxied away instead of hitting this file. Do not move
// this route under src/app/api/.
//
// This file is disposable — it (and everything under src/mocks/ and
// src/app/mock/) is deleted the day the real Go backend lands, and the one
// line in src/lib/api.ts that points at it reverts. Favor obviousness over
// cleverness here; nothing here is meant to outlive this phase.
import { NextRequest } from "next/server";
import { canTransition } from "@/lib/requests";
import { extractJSON, coerceToSchema, coercePriority } from "@/lib/aiDraft";
import type {
  Offering, ServiceRequest, RequestEvent, RequestStatus, CatalogHit, AssetGrantsInput,
  Host, Service, DNSRecord, Project, ApiCatalog, ExternalTool, Database, OSUpdateRun, OSUpdateResult, ReleaseReport, RiskWarning,
} from "@/lib/types";
import {
  db, resetDB, currentUser, switchUser, requestVisible, abilitiesForRequest,
  hydrateRequest, paginate,
} from "@/mocks/db";

export const dynamic = "force-dynamic";

// Single named constant so latency can be dialed to 0 for fast test runs.
const LATENCY_MS = 150;
const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const mockOSRuns: OSUpdateRun[] = [];
// Background scans: the run as created, when it started, when it was cancelled.
const mockScans: { run: OSUpdateRun; started: number; cancelled?: number }[] = [];
const MOCK_SCAN_HOST_MS = 2000;

/** A scan run as of now: each host takes MOCK_SCAN_HOST_MS, concurrency at a
 * time; one in seven fails; a cancel skips the hosts not started by then. */
function scanView(m: (typeof mockScans)[number]): OSUpdateRun {
  const elapsed = Date.now() - m.started;
  const results = (m.run.results ?? []).map((x, i): OSUpdateResult => {
    const start = Math.floor(i / m.run.concurrency) * MOCK_SCAN_HOST_MS;
    if (m.cancelled !== undefined && start >= m.cancelled - m.started) return { ...x, status: "skipped", reason: "cancelled" };
    if (elapsed < start) return x;
    if (elapsed < start + MOCK_SCAN_HOST_MS) return { ...x, status: "running" };
    return x.host_id % 7 === 0
      ? { ...x, status: "failed", reason: "SSH connect: dial tcp 10.0.0.9:22: i/o timeout" }
      : { ...x, status: "success", os_after: x.os_before,
        output: JSON.stringify({ scan_saved: true, web_dns: { names: 2, created: 0, linked: 1, service_links: 1, unresolved: 0, trashed: 0 } }) };
  });
  const counts: OSUpdateRun["counts"] = {};
  for (const x of results) counts[x.status] = (counts[x.status] ?? 0) + 1;
  const busy = results.some((x) => x.status === "pending" || x.status === "running");
  return { ...m.run, results, counts, status: busy ? "running" : m.cancelled !== undefined ? "cancelled" : "done" };
}
// Host backup profiles (host id → own paths; absent list = inherit).
const mockProfiles = new Map<number, { include?: string[]; exclude?: string[] }>();
// Pinned SSH host keys by host id: the first two hosts share a cloned template's key.
const mockHostKeys = new Map<number, { fingerprint: string; key_type: string; pinned_at: string }>([
  [1, { fingerprint: "SHA256:q0dBV3Kp6T7Yx1o9mN2cZ8uR4eF5sH7jL0aW3vX9bQk", key_type: "ecdsa-sha2-nistp256", pinned_at: "2026-11-10T14:02:00Z" }],
  [2, { fingerprint: "SHA256:q0dBV3Kp6T7Yx1o9mN2cZ8uR4eF5sH7jL0aW3vX9bQk", key_type: "ecdsa-sha2-nistp256", pinned_at: "2026-11-11T09:30:00Z" }],
  [3, { fingerprint: "SHA256:Zt4hE1yW8pQ2nB6vK0cM5sJ9xR3aL7uD1fG4oT8iVw0", key_type: "ssh-ed25519", pinned_at: "2026-11-12T18:45:00Z" }],
]);
const MOCK_BACKUP_DEFAULTS = { prefix: "bridge/os-backups", include: ["/"], exclude: ["/tmp", "/var/cache", "/var/lib/docker"], retention_days: 30 };
// A scan with what the backup preview reads: filesystems, a host database,
// containers and a big file.
// SAPASP-CONCURSO before its release: what the scan's release probe found.
const MOCK_RISK_WARNINGS: RiskWarning[] = [
  { code: "foreign_packages", count: 2, detail: "libssl1.1 1.1.1f-1ubuntu2.20, php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1" },
  { code: "modified_conffiles", count: 1, detail: "/etc/proftpd/proftpd.conf" },
  { code: "php_handler_missing", count: 1, detail: "/home/sasc/fcgi-bin/php7.2.fcgi → /usr/bin/php-cgi7.2" },
  { code: "vhost_port_unlistened", count: 1, detail: "sasc.example.gov.br:444" },
];
// What the mock release (which fails like run #8 did) leaves behind.
const mockReport = (release: boolean): ReleaseReport => ({
  ...(release ? {
    warnings_before: MOCK_RISK_WARNINGS,
    dpkg_errors: ["proftpd-core: installed proftpd-core package post-installation script subprocess returned error exit status 1"],
    new_failed_units: ["fail2ban.service", "logrotate.service"],
    stopped_services: ["fail2ban.service"],
    new_dist_files: ["/etc/proftpd/modules.conf.ucf-dist", "/etc/proftpd/proftpd.conf.ucf-dist"],
    lost_php_handlers: ["/home/sasc/fcgi-bin/php7.2.fcgi → /usr/bin/php-cgi7.2"],
  } : {}),
  sites: [
    { name: "portal.example.gov.br", before: 200, after: 200 },
    { name: "sasc.example.gov.br", before: 302, after: release ? 200 : 302, ...(release ? { error: "Deprecated", broken: true } : {}) },
    ...(release ? [{ name: "concurso.example.gov.br", before: 200, after: 0, broken: true }] : []),
  ],
});
const mockScan = (h: { nickname: string; os?: { pretty?: string; arch?: string } }) => JSON.stringify({
  os: h.os?.pretty ?? "", kernel: "6.8.0-45-generic", arch: h.os?.arch ?? "x86_64",
  filesystems: [
    { mount: "/", device: "/dev/sda1", fstype: "ext4", size_bytes: 53_687_091_200, used_bytes: 21_474_836_480, avail_bytes: 32_212_254_720, used_pct: 40 },
    { mount: "/data", device: "/dev/sdb1", fstype: "xfs", size_bytes: 214_748_364_800, used_bytes: 128_849_018_880, avail_bytes: 85_899_345_920, used_pct: 60 },
    { mount: "/mnt/arquivos", device: "nas:/arquivos", fstype: "nfs4", size_bytes: 0, used_bytes: 0, avail_bytes: 0, used_pct: 0, network: true },
  ],
  service_inventory: [{ name: "postgresql", label: "PostgreSQL", kind: "database", host_running: true }],
  parsed_containers: [{ id: "a1", name: "web", image: "nginx:1.25", status: "Up 3 days", ports: "80/tcp" }],
  resource_top: { top_disk: [{ path: "/data/dumps/full.sql", size_bytes: 3_221_225_472 }] },
  release_risks: { foreign_count: 2, foreign_packages: ["libssl1.1 1.1.1f-1ubuntu2.20", "php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1"],
    modified_conffiles: ["/etc/proftpd/proftpd.conf"], apache_listen: [80, 443],
    php_handlers: [{ source: "/etc/apache2/sites-enabled/sasc.conf:5", dir: "/home/sasc/public_html", kind: "fcgi",
      target: "/home/sasc/fcgi-bin/php7.2.fcgi", runs: "/usr/bin/php-cgi7.2", version: "7.2", state: "missing" }],
    warnings: MOCK_RISK_WARNINGS },
});
const profileView = (hostId: number) => {
  const p = mockProfiles.get(hostId) ?? {};
  return { profile: { host_id: hostId, include: p.include ?? null, exclude: p.exclude ?? null },
    defaults: MOCK_BACKUP_DEFAULTS,
    effective: { include: p.include ?? MOCK_BACKUP_DEFAULTS.include, exclude: p.exclude ?? MOCK_BACKUP_DEFAULTS.exclude } };
};

function json(body: unknown, status = 200): Response {
  return Response.json(body, { status });
}

function notFound(error: string): Response {
  return json({ error }, 404);
}

async function readJSON<T>(request: NextRequest): Promise<T> {
  return (await request.json().catch(() => ({}))) as T;
}

// ── CatalogHit builders (GET /api/catalog/search) ───────────────────────

function textMatch(q: string, fields: (string | null | undefined)[]): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  return fields.some((f) => (f ?? "").toLowerCase().includes(needle));
}

const offeringHit = (o: Offering): CatalogHit => ({
  kind: "offering", asset_type: "", id: o.id, name: o.name, description: o.description,
  href: `/catalog/${o.slug}`, slug: o.slug,
});
// Which entidade owns each discovery fixture. The real backend derives this
// from asset_entidades grants; the fixtures carry no grants, so the projection
// is stated here instead of left blank — knowing that pg-prod-01 belongs to
// SEAD-PI is exactly what tells a user whether it is *their* duplicate.
//
// ponytail: display-only. The mock does NOT scope discovery results by
// entidade — every persona sees all 25 assets. B7 must apply VisibleExpr per
// UNION branch; this map only proves the column has somewhere to come from.
const ASSET_ENTIDADE: Record<string, string> = {
  "host:1": "SEAD-PI", "host:2": "SGA", "host:3": "ETIPI", "host:4": "NTGD", "host:5": "SGI",
  "service:1": "SEAD-PI", "service:2": "SEAD-PI", "service:3": "SGI", "service:4": "ETIPI",
  "service:5": "ETIPI", "service:6": "SEAD-PI", "service:7": "SGI", "service:8": "SEAD-PI",
  "service:9": "ETIPI", "service:10": "ETIPI",
  "dns:1": "SEAD-PI", "dns:2": "SGI", "dns:3": "GovPI", "dns:4": "SEAD-PI",
  "project:1": "SEAD-PI", "project:2": "SGI", "project:3": "ETIPI",
  "api_catalog:1": "SEAD-PI", "api_catalog:2": "SGI",
  "tool:1": "ETIPI",
};
const entidadeOf = (assetType: string, id: number) => ASSET_ENTIDADE[`${assetType}:${id}`];

// `detail` is the one type-specific fact each asset contributes to the shared
// discovery table — the FQDN for a host, stack and port for a service, spec
// version for an API. Everything else about the row is type-agnostic.
const hostHit = (h: Host): CatalogHit => ({
  kind: "asset", asset_type: "host", id: h.id, name: h.nickname, description: h.description,
  detail: h.hostname,
  entidade_name: entidadeOf("host", h.id),
  href: `/hosts/${h.oficial_slug}`,
});
const serviceHit = (s: Service): CatalogHit => ({
  kind: "asset", asset_type: "service", id: s.id, name: s.nickname, description: s.description,
  detail: [s.technology_stack, s.port && `:${s.port}`].filter(Boolean).join(" · "),
  entidade_name: entidadeOf("service", s.id),
  href: `/services/${s.id}`,
});
const dnsHit = (d: DNSRecord): CatalogHit => ({
  kind: "asset", asset_type: "dns", id: d.id, name: d.domain, description: d.observacoes,
  detail: d.has_https ? "HTTPS" : "HTTP",
  entidade_name: entidadeOf("dns", d.id),
  href: `/dns/${d.id}`,
});
const projectHit = (p: Project): CatalogHit => ({
  kind: "asset", asset_type: "project", id: p.id, name: p.name, description: p.description,
  detail: p.setor_responsavel,
  entidade_name: entidadeOf("project", p.id),
  href: `/projects/${p.id}`,
});
const apiCatalogHit = (a: ApiCatalog): CatalogHit => ({
  kind: "asset", asset_type: "api_catalog", id: a.id, name: a.name, description: a.description,
  detail: a.spec_version,
  entidade_name: entidadeOf("api_catalog", a.id),
  href: `/atlas/apis/${a.id}`,
});
const toolHit = (t: ExternalTool): CatalogHit => ({
  kind: "asset", asset_type: "tool", id: t.id, name: t.name, description: t.description,
  detail: t.url ? new URL(t.url).host : "",
  entidade_name: entidadeOf("tool", t.id),
  href: "/tools",
});

// Sorting runs over the whole matched set, before paginate() slices it — a
// table that reorders only the rows already on screen tells the user something
// false about the rest. B7's UNION ALL needs the same ORDER BY.
const HIT_SORT_FIELDS: Record<string, (h: CatalogHit) => string> = {
  name: (h) => h.name,
  asset_type: (h) => h.asset_type,
  detail: (h) => h.detail ?? "",
  entidade_name: (h) => h.entidade_name ?? "",
};

function sortHits(hits: CatalogHit[], sort: string | null, dir: string | null): CatalogHit[] {
  const get = sort ? HIT_SORT_FIELDS[sort] : undefined;
  if (!get) return hits;
  const sorted = [...hits].sort((a, b) => get(a).localeCompare(get(b), "pt-BR", { sensitivity: "base" }));
  return dir === "desc" ? sorted.reverse() : sorted;
}

function searchCatalog(q: string, kind: string): CatalogHit[] {
  const hits: CatalogHit[] = [];
  // Assets are pushed FIRST, before offerings. paginate() slices this flat
  // array, so whichever group leads is the one guaranteed a place on page 1 —
  // and the page exists to show you what already runs before you request a
  // duplicate. A broad query matching every offering must not be able to push
  // pg-prod-01 onto page 2. This mirrors the constraint the plan puts on B7's
  // UNION ALL; the client re-groups for display either way (CatalogSearch.tsx:95).
  //
  // The catalog page asks for each group by kind in its own request, so it
  // never depends on how one mixed page happens to be sliced.
  if (kind !== "offering") {
    hits.push(...db.hosts.filter((h) => textMatch(q, [h.nickname, h.hostname, h.description])).map(hostHit));
    hits.push(...db.services.filter((s) => textMatch(q, [s.nickname, s.description, s.technology_stack])).map(serviceHit));
    hits.push(...db.dns.filter((d) => textMatch(q, [d.domain, d.observacoes])).map(dnsHit));
    hits.push(...db.projects.filter((p) => textMatch(q, [p.name, p.description])).map(projectHit));
    hits.push(...db.apiCatalogs.filter((a) => textMatch(q, [a.name, a.description, a.title])).map(apiCatalogHit));
    hits.push(...db.tools.filter((t) => textMatch(q, [t.name, t.description])).map(toolHit));
  }
  if (kind !== "asset") {
    hits.push(...liveOfferings().filter((o) => o.is_active && textMatch(q, [o.name, o.description, o.category])).map(offeringHit));
  }
  return hits;
}

// ── Request bodies (only what our handlers actually read) ──────────────

type OfferingBody = Partial<Offering> & AssetGrantsInput;
interface CreateRequestBody { offering_id: number; title: string; priority?: string; requester_entidade_id?: number; contact_name?: string; contact_phone?: string; form_data?: Record<string, unknown> }
interface PatchRequestBody { title?: string; priority?: string; form_data?: Record<string, unknown> }
interface TransitionBody {
  to: RequestStatus; note?: string; assignee_user_id?: number;
  fulfilled_asset_type?: string; fulfilled_asset_id?: number;
}
interface CommentBody { body: string }
interface SwitchUserBody { role: "admin" | "editor" | "viewer"; entidade_slug: string }

// GET /api/tags — closes a gap task A4 had to work around (see task-a5-brief.md):
// tagsAPI.list (src/lib/api.ts:716) does `Array.isArray(data) ? data : []`, so
// this must return a bare array, never the {data,meta} list envelope. Static,
// plausible-per-entity-type set; no seed data backs it (no form/inventory
// record actually stores a tags list to read from yet).
const TAGS_BY_TYPE: Record<string, string[]> = {
  host: ["produção", "homologação", "desenvolvimento", "crítico", "monitorado"],
  service: ["api", "banco-de-dados", "cache", "mensageria", "autenticação"],
  dns: ["público", "interno", "cdn", "e-mail"],
  project: ["prioritário", "legado", "modernização"],
};
const ALL_TAGS = Array.from(new Set(Object.values(TAGS_BY_TYPE).flat()));

// ── Offering writes ───────────────────────────────────────────────────────
// Stands in for B5's ~60 lines of server validation and B3's ResolveGrants:
// only known columns are copied (a stray `id` or `created_at` in the body must
// not land), grant inputs are lifted into `entidades`, and the same checks the
// Go handler will make return the same 400/409 so the UI's error path is real.

const REQUEST_TYPES = ["vm", "service", "dns", "api_token", "feature", "account", "support"];
const GLPI_MODES = ["inherit", "never", "always"];

const liveOfferings = () => db.offerings.filter((o) => !o.deleted_at);

/** Same convention as entidades: an empty slug is derived server-side. */
function slugify(name: string): string {
  return name.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

function pickDNS(body: Partial<DNSRecord>): Partial<DNSRecord> {
  const keys = ["domain", "has_https", "situacao", "observacoes", "tags", "host_ids", "service_ids", "project_ids"] as const;
  return Object.fromEntries(keys.filter((k) => body[k] !== undefined).map((k) => [k, body[k]]));
}

function pickOffering(body: OfferingBody): Partial<Offering> {
  const out: Partial<Offering> = {};
  for (const k of ["slug", "name", "category", "description", "request_type", "templates", "use_cases",
    "form_schema", "approver_entidade_id", "glpi_mode", "is_active", "sort_order"] as const) {
    if (body[k] !== undefined) (out as Record<string, unknown>)[k] = body[k];
  }
  if (body.creator_entidade_id !== undefined || body.responsible_entidade_ids !== undefined || body.is_global !== undefined) {
    out.entidades = {
      creator_entidade_id: body.creator_entidade_id ?? null,
      responsible_entidade_ids: body.responsible_entidade_ids ?? [],
      is_global: body.is_global ?? false,
    };
  }
  return out;
}

// ── DNS certificate scan ──────────────────────────────────────────────────
// Fakes a probe, varied by id so one scan shows every tier: id%3 → 1 valid
// (90d), 2 expiring in 5d, 0 expired. Dates are relative to now; the seed stays
// unscanned so the "Not scanned" state is what a fresh mock shows.
function mockCertScan(d: DNSRecord): DNSRecord {
  const DAY = 86_400_000;
  const now = Date.now();
  const left = [-3, 90, 5][d.id % 3];
  const iso = (ms: number) => new Date(now + ms).toISOString();
  return Object.assign(d, {
    cert_not_before: iso((left - 90) * DAY),
    cert_expires_at: iso(left * DAY),
    cert_issuer: "R11",
    cert_subject: d.domain,
    cert_sans: `${d.domain}, www.${d.domain}`,
    cert_error: left < 0 ? "x509: certificate has expired or is not yet valid" : "",
    cert_checked_at: iso(0),
    // Observation varies on the same id%3: online, no content, offline.
    obs_record_type: "A",
    obs_target: "200.23.153.234",
    obs_http_status: [0, 301, 404][d.id % 3],
    obs_https_status: [0, 200, 503][d.id % 3],
    obs_status: (["offline", "online", "no_content"] as const)[d.id % 3],
    observed_at: iso(0),
  });
}

function validateOffering(o: Offering): Response | null {
  if (!o.name.trim()) return json({ error: "name is required" }, 400);
  if (!REQUEST_TYPES.includes(o.request_type)) return json({ error: `invalid request_type: ${o.request_type}` }, 400);
  if (!GLPI_MODES.includes(o.glpi_mode)) return json({ error: `invalid glpi_mode: ${o.glpi_mode}` }, 400);
  if (!Array.isArray(o.form_schema?.fields)) return json({ error: "form_schema.fields must be an array" }, 400);
  if (liveOfferings().some((x) => x.slug === o.slug && x.id !== o.id)) return json({ error: `slug already in use: ${o.slug}` }, 409);
  return null;
}


// ── AI assist ─────────────────────────────────────────────────────────────
// Bridge already ships an OpenAI-compatible LLM client (internal/integrations/
// llm) whose base URL, model and encrypted API key live in app settings. So the
// mock does NOT hold a key or pick a provider — it forwards to the real Go
// backend and reuses whatever the admin configured. If Go is down or the LLM is
// unconfigured this degrades to 503 and the UI simply hides the affordance.
//
// ponytail: prompt-building and JSON coercion live here because at cutover they
// move to a Go handler verbatim; the browser only ever sends free text.
const GO_API = process.env.API_URL || "http://localhost:8080";

async function goFetch(path: string, init: RequestInit, request: NextRequest): Promise<Response> {
  return fetch(`${GO_API}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", cookie: request.headers.get("cookie") ?? "", ...(init.headers ?? {}) },
  });
}

// ── Dispatcher ───────────────────────────────────────────────────────────

async function dispatch(method: string, request: NextRequest, segs: string[]): Promise<Response> {
  await delay(LATENCY_MS);
  const p = segs.join("/");
  const qs = request.nextUrl.searchParams;

  // Mock-internal test hooks. Reachable only under this route; nothing under
  // src/components or src/lib may reference them.
  if (method === "POST" && p === "__reset") {
    resetDB();
    return json({ status: "ok" });
  }
  if (method === "POST" && p === "__user") {
    const body = await readJSON<SwitchUserBody>(request);
    try {
      return json(switchUser(body));
    } catch (err) {
      return json({ error: err instanceof Error ? err.message : "mock: invalid persona" }, 400);
    }
  }

  // ── Shell (mandatory: PageShell redirects to /login without these) ────
  if (method === "GET" && p === "auth/status") {
    return json({
      setup_required: false,
      authenticated: true,
      providers: [{ name: "local", type: "direct", label: "Local", icon: "key", color: "#6366f1" }],
    });
  }
  if (method === "GET" && p === "auth/me") {
    return json(currentUser());
  }
  if (p === "auth/me/home") {
    if (method === "GET") return json(db.homePrefs[currentUser().id] ?? {});
    if (method === "PUT") {
      const body = await readJSON<unknown>(request);
      if (!body || typeof body !== "object" || Array.isArray(body)) return json({ error: "home preferences must be a JSON object" }, 400);
      db.homePrefs[currentUser().id] = body;
      return json(body);
    }
  }
  if (method === "GET" && p === "settings/appearance") {
    return json({ app_name: "Bridge", app_color: "#6366f1", app_logo: "" });
  }
  if (method === "GET" && segs[0] === "enums" && segs.length === 2) {
    return json(paginate(db.enums[segs[1]] ?? [], qs.get("page"), qs.get("per_page")));
  }
  if (method === "GET" && p === "entidades") {
    return json(paginate(db.entidades, qs.get("page"), qs.get("per_page")));
  }
  if (method === "GET" && p === "tags") {
    const type = qs.get("type");
    return json(type ? (TAGS_BY_TYPE[type] ?? []) : ALL_TAGS);
  }

  // ── Contacts + share bundles (the /shares page) ─────────────────────
  // Mirrors internal/vault/bundle.go: admins see and act on every bundle, the
  // rest only on their own (others 404); reveal/revoke-all are admin-only.
  // ponytail: no create here — that form talks to the real backend. Edit
  // (details, items, validity) is mocked so /shares can be exercised.
  if (method === "GET" && p === "contacts") return json(paginate(db.contacts, qs.get("page"), qs.get("per_page")));
  // Public redeem (the /share/{token} guest page). ponytail: resolves secret
  // items only, each as a Keycloak API key with its scopes; api docs and wiki
  // stay empty here.
  if (method === "GET" && segs[0] === "share-bundle" && segs.length === 2) {
    const b = db.shareBundles.find((x) => x.token === segs[1]);
    if (!b || b.revoked_at || b.deleted_at) return notFound("share bundle not found");
    if (b.passphrase && qs.get("passphrase") !== b.passphrase) return json({ error: "passphrase required or incorrect" }, 401);
    const secrets = b.items.filter((it) => it.type === "secret").map((it) => ({
      name: it.label,
      type: "api_key",
      payload: JSON.stringify({ value: "mock-client-secret", client_id: it.label }),
      key: {
        client_id: it.label,
        api_name: "API Servidores",
        api_base_url: "https://gateway.sead.pi.gov.br/servidores",
        rate_limit_per_minute: 60,
        token_url: "https://gateway.sead.pi.gov.br/realms/apis/protocol/openid-connect/token",
        scopes: [
          { name: "servidores:cadastro", kind: "route", description: "Dados cadastrais dos servidores (/api/cadastro)." },
          { name: "servidores:lotacao", kind: "route", description: "Lotação e vínculo atual." },
          { name: "servidores:demo", kind: "modifier", description: "Anonimiza CPF e nome nas respostas." },
          { name: "servidores:legado" },
        ],
      },
    }));
    return json({ title: b.title, description: b.description, secrets, api_docs: [], wiki: [] });
  }
  if (segs[0] === "share-bundles") {
    const actor = currentUser();
    const admin = actor.role === "admin";
    const mine = db.shareBundles.filter((b) => admin || b.created_by === actor.id);
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const view = ({ token, passphrase, ...b }: (typeof db.shareBundles)[number]) => b;
    if (method === "GET" && segs.length === 1) return json(paginate(mine.map(view), qs.get("page"), qs.get("per_page")));
    if (method === "POST" && p === "share-bundles/revoke-all") {
      if (!admin) return json({ error: "forbidden" }, 403);
      const now = new Date().toISOString();
      const live = db.shareBundles.filter((b) => !b.revoked_at);
      live.forEach((b) => { b.revoked_at = now; });
      return json({ revoked: live.length });
    }
    const b = mine.find((x) => x.id === Number(segs[1]));
    if (!b) return notFound("share bundle not found");
    if (method === "PATCH" && segs.length === 2) {
      const body = await readJSON<{ ttl_seconds?: number; max_views?: number | null }>(request);
      const ttl = body.ttl_seconds ?? 86400;
      Object.assign(b, { revoked_at: null, deleted_at: null, expires_at: ttl < 0 ? null : new Date(Date.now() + ttl * 1000).toISOString() });
      if (body.max_views !== undefined) b.max_views = body.max_views && body.max_views > 0 ? body.max_views : null;
      return json(view(b));
    }
    if (method === "PUT" && segs[2] === "details") {
      const body = await readJSON<{ title?: string; description?: string; recipient_contact_id?: number; recipient_label?: string; passphrase?: string }>(request);
      if (body.title !== undefined) b.title = body.title.trim();
      if (body.description !== undefined) b.description = body.description;
      if (body.recipient_label !== undefined) b.recipient_label = body.recipient_label.trim();
      if (body.recipient_contact_id !== undefined) {
        const c = db.contacts.find((x) => x.id === body.recipient_contact_id);
        if (body.recipient_contact_id > 0 && !c) return json({ error: "recipient contact not found" }, 400);
        b.recipient_contact_id = c ? c.id : null;
        b.recipient_name = c ? c.name : "";
      }
      if (body.passphrase !== undefined) {
        b.passphrase = body.passphrase || undefined;
        b.has_passphrase = !!body.passphrase;
      }
      return json(view(b));
    }
    if (method === "PUT" && segs[2] === "items") {
      const body = await readJSON<{ items: { type: string; ref_id?: number; ref_key?: string; selector?: unknown }[] }>(request);
      if (!body.items?.length) return json({ error: "a share bundle must contain at least one item" }, 400);
      const old = new Map(b.items.map((i) => [`${i.type}:${i.ref_id}:${i.ref_key ?? ""}`, i]));
      b.items = body.items.map((i) => old.get(`${i.type}:${i.ref_id ?? 0}:${i.ref_key ?? ""}`) ?? {
        type: i.type as (typeof b.items)[number]["type"], ref_id: i.ref_id ?? 0, ref_key: i.ref_key, label: i.ref_key ?? `#${i.ref_id}`,
        selector: i.selector ? JSON.stringify(i.selector) : undefined,
      });
      return json(view(b));
    }
    if (method === "DELETE" && segs.length === 2) {
      b.revoked_at = new Date().toISOString();
      return new Response(null, { status: 204 });
    }
    if (method === "GET" && segs[2] === "access-log") {
      const rows = db.shareAccessLog.filter((e) => e.bundle_id === b.id).sort((x, y) => y.accessed_at.localeCompare(x.accessed_at));
      return json(paginate(rows, qs.get("page"), qs.get("per_page")));
    }
    if (method === "POST" && segs[2] === "reveal") {
      if (!admin) return notFound("share bundle not found");
      if (!b.token) return json({ error: "this link was created before links were stored and cannot be recovered" }, 409);
      db.shareAccessLog.push({ bundle_id: b.id, accessed_at: new Date().toISOString(), remote_ip: "", user_agent: "", used_passphrase: false, action: "reveal", actor_name: actor.display_name });
      return json({ token: b.token, url: `/share/${b.token}`, passphrase: b.passphrase ?? "" });
    }
    if (method === "POST" && segs[2] === "send") {
      // Nothing is mailed in the mock; addresses ending in "@fail.test" fail, to show the per-address error.
      const body = await readJSON<{ emails?: string[] }>(request);
      const contactEmail = db.contacts.find((c) => c.id === b.recipient_contact_id)?.email;
      const emails = body.emails?.length ? body.emails : contactEmail ? [contactEmail] : [];
      if (!emails.length) return json({ error: "no email address: pass emails or link a recipient contact that has one" }, 400);
      const results = emails.map((raw) => {
        const email = raw.match(/<([^>]+)>/)?.[1] ?? raw.trim();
        const sent = !email.endsWith("@fail.test");
        if (sent) db.shareAccessLog.push({ bundle_id: b.id, accessed_at: new Date().toISOString(), remote_ip: "", user_agent: "", used_passphrase: false, action: "send", actor_name: actor.display_name, sent_to: email });
        return { email, sent, error: sent ? undefined : "550 mailbox unavailable" };
      });
      return json({ results });
    }
  }

  // ── Discovery fixtures ──────────────────────────────────────────────
  // OS rows of the dependency inventory, derived from the seeded hosts' os.
  if (method === "GET" && p === "vulnerabilities/dependencies/groups" && qs.get("kind") === "os") {
    const groups = new Map<string, { kind: "os"; name: string; version: string; hosts: number; installs: number; cve_count: number; max_severity: ""; eol: boolean; eol_date: string; evaluated: number; lookup: ""; version_source: "os_release"; instances: { id: number; host_id: number; host_nickname: string; host_slug: string; container: string; image: string; service_id: null; cve_count: number }[] }>();
    for (const h of db.hosts) {
      if (!h.os?.id) continue;
      const k = `${h.os.id}|${h.os.version}`;
      const g = groups.get(k) ?? { kind: "os", name: h.os.id, version: h.os.version ?? "", hosts: 0, installs: 0, cve_count: 0, max_severity: "", eol: !!h.os.eol, eol_date: h.os.eol_date ?? "", evaluated: 1, lookup: "", version_source: "os_release", instances: [] };
      g.hosts++; g.installs++;
      g.instances.push({ id: h.id, host_id: h.id, host_nickname: h.nickname, host_slug: h.oficial_slug, container: "", image: "", service_id: null, cve_count: 0 });
      groups.set(k, g);
    }
    return json(paginate([...groups.values()], qs.get("page"), qs.get("per_page")));
  }
  if (p === "ssh/scans" && method === "POST") {
    const body = await readJSON<{ host_ids: number[]; concurrency?: number; method?: "password" | "key" }>(request);
    if (!body.host_ids?.length) return json({ error: "host_ids must have 1 to 500 hosts" }, 400);
    const id = mockScans.length + 1;
    const results: OSUpdateResult[] = body.host_ids.map((hid) => {
      const h = db.hosts.find((x) => x.id === hid)!;
      return { run_id: id, host_id: hid, host_nickname: h.nickname, host_slug: h.oficial_slug, status: "pending", os_before: h.os?.pretty,
        reboot_required: false, rebooted: false };
    });
    const m = { started: Date.now(), run: { id, kind: "scan" as const, reboot: false, concurrency: Math.min(body.concurrency || 10, 10), status: "running" as const,
      backup: false, keep_backup: false, auth_method: body.method, requested_name: "Mock", created_at: new Date().toISOString(), counts: {}, results } };
    mockScans.unshift(m);
    return json(scanView(m), 202);
  }
  if (p === "ssh/scans" && method === "GET") {
    const hostId = Number(qs.get("host_id") ?? 0);
    return json(mockScans.map(scanView)
      .filter((r) => (qs.get("active") !== "1" || r.status === "running") && (!hostId || r.results?.some((x) => x.host_id === hostId)))
      .map((r) => ({ ...r, results: undefined })));
  }
  const scanPath = p.match(/^ssh\/scans\/(\d+)(\/cancel)?$/);
  if (scanPath) {
    const m = mockScans.find((x) => x.run.id === Number(scanPath[1]));
    if (!m) return json({ error: "run not found" }, 404);
    if (scanPath[2] && method === "POST" && scanView(m).status === "running") m.cancelled = Date.now();
    return json(scanView(m));
  }
  // OS update runs: a run finishes at once, each host "upgrades" 3 packages.
  // ponytail: module memory, not reset by __reset.
  if (p === "ssh/os-updates/runs" && method === "POST") {
    const body = await readJSON<{ kind: OSUpdateRun["kind"]; host_ids: number[]; reboot: boolean; concurrency: number; backup?: boolean; keep_backup?: boolean;
      backup_include?: string[]; backup_exclude?: string[]; update_first?: boolean; allow_no_backup?: boolean }>(request);
    const backup = body.kind === "backup" || (body.kind !== "check" && !!body.backup);
    if (body.kind === "backup") { body.keep_backup = true; body.reboot = false; }
    const id = mockOSRuns.length + 1;
    const now = new Date().toISOString();
    const results: OSUpdateResult[] = body.host_ids.map((hid) => {
      const h = db.hosts.find((x) => x.id === hid)!;
      const check = body.kind === "check";
      if (body.kind === "backup") return { run_id: id, host_id: hid, host_nickname: h.nickname, host_slug: h.oficial_slug, status: "success" as const,
        os_before: h.os?.pretty, reboot_required: false, rebooted: false, started_at: now, finished_at: now,
        backup_key: `bridge/os-backups/${h.oficial_slug}/run${id}.tar.zst`, backup_size: 3_400_000_000, backup_status: "done" as const,
        backup_include: body.backup_include?.length ? body.backup_include : profileView(hid).effective.include,
        backup_exclude: body.backup_exclude?.length ? body.backup_exclude : profileView(hid).effective.exclude };
      const eff = profileView(hid).effective;
      const release = body.kind === "release";
      return { run_id: id, host_id: hid, host_nickname: h.nickname, host_slug: h.oficial_slug, pkg_manager: "apt",
        ...(release ? { status: "failed" as const, reason: "do-release-upgrade failed; dpkg could not configure: proftpd-core (installed proftpd-core package post-installation script subprocess returned error exit status 1)" }
          : { status: "success" as const }),
        ...(check ? {} : { report: mockReport(release) }),
        os_before: h.os?.pretty, os_after: h.os?.pretty, upgradable: check ? 3 : 0, security_upgradable: check ? 1 : 0, upgraded: check ? undefined : 3,
        reboot_required: !check && !body.reboot, rebooted: !check && body.reboot, output: "@@pm apt\n@@upgradable 3\n@@security 1\n", started_at: now, finished_at: now,
        ...(backup ? { backup_key: `bridge/os-backups/${h.oficial_slug}/run${id}.tar.zst`, backup_size: 3_400_000_000, backup_status: body.keep_backup ? "done" as const : "deleted" as const,
          backup_include: body.backup_include?.length ? body.backup_include : eff.include, backup_exclude: body.backup_exclude?.length ? body.backup_exclude : eff.exclude } : {}) };
    });
    const run: OSUpdateRun = { id, kind: body.kind, reboot: body.reboot, concurrency: body.concurrency, status: "done", requested_name: "Mock",
      backup, keep_backup: backup && !!body.keep_backup, update_first: body.kind === "release" && !!body.update_first,
      backup_include: body.backup_include?.length ? body.backup_include : undefined, backup_exclude: body.backup_exclude?.length ? body.backup_exclude : undefined,
      created_at: now, finished_at: now, counts: body.kind === "release" ? { failed: results.length } : { success: results.length }, results };
    mockOSRuns.unshift(run);
    return json(run, 202);
  }
  if (p === "ssh/os-updates/restores" && method === "POST") {
    const body = await readJSON<{ run_id: number; host_id: number; mode: "staging" | "paths"; paths: string[] }>(request);
    const src = mockOSRuns.find((r) => r.id === body.run_id)?.results?.find((x) => x.host_id === body.host_id);
    if (!src || src.backup_status !== "done") return json({ error: "no backup kept for this host in this run" }, 404);
    const id = mockOSRuns.length + 1;
    const now = new Date().toISOString();
    const inPlace = body.mode === "paths";
    const result: OSUpdateResult = { ...src, run_id: id, status: "success", reason: undefined, upgradable: undefined, security_upgradable: undefined, upgraded: undefined,
      rebooted: false, reboot_required: false, output: `@@restored_to ${inPlace ? "/" : `/var/lib/bridge-restore/run-${id}`}\n`, started_at: now, finished_at: now,
      ...(inPlace ? { backup_key: `bridge/os-backups/${src.host_slug}/run${id}.tar.zst`, backup_size: 52_000_000, backup_status: "done" as const }
        : { backup_key: undefined, backup_size: undefined, backup_status: undefined }) };
    const run: OSUpdateRun = { id, kind: "restore", reboot: false, concurrency: 1, status: "done", requested_name: "Mock", backup: inPlace, keep_backup: inPlace,
      restore_mode: body.mode, restore_paths: body.paths, source_run_id: body.run_id, source_host_id: body.host_id,
      created_at: now, finished_at: now, counts: { success: 1 }, results: [result] };
    mockOSRuns.unshift(run);
    return json(run, 202);
  }
  if (p === "ssh/os-updates/config" && method === "GET") return json({ backup_available: true, backup_include: MOCK_BACKUP_DEFAULTS.include,
    backup_exclude: MOCK_BACKUP_DEFAULTS.exclude, retention_days: MOCK_BACKUP_DEFAULTS.retention_days, backup_prefix: MOCK_BACKUP_DEFAULTS.prefix });
  if (p === "ssh/os-updates/backups" && method === "GET") {
    const hostId = Number(qs.get("host_id") ?? 0);
    const kept = mockOSRuns.flatMap((r) => (r.results ?? []).filter((x) => x.backup_status === "done" && (!hostId || x.host_id === hostId))
      .map((x) => ({ ...x, output: undefined, run_kind: r.kind,
        expires_at: x.finished_at ? new Date(Date.parse(x.finished_at) + 30 * 86_400_000).toISOString() : undefined })));
    if (hostId) return json(kept);
    const newest = new Map<number, (typeof kept)[number]>();
    for (const x of kept) if (!newest.has(x.host_id)) newest.set(x.host_id, x);
    return json([...newest.values()]);
  }
  const hostConfigPath = p.match(/^ssh\/host-config\/([^/]+)$/);
  if (hostConfigPath && method === "GET") {
    const h = db.hosts.find((x) => x.oficial_slug === hostConfigPath[1]);
    if (!h) return notFound("host not found");
    return json({ config: `Host ${h.oficial_slug}\n    HostName ${h.hostname}\n    User ${h.user}\n`, host_key: mockHostKeys.get(h.id) ?? null });
  }
  const hostKeyResetPath = p.match(/^hosts\/([^/]+)\/ssh-host-key\/reset$/);
  if (hostKeyResetPath && method === "POST") {
    const h = db.hosts.find((x) => x.oficial_slug === hostKeyResetPath[1]);
    if (!h) return notFound("host not found");
    mockHostKeys.delete(h.id);
    return json({ status: "reset" });
  }
  if (p === "ssh/host-keys/duplicates" && method === "GET") {
    const groups = new Map<string, { fingerprint: string; key_type: string; hosts: { id: number; oficial_slug: string; nickname: string }[] }>();
    for (const h of db.hosts) {
      const k = mockHostKeys.get(h.id);
      if (!k) continue;
      const g = groups.get(k.fingerprint) ?? { fingerprint: k.fingerprint, key_type: k.key_type, hosts: [] };
      g.hosts.push({ id: h.id, oficial_slug: h.oficial_slug, nickname: h.nickname });
      groups.set(k.fingerprint, g);
    }
    return json([...groups.values()].filter((g) => g.hosts.length > 1));
  }
  if (p === "ssh/custom-operations" && method === "GET") return json(paginate([], null, null));
  const profilePath = p.match(/^hosts\/([^/]+)\/backup-profile$/);
  if (profilePath) {
    const h = db.hosts.find((x) => x.oficial_slug === profilePath[1]);
    if (!h) return notFound("host not found");
    if (method === "PUT") {
      const body = await readJSON<{ include?: string[]; exclude?: string[] }>(request);
      const cur = { ...mockProfiles.get(h.id) };
      if (body.include) cur.include = body.include.length ? body.include : undefined;
      if (body.exclude) cur.exclude = body.exclude.length ? body.exclude : undefined;
      mockProfiles.set(h.id, cur);
    }
    return json(profileView(h.id));
  }
  // A host's page: just what its Overview and Manutenção tabs need.
  const hostPath = p.match(/^hosts\/([^/]+)$/);
  if (hostPath && method === "GET") {
    const h = db.hosts.find((x) => x.oficial_slug === hostPath[1]);
    if (!h) return notFound("host not found");
    return json({ host: h, tags: [], responsaveis: [], chamados: [], dns_records: [], services: [], projects: [], entidades: [],
      last_scan: { id: 1, data: mockScan(h), scanned_at: new Date().toISOString() } });
  }
  const backupPath = p.match(/^ssh\/os-updates\/runs\/(\d+)\/hosts\/(\d+)\/backup$/);
  if (backupPath) {
    const x = mockOSRuns.find((r) => r.id === Number(backupPath[1]))?.results?.find((r) => r.host_id === Number(backupPath[2]));
    if (!x || x.backup_status !== "done") return json({ error: "no backup for this host in this run" }, 404);
    if (method === "DELETE") { x.backup_status = "deleted"; return json({ status: "deleted" }); }
    return json({ url: `data:text/plain,mock backup ${x.backup_key}`, key: x.backup_key, size: x.backup_size, expires_in: 3600 });
  }
  if (p === "ssh/os-updates/runs" && method === "GET") {
    const hostId = Number(qs.get("host_id") ?? 0);
    return json(mockOSRuns.filter((r) => !hostId || r.results?.some((x) => x.host_id === hostId)).map((r) => ({ ...r, results: undefined })));
  }
  if (p.startsWith("ssh/os-updates/runs/") && method === "GET") {
    const run = mockOSRuns.find((r) => r.id === Number(p.split("/")[3]));
    return run ? json(run) : json({ error: "run not found" }, 404);
  }
  if (p === "ssh/os-updates/latest" && method === "GET") {
    const seen = new Map<number, OSUpdateResult>();
    for (const r of mockOSRuns) for (const x of r.results ?? []) if (!seen.has(x.host_id)) seen.set(x.host_id, { ...x, output: undefined });
    return json([...seen.values()]);
  }
  if (method === "GET" && p === "hosts") return json(paginate(db.hosts, qs.get("page"), qs.get("per_page")));
  if (method === "GET" && p === "services") return json(paginate(db.services, qs.get("page"), qs.get("per_page")));
  if (method === "GET" && p === "dns") return json(paginate(db.dns, qs.get("page"), qs.get("per_page")));
  // Literal route first: "cert-scan" must not be read as a dns id.
  if (method === "POST" && p === "dns/cert-scan") {
    // ponytail: the mock probes every record's cert too; only has_https ones are counted.
    const observed = db.dns.map(mockCertScan);
    const scanned = observed.filter((d) => d.has_https);
    const failed = scanned.filter((d) => d.cert_error).length;
    const online = observed.filter((d) => d.obs_status === "online").length;
    return json({ scanned: scanned.length, ok: scanned.length - failed, failed, observed: observed.length, online });
  }
  if (method === "GET" && segs[0] === "dns" && segs.length === 2) {
    const record = db.dns.find((d) => d.id === Number(segs[1]));
    return record
      ? json({ dns_record: record, tags: record.tags ?? [], host_ids: record.host_ids ?? [], service_ids: record.service_ids ?? [], project_ids: record.project_ids ?? [], responsaveis: [] })
      : notFound("DNS record not found");
  }
  // ponytail: links live on the row itself; grants/responsaveis are ignored by the mock.
  if (method === "POST" && p === "dns") {
    if (currentUser().role === "viewer") return json({ error: "forbidden" }, 403);
    const body = await readJSON<Partial<DNSRecord>>(request);
    const now = new Date().toISOString();
    const record: DNSRecord = {
      id: Math.max(0, ...db.dns.map((d) => d.id)) + 1,
      domain: "", has_https: false, situacao: "active", responsavel: "", observacoes: "",
      created_at: now, updated_at: now,
      ...pickDNS(body),
    };
    db.dns.push(record);
    return json(record, 201);
  }
  if (method === "PUT" && segs[0] === "dns" && segs.length === 2) {
    if (currentUser().role === "viewer") return json({ error: "forbidden" }, 403);
    const record = db.dns.find((d) => d.id === Number(segs[1]));
    if (!record) return notFound("DNS record not found");
    Object.assign(record, pickDNS(await readJSON<Partial<DNSRecord>>(request)), { updated_at: new Date().toISOString() });
    return json(record);
  }
  // Mirrors /api/proxmox/*. ponytail: static per-server summary, no mock
  // cluster — seed Proxmox guests into db.hosts if the UI ever needs them.
  if (segs[0] === "proxmox") {
    if (currentUser().role !== "admin") return json({ error: "forbidden" }, 403);
    const view = ({ token, ...s }: (typeof db.proxmoxServers)[number]) => ({ ...s, has_token: !!token });
    type Body = Partial<{ name: string; base_url: string; token_id: string; token_secret: string; skip_verify: boolean; enabled: boolean }>;
    const apply = (s: (typeof db.proxmoxServers)[number], b: Body) => {
      const { token_secret, ...rest } = b;
      Object.assign(s, rest, { updated_at: new Date().toISOString() });
      if (token_secret?.trim() && token_secret !== "••••••••") s.token = token_secret;
    };
    if (method === "POST" && p === "proxmox/test") return json({ success: true, version: "8.2.4" });
    if (method === "POST" && p === "proxmox/sync") {
      const servers = db.proxmoxServers.filter((s) => s.enabled);
      if (!servers.length) return json({ error: "no Proxmox server enabled" }, 400);
      return json({ servers: servers.map((s) => ({ server_id: s.id, name: s.name, found: db.hosts.length, created: 0, updated: db.hosts.length, deactivated: 0, no_ip: 0 })) });
    }
    if (p === "proxmox/servers" && method === "GET") return json(paginate(db.proxmoxServers.map(view), qs.get("page"), qs.get("per_page")));
    if (p === "proxmox/servers" && method === "POST") {
      const b = await readJSON<Body>(request);
      if (!b.name?.trim() || !b.base_url?.trim() || !b.token_id?.trim() || !b.token_secret?.trim()) return json({ error: "name, base_url, token_id and token_secret are required" }, 400);
      if (db.proxmoxServers.some((s) => s.name === b.name)) return json({ error: "could not add the Proxmox server (name already in use?)" }, 409);
      const now = new Date().toISOString();
      const s = { id: Math.max(0, ...db.proxmoxServers.map((x) => x.id)) + 1, name: "", base_url: "", token_id: "", has_token: true, skip_verify: false, enabled: true, created_at: now, updated_at: now };
      apply(s, b);
      db.proxmoxServers.push(s);
      return json(view(s), 201);
    }
    if (segs[1] === "servers" && segs.length === 3) {
      const s = db.proxmoxServers.find((x) => x.id === Number(segs[2]));
      if (method === "DELETE") {
        db.proxmoxServers = db.proxmoxServers.filter((x) => x !== s);
        return json({ status: "deleted" });
      }
      if (!s) return notFound("Proxmox server not found");
      if (method === "PUT") {
        apply(s, await readJSON<Body>(request));
        return json(view(s));
      }
    }
  }
  // Mirrors POST /api/coolify/dns-sync: idempotent — a second run creates nothing.
  if (method === "POST" && p === "coolify/dns-sync") {
    if (currentUser().role !== "admin") return json({ error: "forbidden" }, 403);
    const found = [
      { domain: "bridge.sead.pi.gov.br", has_https: true, host: 1 },
      { domain: "grafana.10.0.0.12.sslip.io", has_https: false, host: 0 },
    ];
    let created = 0, linksAdded = 0, noHost = 0;
    for (const f of found) {
      if (!f.host) noHost++;
      if (db.dns.some((d) => d.domain === f.domain)) continue;
      const now = new Date().toISOString();
      db.dns.push({
        id: Math.max(0, ...db.dns.map((d) => d.id)) + 1,
        domain: f.domain, has_https: f.has_https, situacao: "active", responsavel: "",
        observacoes: "Coolify: mock", created_at: now, updated_at: now,
        host_ids: f.host ? [f.host] : [],
      });
      created++;
      if (f.host) linksAdded++;
    }
    return json({ found: found.length, created, existing: found.length - created, links_added: linksAdded, no_host: noHost });
  }
  if (method === "POST" && segs[0] === "dns" && segs.length === 3 && segs[2] === "cert-scan") {
    const record = db.dns.find((d) => d.id === Number(segs[1]));
    return record ? json(mockCertScan(record)) : notFound("DNS record not found");
  }
  if (method === "GET" && p === "projects") return json(paginate(db.projects, qs.get("page"), qs.get("per_page")));
  // ── Atlas databases ──────────────────────────────────────────────────
  if (segs[0] === "databases") {
    const live = () => db.databases.filter((d) => !d.deleted_at);
    if (method === "GET" && p === "databases") {
      const inst = Number(qs.get("instance_service_id")), svc = Number(qs.get("service_id")), proj = Number(qs.get("project_id"));
      const rows = live().filter((d) => (!inst || d.instance_service_id === inst) && (!svc || d.consumer_service_ids.includes(svc)) &&
        (!proj || d.project_ids.includes(proj)));
      return json(paginate(rows, qs.get("page"), qs.get("per_page")));
    }
    if (method === "GET" && p === "databases/trash") return json(paginate(db.databases.filter((d) => d.deleted_at), null, null));
    if (method === "GET" && p === "databases/instances") {
      const rows = db.services.filter((s) => ["database", "cache"].includes(s.service_type) || s.service_kind === "database").map((s) => {
        const h = db.hosts.find((x) => (s.host_ids ?? []).includes(x.id));
        const image = s.container_image || s.technology_stack;
        return {
          id: s.id, nickname: s.nickname, service_subtype: s.service_subtype, container_name: s.container_name, container_image: image,
          container_status: s.container_status, port: s.port, host_id: h?.id ?? null, host_nickname: h?.nickname ?? "", host_slug: h?.oficial_slug ?? "",
          engine: /redis/i.test(image) ? "redis" : /postgres/i.test(image) ? "postgres" : "other", service_kind: s.service_kind ?? "",
          coolify_resource_type: "", registered_count: live().filter((d) => d.instance_service_id === s.id).length,
        };
      });
      return json(paginate(rows, null, null));
    }
    const canWrite = currentUser().role !== "viewer";
    if (method === "POST" && p === "databases") {
      if (!canWrite) return json({ error: "forbidden" }, 403);
      const body = await readJSON<Partial<Database>>(request);
      const now = new Date().toISOString();
      const row: Database = {
        id: Math.max(0, ...db.databases.map((d) => d.id)) + 1, name: "", db_name: "", description: "", use_cases: "", engine: "postgres",
        instance_service_id: null, external_provider: "", host: "", port: null, connection_mode: "none", tunnel_host_id: null, tunnel_target: "",
        tls_mode: "prefer", has_credential: false, sensitivity: "interno", environment: "prod", backup_policy: "", backup_retention_days: null,
        last_backup_at: null, created_by: currentUser().id, created_at: now, updated_at: now, consumer_service_ids: [], project_ids: [], responsaveis: [],
        ...body,
      };
      db.databases.push(row);
      return json(row, 201);
    }
    const id = Number(segs[1]);
    const row = db.databases.find((d) => d.id === id);
    if (segs.length === 3 && segs[2] === "restore" && method === "POST") {
      if (!row) return notFound("database not found in trash");
      row.deleted_at = null;
      return json({ status: "restored" });
    }
    if (segs.length === 2 && row && !row.deleted_at) {
      if (method === "GET") return json(row);
      if (method === "PUT") {
        if (!canWrite) return json({ error: "forbidden" }, 403);
        Object.assign(row, await readJSON<Partial<Database>>(request), { updated_at: new Date().toISOString() });
        return json(row);
      }
      if (method === "DELETE") {
        row.deleted_at = new Date().toISOString();
        return new Response(null, { status: 204 });
      }
    }
    return notFound("database not found");
  }
  if (method === "GET" && p === "api-catalog") {
    const svc = Number(qs.get("service_id")), proj = Number(qs.get("project_id"));
    const rows = db.apiCatalogs.filter((a) => (!svc || a.service_ids.includes(svc)) && (!proj || a.project_ids.includes(proj)));
    return json(paginate(rows, qs.get("page"), qs.get("per_page")));
  }
  if (method === "GET" && p === "tools") return json(paginate(db.tools, qs.get("page"), qs.get("per_page")));

  // ── Catalog search ───────────────────────────────────────────────────
  if (method === "GET" && p === "catalog/search") {
    const hits = searchCatalog((qs.get("q") ?? "").trim(), qs.get("kind") ?? "all");
    return json(paginate(sortHits(hits, qs.get("sort"), qs.get("dir")), qs.get("page"), qs.get("per_page")));
  }

  // ── Offerings ────────────────────────────────────────────────────────
  // Soft-deleted offerings vanish from every list but stay in db.offerings so
  // offeringFor() keeps hydrating the requests that reference them.
  if (method === "GET" && p === "offerings") {
    let rows = liveOfferings();
    const active = qs.get("active");
    if (active != null) rows = rows.filter((o) => o.is_active === (active === "true"));
    const category = qs.get("category");
    if (category) rows = rows.filter((o) => o.category === category);
    const q = qs.get("q");
    if (q) rows = rows.filter((o) => textMatch(q, [o.name, o.description, o.category, ...(o.use_cases ?? [])]));
    return json(paginate(rows, qs.get("page"), qs.get("per_page")));
  }
  if (method === "POST" && p === "offerings") {
    if (currentUser().role === "viewer") return json({ error: "forbidden: catalog.manage required" }, 403);
    const body = await readJSON<OfferingBody>(request);
    const now = new Date().toISOString();
    const offering: Offering = {
      id: Math.max(0, ...db.offerings.map((o) => o.id)) + 1,
      slug: "", name: "", category: "", description: "", request_type: "support",
      form_schema: { fields: [] }, approver_entidade_id: null, glpi_mode: "inherit",
      is_active: true, sort_order: db.offerings.length + 1,
      created_at: now, updated_at: now,
      ...pickOffering(body),
    };
    if (!offering.slug) offering.slug = slugify(offering.name);
    const invalid = validateOffering(offering);
    if (invalid) return invalid;
    db.offerings.push(offering);
    return json(offering, 201);
  }
  if (segs[0] === "offerings" && segs.length === 2) {
    const id = Number(segs[1]);
    const offering = liveOfferings().find((o) => o.id === id);
    if (!offering) return notFound(`offering ${id} not found`);
    if (method === "GET") return json(offering);
    if (method === "PUT") {
      if (currentUser().role === "viewer") return json({ error: "forbidden: catalog.manage required" }, 403);
      const body = await readJSON<OfferingBody>(request);
      const next: Offering = { ...offering, ...pickOffering(body), updated_at: new Date().toISOString() };
      if (!next.slug) next.slug = slugify(next.name);
      const invalid = validateOffering(next);
      if (invalid) return invalid;
      Object.assign(offering, next);
      return json(offering);
    }
    if (method === "DELETE") {
      if (currentUser().role !== "admin") return json({ error: "forbidden: admin required" }, 403);
      offering.deleted_at = new Date().toISOString();
      return json({ status: "ok" });
    }
  }

  // ── AI assist (proxied to the real backend; see GO_API above) ────────
  if (method === "GET" && p === "ai/status") {
    // Availability is a question, never an error. Backend down, not signed in,
    // LLM unconfigured — all mean the same thing to the caller ("no AI"), and
    // answering 401 here would log a console error on every form open for a
    // capability the page is perfectly happy to do without.
    const unavailable = { enabled: false, configured: false, model: "" };
    try {
      const upstream = await goFetch("/api/ai/status", { method: "GET" }, request);
      if (!upstream.ok) return json(unavailable);
      return json(await upstream.json());
    } catch {
      return json(unavailable);
    }
  }

  if (method === "POST" && p === "ai/assist/request-form") {
    const body = await readJSON<{ offering_id: number; description: string }>(request);
    const offering = db.offerings.find((o) => o.id === body.offering_id);
    if (!offering) return notFound(`offering ${body.offering_id} not found`);
    if (!body.description?.trim()) return json({ error: "description is required" }, 400);

    const fields = offering.form_schema.fields;
    // The schema IS the prompt: whatever an admin authored is what the model is
    // asked to fill, so this needs no update when offerings change.
    const spec = fields.map((f) => ({
      key: f.key, type: f.type, required: f.required, label: f.label_pt,
      ...(f.options ? { options: f.options } : {}),
      ...(f.min != null ? { min: f.min } : {}), ...(f.max != null ? { max: f.max } : {}),
      ...(f.pattern ? { pattern: f.pattern } : {}),
    }));

    const prompt = [
      `Você preenche formulários de solicitação de TI do governo do Piauí.`,
      `Oferta: "${offering.name}" — ${offering.description}`,
      `Campos (JSON): ${JSON.stringify(spec)}`,
      ``,
      `Pedido do usuário: "${body.description.trim()}"`,
      ``,
      `Responda APENAS com um objeto JSON, sem texto ao redor e sem cerca de markdown:`,
      `{"title": string, "priority": "low"|"medium"|"high"|"critical", "form_data": {chave: valor}}`,
      `- title: uma linha objetiva, em português, descrevendo o pedido.`,
      `- Tipos: text/textarea/date => string; number => número; checkbox => booleano; select => exatamente uma das options; tags => array de strings.`,
      `- Campos do tipo asset_ref: NÃO preencha.`,
      `- Se o pedido não der base para um campo, OMITA esse campo. Nunca invente hostnames, domínios, IPs ou nomes de sistemas.`,
    ].join("\n");

    let completion: string;
    try {
      const upstream = await goFetch("/api/ai/chat", { method: "POST", body: JSON.stringify({ message: prompt }) }, request);
      if (!upstream.ok) {
        const err = await upstream.json().catch(() => ({}));
        return json({ error: (err as { error?: string }).error || "AI request failed" }, upstream.status);
      }
      completion = ((await upstream.json()) as { response?: string }).response ?? "";
    } catch {
      return json({ error: "AI backend unreachable" }, 503);
    }

    const parsed = extractJSON(completion);
    if (!parsed) return json({ error: "AI returned no usable JSON" }, 502);

    const priority = coercePriority(parsed.priority);
    const form_data = coerceToSchema(fields, parsed.form_data);
    return json({
      title: typeof parsed.title === "string" ? parsed.title.slice(0, 200) : "",
      priority,
      form_data,
      filled_keys: Object.keys(form_data),
    });
  }

  // ── Service requests ─────────────────────────────────────────────────
  if (method === "GET" && p === "service-requests") {
    const actor = currentUser();
    let rows = db.requests.filter((r) => requestVisible(r));
    const view = qs.get("view");
    if (view === "mine") rows = rows.filter((r) => r.requester_user_id === actor.id);
    else if (view === "approve") rows = rows.filter((r) => abilitiesForRequest(r).approve);
    else if (view === "fulfill") rows = rows.filter((r) => abilitiesForRequest(r).fulfill);
    const status = qs.get("status");
    if (status) rows = rows.filter((r) => r.status === status);
    const offeringId = qs.get("offering_id");
    if (offeringId) rows = rows.filter((r) => r.offering_id === Number(offeringId));
    const q = qs.get("q");
    if (q) rows = rows.filter((r) => textMatch(q, [r.title]));
    return json(paginate(rows.map(hydrateRequest), qs.get("page"), qs.get("per_page")));
  }
  if (method === "POST" && p === "service-requests") {
    const body = await readJSON<CreateRequestBody>(request);
    const offering = db.offerings.find((o) => o.id === body.offering_id);
    if (!offering) return notFound(`offering ${body.offering_id} not found`);
    const actor = currentUser();
    const now = new Date().toISOString();
    const req: ServiceRequest = {
      id: Math.max(0, ...db.requests.map((r) => r.id)) + 1,
      offering_id: offering.id,
      title: body.title,
      status: "submitted",
      priority: body.priority ?? "medium",
      form_data: body.form_data ?? {},
      form_schema_snapshot: offering.form_schema,
      requester_user_id: actor.id,
      // Contact defaults to the actor but is whatever the form sent — the
      // request still belongs to actor.id either way.
      contact_name: body.contact_name?.trim() || actor.display_name,
      contact_phone: body.contact_phone?.trim() || "",
      // Honour the caller's choice ONLY if it is one of their own entidades —
      // otherwise fall back to the primary. Mirrors the membership check B4/B5
      // must perform: filing under an entidade you don't belong to would widen
      // who can see the request.
      requester_entidade_id:
        (body.requester_entidade_id != null && actor.entidades?.some((e) => e.id === body.requester_entidade_id)
          ? body.requester_entidade_id
          : actor.entidades?.[0]?.id) ?? null,
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
      created_at: now,
      updated_at: now,
    };
    db.requests.push(req);
    db.events.push({
      id: Math.max(0, ...db.events.map((e) => e.id)) + 1,
      request_id: req.id, user_id: actor.id, user_name: actor.display_name,
      kind: "status", body: "Solicitação criada", from_status: "", to_status: "submitted", created_at: now,
    });
    return json(hydrateRequest(req), 201);
  }

  if (segs[0] === "service-requests" && segs.length >= 2) {
    const id = Number(segs[1]);
    const req = db.requests.find((r) => r.id === id);
    const visible = !!req && requestVisible(req);

    if (segs.length === 2) {
      if (method === "GET") {
        if (!req || !visible) return notFound(`service request ${id} not found`);
        const offering = db.offerings.find((o) => o.id === req.offering_id);
        const reqEvents = db.events.filter((e) => e.request_id === id).sort((a, b) => a.created_at.localeCompare(b.created_at));
        return json({ request: hydrateRequest(req), offering, events: reqEvents, can: abilitiesForRequest(req) });
      }
      if (method === "PATCH") {
        if (!req || !visible) return notFound(`service request ${id} not found`);
        if (!abilitiesForRequest(req).edit) return json({ error: "forbidden: cannot edit this request" }, 403);
        const body = await readJSON<PatchRequestBody>(request);
        if (body.title != null) req.title = body.title;
        if (body.priority != null) req.priority = body.priority;
        if (body.form_data != null) req.form_data = body.form_data;
        req.updated_at = new Date().toISOString();
        return json(hydrateRequest(req));
      }
    }

    if (segs.length === 3 && segs[2] === "transition" && method === "POST") {
      if (!req || !visible) return notFound(`service request ${id} not found`);
      const body = await readJSON<TransitionBody>(request);
      if (!canTransition(req.status, body.to)) {
        return json({ error: `illegal transition from "${req.status}" to "${body.to}"` }, 422);
      }
      const actor = currentUser();
      const now = new Date().toISOString();
      const from = req.status;
      req.status = body.to;
      req.updated_at = now;
      if (body.assignee_user_id != null) req.assignee_user_id = body.assignee_user_id;
      if (body.to === "approved" || body.to === "rejected") {
        req.decided_by_user_id = actor.id;
        req.decided_at = now;
      }
      if (body.to === "delivered") {
        req.delivered_by_user_id = actor.id;
        req.delivered_at = now;
        if (body.fulfilled_asset_type) req.fulfilled_asset_type = body.fulfilled_asset_type;
        if (body.fulfilled_asset_id != null) req.fulfilled_asset_id = body.fulfilled_asset_id;
      }
      db.events.push({
        id: Math.max(0, ...db.events.map((e) => e.id)) + 1,
        request_id: id, user_id: actor.id, user_name: actor.display_name,
        kind: "status", body: body.note ?? "", from_status: from, to_status: body.to, created_at: now,
      });
      return json(hydrateRequest(req));
    }

    if (segs.length === 3 && segs[2] === "comments" && method === "POST") {
      if (!req || !visible) return notFound(`service request ${id} not found`);
      const body = await readJSON<CommentBody>(request);
      const actor = currentUser();
      const now = new Date().toISOString();
      const event: RequestEvent = {
        id: Math.max(0, ...db.events.map((e) => e.id)) + 1,
        request_id: id, user_id: actor.id, user_name: actor.display_name,
        kind: "comment", body: body.body, from_status: "", to_status: "", created_at: now,
      };
      db.events.push(event);
      return json(event, 201);
    }

    if (segs.length === 4 && segs[2] === "glpi" && segs[3] === "refresh" && method === "POST") {
      if (!req || !visible) return notFound(`service request ${id} not found`);
      if (req.external_source === "glpi") req.cached_at = new Date().toISOString();
      return json(hydrateRequest(req));
    }
  }

  console.warn(`mock: no handler for ${method} /api/${p}`);
  return notFound(`mock: no handler for ${method} /api/${p}`);
}

type RouteContext = { params: Promise<{ path: string[] }> };

async function handle(method: string, request: NextRequest, ctx: RouteContext): Promise<Response> {
  const { path } = await ctx.params;
  return dispatch(method, request, path ?? []);
}

export async function GET(request: NextRequest, ctx: RouteContext) { return handle("GET", request, ctx); }
export async function POST(request: NextRequest, ctx: RouteContext) { return handle("POST", request, ctx); }
export async function PUT(request: NextRequest, ctx: RouteContext) { return handle("PUT", request, ctx); }
export async function PATCH(request: NextRequest, ctx: RouteContext) { return handle("PATCH", request, ctx); }
export async function DELETE(request: NextRequest, ctx: RouteContext) { return handle("DELETE", request, ctx); }
