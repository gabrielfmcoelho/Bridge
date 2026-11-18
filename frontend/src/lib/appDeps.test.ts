import { test } from "node:test";
import assert from "node:assert/strict";
import type { AppDepAssetLink, AppDepCatalog, AppDepSource } from "./types.ts";
import {
  buildCiGraph, buildLandscapeGraph, endpoint, indexByCi, isManual, linkCounts, linkRows, linkSourceKey, reportRows, rowMatches,
  worstCriticality,
} from "./appDeps.ts";

// catalog-parser's examples/ (shop-api, shop-web), as the API returns them.
const shopApi: AppDepCatalog = {
  ci: "shop-api",
  description: "Orders and payments API of the web shop",
  owners: [{ name: "Jane Doe", email: "jane@example.org", username: "jdoe" }],
  entity: [{ name: "Example Corp", slug: "example-corp", type: "company" }],
  git: { type: "github", host: "github.com", path: "example-corp/shop-api" },
  metadata: {},
  components: [
    { name: "api", category: "application", technology: "rest-api", protocol: "https", port: 8443, criticality: "high", description: "Public REST API" },
    { name: "worker", category: "application", technology: "celery", criticality: "medium", description: "Sends order e-mails" },
    { name: "orders-db", category: "data", technology: "postgresql", criticality: "high", description: "Orders tables" },
  ],
  dependencies: {
    upstream: [
      { ci: "payment-gateway", local_component: "worker", category: "service", technology: "rest-api", protocol: "https", port: 443, criticality: "high", description: "Card payments provider" },
    ],
    downstream: [
      { ci: "shop-web", component: "frontend", local_component: "api" },
      { ci: "bi-dashboard", local_component: "api" },
    ],
  },
};

const shopWeb: AppDepCatalog = {
  ci: "shop-web",
  description: "Storefront single-page application",
  owners: [{ name: "John Roe", email: "john@example.org", username: "jroe" }],
  entity: [{ name: "Example Corp", slug: "example-corp", type: "company" }],
  git: { type: "github", host: "github.com", path: "example-corp/shop-web" },
  metadata: {},
  components: [
    { name: "frontend", category: "application", technology: "react", protocol: "https", port: 443, criticality: "high", description: "Storefront served by a CDN" },
  ],
  dependencies: { upstream: [{ ci: "shop-api", component: "api", local_component: "frontend" }], downstream: [] },
};

const src = (id: number, path: string, catalog: AppDepCatalog | null, parse_error?: string): AppDepSource => ({
  gitlab_project_id: id, project_path: path, web_url: `https://gl/${path}`, ref: "main", commit_id: "c", file_url: "",
  catalog, parse_error, synced_at: "2026-11-17T00:00:00Z",
});

const sources = [src(2, "g/shop-web", shopWeb), src(1, "g/shop-api", shopApi), src(3, "g/broken", null, "ci is missing")];

test("indexByCi keeps parse failures apart and the first repository of a duplicate CI", () => {
  const dup = src(4, "z/shop-api-fork", { ...shopApi, description: "fork" });
  const idx = indexByCi([...sources, dup]);
  assert.deepEqual([...idx.byCi.keys()].sort(), ["shop-api", "shop-web"]);
  assert.equal(idx.byCi.get("shop-api")!.source.project_path, "g/shop-api");
  assert.equal(idx.duplicates.get("shop-api")![0].project_path, "z/shop-api-fork");
  assert.equal(idx.failed.length, 1);
});

test("reportRows sums each CI", () => {
  const [api, web] = reportRows(indexByCi(sources));
  assert.equal(api.ci, "shop-api");
  assert.deepEqual(api.technologies, ["rest-api", "celery", "postgresql"]);
  assert.equal(api.components, 3);
  assert.equal(api.upstream, 1);
  assert.equal(api.downstream, 2);
  assert.equal(api.criticality, "high");
  assert.equal(web.ci, "shop-web");
  assert.ok(rowMatches(api, shopApi, "JDOE"));
  assert.ok(rowMatches(api, shopApi, "postgres"));
  assert.ok(!rowMatches(web, shopWeb, "celery"));
});

test("worstCriticality and endpoint", () => {
  assert.equal(worstCriticality(["low", undefined, "medium"]), "medium");
  assert.equal(worstCriticality(["custom", "low"]), "low");
  assert.equal(worstCriticality([undefined, ""]), undefined);
  assert.equal(endpoint("https", 8443), "https:8443");
  assert.equal(endpoint("tcp"), "tcp");
  assert.equal(endpoint(undefined, 5432), ":5432");
  assert.equal(endpoint(), "");
});

test("linkRows resolves a catalogued peer's traits and keeps an external's inline ones", () => {
  const idx = indexByCi(sources);
  const web = linkRows(shopWeb, idx);
  assert.equal(web.length, 1);
  assert.deepEqual(
    { other: web[0].other, technology: web[0].technology, endpoint: web[0].endpoint, catalogued: web[0].catalogued },
    { other: "shop-api", technology: "rest-api", endpoint: "https:8443", catalogued: true },
  );
  const api = linkRows(shopApi, idx);
  assert.deepEqual(api.map((r) => [r.direction, r.other, r.endpoint, r.catalogued]), [
    ["upstream", "payment-gateway", "https:443", false],
    ["downstream", "shop-web", "https:8443", true],
    ["downstream", "bi-dashboard", "https:8443", false],
  ]);
  assert.equal(api[0].technology, "rest-api");
});

test("buildCiGraph draws what the CI declares, like catalog-diagram", () => {
  const g = buildCiGraph("shop-api", indexByCi(sources))!;
  const ids = g.nodes.map((n) => n.id).sort();
  assert.deepEqual(ids, [
    "c:shop-api/api", "c:shop-api/orders-db", "c:shop-api/worker", "c:shop-web/frontend",
    "x:bi-dashboard", "x:payment-gateway",
  ]);
  assert.deepEqual(g.groups.map((x) => [x.ci, x.focus, x.members.length]).sort(), [["shop-api", true, 3], ["shop-web", false, 1]]);
  const edges = Object.fromEntries(g.edges.map((e) => [`${e.source}->${e.target}`, e.label]));
  assert.deepEqual(edges, {
    "c:shop-api/worker->x:payment-gateway": "https:443",
    "c:shop-web/frontend->c:shop-api/api": "https:8443",
    "x:bi-dashboard->c:shop-api/api": "https:8443",
  });
  const db = g.nodes.find((n) => n.id === "c:shop-api/orders-db")!;
  assert.equal(db.data, true);
  const frontend = g.nodes.find((n) => n.id === "c:shop-web/frontend")!;
  assert.equal(frontend.technology, "react", "a peer's component is drawn with its own traits");
  assert.equal(frontend.catalogued, true);
  assert.equal(buildCiGraph("nope", indexByCi(sources)), null);
});

test("buildLandscapeGraph draws a mirrored link once and gives every CI its frame", () => {
  const g = buildLandscapeGraph(indexByCi(sources));
  const mirror = g.edges.filter((e) => e.source === "c:shop-web/frontend" && e.target === "c:shop-api/api");
  assert.equal(mirror.length, 1);
  assert.equal(g.edges.length, 3);
  assert.deepEqual(g.groups.map((x) => x.ci).sort(), ["shop-api", "shop-web"]);
  assert.ok(g.groups.every((x) => !x.focus));
});

test("a link without a component to a catalogued CI ends at its CI node; unknown local ends at the own CI node", () => {
  const lonely: AppDepCatalog = {
    ...shopWeb, ci: "reports", components: [],
    dependencies: { upstream: [{ ci: "shop-api", local_component: "ghost" }], downstream: [] },
  };
  const g = buildCiGraph("reports", indexByCi([...sources, src(5, "g/reports", lonely)]))!;
  assert.deepEqual(g.edges.map((e) => [e.source, e.target]), [["n:reports", "n:shop-api"]]);
  assert.ok(g.nodes.find((n) => n.id === "n:shop-api")!.catalogued);
});

test("linkCounts tallies each CI's assets; the source key and manual flag read the sources", () => {
  const l = (ci: string, entity_type: AppDepAssetLink["entity_type"], entity_id: number, sources: AppDepAssetLink["sources"]): AppDepAssetLink =>
    ({ ci_id: 1, ci, entity_type, entity_id, sources });
  const links = [l("a", "host", 1, ["repo"]), l("a", "host", 2, ["scan"]), l("a", "service", 3, ["manual", "repo"]), l("b", "dns", 4, ["group"])];
  const c = linkCounts(links);
  assert.deepEqual(c.get("a"), { host: 2, service: 1, dns: 0, project: 0 });
  assert.deepEqual(c.get("b"), { host: 0, service: 0, dns: 1, project: 0 });
  assert.equal(c.get("z"), undefined);
  assert.equal(linkSourceKey(links[2]), "appDeps.source.manual");
  assert.equal(linkSourceKey(links[1]), "appDeps.source.scan");
  assert.ok(isManual(links[2]));
  assert.ok(!isManual(links[0]));
});
