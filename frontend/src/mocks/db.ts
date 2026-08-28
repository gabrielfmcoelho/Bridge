// Mock transport layer — in-memory state + the visibility predicate.
//
// This file (and everything under src/mocks/ and src/app/mock/) exists only
// so the Catalog & Requests UI can be built and clicked through before a
// single line of Go backend exists. When the real API lands, this whole
// directory is deleted and the one-line NEXT_PUBLIC_USE_MOCK_API branch in
// src/lib/api.ts reverts. Nothing under src/components or src/lib may import
// from here.
import { TRANSITIONS } from "@/lib/requests";
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
  RequestAbilities,
  RequestStatus,
} from "@/lib/types";
import { seed } from "./seed";
// The visibility + ability predicates live in ./scope.ts, which is
// deliberately free of cross-directory value imports so scope.test.ts can
// load it directly under plain `node --test` — see that file's header
// comment for why. db.ts itself is only ever loaded by tsc/webpack, so the
// `@/lib/requests` import above (needed for `isTerminal` below) is safe here.
import { isVisible, computeAbilities, toActor } from "./scope";

export interface Seed {
  entidades: Entidade[];
  users: User[];
  /** id of the user record that /api/auth/me and the visibility predicate act
   *  as. __user mutates users.find(u => u.id === actorId) in place. */
  actorId: number;
  enums: Record<string, EnumOption[]>;
  hosts: Host[];
  services: Service[];
  dns: DNSRecord[];
  projects: Project[];
  apiCatalogs: ApiCatalog[];
  tools: ExternalTool[];
  offerings: Offering[];
  requests: ServiceRequest[];
  events: RequestEvent[];
}

// ponytail: dev-singleton pin on globalThis — Fast Refresh re-evaluates this
// module on every save, so module-scope state would reset mid-session. This
// is the standard Next.js dev-singleton idiom; the narrow `any` is the
// documented exception (see task brief).
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const g = globalThis as any;
if (!g.__bridgeMockDB) g.__bridgeMockDB = seed();
export const db: Seed = g.__bridgeMockDB;

/** POST /mock/api/__reset — re-seed in place. State dies on server restart
 *  regardless; this just lets a running dev server re-seed on demand. */
export function resetDB(): void {
  Object.assign(db, seed());
}

export function currentUser(): User {
  const u = db.users.find((u) => u.id === db.actorId);
  if (!u) throw new Error("mock: actor user missing from seed");
  return u;
}

/** POST /mock/api/__user — switch the acting persona's role + entidade
 *  membership in place (same user id, so "my requests" stays traceable
 *  across a persona switch). */
export function switchUser(input: { role: User["role"]; entidade_slug: string }): User {
  const entidade = db.entidades.find((e) => e.slug === input.entidade_slug);
  if (!entidade) throw new Error(`mock: unknown entidade_slug "${input.entidade_slug}"`);
  const actor = currentUser();
  actor.role = input.role;
  actor.entidades = [{ id: entidade.id, name: entidade.name, slug: entidade.slug, is_primary: true }];
  actor.updated_at = new Date().toISOString();
  return actor;
}

// Derived once from TRANSITIONS (src/lib/requests.ts) — the single source of
// truth for the state machine. Passed into computeAbilities below rather
// than imported by scope.ts itself (see that file's header comment).
const isTerminal = (s: RequestStatus) => TRANSITIONS[s].length === 0;

// ── Wrappers bound to the live db + acting persona ──────────────────────

export function offeringFor(request: Pick<ServiceRequest, "offering_id">): Offering | undefined {
  return db.offerings.find((o) => o.id === request.offering_id);
}

export function requestVisible(request: ServiceRequest): boolean {
  return isVisible(db.entidades, toActor(currentUser()), request, offeringFor(request)?.approver_entidade_id);
}

export function abilitiesForRequest(request: ServiceRequest): RequestAbilities {
  return computeAbilities(db.entidades, toActor(currentUser()), request, offeringFor(request)?.approver_entidade_id, isTerminal);
}

/** Joins display-only fields the list/detail endpoints return
 *  (offering_name, requester_name, entidade_name) — computed on read so the
 *  seed itself never has to keep them in sync with the entities they mirror. */
export function hydrateRequest(r: ServiceRequest): ServiceRequest {
  const offering = offeringFor(r);
  const requester = db.users.find((u) => u.id === r.requester_user_id);
  const entidade = r.requester_entidade_id != null ? db.entidades.find((e) => e.id === r.requester_entidade_id) : undefined;
  return { ...r, offering_name: offering?.name, requester_name: requester?.display_name, entidade_name: entidade?.name };
}

// ── R4 list envelope + pagination (mirrors internal/api/list.go exactly:
// per_page omitted/0 ⇒ unbounded, page<=1 ⇒ page 1, per_page clamped to 200) ──

export interface ListEnvelope<T> {
  data: T[];
  meta: { page: number; per_page: number; total: number };
}

export function paginate<T>(rows: T[], pageParam: string | null, perPageParam: string | null): ListEnvelope<T> {
  let page = 1;
  if (pageParam) {
    const n = parseInt(pageParam, 10);
    if (Number.isFinite(n) && n > 1) page = n;
  }
  let perPage = 0;
  if (perPageParam) {
    const n = parseInt(perPageParam, 10);
    if (Number.isFinite(n) && n > 0) perPage = Math.min(n, 200);
  }
  const total = rows.length;
  if (perPage <= 0) return { data: rows, meta: { page: 1, per_page: total, total } };
  const start = (page - 1) * perPage;
  return { data: rows.slice(start, start + perPage), meta: { page, per_page: perPage, total } };
}
