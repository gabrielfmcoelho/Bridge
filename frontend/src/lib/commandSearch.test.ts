// Run: node --test src/lib/commandSearch.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { normalize, searchEntries, type SearchEntry } from "./commandSearch.ts";

const E: SearchEntry[] = [
  { kind: "page", key: "p1", label: "Serviços", href: "/services" },
  { kind: "page", key: "p2", label: "Hosts", href: "/hosts" },
  { kind: "host", key: "h1", label: "Banco principal", sub: "db-prod-01", href: "/hosts/db-prod-01", terms: ["pg.sead.pi"] },
  { kind: "host", key: "h2", label: "Proxy", sub: "proxy-prod", href: "/hosts/proxy-prod" },
  { kind: "dns", key: "d1", label: "api.prod.pi.gov.br", href: "/dns/1" },
  { kind: "service", key: "s1", label: "Serviço de ponto", href: "/services/1" },
];

const keys = (groups: ReturnType<typeof searchEntries>) => groups.flatMap((g) => g.items.map((i) => i.key));

test("normalize strips accents and case", () => {
  assert.equal(normalize("ServIÇO Ação"), "servico acao");
});

test("empty query lists pages only", () => {
  const g = searchEntries(E, "  ");
  assert.deepEqual(g.map((x) => x.kind), ["page"]);
  assert.deepEqual(keys(g), ["p1", "p2"]);
});

test("accent-insensitive match across groups, pages first", () => {
  const g = searchEntries(E, "servico");
  assert.deepEqual(g.map((x) => x.kind), ["page", "service"]);
});

test("matches sub and extra terms", () => {
  assert.deepEqual(keys(searchEntries(E, "db-prod")), ["h1"]);
  assert.deepEqual(keys(searchEntries(E, "sead")), ["h1"]);
});

test("prefix beats word start beats substring within a group", () => {
  // "pro": h2's label is a prefix match (0); h1 only matches a word inside its sub (1)
  assert.deepEqual(keys(searchEntries(E, "pro")).filter((k) => k.startsWith("h")), ["h2", "h1"]);
});

test("caps each group", () => {
  const many: SearchEntry[] = Array.from({ length: 9 }, (_, i) => ({ kind: "host", key: `h${i}`, label: `web-${i}`, href: "/" }));
  assert.equal(searchEntries(many, "web", 5)[0].items.length, 5);
});

test("no match → no groups", () => {
  assert.deepEqual(searchEntries(E, "zzz"), []);
});
