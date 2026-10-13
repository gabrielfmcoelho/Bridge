import { test } from "node:test";
import assert from "node:assert/strict";
import { NAV_SECTIONS, isNavItemActive } from "./constants.ts";

test("nav item active: its href, `also` prefixes, whole segments only, home exact", () => {
  const chamados = { href: "/chamados", also: ["/requests"] };
  assert.equal(isNavItemActive(chamados, "/chamados"), true);
  assert.equal(isNavItemActive(chamados, "/chamados/forms"), true);
  assert.equal(isNavItemActive(chamados, "/requests/12"), true);
  assert.equal(isNavItemActive(chamados, "/catalog"), false);
  assert.equal(isNavItemActive({ href: "/hosts" }, "/hosts-extra"), false);
  assert.equal(isNavItemActive({ href: "/" }, "/"), true);
  assert.equal(isNavItemActive({ href: "/" }, "/hosts"), false);
});

test("workspace holds issues and idea boards; one Chamados entry covers requests", () => {
  const hrefs = (key: string) => NAV_SECTIONS.find((s) => s.key === key)!.items.map((i) => i.href);
  assert.deepEqual(hrefs("main"), ["/", "/wiki", "/issues", "/canvas"]);
  assert.deepEqual(hrefs("serviceCatalog"), ["/catalog", "/chamados"]);
  const everyPage = NAV_SECTIONS.flatMap((s) => s.items);
  assert.equal(everyPage.filter((i) => isNavItemActive(i, "/requests")).length, 1);
});
