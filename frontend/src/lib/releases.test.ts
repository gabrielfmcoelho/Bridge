// Run: node --test src/lib/releases.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { splitReleases, isOverdue } from "./releases.ts";
import type { Release } from "./types.ts";

const rel = (id: number, status: string, target_date = "", live_date = ""): Release => ({
  id, project_id: 1, title: `r${id}`, description: "", status, target_date, live_date,
  created_at: "2026-01-01", updated_at: "2026-01-01",
});

test("splitReleases orders each lane", () => {
  const { planned, achieved, canceled } = splitReleases([
    rel(1, "pending"), rel(2, "ongoing", "2026-12-01"), rel(3, "ready", "2026-10-01"),
    rel(4, "live", "", "2026-03-01"), rel(5, "live", "", "2026-08-01"), rel(6, "canceled", "2026-05-01"),
  ]);
  assert.deepEqual(planned.map((r) => r.id), [3, 2, 1]);
  assert.deepEqual(achieved.map((r) => r.id), [5, 4]);
  assert.deepEqual(canceled.map((r) => r.id), [6]);
});

test("isOverdue only for planned releases past their target", () => {
  assert.equal(isOverdue(rel(1, "ongoing", "2026-01-01"), "2026-02-01"), true);
  assert.equal(isOverdue(rel(1, "ongoing", "2026-03-01"), "2026-02-01"), false);
  assert.equal(isOverdue(rel(1, "live", "2026-01-01"), "2026-02-01"), false);
  assert.equal(isOverdue(rel(1, "pending"), "2026-02-01"), false);
});
