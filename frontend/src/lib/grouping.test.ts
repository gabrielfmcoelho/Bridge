import { test } from "node:test";
import assert from "node:assert/strict";
import { groupItems, type Relation } from "./grouping.ts";

const hosts = [{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }];
const services = new Map([[10, "web"], [11, "api"]]);
const relations: Relation[] = [
  { a: "service", a_id: 10, b: "host", b_id: 1 },
  { a: "service", a_id: 11, b: "host", b_id: 1 }, // host 1 runs two services
  { a: "service", a_id: 10, b: "host", b_id: 2 },
  { a: "service", a_id: 99, b: "host", b_id: 3 }, // unnamed (deleted / out of scope)
  { a: "dns", a_id: 5, b: "host", b_id: 4 }, // other entity pair: ignored
];

test("groups by related entity, multi-membership, unlinked last", () => {
  const got = groupItems(hosts, "host", "service", relations, services, "none");
  assert.deepEqual(
    got.map((g) => [g.id, g.label, g.items.map((h) => h.id)]),
    [
      [11, "api", [1]],
      [10, "web", [1, 2]],
      [null, "none", [3, 4]],
    ],
  );
});

test("reads links in either direction", () => {
  const got = groupItems([{ id: 10 }], "service", "host", relations, new Map([[1, "h1"], [2, "h2"]]), "none");
  assert.deepEqual(got.map((g) => g.label), ["h1", "h2"]);
});

test("no unlinked group when everything is linked", () => {
  const got = groupItems([{ id: 2 }], "host", "service", relations, services, "none");
  assert.deepEqual(got.map((g) => g.id), [10]);
});
