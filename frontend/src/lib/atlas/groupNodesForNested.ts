import Dagre from "@dagrejs/dagre";
import type { Node, Edge } from "@xyflow/react";
import type { AtlasIndexes, AtlasFilters } from "./types";
import type { LineageNode, LineageEdge } from "@/lib/lineage/types";

const ENTITY_W = 220;
const ENTITY_H = 60;
const DAG_PADDING = 24;
const DOMAIN_PADDING = 36;
const SUB_NODESEP = 18;
const SUB_RANKSEP = 36;
const DOMAIN_NODESEP = 48;
const DOMAIN_RANKSEP = 80;

export interface NestedLayout {
  nodes: Node[];
  edges: Edge[];
}

/**
 * Builds React Flow nodes with nested parents:
 *
 *   domain frame  (top-level, parentId undefined)
 *   └── DAG frame  (parentId = domain id, extent: "parent")
 *       └── task / dbt_model leaves (parentId = DAG id, extent: "parent")
 *
 * External tables (those written to / read from by a DAG's contents) are
 * rendered as top-level nodes positioned to the right of their producing DAG.
 *
 * Layout strategy:
 *  - Dagre laid out PER DAG to size + position children
 *  - Dagre laid out PER domain to position DAGs (and external tables linked
 *    to them, treated as same-level nodes for ranking)
 *  - Dagre laid out across DOMAINS to position the domain frames
 */
export function groupNodesForNested(
  idx: AtlasIndexes,
  filters: AtlasFilters,
  collapsedDagIds: Set<string> = new Set(),
): NestedLayout {
  const domains = filters.domains.length > 0 ? filters.domains : idx.namespaces;
  const result: Node[] = [];
  const edges: Edge[] = [];
  const visibleIds = new Set<string>();

  // Track external tables we've already added at the domain level.
  const seenExternal = new Set<string>();

  // --- Per-DAG layout -------------------------------------------------------
  interface DagSize { id: string; w: number; h: number; }
  interface DomainEntry { ns: string; dags: DagSize[]; }
  const domainEntries: DomainEntry[] = [];

  for (const ns of domains) {
    const dagNodes = (idx.nodesByType.get("dag") ?? []).filter(n => n.namespace === ns);
    const dagSizes: DagSize[] = [];

    for (const dag of dagNodes) {
      // Children: tasks and dbt models executed from the DAG's tasks.
      const children: LineageNode[] = [];
      const seenChildren = new Set<string>();
      const childIds = idx.childrenOf.get(dag.id) ?? [];
      for (const cid of childIds) {
        const c = idx.nodesById.get(cid);
        if (!c) continue;
        if (c.type === "task" || c.type === "task_group") {
          if (!seenChildren.has(c.id)) { seenChildren.add(c.id); children.push(c); }
          // also pull in models executed by tasks (treated as children of the DAG)
          if (c.type === "task") {
            for (const e of idx.outEdges.get(c.id) ?? []) {
              if (e.kind !== "executes") continue;
              const model = idx.nodesById.get(e.target);
              if (!model || model.type !== "dbt_model") continue;
              if (!seenChildren.has(model.id)) { seenChildren.add(model.id); children.push(model); }
            }
          }
        }
      }

      if (collapsedDagIds.has(dag.id)) {
        const w = ENTITY_W + 40;
        const h = ENTITY_H + 8;
        result.push({
          id: dag.id,
          type: "pipelineDagFrame",
          data: { node: dag, collapsed: true, childCount: children.length },
          position: { x: 0, y: 0 }, // filled in later
          style: { width: w, height: h },
        });
        visibleIds.add(dag.id);
        dagSizes.push({ id: dag.id, w, h });
        continue;
      }

      // Layout the children with Dagre to compute frame size.
      const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
      g.setGraph({ rankdir: "TB", ranker: "tight-tree", nodesep: SUB_NODESEP, ranksep: SUB_RANKSEP, marginx: DAG_PADDING, marginy: DAG_PADDING + 28 });
      for (const c of children) g.setNode(c.id, { width: ENTITY_W, height: ENTITY_H });
      // Edges between siblings (depends_on, executes within this DAG)
      const childSet = new Set(children.map(c => c.id));
      for (const e of idx.raw.edges) {
        if (!childSet.has(e.source) || !childSet.has(e.target)) continue;
        if (e.kind === "depends_on" || e.kind === "executes" || e.kind === "ref") {
          g.setEdge(e.source, e.target);
        }
      }
      Dagre.layout(g);
      const bb = computeBoundingBox(g);
      const frameW = Math.max(ENTITY_W + DAG_PADDING * 2, bb.w + DAG_PADDING * 2);
      const frameH = Math.max(ENTITY_H + DAG_PADDING * 2 + 28, bb.h + DAG_PADDING * 2 + 28);

      // Push the DAG frame.
      result.push({
        id: dag.id,
        type: "pipelineDagFrame",
        data: { node: dag, collapsed: false, childCount: children.length },
        position: { x: 0, y: 0 },
        style: { width: frameW, height: frameH },
      });
      visibleIds.add(dag.id);

      // Push children, positioned relative to the parent frame.
      for (const c of children) {
        const pos = g.node(c.id);
        result.push({
          id: c.id,
          type: "pipelineEntity",
          data: { node: c },
          parentId: dag.id,
          extent: "parent",
          position: {
            x: pos.x - ENTITY_W / 2,
            y: pos.y - ENTITY_H / 2,
          },
          style: { width: ENTITY_W, height: ENTITY_H },
        });
        visibleIds.add(c.id);
      }

      dagSizes.push({ id: dag.id, w: frameW, h: frameH });
    }

    domainEntries.push({ ns, dags: dagSizes });
  }

  // --- External tables: those written by / read by anything visible ---------
  // Render at the domain frame level (alongside DAGs).
  for (const ns of domains) {
    const localExternalTables: LineageNode[] = [];
    for (const dagSize of domainEntries.find(d => d.ns === ns)?.dags ?? []) {
      const dagId = dagSize.id;
      // Find tables read or written by *any* task in this DAG.
      const collectTables = (taskId: string) => {
        for (const e of idx.outEdges.get(taskId) ?? []) {
          if (e.kind === "writes" || e.kind === "reads") {
            const tbl = idx.nodesById.get(e.target);
            if (tbl?.type === "table" && !seenExternal.has(tbl.id)) {
              seenExternal.add(tbl.id);
              localExternalTables.push(tbl);
            }
          }
        }
      };
      for (const cid of idx.childrenOf.get(dagId) ?? []) {
        const c = idx.nodesById.get(cid);
        if (c?.type === "task") collectTables(cid);
      }
    }
    // Tables get to live at the top-level under no parent.
    for (const tbl of localExternalTables) {
      result.push({
        id: tbl.id,
        type: "pipelineEntity",
        data: { node: tbl, isExternal: true },
        position: { x: 0, y: 0 },
        style: { width: ENTITY_W, height: ENTITY_H },
      });
      visibleIds.add(tbl.id);
    }
  }

  // --- Layout the domain frames + their contents (top-level Dagre) ----------
  // Each domain becomes a single super-node sized by the bounding box of its DAGs
  // + tables. Then we lay domains out left → right.
  interface DomainBox { ns: string; w: number; h: number; }
  const domainBoxes: DomainBox[] = [];
  const inDomainOffsets = new Map<string, { x: number; y: number }>();
  const inDomainPositions = new Map<string, { ns: string; cx: number; cy: number; w: number; h: number }>();

  for (const entry of domainEntries) {
    // Build a sub-graph: DAGs of this domain + external tables related to those DAGs.
    const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
    g.setGraph({ rankdir: "LR", ranker: "tight-tree", nodesep: DOMAIN_NODESEP, ranksep: DOMAIN_RANKSEP, marginx: DOMAIN_PADDING, marginy: DOMAIN_PADDING + 28 });

    for (const d of entry.dags) g.setNode(d.id, { width: d.w, height: d.h });

    // Add external tables in this domain.
    const externals: string[] = [];
    for (const tbl of result) {
      if ((tbl.data as { isExternal?: boolean }).isExternal && (tbl.data as { node: LineageNode }).node.namespace === entry.ns) {
        externals.push(tbl.id);
        g.setNode(tbl.id, { width: ENTITY_W, height: ENTITY_H });
      }
    }

    // Edges: DAG → table where any task in the DAG writes the table.
    for (const dagSize of entry.dags) {
      for (const cid of idx.childrenOf.get(dagSize.id) ?? []) {
        const c = idx.nodesById.get(cid);
        if (c?.type !== "task") continue;
        for (const e of idx.outEdges.get(cid) ?? []) {
          if (e.kind === "writes" && externals.includes(e.target)) {
            g.setEdge(dagSize.id, e.target);
          } else if (e.kind === "reads" && externals.includes(e.target)) {
            g.setEdge(e.target, dagSize.id);
          }
        }
      }
    }

    Dagre.layout(g);
    const bb = computeBoundingBox(g);
    const w = bb.w + DOMAIN_PADDING * 2;
    const h = bb.h + DOMAIN_PADDING * 2 + 28;
    domainBoxes.push({ ns: entry.ns, w, h });

    // Translate node centers → top-left positions, with a domain-local origin.
    for (const d of entry.dags) {
      const p = g.node(d.id);
      inDomainPositions.set(d.id, { ns: entry.ns, cx: p.x, cy: p.y, w: d.w, h: d.h });
    }
    for (const tid of externals) {
      const p = g.node(tid);
      inDomainPositions.set(tid, { ns: entry.ns, cx: p.x, cy: p.y, w: ENTITY_W, h: ENTITY_H });
    }
  }

  // Place domains left to right with vertical centering.
  let nextX = 0;
  const DOMAIN_GAP = 60;
  for (const box of domainBoxes) {
    inDomainOffsets.set(box.ns, { x: nextX, y: 0 });
    nextX += box.w + DOMAIN_GAP;
  }

  // Inject domain frame nodes.
  for (const box of domainBoxes) {
    const offset = inDomainOffsets.get(box.ns)!;
    result.unshift({
      id: `__domain_${box.ns}`,
      type: "pipelineDomainFrame",
      data: { ns: box.ns },
      position: { x: offset.x, y: offset.y },
      style: { width: box.w, height: box.h },
      draggable: false,
      selectable: false,
    });
    visibleIds.add(`__domain_${box.ns}`);
  }

  // Apply final positions for DAG frames + external tables relative to their domain frame.
  for (const node of result) {
    if (node.type === "pipelineDomainFrame") continue;
    const pos = inDomainPositions.get(node.id);
    if (!pos) continue;
    const offset = inDomainOffsets.get(pos.ns);
    if (!offset) continue;
    // Position node relative to domain (which is the parent we'll assign now).
    node.parentId = `__domain_${pos.ns}`;
    node.extent = "parent";
    node.position = {
      x: pos.cx - pos.w / 2,
      y: pos.cy - pos.h / 2,
    };
  }

  // Edges: keep only those between visible nodes; styling matches the graph aesthetic.
  const KEEP_KINDS = new Set(["depends_on", "executes", "writes", "reads", "ref", "uses_source", "triggers"]);
  const seenEdge = new Set<string>();
  for (const e of idx.raw.edges) {
    if (!KEEP_KINDS.has(e.kind)) continue;
    if (!visibleIds.has(e.source) || !visibleIds.has(e.target)) continue;
    const key = `${e.source}__${e.target}__${e.kind}`;
    if (seenEdge.has(key)) continue;
    seenEdge.add(key);
    edges.push({
      id: key,
      source: e.source,
      target: e.target,
      type: "default",
      data: { kind: e.kind, edge: e as LineageEdge },
      style: edgeStyleFor(e.kind),
    });
  }

  return { nodes: result, edges };
}

function computeBoundingBox(g: InstanceType<typeof Dagre.graphlib.Graph>): { w: number; h: number } {
  let maxX = 0, maxY = 0;
  g.nodes().forEach((n: string) => {
    const node = g.node(n) as { x: number; y: number; width: number; height: number };
    const r = node.x + node.width / 2;
    const b = node.y + node.height / 2;
    if (r > maxX) maxX = r;
    if (b > maxY) maxY = b;
  });
  return { w: maxX, h: maxY };
}

function edgeStyleFor(kind: string): React.CSSProperties {
  switch (kind) {
    case "writes":      return { stroke: "#f87171", strokeWidth: 1.5 };
    case "reads":       return { stroke: "#7dd3fc", strokeWidth: 1 };
    case "uses_source": return { stroke: "#60a5fa", strokeWidth: 1.25 };
    case "ref":         return { stroke: "var(--text-primary)", strokeWidth: 1.25 };
    case "executes":    return { stroke: "#10b981", strokeWidth: 1.25, strokeDasharray: "4 3" };
    case "depends_on":  return { stroke: "var(--border-default)", strokeWidth: 1 };
    case "triggers":    return { stroke: "#3b82f6", strokeWidth: 1.5, strokeDasharray: "5 4" };
    default:            return { stroke: "var(--border-default)", strokeWidth: 1 };
  }
}
