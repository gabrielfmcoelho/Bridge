// Run: node --test src/lib/offeringSchema.test.ts   (Node 24 native TS, no runner dep)
//
// An admin pastes form_schema / templates as JSON. Whatever they paste is what
// every future requester will be asked, so the parser must reject anything
// DynamicField cannot render — and say which field, not just "invalid".
import { test } from "node:test";
import assert from "node:assert/strict";
import { parseFormSchema, parseTemplates } from "./offeringSchema.ts";
import type { FormField } from "./types.ts";

const field = (over: Partial<FormField>): FormField =>
  ({ key: "k", type: "text", label_en: "K", label_pt: "K", required: false, ...over }) as FormField;

const codes = (r: { errors: { code: string }[] }) => r.errors.map((e) => e.code);

test("a valid schema round-trips with no issues", () => {
  const schema = { fields: [field({ key: "ambiente", type: "select", options: ["Prod", "Dev"], required: true })] };
  const r = parseFormSchema(JSON.stringify(schema));
  assert.deepEqual(r.errors, []);
  assert.deepEqual(r.value, schema);
});

test("a bare array is accepted and normalised to {fields}", () => {
  const r = parseFormSchema(JSON.stringify([field({ key: "nome" })]));
  assert.deepEqual(r.errors, []);
  assert.equal(r.value?.fields.length, 1);
});

test("missing `required` defaults to false instead of failing", () => {
  const noRequired: Partial<FormField> = field({ key: "nome" });
  delete noRequired.required;
  const r = parseFormSchema(JSON.stringify({ fields: [noRequired] }));
  assert.deepEqual(r.errors, []);
  assert.equal(r.value?.fields[0].required, false);
});

test("blank text is 'nothing yet': no value, but no issue to shout about either", () => {
  assert.deepEqual(parseFormSchema("  \n"), { value: null, errors: [] });
});

test("broken JSON reports invalidJson and no value", () => {
  const r = parseFormSchema('{"fields": [');
  assert.deepEqual(codes(r), ["invalidJson"]);
  assert.equal(r.value, null);
});

test("`fields` that is not an array, and an empty field list, are both issues", () => {
  assert.deepEqual(codes(parseFormSchema('{"fields": "nope"}')), ["notArray"]);
  assert.deepEqual(codes(parseFormSchema('{"fields": []}')), ["empty"]);
});

test("all field-level issues are collected, each naming its 1-based position", () => {
  const r = parseFormSchema(
    JSON.stringify({
      fields: [
        field({ key: "Bad Key" }),
        field({ key: "dup" }),
        field({ key: "dup" }),
        field({ key: "t", type: "foo" as FormField["type"] }),
        field({ key: "l", label_en: "" }),
      ],
    }),
  );
  assert.deepEqual(r.errors, [
    { code: "badKey", n: 1, key: "Bad Key" },
    { code: "dupKey", n: 3, key: "dup" },
    { code: "badType", n: 4, key: "t", value: "foo" },
    { code: "missingLabel", n: 5, key: "l" },
  ]);
  assert.equal(r.value, null, "any issue means nothing is saved");
});

test("type-specific rules: select needs options, asset_ref needs a known asset_type, min<=max, pattern compiles", () => {
  const r = parseFormSchema(
    JSON.stringify({
      fields: [
        field({ key: "s", type: "select" }),
        field({ key: "a", type: "asset_ref", asset_type: "unicorn" as FormField["asset_type"] }),
        field({ key: "n", type: "number", min: 10, max: 1 }),
        field({ key: "p", type: "text", pattern: "[" }),
      ],
    }),
  );
  assert.deepEqual(codes(r), ["noOptions", "badAssetType", "minMax", "badPattern"]);
});

test("depends_on must point at another field's key", () => {
  const ok = parseFormSchema(
    JSON.stringify({ fields: [field({ key: "amb", type: "select", options: ["x"] }), field({ key: "j", depends_on: { key: "amb", equals: ["x"] } })] }),
  );
  assert.deepEqual(ok.errors, []);
  const self = parseFormSchema(JSON.stringify({ fields: [field({ key: "j", depends_on: { key: "j", equals: ["x"] } })] }));
  assert.deepEqual(codes(self), ["badDependsOn"]);
  const missing = parseFormSchema(JSON.stringify({ fields: [field({ key: "j", depends_on: { key: "ghost", equals: ["x"] } })] }));
  assert.deepEqual(codes(missing), ["badDependsOn"]);
});

test("templates: blank text is valid and means none", () => {
  assert.deepEqual(parseTemplates("   \n", []), { value: [], errors: [] });
});

test("templates: every values key must exist in the schema", () => {
  const fields = [field({ key: "vcpu", type: "number" })];
  const good = parseTemplates(JSON.stringify([{ key: "p", name_en: "P", name_pt: "P", values: { vcpu: 2 } }]), fields);
  assert.deepEqual(good.errors, []);
  assert.equal(good.value[0].summary_en, "", "summary is optional and defaults to empty");
  const bad = parseTemplates(JSON.stringify([{ key: "p", name_en: "P", name_pt: "P", values: { ram: 4 } }]), fields);
  assert.deepEqual(bad.errors, [{ code: "unknownValueKey", n: 1, key: "ram" }]);
});

test("templates: must be an array; keys slug-like and unique; names present — with template-specific codes", () => {
  assert.deepEqual(codes(parseTemplates('{"key":"x"}', [])), ["notArray"]);
  const r = parseTemplates(
    JSON.stringify([
      { key: "Bad Key", name_en: "A", name_pt: "A", values: {} },
      { key: "a", name_en: "A", name_pt: "A", values: {} },
      { key: "a", name_en: "", name_pt: "A", values: {} },
    ]),
    [],
  );
  assert.deepEqual(codes(r), ["tplBadKey", "tplDupKey", "tplMissingLabel"]);
});

test("templates: kebab-case keys are fine — a template key never becomes a form_data key", () => {
  // The seed's own templates ("api-frontend", "app-com-banco") must validate.
  const r = parseTemplates(JSON.stringify([{ key: "api-frontend", name_en: "A", name_pt: "A", values: {} }]), []);
  assert.deepEqual(r.errors, []);
});
