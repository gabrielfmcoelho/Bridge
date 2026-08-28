// Run: node --test src/mocks/scope.test.ts   (Node native TS, no runner dep)
//
// Exercises the visibility predicate against small literal fixtures — same
// style as src/lib/entidades.test.ts — decoupled from the full seed.ts data
// so a scoping regression fails here even if the seed's own scenarios
// happen not to trip it.
import { test } from "node:test";
import assert from "node:assert/strict";
import { isVisible, computeAbilities, type Actor } from "./scope.ts";
import type { Entidade } from "../lib/types.ts";

const e = (id: number, name: string, parent_id: number | null): Entidade => ({
  id, name, slug: name.toLowerCase(), parent_id, description: "", created_at: "", updated_at: "",
});
// GovPI(1) > ETIPI(2) ; GovPI(1) > SEAD-PI(3) > { SGA(4), SGP(5) }
// ETIPI is a SIBLING of SEAD-PI, not an ancestor of SGA — the tree shape the
// cross-branch test below depends on.
const tree = [e(1, "GovPI", null), e(2, "ETIPI", 1), e(3, "SEAD-PI", 1), e(4, "SGA", 3), e(5, "SGP", 3)];

const viewerSGA: Actor = { id: 10, role: "viewer", entidadeIds: [4] };
const editorETIPI: Actor = { id: 20, role: "editor", entidadeIds: [2] };
const editorGovPI: Actor = { id: 30, role: "editor", entidadeIds: [1] };
const admin: Actor = { id: 40, role: "admin", entidadeIds: [] };

const ownRequest = { requester_user_id: 10, requester_entidade_id: 4 };
const sgpPeerRequest = { requester_user_id: 11, requester_entidade_id: 5 };
const sgaRequestApprovedByETIPI = { requester_user_id: 10, requester_entidade_id: 4 };

test("viewer in SGA sees their own request but not a peer's in SGP", () => {
  assert.equal(isVisible(tree, viewerSGA, ownRequest, null), true);
  assert.equal(isVisible(tree, viewerSGA, sgpPeerRequest, null), false);
});

test("editor in ETIPI sees an SGA-created request whose offering's approver is ETIPI (cross-branch)", () => {
  // ETIPI is a sibling of SEAD-PI, not an ancestor of SGA — a naive
  // ancestors-only rule would get this wrong.
  assert.equal(isVisible(tree, editorETIPI, sgaRequestApprovedByETIPI, /* approver */ 2), true);
  // Without the offering's approver in scope, ETIPI has no other route to
  // an SGA-created request.
  assert.equal(isVisible(tree, editorETIPI, sgaRequestApprovedByETIPI, null), false);
});

test("editor in GovPI sees everything below (ancestor rule)", () => {
  assert.equal(isVisible(tree, editorGovPI, sgpPeerRequest, null), true);
  assert.equal(isVisible(tree, editorGovPI, sgaRequestApprovedByETIPI, null), true);
});

test("admin sees all", () => {
  assert.equal(isVisible(tree, admin, sgpPeerRequest, null), true);
  assert.equal(isVisible(tree, admin, { requester_user_id: 999, requester_entidade_id: null }, null), true);
});

test("computeAbilities: approve requires editor+ role and approver_entidade_id in scope", () => {
  // isTerminal is irrelevant to .approve — a fixed stub is enough here; the
  // real one (derived from TRANSITIONS) is wired in by db.ts.
  const neverTerminal = () => false;
  const submitted = { requester_user_id: 10, status: "submitted" as const };
  assert.equal(computeAbilities(tree, editorETIPI, submitted, 2, neverTerminal).approve, true);
  // viewer role never approves, even with a matching scope.
  assert.equal(computeAbilities(tree, viewerSGA, { ...submitted, requester_user_id: 999 }, 4, neverTerminal).approve, false);
  // No approver assigned on the offering ⇒ admin-only.
  assert.equal(computeAbilities(tree, editorETIPI, submitted, null, neverTerminal).approve, false);
  assert.equal(computeAbilities(tree, admin, submitted, null, neverTerminal).approve, true);
});

test("computeAbilities: cancel requires a non-terminal status for EVERYONE, admin included", () => {
  // Regression for fix round 1: TRANSITIONS.delivered/rejected/cancelled are
  // all [] (terminal), so a "cancel" that ignores that would always 422 on
  // the real transition endpoint — admin's short-circuit must not skip it.
  const isTerminal = (s: string) => s === "delivered" || s === "rejected" || s === "cancelled";
  const delivered = { requester_user_id: 999, status: "delivered" as const };
  const submitted = { requester_user_id: 999, status: "submitted" as const };
  assert.equal(computeAbilities(tree, admin, delivered, null, isTerminal).cancel, false);
  assert.equal(computeAbilities(tree, admin, submitted, null, isTerminal).cancel, true);
  // Same status, non-admin requester: also gated on non-terminal, not just "is the requester".
  const requester: Actor = { id: 999, role: "viewer", entidadeIds: [] };
  assert.equal(computeAbilities(tree, requester, delivered, null, isTerminal).cancel, false);
  assert.equal(computeAbilities(tree, requester, submitted, null, isTerminal).cancel, true);
});
