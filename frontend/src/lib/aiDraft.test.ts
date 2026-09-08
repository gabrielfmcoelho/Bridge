// Run: node --test src/lib/aiDraft.test.ts   (Node 24 native TS, no runner dep)
//
// A model reply is untrusted input. These cover the two things that must hold:
// usable JSON is found inside whatever prose the model wrapped it in, and
// nothing survives that the offering's schema did not authorise.
import { test } from "node:test";
import assert from "node:assert/strict";
import { extractJSON, coerceToSchema, coercePriority } from "./aiDraft.ts";
import type { FormField } from "./types.ts";

const f = (over: Partial<FormField>): FormField =>
  ({ key: "k", type: "text", label_en: "K", label_pt: "K", required: false, ...over }) as FormField;

test("extractJSON digs the object out of a markdown fence", () => {
  const reply = 'Claro! Segue o JSON:\n```json\n{"title":"Nova VM","priority":"high"}\n```\nEspero ter ajudado.';
  assert.deepEqual(extractJSON(reply), { title: "Nova VM", priority: "high" });
});

test("extractJSON returns null rather than throwing on junk", () => {
  assert.equal(extractJSON("desculpe, não consigo ajudar"), null);
  assert.equal(extractJSON('{"broken": '), null);
  assert.equal(extractJSON("[1,2,3]"), null, "a bare array is not a draft");
});

test("numbers outside the declared range are dropped, not clamped", () => {
  const fields = [f({ key: "vcpu", type: "number", min: 1, max: 64 })];
  // Clamping would silently hand the user a number they never asked for.
  assert.deepEqual(coerceToSchema(fields, { vcpu: 9999 }), {});
  assert.deepEqual(coerceToSchema(fields, { vcpu: 0 }), {});
  assert.deepEqual(coerceToSchema(fields, { vcpu: 8 }), { vcpu: 8 });
  assert.deepEqual(coerceToSchema(fields, { vcpu: "8" }), { vcpu: 8 }, "string digits are coerced");
});

test("a select value the offering never declared is discarded", () => {
  const fields = [f({ key: "ambiente", type: "select", options: ["Produção", "Homologação"] })];
  assert.deepEqual(coerceToSchema(fields, { ambiente: "Staging" }), {});
  assert.deepEqual(coerceToSchema(fields, { ambiente: "Produção" }), { ambiente: "Produção" });
});

test("a value failing the field's pattern is discarded", () => {
  const fields = [f({ key: "nome_banco", pattern: "^[a-z][a-z0-9_]*$" })];
  assert.deepEqual(coerceToSchema(fields, { nome_banco: "Portal-Servidor" }), {});
  assert.deepEqual(coerceToSchema(fields, { nome_banco: "portal_servidor" }), { nome_banco: "portal_servidor" });
});

test("asset_ref is never filled from a model", () => {
  // A guessed id would silently attach someone else's host to the request.
  const fields = [f({ key: "host_alvo", type: "asset_ref", asset_type: "host" })];
  assert.deepEqual(coerceToSchema(fields, { host_alvo: 3 }), {});
});

test("keys the schema does not declare cannot get through", () => {
  const fields = [f({ key: "descricao", type: "textarea" })];
  assert.deepEqual(
    coerceToSchema(fields, { descricao: "texto", is_admin: true, __proto__: { polluted: 1 } }),
    { descricao: "texto" }
  );
});

test("over-length text is dropped rather than truncated", () => {
  const fields = [f({ key: "titulo", max_length: 10 })];
  assert.deepEqual(coerceToSchema(fields, { titulo: "x".repeat(11) }), {});
});

test("coercePriority only accepts the four known levels", () => {
  assert.equal(coercePriority("critical"), "critical");
  assert.equal(coercePriority("urgentíssimo"), "");
  assert.equal(coercePriority(undefined), "");
});
