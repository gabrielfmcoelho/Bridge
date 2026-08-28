// Run: node --test src/lib/requests.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { TRANSITIONS, canTransition } from "./requests.ts";
import type { RequestStatus } from "./types.ts";

const STATUSES: RequestStatus[] = [
  "submitted", "under_review", "approved",
  "in_progress", "delivered", "rejected", "cancelled", "needs_info",
];

test("terminal states have no transitions", () => {
  assert.deepEqual(TRANSITIONS.delivered, []);
  assert.deepEqual(TRANSITIONS.rejected, []);
  assert.deepEqual(TRANSITIONS.cancelled, []);
});

test("canTransition: submitted cannot jump straight to delivered", () => {
  assert.equal(canTransition("submitted", "delivered"), false);
});

test("canTransition: approved can move to in_progress", () => {
  assert.equal(canTransition("approved", "in_progress"), true);
});

test("every value in TRANSITIONS is itself a valid RequestStatus", () => {
  const valid = new Set<string>(STATUSES);
  for (const targets of Object.values(TRANSITIONS)) {
    for (const t of targets) assert.ok(valid.has(t), `${t} is not a valid RequestStatus`);
  }
});
