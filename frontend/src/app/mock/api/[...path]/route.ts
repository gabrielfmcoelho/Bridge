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
  Host, Service, DNSRecord, Project, ApiCatalog, ExternalTool,
} from "@/lib/types";
import {
  db, resetDB, currentUser, switchUser, requestVisible, abilitiesForRequest,
  hydrateRequest, paginate,
} from "@/mocks/db";

export const dynamic = "force-dynamic";

// Single named constant so latency can be dialed to 0 for fast test runs.
const LATENCY_MS = 150;
const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

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

  // ── Discovery fixtures ──────────────────────────────────────────────
  if (method === "GET" && p === "hosts") return json(paginate(db.hosts, qs.get("page"), qs.get("per_page")));
  if (method === "GET" && p === "services") return json(paginate(db.services, qs.get("page"), qs.get("per_page")));
  if (method === "GET" && p === "dns") return json(paginate(db.dns, qs.get("page"), qs.get("per_page")));
  if (method === "GET" && p === "projects") return json(paginate(db.projects, qs.get("page"), qs.get("per_page")));
  if (method === "GET" && p === "api-catalog") return json(paginate(db.apiCatalogs, qs.get("page"), qs.get("per_page")));
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
