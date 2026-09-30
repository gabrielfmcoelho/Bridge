import { test } from "node:test";
import assert from "node:assert/strict";
import { apiInsights, apiBreakdowns, isSpecStale, matchesApiFilters, emptyApiFilters } from "./apiInsights.ts";
import type { ApiCatalog } from "@/lib/types";

const t = (k: string, v?: Record<string, string>) => (v ? `${k}(${Object.values(v).join(",")})` : k);
const now = Date.parse("2026-09-30T00:00:00Z");
const old = "2026-07-01T00:00:00Z";
const fresh = "2026-09-20T00:00:00Z";
const api = (p: Partial<ApiCatalog>) =>
  ({ id: 1, name: "a", source_type: "upload", spec_version: "3.0.0", updated_at: fresh, operation_count: 5, service_ids: [], project_ids: [], ...p }) as ApiCatalog;

const apis = [
  api({ id: 1, source_type: "url", updated_at: old, base_url: "https://x", project_ids: [7], operation_count: 60 }),
  api({ id: 2, source_type: "url", updated_at: fresh, service_ids: [3], project_ids: [7] }),
  api({ id: 3, spec_version: "2.0", updated_at: old }),
];

test("stale is a URL import untouched for SPEC_STALE_DAYS", () => {
  assert.equal(isSpecStale(apis[0], now), true);
  assert.equal(isSpecStale(apis[1], now), false);
  assert.equal(isSpecStale(apis[2], now), false, "uploads never go stale");
});

test("insights count and carry filters", () => {
  const by = Object.fromEntries(apiInsights(apis, t, now).map((i) => [i.key, i]));
  assert.equal(by.total.value, 3);
  assert.equal(by.total.hint, "atlas.apis.kpi.totalHint(2,1)");
  assert.equal(by.operations.value, 70);
  assert.equal(by.noLinks.value, 1);
  assert.deepEqual(by.noLinks.filter, { links: "none" });
  assert.equal(by.noBaseUrl.value, 2);
  assert.equal(by.specStale.value, 1);
  assert.equal(by["spec:3.0.0"].value, 2);
  assert.deepEqual(by["spec:2.0"].filter, { spec: "2.0" });
});

test("every insight filter selects exactly what it counts", () => {
  for (const i of apiInsights(apis, t, now)) {
    if (!i.filter) continue;
    const hits = apis.filter((a) => matchesApiFilters(a, { ...emptyApiFilters, ...i.filter }, now)).length;
    assert.equal(hits, i.value, i.key);
  }
});

test("link filters and breakdowns", () => {
  assert.deepEqual(apis.filter((a) => matchesApiFilters(a, { ...emptyApiFilters, project: "7" }, now)).map((a) => a.id), [1, 2]);
  assert.deepEqual(apis.filter((a) => matchesApiFilters(a, { ...emptyApiFilters, service: "3" }, now)).map((a) => a.id), [2]);
  const b = apiBreakdowns(apis, (id) => (id === 7 ? "Portal" : undefined));
  assert.deepEqual(b.projects.map((r) => [r.label, r.count]), [["Portal", 2], ["atlas.apis.dash.unlinked", 1]]);
  assert.deepEqual(b.projects[0].filter, { project: "7" });
  assert.deepEqual(b.services.map((r) => [r.label, r.count]), [["#3", 1], ["atlas.apis.dash.unlinked", 2]]);
  assert.deepEqual(b.source.map((r) => [r.key, r.count]), [["url", 2], ["upload", 1]]);
  assert.deepEqual(b.source[0].filter, { source: "url" });
});
