// Run: node --test src/lib/shareBundles.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { bundleStatus, groupItems, recipientText } from "./shareBundles.ts";

const NOW = Date.parse("2026-10-05T12:00:00Z");
const base = { revoked_at: null, deleted_at: null, expires_at: "2026-10-06T00:00:00Z", max_views: null, view_count: 0 };

test("bundleStatus picks the most decisive reason", () => {
  assert.equal(bundleStatus(base, NOW), "live");
  assert.equal(bundleStatus({ ...base, expires_at: null }, NOW), "live");
  assert.equal(bundleStatus({ ...base, expires_at: "2026-10-05T11:59:59Z" }, NOW), "expired");
  assert.equal(bundleStatus({ ...base, max_views: 3, view_count: 3 }, NOW), "exhausted");
  assert.equal(bundleStatus({ ...base, max_views: 3, view_count: 2 }, NOW), "live");
  assert.equal(bundleStatus({ ...base, deleted_at: "2026-10-01T00:00:00Z", expires_at: "2026-09-01T00:00:00Z" }, NOW), "archived");
  assert.equal(bundleStatus({ ...base, revoked_at: "2026-10-02T00:00:00Z", deleted_at: "2026-10-01T00:00:00Z" }, NOW), "revoked");
});

test("groupItems groups by kind in a fixed order and drops empty groups", () => {
  const groups = groupItems([
    { type: "secret", ref_id: 1, label: "key" },
    { type: "wiki_doc", ref_id: 0, ref_key: "a", label: "Guia" },
    { type: "api_doc", ref_id: 2, label: "Servidores" },
    { type: "wiki_collection", ref_id: 0, ref_key: "b", label: "Integrações" },
  ]);
  assert.deepEqual(groups, [
    { group: "api", labels: ["Servidores"] },
    { group: "wiki", labels: ["Guia", "Integrações"] },
    { group: "secret", labels: ["key"] },
  ]);
  assert.deepEqual(groupItems([]), []);
});

test("recipientText joins contact and label", () => {
  assert.equal(recipientText({ recipient_name: "Ana", recipient_label: "PGE" }), "Ana · PGE");
  assert.equal(recipientText({ recipient_label: "PGE" }), "PGE");
  assert.equal(recipientText({}), "");
});
