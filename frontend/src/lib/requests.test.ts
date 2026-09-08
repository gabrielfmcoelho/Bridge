// Run: node --test src/lib/requests.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { TRANSITIONS, canTransition } from "./requests.ts";
import type { RequestStatus } from "./types.ts";

const STATUSES: RequestStatus[] = [
  "submitted", "under_review", "approved",
  "in_progress", "delivered", "rejected", "cancelled", "needs_info",
];

test("terminal states have no transitions", () => {
  assert.deepEqual(TRANSITIONS.delivered, []);
  assert.deepEqual(TRANSITIONS.rejected, []);
  assert.deepEqual(TRANSITIONS.cancelled, []);
});

test("canTransition: submitted cannot jump straight to delivered", () => {
  assert.equal(canTransition("submitted", "delivered"), false);
});

test("canTransition: approved can move to in_progress", () => {
  assert.equal(canTransition("approved", "in_progress"), true);
});

test("every value in TRANSITIONS is itself a valid RequestStatus", () => {
  const valid = new Set<string>(STATUSES);
  for (const targets of Object.values(TRANSITIONS)) {
    for (const t of targets) assert.ok(valid.has(t), `${t} is not a valid RequestStatus`);
  }
});

// ── availableTransitions ───────────────────────────────────────────────────
// This is what drives the detail page's action buttons. Two things must hold:
// a button is never offered for an illegal move, and never offered for a move
// the server would refuse. The second half is checked against the mock's own
// computeAbilities so the UI and the authority function cannot drift apart.

import { availableTransitions, TRANSITION_ACTION_KEY, validateFormData, emptyFormValues, isFieldEmpty } from "./requests.ts";
import { computeAbilities, type Actor } from "../mocks/scope.ts";
import type { Entidade, FormField, RequestAbilities } from "./types.ts";

const ALL_ABILITY_COMBOS: RequestAbilities[] = [];
for (const approve of [true, false])
  for (const fulfill of [true, false])
    for (const cancel of [true, false])
      for (const edit of [true, false])
        ALL_ABILITY_COMBOS.push({ approve, fulfill, cancel, edit });

test("availableTransitions never offers an illegal move", () => {
  for (const from of STATUSES)
    for (const can of ALL_ABILITY_COMBOS)
      for (const to of availableTransitions(from, can))
        assert.ok(canTransition(from, to), `${from} -> ${to} is not a legal transition`);
});

test("availableTransitions offers nothing when the actor may do nothing", () => {
  const none: RequestAbilities = { approve: false, fulfill: false, cancel: false, edit: false };
  for (const from of STATUSES) assert.deepEqual(availableTransitions(from, none), []);
});

test("terminal statuses offer nothing even to an all-powerful actor", () => {
  const all: RequestAbilities = { approve: true, fulfill: true, cancel: true, edit: true };
  for (const from of ["delivered", "rejected", "cancelled"] as RequestStatus[]) {
    assert.deepEqual(availableTransitions(from, all), []);
  }
});

test("every offerable target has a button label", () => {
  const all: RequestAbilities = { approve: true, fulfill: true, cancel: true, edit: true };
  for (const from of STATUSES)
    for (const to of availableTransitions(from, all))
      assert.ok(TRANSITION_ACTION_KEY[to], `no label key for target ${to}`);
});

// GovPI(1) > ETIPI(2) ; GovPI(1) > SEAD-PI(3) > SGA(4) — ETIPI is a SIBLING
// of SEAD-PI, the cross-branch approver shape (mirrors scope.test.ts).
const tree: Entidade[] = [
  { id: 1, name: "GovPI", slug: "govpi", parent_id: null, description: "", created_at: "", updated_at: "" },
  { id: 2, name: "ETIPI", slug: "etipi", parent_id: 1, description: "", created_at: "", updated_at: "" },
  { id: 3, name: "SEAD-PI", slug: "sead-pi", parent_id: 1, description: "", created_at: "", updated_at: "" },
  { id: 4, name: "SGA", slug: "sga", parent_id: 3, description: "", created_at: "", updated_at: "" },
];
const requesterSGA: Actor = { id: 10, role: "viewer", entidadeIds: [4] };
const approverETIPI: Actor = { id: 20, role: "editor", entidadeIds: [2] };
const isTerminal = (s: RequestStatus) => TRANSITIONS[s].length === 0;

test("no button the UI shows would be refused by the mock's own authority rules", () => {
  for (const actor of [requesterSGA, approverETIPI]) {
    for (const status of STATUSES) {
      const request = { requester_user_id: 10, status };
      const can = computeAbilities(tree, actor, request, 2, isTerminal);
      for (const to of availableTransitions(status, can)) {
        // Legal move...
        assert.ok(canTransition(status, to));
        // ...and at least one ability was true, i.e. the button is not a
        // move this actor has no standing to make.
        assert.ok(can.approve || can.fulfill || can.cancel || can.edit);
      }
    }
  }
});

test("a viewer who is not the requester is offered nothing at all", () => {
  const bystander: Actor = { id: 99, role: "viewer", entidadeIds: [4] };
  for (const status of STATUSES) {
    const can = computeAbilities(tree, bystander, { requester_user_id: 10, status }, 2, isTerminal);
    assert.deepEqual(availableTransitions(status, can), [], `bystander got buttons at ${status}`);
  }
});

test("the cross-branch approver can move a request its own tree branch cannot see", () => {
  const can = computeAbilities(tree, approverETIPI, { requester_user_id: 10, status: "under_review" }, 2, isTerminal);
  assert.deepEqual(availableTransitions("under_review", can).sort(), ["approved", "needs_info", "rejected"]);
});

// ── form_schema validation (shared by the create and edit modals) ──────────

// Renders whichever interpolation var the message actually carries ({min}/{max}).
const tt = (key: string, vars?: Record<string, string>) =>
  vars ? `${key}:${Object.values(vars).join(",")}` : key;
const field = (over: Partial<FormField>): FormField =>
  ({ key: "f", type: "text", label_en: "F", label_pt: "F", required: false, ...over }) as FormField;

test("emptyFormValues gives every field a type-appropriate blank", () => {
  const vals = emptyFormValues([
    field({ key: "a", type: "text" }), field({ key: "b", type: "checkbox" }),
    field({ key: "c", type: "tags" }), field({ key: "d", type: "asset_ref" }),
  ]);
  assert.deepEqual(vals, { a: "", b: false, c: [], d: null });
});

test("an unchecked checkbox is an answer, not a blank", () => {
  assert.equal(isFieldEmpty(field({ type: "checkbox" }), false), false);
  assert.deepEqual(validateFormData([field({ type: "checkbox", required: true })], { f: false }, tt), {});
});

test("required fields are flagged, optional blanks are not", () => {
  assert.deepEqual(validateFormData([field({ required: true })], { f: "" }, tt), { f: "requests.form.required" });
  assert.deepEqual(validateFormData([field({ required: false })], { f: "" }, tt), {});
});

test("max_length, number and select constraints only apply to non-blank values", () => {
  assert.deepEqual(validateFormData([field({ max_length: 3 })], { f: "abcd" }, tt), { f: "requests.form.maxLength:3" });
  assert.deepEqual(validateFormData([field({ type: "number" })], { f: "x" }, tt), { f: "requests.form.invalidNumber" });
  assert.deepEqual(validateFormData([field({ type: "select", options: ["a"] })], { f: "b" }, tt), { f: "requests.form.invalidOption" });
  assert.deepEqual(validateFormData([field({ type: "select", options: ["a"] })], { f: "" }, tt), {});
});

// ── Rules the offering declares (min/max, pattern, depends_on) ─────────────

import { visibleFields, visibleValues } from "./requests.ts";

const numField = (over: Partial<FormField>): FormField =>
  ({ key: "n", type: "number", label_en: "N", label_pt: "N", required: false, ...over }) as FormField;

test("numeric bounds are enforced at both ends", () => {
  const f = [numField({ min: 1, max: 64 })];
  assert.deepEqual(validateFormData(f, { n: 0 }, tt), { n: "requests.form.min:1" });
  assert.deepEqual(validateFormData(f, { n: 65 }, tt), { n: "requests.form.max:64" });
  assert.deepEqual(validateFormData(f, { n: 8 }, tt), {});
});

test("pattern rejects a bad value and accepts a good one", () => {
  const f = [field({ pattern: "^[a-z][a-z0-9_]*$" })];
  assert.deepEqual(validateFormData(f, { f: "9bad-name" }, tt), { f: "requests.form.invalidFormat" });
  assert.deepEqual(validateFormData(f, { f: "portal_servidor" }, tt), {});
});

test("an unparseable authored pattern is ignored, not thrown", () => {
  // One admin typo in a regex must not take the whole form down.
  const f = [field({ pattern: "([unclosed" })];
  assert.deepEqual(validateFormData(f, { f: "anything" }, tt), {});
});

test("a hidden field is not rendered, not validated, and not submitted", () => {
  const fields: FormField[] = [
    field({ key: "ambiente", type: "select", options: ["Produção", "Homologação"], required: true }),
    field({ key: "justificativa", required: true, depends_on: { key: "ambiente", equals: ["Produção"] } }),
  ];
  const homolog = { ambiente: "Homologação", justificativa: "" };
  assert.deepEqual(visibleFields(fields, homolog).map((f) => f.key), ["ambiente"]);
  assert.deepEqual(validateFormData(fields, homolog, tt), {}, "hidden required field must not block submit");

  const prod = { ambiente: "Produção", justificativa: "" };
  assert.deepEqual(visibleFields(fields, prod).map((f) => f.key), ["ambiente", "justificativa"]);
  assert.deepEqual(validateFormData(fields, prod, tt), { justificativa: "requests.form.required" });
});

test("switching away from a condition drops the stale answer", () => {
  const fields: FormField[] = [
    field({ key: "tipo", type: "select", options: ["Simples", "SAN"], required: true }),
    field({ key: "extras", type: "tags", depends_on: { key: "tipo", equals: ["SAN"] } }),
  ];
  // Answered while SAN was selected, then switched back to Simples.
  const values = { tipo: "Simples", extras: ["a.gov.br"] };
  assert.deepEqual(visibleValues(fields, values), { tipo: "Simples" });
});
