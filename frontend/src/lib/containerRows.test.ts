import { test } from "node:test";
import assert from "node:assert/strict";
import { containerRows } from "./containerRows.ts";

const ps = { id: "4b82b86b1b62", name: "form-totem-nextjs", image: "img", status: "Up 13 months", ports: "0.0.0.0:3000->3000/tcp" };

// SAPASP-AVALIE04: docker ps answered, docker stats did not.
test("a container without stats still gets a row", () => {
  const rows = containerRows({ parsed_containers: [ps], container_stats: null });
  assert.equal(rows.length, 1);
  assert.equal(rows[0].name, "form-totem-nextjs");
  assert.equal(rows[0].container, ps);
  assert.equal(rows[0].cpu, "");
});

test("stats join the docker ps row by name", () => {
  const rows = containerRows({ parsed_containers: [ps], container_stats: ["form-totem-nextjs  CPU:1.5%  MEM:30MiB / 1.9GiB  NET:1kB / 2kB"] });
  assert.equal(rows.length, 1);
  assert.equal(rows[0].cpuNum, 1.5);
  assert.equal(rows[0].container, ps);
});

test("a stats row with no docker ps match still renders", () => {
  const rows = containerRows({ container_stats: ["old  CPU:0.1%  MEM:1MiB / 1GiB  NET:0B / 0B"] });
  assert.equal(rows.length, 1);
  assert.equal(rows[0].name, "old");
  assert.equal(rows[0].container, undefined);
});
