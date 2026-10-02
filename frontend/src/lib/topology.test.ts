// Run: node --test src/lib/topology.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildTopologyView } from "./topology.ts";
import type { GraphData, GraphNode } from "./types.ts";

const node = (id: string, data: Record<string, unknown> = {}): GraphNode => ({
  id, type: id.split("-")[0] as GraphNode["type"], label: id, data,
});
const edge = (source: string, target: string) => ({ source, target, label: "" });

// host-1 runs svc 1..3; svc-1 has dns-1, project-1, api-1 and depends on svc-9 (on host-2).
const graph: GraphData = {
  nodes: [
    node("host-1"), node("host-2"),
    node("service-1", { coolify_stack: "gateway" }), node("service-2", { coolify_stack: "gateway" }),
    node("service-3", { coolify_stack: "solo" }), node("service-9"),
    node("dns-1"), node("project-1"), node("api-1"),
  ],
  edges: [
    edge("service-1", "host-1"), edge("service-2", "host-1"), edge("service-3", "host-1"),
    edge("service-9", "host-2"), edge("service-1", "service-9"),
    edge("service-1", "dns-1"), edge("dns-1", "host-1"), edge("service-1", "project-1"),
    edge("service-2", "project-1"), edge("api-1", "service-1"),
  ],
};
const ids = (v: { nodes: GraphNode[] }) => v.nodes.map((n) => n.id).sort();

test("service focus: its relations, host siblings collapsed with a count", () => {
  const v = buildTopologyView(graph, { focus: "service-1" });
  assert.deepEqual(ids(v), ["api-1", "dns-1", "host-1", "project-1", "service-1", "service-9"].sort());
  assert.deepEqual(v.hosts["host-1"], { count: 2, expanded: false });
  assert.equal(v.hosts["host-2"], undefined, "host-2 is not in focus");
  assert.ok(v.edges.every((e) => ids(v).includes(e.source) && ids(v).includes(e.target)));
});

test("expanding a host reveals its other services", () => {
  const v = buildTopologyView(graph, { focus: "service-1", overrides: { "host-1": true } });
  assert.ok(ids(v).includes("service-2") && ids(v).includes("service-3"));
  assert.deepEqual(v.hosts["host-1"], { count: 2, expanded: true });
});

test("host focus: its services, DNS and their projects", () => {
  const v = buildTopologyView(graph, { focus: "host-1" });
  assert.deepEqual(ids(v), ["dns-1", "host-1", "project-1", "service-1", "service-2", "service-3"].sort());
  assert.deepEqual(v.hosts, {}, "nothing collapsed on the focused host");
});

test("project focus: services plus their hosts and DNS", () => {
  const v = buildTopologyView(graph, { focus: "project-1" });
  assert.deepEqual(ids(v), ["dns-1", "host-1", "project-1", "service-1", "service-2"].sort());
  assert.deepEqual(v.hosts["host-1"], { count: 1, expanded: false });
});

test("full map: big hosts start collapsed; a service hides only if all its hosts are", () => {
  const big: GraphData = {
    nodes: [node("host-1"), node("host-2"), ...Array.from({ length: 9 }, (_, i) => node(`service-${i}`))],
    edges: [...Array.from({ length: 9 }, (_, i) => edge(`service-${i}`, "host-1")), edge("service-0", "host-2")],
  };
  const v = buildTopologyView(big);
  assert.deepEqual(v.hosts["host-1"], { count: 9, expanded: false });
  assert.deepEqual(ids(v), ["host-1", "host-2", "service-0"], "service-0 also runs on expanded host-2");
  assert.equal(buildTopologyView(big, { expandAll: true }).nodes.length, 11);
});

test("groups: two or more visible services sharing the key", () => {
  const v = buildTopologyView(graph, { focus: "host-1", groupBy: "stack" });
  assert.deepEqual(v.groups, [{ id: "group-stack-gateway", label: "gateway", members: ["service-1", "service-2"] }]);
  assert.deepEqual(buildTopologyView(graph, { focus: "host-1", groupBy: "none" }).groups, []);
  // Hidden siblings never form a group.
  assert.deepEqual(buildTopologyView(graph, { focus: "service-1" }).groups, []);
});
