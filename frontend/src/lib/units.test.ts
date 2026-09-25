import { test } from "node:test";
import assert from "node:assert/strict";
import { formatSize } from "./units.ts";

test("spells out df/free size units", () => {
  assert.equal(formatSize("7,2Gi"), "7,2 GiB");
  assert.equal(formatSize("118G"), "118 GB");
  assert.equal(formatSize("512Mi"), "512 MiB");
  assert.equal(formatSize("1.5T"), "1.5 TB");
  assert.equal(formatSize("3.8GiB"), "3.8 GiB");
});

test("leaves non-sizes alone", () => {
  assert.equal(formatSize("4 vCPU"), "4 vCPU");
  assert.equal(formatSize(""), "");
  assert.equal(formatSize("n/a"), "n/a");
});
