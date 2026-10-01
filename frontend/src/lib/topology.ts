// Topology view: what the graph shows for a focused asset (or the whole map),
// which hosts keep their other services collapsed, and how services group by
// stack / Coolify environment / Coolify project. Pure and runtime-import free
// (Node runs src/lib/topology.test.ts directly).
import type { GraphData, GraphEdge, GraphNode } from "./types";

export type GroupBy = "stack" | "environment" | "coolifyProject" | "none";

export const GROUP_FIELD: Record<Exclude<GroupBy, "none">, string> = {
  stack: "coolify_stack",
  environment: "coolify_environment",
  coolifyProject: "coolify_project",
};

/** Hosts with more services than this start collapsed on the full map. */
export const COLLAPSE_ABOVE = 8;

export interface HostToggle {
  /** Services this host hides when collapsed (or revealed when expanded). */
  count: number;
  expanded: boolean;
}

export interface TopologyGroup {
  id: string;
  label: string;
  members: string[];
}

export interface TopologyView {
  nodes: GraphNode[];
  edges: GraphEdge[];
  /** Only hosts that have something to collapse/expand. */
  hosts: Record<string, HostToggle>;
  groups: TopologyGroup[];
}

export interface ViewOptions {
  /** Node id in focus ("service-12"); none = the whole map. */
  focus?: string;
  /** Per-host user choice: true = expanded, false = collapsed. */
  overrides?: Record<string, boolean>;
  expandAll?: boolean;
  groupBy?: GroupBy;
}

/** The asset plus what is related to it, typed by what it is — never the
 *  other services of its host (those stay collapsed in the host card). */
function focusSet(focus: string, type: Map<string, string>, adj: Map<string, Set<string>>): Set<string> {
  const near = adj.get(focus) ?? new Set<string>();
  const keep = new Set<string>([focus, ...near]);
  const t = type.get(focus);
  const servicesNear = [...near].filter((id) => type.get(id) === "service");
  const extend = (kinds: string[]) => {
    for (const svc of servicesNear) {
      for (const n of adj.get(svc) ?? []) if (kinds.includes(type.get(n) ?? "")) keep.add(n);
    }
  };
  if (t === "host" || t === "dns") extend(["project"]);
  if (t === "project") extend(["host", "dns"]);
  return keep;
}

export function buildTopologyView(graph: GraphData, opts: ViewOptions = {}): TopologyView {
  const { focus, overrides = {}, expandAll = false, groupBy = "stack" } = opts;
  const type = new Map(graph.nodes.map((n) => [n.id, n.type as string]));
  const adj = new Map<string, Set<string>>();
  const link = (a: string, b: string) => {
    if (!adj.has(a)) adj.set(a, new Set());
    adj.get(a)!.add(b);
  };
  for (const e of graph.edges) {
    if (!type.has(e.source) || !type.has(e.target)) continue;
    link(e.source, e.target);
    link(e.target, e.source);
  }
  const servicesOf = (host: string) => [...(adj.get(host) ?? [])].filter((id) => type.get(id) === "service");
  const isExpanded = (host: string, byDefault: boolean) => expandAll || (overrides[host] ?? byDefault);

  const hosts: Record<string, HostToggle> = {};
  let visible: Set<string>;

  if (focus && type.has(focus)) {
    const base = focusSet(focus, type, adj);
    visible = new Set(base);
    for (const host of base) {
      if (type.get(host) !== "host") continue;
      const siblings = servicesOf(host).filter((s) => !base.has(s));
      if (siblings.length === 0) continue;
      const expanded = isExpanded(host, false);
      hosts[host] = { count: siblings.length, expanded };
      if (expanded) siblings.forEach((s) => visible.add(s));
    }
  } else {
    visible = new Set(type.keys());
    const collapsed = new Set<string>();
    for (const [id, t] of type) {
      if (t !== "host") continue;
      const services = servicesOf(id);
      if (services.length === 0) continue;
      const expanded = isExpanded(id, services.length <= COLLAPSE_ABOVE);
      hosts[id] = { count: services.length, expanded };
      if (!expanded) collapsed.add(id);
    }
    // A service disappears only when every host it runs on is collapsed.
    for (const [id, t] of type) {
      if (t !== "service") continue;
      const onHosts = [...(adj.get(id) ?? [])].filter((n) => type.get(n) === "host");
      if (onHosts.length > 0 && onHosts.every((h) => collapsed.has(h))) visible.delete(id);
    }
  }

  const nodes = graph.nodes.filter((n) => visible.has(n.id));
  const edges = graph.edges.filter((e) => visible.has(e.source) && visible.has(e.target));

  const groups: TopologyGroup[] = [];
  if (groupBy !== "none") {
    const field = GROUP_FIELD[groupBy];
    const byKey = new Map<string, string[]>();
    for (const n of nodes) {
      const key = n.type === "service" ? n.data?.[field] : undefined;
      if (typeof key !== "string" || key.trim() === "") continue;
      byKey.set(key, [...(byKey.get(key) ?? []), n.id]);
    }
    // ponytail: a "group" of one service is noise — needs at least two.
    for (const [key, members] of byKey) {
      if (members.length >= 2) groups.push({ id: `group-${groupBy}-${key}`, label: key, members });
    }
  }

  return { nodes, edges, hosts, groups };
}
