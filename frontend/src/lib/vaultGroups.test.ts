// Run: node --test src/lib/vaultGroups.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { groupVault } from "./vaultGroups.ts";
import type { Secret } from "./types.ts";

const s = (id: number, over: Partial<Secret>): Secret => ({
  id, type: "password", scope: "avulso", visibility: "shared", owner_user_id: 1, name: `s${id}`,
  key_version: 1, created_by: 1, created_at: "", updated_at: "", ...over,
});

test("env vars of one environment on one asset form a bundle", () => {
  const rows = groupVault([
    s(1, { type: "env_var", scope: "service", parent_id: 7, group_label: "prod", name: "DB_URL" }),
    s(2, { type: "env_var", scope: "service", parent_id: 7, group_label: "prod", name: "API_KEY" }),
    s(3, { type: "env_var", scope: "service", parent_id: 7, group_label: "staging", name: "DB_URL" }),
    s(4, { type: "env_var", scope: "service", parent_id: 8, group_label: "prod", name: "DB_URL" }),
  ]);
  assert.deepEqual(rows.map((r) => (r.kind === "bundle" ? r.items.length : -1)), [2, 1, 1]);
});

test("repeated credentials group under the shared copy", () => {
  const rows = groupVault([
    s(1, { scope: "host", parent_id: 1, name: "password", dup_group: "aa", dup_count: 3 }),
    s(2, { scope: "host", parent_id: 2, name: "password", dup_group: "aa", dup_count: 3 }),
    s(3, { scope: "avulso", name: "deploy", dup_group: "aa", dup_count: 3 }),
    s(4, { name: "alone" }),
  ]);
  assert.equal(rows.length, 2);
  const [rep, single] = rows;
  assert.equal(rep.kind, "repeated");
  assert.equal(rep.kind === "repeated" && rep.lead.id, 3);
  assert.equal(rep.kind === "repeated" && rep.items.length, 3);
  assert.equal(single.kind, "secret");
});

test("a repeated group reduced to one by filters is a plain row", () => {
  const rows = groupVault([s(1, { dup_group: "bb", dup_count: 2 })]);
  assert.equal(rows[0].kind, "secret");
});
