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
import type {
  Offering, ServiceRequest, RequestEvent, RequestStatus, CatalogHit,
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
const hostHit = (h: Host): CatalogHit => ({
  kind: "asset", asset_type: "host", id: h.id, name: h.nickname, description: h.hostname,
  href: `/hosts/${h.oficial_slug}`,
});
const serviceHit = (s: Service): CatalogHit => ({
  kind: "asset", asset_type: "service", id: s.id, name: s.nickname, description: s.description,
  href: `/services/${s.id}`,
});
const dnsHit = (d: DNSRecord): CatalogHit => ({
  kind: "asset", asset_type: "dns", id: d.id, name: d.domain, description: d.observacoes,
  href: `/dns/${d.id}`,
});
const projectHit = (p: Project): CatalogHit => ({
  kind: "asset", asset_type: "project", id: p.id, name: p.name, description: p.description,
  href: `/projects/${p.id}`,
});
const apiCatalogHit = (a: ApiCatalog): CatalogHit => ({
  kind: "asset", asset_type: "api_catalog", id: a.id, name: a.name, description: a.description,
  href: `/atlas/apis/${a.id}`,
});
const toolHit = (t: ExternalTool): CatalogHit => ({
  kind: "asset", asset_type: "tool", id: t.id, name: t.name, description: t.description,
  href: "/tools",
});

function searchCatalog(q: string, kind: string): CatalogHit[] {
  const hits: CatalogHit[] = [];
  if (kind !== "asset") {
    hits.push(...db.offerings.filter((o) => o.is_active && textMatch(q, [o.name, o.description, o.category])).map(offeringHit));
  }
  if (kind !== "offering") {
    hits.push(...db.hosts.filter((h) => textMatch(q, [h.nickname, h.hostname, h.description])).map(hostHit));
    hits.push(...db.services.filter((s) => textMatch(q, [s.nickname, s.description, s.technology_stack])).map(serviceHit));
    hits.push(...db.dns.filter((d) => textMatch(q, [d.domain, d.observacoes])).map(dnsHit));
    hits.push(...db.projects.filter((p) => textMatch(q, [p.name, p.description])).map(projectHit));
    hits.push(...db.apiCatalogs.filter((a) => textMatch(q, [a.name, a.description, a.title])).map(apiCatalogHit));
    hits.push(...db.tools.filter((t) => textMatch(q, [t.name, t.description])).map(toolHit));
  }
  return hits;
}

// ── Request bodies (only what our handlers actually read) ──────────────

type OfferingBody = Partial<Offering>;
interface CreateRequestBody { offering_id: number; title: string; priority?: string; form_data?: Record<string, unknown> }
interface PatchRequestBody { title?: string; priority?: string; form_data?: Record<string, unknown> }
interface TransitionBody {
  to: RequestStatus; note?: string; assignee_user_id?: number;
  fulfilled_asset_type?: string; fulfilled_asset_id?: number;
}
interface CommentBody { body: string }
interface SwitchUserBody { role: "admin" | "editor" | "viewer"; entidade_slug: string }

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
    return json(paginate(hits, qs.get("page"), qs.get("per_page")));
  }

  // ── Offerings ────────────────────────────────────────────────────────
  if (method === "GET" && p === "offerings") {
    let rows = db.offerings;
    const active = qs.get("active");
    if (active != null) rows = rows.filter((o) => o.is_active === (active === "true"));
    const category = qs.get("category");
    if (category) rows = rows.filter((o) => o.category === category);
    const q = qs.get("q");
    if (q) rows = rows.filter((o) => textMatch(q, [o.name, o.description]));
    return json(paginate(rows, qs.get("page"), qs.get("per_page")));
  }
  if (method === "POST" && p === "offerings") {
    const body = await readJSON<OfferingBody>(request);
    const now = new Date().toISOString();
    const offering: Offering = {
      id: Math.max(0, ...db.offerings.map((o) => o.id)) + 1,
      slug: body.slug ?? "",
      name: body.name ?? "",
      category: body.category ?? "",
      description: body.description ?? "",
      request_type: body.request_type ?? "support",
      form_schema: body.form_schema ?? { fields: [] },
      approver_entidade_id: body.approver_entidade_id ?? null,
      glpi_mode: body.glpi_mode ?? "inherit",
      is_active: body.is_active ?? true,
      sort_order: body.sort_order ?? db.offerings.length + 1,
      created_at: now,
      updated_at: now,
    };
    db.offerings.push(offering);
    return json(offering, 201);
  }
  if (segs[0] === "offerings" && segs.length === 2) {
    const id = Number(segs[1]);
    const offering = db.offerings.find((o) => o.id === id);
    if (method === "GET") {
      if (!offering) return notFound(`offering ${id} not found`);
      return json(offering);
    }
    if (method === "PUT") {
      if (!offering) return notFound(`offering ${id} not found`);
      const body = await readJSON<OfferingBody>(request);
      Object.assign(offering, body, { updated_at: new Date().toISOString() });
      return json(offering);
    }
    if (method === "DELETE") {
      if (!offering) return notFound(`offering ${id} not found`);
      db.offerings = db.offerings.filter((o) => o.id !== id);
      return json({ status: "ok" });
    }
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
      requester_entidade_id: actor.entidades?.[0]?.id ?? null,
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
