import { test } from "node:test";
import assert from "node:assert/strict";
import { scriptParams } from "./sshCustomOps.ts";

test("scriptParams: first-use order, deduplicated, only valid names", () => {
  assert.deepEqual(scriptParams("docker rm -f {{container}} && docker volume rm {{volume}} {{container}}"), ["container", "volume"]);
  assert.deepEqual(scriptParams("echo {{Bad}} {{ spaced }} {{9x}} {{ok_1}}"), ["ok_1"]);
  assert.deepEqual(scriptParams("uptime"), []);
});
