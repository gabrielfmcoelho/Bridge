// Run: node --test src/lib/entidades.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { withDepth, descendantIds, creatorOptions } from "./entidades.ts";
import type { Entidade } from "./types.ts";

const e = (id: number, name: string, parent_id: number | null): Entidade => ({
  id, name, slug: name.toLowerCase(), parent_id, description: "", created_at: "", updated_at: "",
});
// GovPI > ETIPI ; GovPI > SEAD-PI > SGA   (parents first, like the API)
const tree = [e(1, "GovPI", null), e(2, "ETIPI", 1), e(3, "SEAD-PI", 1), e(4, "SGA", 3)];

test("withDepth: depth follows parent chain", () => {
  assert.deepEqual(withDepth(tree).map((n) => n.depth), [0, 1, 1, 2]);
});

test("descendantIds: includes roots and every descendant", () => {
  assert.deepEqual([...descendantIds(tree, [3])].sort(), [3, 4]);
  assert.deepEqual([...descendantIds(tree, [1])].sort(), [1, 2, 3, 4]);
});

test("creatorOptions: admin sees all, member sees own subtree only", () => {
  assert.equal(creatorOptions(tree, { role: "admin", entidades: [] }).length, 4);
  const sead = creatorOptions(tree, { role: "editor", entidades: [{ id: 3, name: "SEAD-PI", slug: "sead-pi", is_primary: true }] });
  assert.deepEqual(sead.map((n) => n.id), [3, 4]);
  assert.equal(creatorOptions(tree, { role: "viewer", entidades: [] }).length, 0);
});
