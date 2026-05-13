import Dagre from "@dagrejs/dagre";
import type { Node, Edge } from "@xyflow/react";
import type { AtlasIndexes, AtlasFilters } from "./types";
import type { LineageNode, LineageEdge } from "@/lib/lineage/types";
import type { TableLayer } from "@/lib/lineage/indexes";

const LANE_WIDTH = 320;
const LANE_GUTTER = 64;
const LANE_HEADER_HEIGHT = 56;
const NODE_W = 220;
const NODE_H = 64;

export const SWIMLANES: readonly TableLayer[] = ["source", "bronze", "silver", "gold"];

const LANE_X: Record<TableLayer, number> = {
  source: 0,
  bronze: LANE_WIDTH + LANE_GUTTER,
  silver: 2 * (LANE_WIDTH + LANE_GUTTER),
  gold:   3 * (LANE_WIDTH + LANE_GUTTER),
  iapep:  4 * (LANE_WIDTH + LANE_GUTTER),
  iaspi:  4 * (LANE_WIDTH + LANE_GUTTER),
  other:  4 * (LANE_WIDTH + LANE_GUTTER),
};

export interface SwimlaneLayout {
  nodes: Node[];
  edges: Edge[];
  laneCount: number;
  lanes: TableLayer[];
}

export function layoutSwimlanes(idx: AtlasIndexes, filters: AtlasFilters): SwimlaneLayout {
  const domains = filters.domains;
  const passesDomain = (ns: string | undefined) =>
    domains.length === 0 || (ns ? domains.includes(ns) : false);

  // 1. Collect entities per lane.
  type Entity = { node: LineageNode; lane: TableLayer; ns: string };
  const entities: Entity[] = [];

  for (const tableRec of idx.tables) {
    if (!passesDomain(tableRec.namespace)) continue;
    if (!SWIMLANES.includes(tableRec.layer)) continue;
    entities.push({ node: tableRec.node, lane: tableRec.layer, ns: tableRec.namespace });
  }

  for (const model of idx.nodesByType.get("dbt_model") ?? []) {
    if (!passesDomain(model.namespace)) continue;
    const lane = (model.layer as TableLayer) || inferLaneFromOutputs(idx, model.id);
    if (!lane || !SWIMLANES.includes(lane)) continue;
    entities.push({ node: model, lane, ns: model.namespace ?? "shared" });
  }

  for (const src of idx.nodesByType.get("dbt_source") ?? []) {
    if (!passesDomain(src.namespace)) continue;
    entities.push({ node: src, lane: "source", ns: src.namespace ?? "shared" });
  }

  const entityIds = new Set(entities.map(e => e.node.id));

  // 2. Run Dagre PER lane to get y positions inside each lane.
  const laneByEntity = new Map<string, TableLayer>();
  entities.forEach(e => laneByEntity.set(e.node.id, e.lane));

  const nodes: Node[] = [];

  // Render lane headers first as non-interactive nodes.
  SWIMLANES.forEach((lane, i) => {
    nodes.push({
      id: `__lane_${lane}`,
      type: "swimlaneHeader",
      data: { lane, label: lane },
      position: { x: i * (LANE_WIDTH + LANE_GUTTER), y: 0 },
      draggable: false,
      selectable: false,
      style: { width: LANE_WIDTH, height: LANE_HEADER_HEIGHT },
    });
  });

  for (const lane of SWIMLANES) {
    const inLane = entities.filter(e => e.lane === lane);
    if (inLane.length === 0) continue;
    const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
    g.setGraph({ rankdir: "TB", ranker: "tight-tree", nodesep: 16, ranksep: 28, marginx: 16, marginy: 16 });
    for (const e of inLane) g.setNode(e.node.id, { width: NODE_W, height: NODE_H });
    // Only intra-lane edges + edges into this lane drive ranking.
    for (const ed of idx.raw.edges) {
      if (!entityIds.has(ed.source) || !entityIds.has(ed.target)) continue;
      const sl = laneByEntity.get(ed.source);
      const tl = laneByEntity.get(ed.target);
      if (sl === lane && tl === lane) {
        g.setEdge(ed.source, ed.target);
      } else if (tl === lane && sl && sl !== lane) {
        // Cross-lane inbound: ensure target is laid out as a downstream of its peers.
        // Dagre needs the upstream to exist — add a phantom that we never render.
        const phantomId = `__phantom_${ed.source}_${lane}`;
        if (!g.hasNode(phantomId)) g.setNode(phantomId, { width: 1, height: 1 });
        g.setEdge(phantomId, ed.target);
      }
    }
    Dagre.layout(g);

    for (const e of inLane) {
      const pos = g.node(e.node.id);
      const x = LANE_X[lane] + (pos.x - NODE_W / 2);
      const y = LANE_HEADER_HEIGHT + (pos.y - NODE_H / 2);
      nodes.push({
        id: e.node.id,
        type: "pipelineEntity",
        data: { node: e.node, lane: e.lane, ns: e.ns },
        position: { x, y },
        style: { width: NODE_W, height: NODE_H },
      });
    }
  }

  // 3. Cross-lane edges visible in the graph: filter for those between entities in entityIds and with a meaningful kind.
  const KEEP_KINDS = new Set(["writes", "reads", "uses_source", "ref", "executes"]);
  const edges: Edge[] = [];
  const seen = new Set<string>();
  for (const e of idx.raw.edges) {
    if (!KEEP_KINDS.has(e.kind)) continue;
    if (!entityIds.has(e.source) || !entityIds.has(e.target)) continue;
    const key = `${e.source}__${e.target}__${e.kind}`;
    if (seen.has(key)) continue;
    seen.add(key);
    edges.push({
      id: key,
      source: e.source,
      target: e.target,
      type: "default",
      data: { kind: e.kind, edge: e as LineageEdge },
      style: edgeStyleFor(e.kind),
    });
  }

  return { nodes, edges, laneCount: SWIMLANES.length, lanes: [...SWIMLANES] };
}

function inferLaneFromOutputs(idx: AtlasIndexes, modelId: string): TableLayer | null {
  for (const e of idx.outEdges.get(modelId) ?? []) {
    if (e.kind === "writes") {
      const t = idx.tablesById.get(e.target);
      if (t) return t.layer;
    }
  }
  return null;
}

function edgeStyleFor(kind: string): React.CSSProperties {
  switch (kind) {
    case "writes":      return { stroke: "#f87171", strokeWidth: 2 };
    case "reads":       return { stroke: "#7dd3fc", strokeWidth: 1 };
    case "uses_source": return { stroke: "#60a5fa", strokeWidth: 1.5 };
    case "ref":         return { stroke: "var(--text-primary)", strokeWidth: 1.5 };
    case "executes":    return { stroke: "#10b981", strokeWidth: 1.5 };
    default:            return { stroke: "var(--border-default)", strokeWidth: 1 };
  }
}
