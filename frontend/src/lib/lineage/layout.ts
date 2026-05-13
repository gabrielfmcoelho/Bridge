import dagre from "@dagrejs/dagre";
import type { Node, Edge } from "@xyflow/react";
import type { LineageEdge, LineageNode } from "./types";
import { inferLayer, INFERRED_LAYER_ORDER, type InferredLayer } from "./style";

export const NODE_WIDTH = 200;
export const NODE_HEIGHT = 64;

export interface LayoutOptions {
  rankdir?: "TB" | "LR" | "BT" | "RL";
  nodesep?: number;
  ranksep?: number;
}

export interface LayoutResult {
  nodes: Node[];
  edges: Edge[];
}

/** Lay out the given subset of nodes + edges with dagre. */
export function layout(
  visibleNodes: LineageNode[],
  visibleEdges: LineageEdge[],
  opts: LayoutOptions = {},
): LayoutResult {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: opts.rankdir ?? "LR",
    nodesep: opts.nodesep ?? 40,
    ranksep: opts.ranksep ?? 90,
  });

  const visibleIds = new Set(visibleNodes.map(n => n.id));

  for (const n of visibleNodes) {
    g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  }
  for (const e of visibleEdges) {
    if (!visibleIds.has(e.source) || !visibleIds.has(e.target)) continue;
    if (e.kind === "contains") continue;  // structural — skip for layout
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  const nodes: Node[] = visibleNodes.map(n => {
    const pos = g.node(n.id);
    return {
      id: n.id,
      type: nodeFlowType(n.type),
      position: { x: (pos?.x ?? 0) - NODE_WIDTH / 2, y: (pos?.y ?? 0) - NODE_HEIGHT / 2 },
      data: { node: n },
    };
  });

  const edges: Edge[] = visibleEdges
    .filter(e => visibleIds.has(e.source) && visibleIds.has(e.target))
    .map((e, i) => ({
      id: `e-${i}-${e.kind}`,
      source: e.source,
      target: e.target,
      data: { edge: e },
      type: "lineage",
    }));

  return { nodes, edges };
}

/** Post-layout pass: for every visible DAG, infer its medallion layer from
 *  the label, then for each non-empty layer compute the bounding box of its
 *  DAGs (padded) and prepend a synthetic `layer_group` node behind them.
 *  Group nodes have negative z-index so they render under the DAGs without
 *  intercepting pointer events. */
const GROUP_PAD = 24;
const GROUP_LABEL_HEIGHT = 22;

export function addLayerGroups(flowNodes: Node[]): Node[] {
  const dagsByLayer = new Map<InferredLayer, Node[]>();
  for (const n of flowNodes) {
    const ln = (n.data as { node?: LineageNode } | undefined)?.node;
    if (!ln || ln.type !== "dag") continue;
    const layer = inferLayer(ln.label);
    const arr = dagsByLayer.get(layer) ?? [];
    arr.push(n);
    dagsByLayer.set(layer, arr);
  }

  const groupNodes: Node[] = [];
  for (const layer of INFERRED_LAYER_ORDER) {
    const dags = dagsByLayer.get(layer);
    if (!dags || dags.length === 0) continue;
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const n of dags) {
      minX = Math.min(minX, n.position.x);
      minY = Math.min(minY, n.position.y);
      maxX = Math.max(maxX, n.position.x + NODE_WIDTH);
      maxY = Math.max(maxY, n.position.y + NODE_HEIGHT);
    }
    groupNodes.push({
      id: `layer-group:${layer}`,
      type: "layer_group",
      position: { x: minX - GROUP_PAD, y: minY - GROUP_PAD - GROUP_LABEL_HEIGHT },
      data: { layer, label: layer.toUpperCase() },
      style: {
        width: maxX - minX + 2 * GROUP_PAD,
        height: maxY - minY + 2 * GROUP_PAD + GROUP_LABEL_HEIGHT,
        zIndex: -1,
      },
      selectable: false,
      draggable: false,
      focusable: false,
    });
  }

  // Layer-group nodes go FIRST so xyflow renders them behind the rest.
  return [...groupNodes, ...flowNodes];
}

/** Map our node `type` strings to xyflow node-type keys (we register one per
 *  source type in LineageGraph nodeTypes). */
export function nodeFlowType(t: string): string {
  switch (t) {
    case "namespace": case "dag": case "task_group": case "task":
    case "dbt_source": case "dbt_model": case "dbt_seed": case "dbt_macro":
    case "table": case "column": case "script": case "external":
      return t;
    default:
      return "table";  // safe fallback
  }
}
