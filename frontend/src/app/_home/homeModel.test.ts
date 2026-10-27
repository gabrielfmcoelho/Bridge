import { test } from "node:test";
import assert from "node:assert/strict";
import { canSeeWidget, columnOf, defaultWidgets, newPrefs, pendingSteps, readPrefs, suggestPersonas } from "./homeModel.ts";

test("default widgets: union of the personas plus the common ones, canonical order", () => {
  assert.deepEqual(defaultWidgets([]), ["assistant", "quickActions", "wiki"]);
  assert.deepEqual(defaultWidgets(["atlas", "infra"]), ["assistant", "myIssues", "infraHealth", "atlas", "quickActions", "inventory", "wiki"]);
  assert.deepEqual(defaultWidgets(["chamados"]), ["assistant", "myRequests", "quickActions", "chamados", "wiki"]);
});

test("suggested personas come from permissions, chamados when nothing fits", () => {
  assert.deepEqual(suggestPersonas(["hosts.view", "ssh.operate", "issues.view", "projects.view"]), ["infra", "dev"]);
  assert.deepEqual(suggestPersonas(["hosts.view"]), ["chamados"]);
  assert.deepEqual(suggestPersonas([], "admin"), ["infra", "dev"]);
  assert.deepEqual(suggestPersonas([], "viewer"), ["chamados"]);
});

test("widget visibility follows the permission, admins see all", () => {
  assert.equal(canSeeWidget("infraHealth", [], "viewer"), false);
  assert.equal(canSeeWidget("infraHealth", ["hosts.view"], "viewer"), true);
  assert.equal(canSeeWidget("infraHealth", [], "admin"), true);
  assert.equal(canSeeWidget("wiki", [], "viewer"), true);
});

test("stored prefs: not onboarded → null, unknown ids dropped", () => {
  assert.equal(readPrefs({}), null);
  assert.equal(readPrefs(null), null);
  assert.equal(readPrefs({ personas: ["infra"] }), null);
  assert.deepEqual(
    readPrefs({ onboarded_at: "2026-10-13T12:00:00Z", personas: ["infra", "gone"], widgets: ["wiki", "oldWidget", 3], done_steps: ["scan"] }),
    { v: 1, personas: ["infra"], widgets: ["wiki"], onboarded_at: "2026-10-13T12:00:00Z", done_steps: ["scan"], placement: {} },
  );
  const fresh = newPrefs(["suporte"], new Date("2026-10-13T12:00:00Z"));
  assert.deepEqual(readPrefs(JSON.parse(JSON.stringify(fresh))), fresh);
});

test("pending steps: deduplicated across personas, done ones hidden, wiki last", () => {
  assert.deepEqual(pendingSteps(["dev", "atlas"], ["issues"]).map((s) => s.id), ["projects", "apis", "wiki"]);
  assert.deepEqual(pendingSteps(["chamados"], ["openChamado", "catalog", "wiki"]), []);
});

test("placement: movable widgets follow the stored column, others keep their own", () => {
  const p = readPrefs({ onboarded_at: "x", placement: { myIssues: "side", wiki: "main", infraHealth: "side", gone: "side", myRequests: "up" } })!;
  assert.deepEqual(p.placement, { myIssues: "side" });
  assert.equal(columnOf("myIssues", p), "side");
  assert.equal(columnOf("myIssues", { placement: {} }), "main");
  assert.equal(columnOf("wiki", { placement: { wiki: "main" } }), "side");
});
