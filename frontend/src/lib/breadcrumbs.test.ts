// Run: node --test src/lib/breadcrumbs.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildCrumbs as build } from "./breadcrumbs.ts";
import { NAV_ITEMS } from "./constants.ts";

const buildCrumbs = (p: string) => build(p, NAV_ITEMS);

test("root is a single dashboard crumb with no link", () => {
  assert.deepEqual(buildCrumbs("/"), [{ label: "nav.dashboard" }]);
});

test("top-level static route is one unlinked crumb", () => {
  assert.deepEqual(buildCrumbs("/hosts"), [{ label: "nav.hosts" }]);
});

test("dynamic host leaf keeps the slug string in the query key", () => {
  const [hosts, leaf] = buildCrumbs("/hosts/web-01");
  assert.deepEqual(hosts, { label: "nav.hosts", href: "/hosts" });
  assert.equal(leaf.label, "web-01");
  assert.equal(leaf.raw, true);
  assert.equal(leaf.mono, true);
  assert.equal(leaf.href, undefined);
  assert.deepEqual(leaf.queryKey, ["host", "web-01"]);
  assert.equal(leaf.pick?.({ host: { nickname: "Web 01" } }), "Web 01");
});

test("numeric ids are coerced so the key hits the page's cache entry", () => {
  const leaf = buildCrumbs("/services/12")[1];
  assert.deepEqual(leaf.queryKey, ["service", 12]);
});

test("/atlas prefix renders as text, not a link", () => {
  const [atlas, apis, leaf] = buildCrumbs("/atlas/apis/7");
  assert.deepEqual(atlas, { label: "nav.atlas" });
  assert.deepEqual(apis, { label: "nav.apis", href: "/atlas/apis" });
  assert.deepEqual(leaf.queryKey, ["api-catalog", 7]);
  assert.equal(leaf.pick?.({ name: "Billing" }), "Billing");
});

test("nested static routes resolve without a nav item", () => {
  assert.deepEqual(buildCrumbs("/secrets/trash"), [
    { label: "nav.vault", href: "/secrets" },
    { label: "nav.trash" },
  ]);
  assert.deepEqual(buildCrumbs("/chamados/forms"), [
    { label: "nav.chamados", href: "/chamados" },
    { label: "nav.chamadosForms" },
  ]);
});

test("unknown segments fall back to raw text", () => {
  assert.deepEqual(buildCrumbs("/foo/bar"), [
    { label: "foo", raw: true, href: "/foo" },
    { label: "bar", raw: true },
  ]);
});

test("query string and trailing slash are ignored", () => {
  assert.deepEqual(
    buildCrumbs("/hosts/web-01/?x=1").map((c) => c.label),
    ["nav.hosts", "web-01"],
  );
});
