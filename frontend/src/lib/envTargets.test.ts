// Run: node --test src/lib/envTargets.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildEnvTargets } from "./envTargets.ts";

test("buildEnvTargets: project only", () => {
  assert.deepEqual(buildEnvTargets(5, []), [{ scope: "projeto", parent_id: 5 }]);
});

test("buildEnvTargets: project + services fans out one target per service", () => {
  assert.deepEqual(buildEnvTargets(5, [2, 3]), [
    { scope: "projeto", parent_id: 5 },
    { scope: "service", parent_id: 2 },
    { scope: "service", parent_id: 3 },
  ]);
});
