// Run: node --test src/lib/vulns.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { linkifyParts, nvdURL, parseVia, safeHref } from "./vulns.ts";

test("safeHref only accepts http(s)", () => {
  assert.equal(safeHref("https://example.test/a"), "https://example.test/a");
  assert.equal(safeHref("javascript:alert(1)"), null);
  assert.equal(safeHref("data:text/html,x"), null);
  assert.equal(safeHref("not a url"), null);
});

test("linkifyParts keeps payloads as text and splits ZAP URL lists", () => {
  const parts = linkifyParts("http://a.test/login/ | http://a.test/x?next=%2F <script>alert(1)</script>");
  assert.deepEqual(parts.filter((p) => p.href).map((p) => p.href), ["http://a.test/login/", "http://a.test/x?next=%2F"]);
  assert.equal(parts.map((p) => p.text).join(""), "http://a.test/login/ | http://a.test/x?next=%2F <script>alert(1)</script>");
  assert.equal(linkifyParts("javascript:alert(1)").some((p) => p.href), false);
});

test("parseVia and nvdURL", () => {
  assert.deepEqual(parseVia("dns:portal.sead.pi.gov.br"), { type: "dns", name: "portal.sead.pi.gov.br" });
  assert.equal(parseVia(""), null);
  assert.equal(nvdURL("CVE-2026-72898"), "https://nvd.nist.gov/vuln/detail/CVE-2026-72898");
  assert.equal(nvdURL("OSVDB-3280"), null);
});
