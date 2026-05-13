import type { LineageIndexes } from "./indexes";

// Edges that represent meaningful data flow (used by trace mode and the
// "show only gaps" filter to determine reachability).
export const FLOW_KINDS = new Set([
  "executes", "invokes", "triggers", "reads", "writes",
  "ref", "uses_source", "column_lineage",
]);

export function traceUpstream(
  idx: LineageIndexes,
  startId: string,
  maxDepth = 6,
): { nodes: Set<string>; edges: Set<string> } {
  const nodes = new Set<string>([startId]);
  const edges = new Set<string>();
  const queue: Array<[string, number]> = [[startId, 0]];
  while (queue.length) {
    const [cur, depth] = queue.shift()!;
    if (depth >= maxDepth) continue;
    for (const e of idx.inEdges.get(cur) ?? []) {
      if (!FLOW_KINDS.has(e.kind)) continue;
      const key = edgeKey(e.source, e.target, e.kind);
      if (edges.has(key)) continue;
      edges.add(key);
      if (!nodes.has(e.source)) {
        nodes.add(e.source);
        queue.push([e.source, depth + 1]);
      }
    }
  }
  return { nodes, edges };
}

export function traceDownstream(
  idx: LineageIndexes,
  startId: string,
  maxDepth = 6,
): { nodes: Set<string>; edges: Set<string> } {
  const nodes = new Set<string>([startId]);
  const edges = new Set<string>();
  const queue: Array<[string, number]> = [[startId, 0]];
  while (queue.length) {
    const [cur, depth] = queue.shift()!;
    if (depth >= maxDepth) continue;
    for (const e of idx.outEdges.get(cur) ?? []) {
      if (!FLOW_KINDS.has(e.kind)) continue;
      const key = edgeKey(e.source, e.target, e.kind);
      if (edges.has(key)) continue;
      edges.add(key);
      if (!nodes.has(e.target)) {
        nodes.add(e.target);
        queue.push([e.target, depth + 1]);
      }
    }
  }
  return { nodes, edges };
}

/** Direct neighborhood — node itself + 1 hop in either direction. */
export function neighborhood(
  idx: LineageIndexes,
  id: string,
  kinds: Set<string> = FLOW_KINDS,
): { nodes: Set<string>; edges: Set<string> } {
  const nodes = new Set<string>([id]);
  const edges = new Set<string>();
  for (const e of idx.outEdges.get(id) ?? []) {
    if (!kinds.has(e.kind)) continue;
    edges.add(edgeKey(e.source, e.target, e.kind));
    nodes.add(e.target);
  }
  for (const e of idx.inEdges.get(id) ?? []) {
    if (!kinds.has(e.kind)) continue;
    edges.add(edgeKey(e.source, e.target, e.kind));
    nodes.add(e.source);
  }
  return { nodes, edges };
}

export function edgeKey(source: string, target: string, kind: string): string {
  return `${source}${target}${kind}`;
}
