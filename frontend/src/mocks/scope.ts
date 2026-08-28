// Pure visibility + ability predicates. Deliberately free of any
// cross-directory *value* import.
//
// ponytail: src/lib/entidades.ts already has an equivalent `descendantIds`,
// and reusing it was the first draft here — but this file has to load
// cleanly both under tsc (moduleResolution "bundler" without
// allowImportingTsExtensions rejects a ".ts"-suffixed relative specifier)
// and under plain `node --test` (which requires exactly that suffix to
// resolve a relative specifier at all — no tsconfig, no bundler). Since I
// can't touch tsconfig.json, no import spelling satisfies both loaders at
// once. `import type` is erased entirely by Node's type-stripping and never
// hits this wall (see the `@/lib/types` import below), so only this one
// small closure is inlined rather than imported. If lib/entidades.ts's
// version changes shape, mirror it here too.
import type { Entidade, User, RequestAbilities, RequestStatus } from "@/lib/types";

export interface Actor {
  id: number;
  role: User["role"];
  entidadeIds: number[];
}

export function toActor(user: User): Actor {
  return { id: user.id, role: user.role, entidadeIds: (user.entidades ?? []).map((e) => e.id) };
}

function descendantIds(entidades: Entidade[], roots: Iterable<number>): Set<number> {
  const out = new Set<number>(roots);
  // Parents come first in the seed/API order, so one forward pass closes the set.
  for (const e of entidades) {
    if (e.parent_id != null && out.has(e.parent_id)) out.add(e.id);
  }
  return out;
}

/** The acting user's visible set: their entidade memberships plus every
 *  descendant ("above sees all that below sees"). Admin's scope is every
 *  entidade — this also covers the "no approver assigned ⇒ admin sees it
 *  anyway" edge case in computeAbilities below, since it's a real id set,
 *  not a boolean short-circuit. */
export function visibleScope(entidades: Entidade[], actor: Actor): Set<number> {
  if (actor.role === "admin") return new Set(entidades.map((e) => e.id));
  return descendantIds(entidades, actor.entidadeIds);
}

// visible(request) =
//      request.requester_user_id === me.id
//   || scope.includes(request.requester_entidade_id)
//   || scope.includes(offeringOf(request).approver_entidade_id)
export function isVisible(
  entidades: Entidade[],
  actor: Actor,
  request: { requester_user_id: number; requester_entidade_id?: number | null },
  offeringApproverEntidadeId?: number | null,
): boolean {
  if (actor.role === "admin") return true;
  if (request.requester_user_id === actor.id) return true;
  const scope = visibleScope(entidades, actor);
  if (request.requester_entidade_id != null && scope.has(request.requester_entidade_id)) return true;
  if (offeringApproverEntidadeId != null && scope.has(offeringApproverEntidadeId)) return true;
  return false;
}

/** Mirrors the RequestAbilities rules exactly (task-a2-brief.md):
 *   approve: submitted/under_review, role editor+, approver_entidade_id in scope
 *   fulfill: approved/in_progress, role editor+, approver_entidade_id in scope
 *   cancel:  non-terminal status, and (requester or admin) — non-terminal applies to admin too
 *   edit:    requester, status submitted or needs_info
 *  `isTerminal` is injected rather than imported so this file stays free of
 *  the cross-directory value import described above — db.ts supplies the
 *  real one, derived from TRANSITIONS (src/lib/requests.ts), so there is
 *  still exactly one copy of the state machine. */
export function computeAbilities(
  entidades: Entidade[],
  actor: Actor,
  request: { requester_user_id: number; status: RequestStatus },
  offeringApproverEntidadeId: number | null | undefined,
  isTerminal: (status: RequestStatus) => boolean,
): RequestAbilities {
  const roleEditorPlus = actor.role === "editor" || actor.role === "admin";
  const scope = visibleScope(entidades, actor);
  // No approver assigned ⇒ admin-only (mirrors entidades.md's "no grant rows
  // ⇒ admin-only" precedent for unassigned assets).
  const approverMatch =
    actor.role === "admin" || (offeringApproverEntidadeId != null && scope.has(offeringApproverEntidadeId));
  const approve = roleEditorPlus && approverMatch && (request.status === "submitted" || request.status === "under_review");
  const fulfill = roleEditorPlus && approverMatch && (request.status === "approved" || request.status === "in_progress");
  const isRequester = request.requester_user_id === actor.id;
  // Non-terminal for EVERYONE, admin included — TRANSITIONS[status] is []
  // for delivered/rejected/cancelled, so an admin "cancelling" one of those
  // would always 422. Every other status's TRANSITIONS entry does include
  // "cancelled" as a legal target (see scope.test.ts), so this is exactly
  // "can legally transition to cancelled", not a separate rule.
  const cancel = !isTerminal(request.status) && (actor.role === "admin" || isRequester);
  const edit = isRequester && (request.status === "submitted" || request.status === "needs_info");
  return { approve, fulfill, cancel, edit };
}
