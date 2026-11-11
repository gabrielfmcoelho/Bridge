import { test } from "node:test";
import assert from "node:assert/strict";
import { hostIdsByOS, osFamilies, osKey, sortOSGroups } from "./osView.ts";
import type { DependencyGroup, Host } from "@/lib/types";

const g = (name: string, version: string, hosts: number, eol = false) => ({ kind: "os", name, version, hosts, eol } as DependencyGroup);

test("OS groups sort by distro, newest version first (numeric)", () => {
  const out = sortOSGroups([g("ubuntu", "20.04", 1), g("debian", "9", 1), g("ubuntu", "24.04", 1), g("debian", "12", 1)]);
  assert.deepEqual(out.map((x) => `${x.name} ${x.version}`), ["debian 12", "debian 9", "ubuntu 24.04", "ubuntu 20.04"]);
});

test("families add up hosts and EOL hosts, most hosts first", () => {
  const out = osFamilies([g("debian", "12", 2), g("ubuntu", "22.04", 3), g("ubuntu", "18.04", 2, true), g("alpine", "3.19", 2)]);
  assert.deepEqual(out, [
    { name: "ubuntu", hosts: 5, eolHosts: 2 },
    { name: "alpine", hosts: 2, eolHosts: 0 },
    { name: "debian", hosts: 2, eolHosts: 0 },
  ]);
});

test("host ids per OS group come from the host list, unscanned hosts left out", () => {
  const h = (id: number, os?: Host["os"]) => ({ id, os } as Host);
  const m = hostIdsByOS([h(1, { id: "ubuntu", version: "22.04" }), h(2, { id: "ubuntu", version: "22.04" }), h(3, { pretty: "FreeBSD" }), h(4)]);
  assert.deepEqual([...m], [[osKey("ubuntu", "22.04"), [1, 2]]]);
});
