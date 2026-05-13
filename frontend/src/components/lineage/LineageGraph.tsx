"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ReactFlow, Background, Controls, MiniMap,
  type Node, type Edge, type NodeMouseHandler,
  useNodesState, useEdgesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { LineageIndexes } from "@/lib/lineage/indexes";
import type { LineageEdge as LineageEdgeT, LineageNode } from "@/lib/lineage/types";
import { layout, addLayerGroups } from "@/lib/lineage/layout";
import { NODE_TYPES } from "./nodes/NodeRegistry";
import LineageEdgeComponent from "./LineageEdge";
import LineageToolbar, { type GraphFilters } from "./LineageToolbar";
import LineageLegend from "./LineageLegend";
import DetailDrawer from "./DetailDrawer";
import SearchOmnibar from "./SearchOmnibar";
import { minimapColor } from "@/lib/lineage/style";
import { traceDownstream, traceUpstream, neighborhood, FLOW_KINDS } from "@/lib/lineage/traversal";

const edgeTypes = { lineage: LineageEdgeComponent };

interface Props {
  indexes: LineageIndexes;
  focusId?: string | null;
}

export default function LineageGraph({ indexes, focusId }: Props) {
  const [filters, setFilters] = useState<GraphFilters>({
    namespace: "all",
    layer: "all",
    type: "all",
    onlyGaps: false,
    showColumnEdges: false,
    trace: "none",
  });
  // Auto-expand every namespace on first mount so users see DAGs without
  // having to discover that namespaces are click-to-expand.
  const [expanded, setExpanded] = useState<Set<string>>(() => {
    const init = new Set<string>();
    for (const ns of indexes.nodesByType.get("namespace") ?? []) init.add(ns.id);
    return init;
  });
  const [selectedId, setSelectedId] = useState<string | null>(focusId ?? null);
  const [searchOpen, setSearchOpen] = useState(false);

  // Sync external focus → selection (Issues click-through deep links).
  // React's recommended "adjust state during render based on prop change".
  const [prevFocusId, setPrevFocusId] = useState<string | null | undefined>(focusId);
  if (focusId !== prevFocusId) {
    setPrevFocusId(focusId);
    if (focusId) {
      setSelectedId(focusId);
      setExpanded(prev => {
        const next = new Set(prev);
        ancestorPath(indexes, focusId).forEach(id => next.add(id));
        return next;
      });
    }
  }

  // Keyboard shortcuts
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setSearchOpen(true);
      }
      if (e.key === "f") {
        const evt = new CustomEvent("lineage:fit");
        window.dispatchEvent(evt);
      }
      if (e.key === "Escape") setSearchOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // Compute visible set ------------------------------------------------
  const visible = useMemo(() => computeVisible(indexes, expanded, selectedId, filters), [indexes, expanded, selectedId, filters]);

  const { nodes: rawLayoutedNodes, edges: layoutedEdges } = useMemo(
    () => layout(visible.nodes, visible.edges, { rankdir: "LR" }),
    [visible.nodes, visible.edges],
  );
  // Medallion bands behind DAG nodes.
  const layoutedNodes = useMemo(() => addLayerGroups(rawLayoutedNodes), [rawLayoutedNodes]);

  // Apply gap markers + trace dim
  const finalNodes = useMemo(() => layoutedNodes.map(n => decorate(n, indexes, visible.tracePath?.nodes, selectedId)), [layoutedNodes, indexes, visible.tracePath, selectedId]);
  const finalEdges = useMemo(() => layoutedEdges.map(e => decorateEdge(e, visible.tracePath?.edges)), [layoutedEdges, visible.tracePath]);

  const [nodes, setNodes, onNodesChange] = useNodesState(finalNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(finalEdges);
  useEffect(() => setNodes(finalNodes), [finalNodes, setNodes]);
  useEffect(() => setEdges(finalEdges), [finalEdges, setEdges]);

  const onNodeClick: NodeMouseHandler = useCallback((_, n) => {
    setSelectedId(n.id);
    // Container nodes — single click also toggles expansion. Leaf nodes
    // just select for the drawer.
    const lineageNode = (n.data as { node: LineageNode } | undefined)?.node;
    const isContainer = lineageNode && ["namespace", "dag", "task_group"].includes(lineageNode.type);
    if (isContainer) {
      setExpanded(prev => {
        const next = new Set(prev);
        if (next.has(n.id)) next.delete(n.id); else next.add(n.id);
        return next;
      });
    }
  }, []);

  const onNodeDoubleClick: NodeMouseHandler = useCallback((_, n) => {
    // Double-click on any node toggles expansion (useful for non-container
    // nodes too — expanding shows direct lineage neighbors).
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(n.id)) next.delete(n.id); else next.add(n.id);
      return next;
    });
  }, []);

  const selectedNode = selectedId ? indexes.nodesById.get(selectedId) ?? null : null;

  return (
    <div className="flex flex-col h-full">
      <LineageToolbar
        indexes={indexes}
        filters={filters}
        onChange={setFilters}
        onOpenSearch={() => setSearchOpen(true)}
      />
      <div className="relative flex-1 min-h-0 border-t border-[var(--border-subtle)]" style={{ background: "var(--bg-base)" }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onNodeClick={onNodeClick}
          onNodeDoubleClick={onNodeDoubleClick}
          nodeTypes={NODE_TYPES}
          edgeTypes={edgeTypes}
          fitView
          fitViewOptions={{ maxZoom: 1, padding: 0.2 }}
          proOptions={{ hideAttribution: true }}
          minZoom={0.05}
          maxZoom={2}
        >
          <Background color="var(--border-subtle)" gap={24} size={1} />
          <Controls
            style={{ background: "var(--bg-surface)", borderColor: "var(--border-default)", borderRadius: "var(--radius-md)", overflow: "hidden" }}
          />
          <MiniMap
            nodeColor={(n) => minimapColor(typeof n.type === "string" ? n.type : "")}
            className="hidden md:block"
            style={{ background: "var(--bg-surface)", border: "1px solid var(--border-subtle)", borderRadius: "var(--radius-md)" }}
          />
        </ReactFlow>
        <div className="absolute bottom-3 left-3 pointer-events-none">
          <LineageLegend />
        </div>
      </div>
      <DetailDrawer
        node={selectedNode}
        indexes={indexes}
        onClose={() => setSelectedId(null)}
        onSelect={(id) => setSelectedId(id)}
      />
      <SearchOmnibar
        open={searchOpen}
        indexes={indexes}
        onClose={() => setSearchOpen(false)}
        onPick={(id) => { setSelectedId(id); setSearchOpen(false); setExpanded(prev => new Set([...prev, ...ancestorPath(indexes, id)])); }}
      />
    </div>
  );
}

// --- Visibility computation -------------------------------------------------

function computeVisible(
  idx: LineageIndexes,
  expanded: Set<string>,
  selectedId: string | null,
  filters: GraphFilters,
): { nodes: LineageNode[]; edges: LineageEdgeT[]; tracePath?: { nodes: Set<string>; edges: Set<string> } } {
  const namespaces = idx.nodesByType.get("namespace") ?? [];
  const dags = idx.nodesByType.get("dag") ?? [];
  const visibleSet = new Set<string>();

  // Always show namespaces unless filtered
  for (const ns of namespaces) {
    if (filters.namespace !== "all" && ns.label !== filters.namespace) continue;
    visibleSet.add(ns.id);
    if (!expanded.has(ns.id)) continue;
    // Show DAGs inside this namespace
    for (const dag of dags) {
      if (dag.namespace !== ns.label) continue;
      visibleSet.add(dag.id);
      if (!expanded.has(dag.id)) continue;
      // Expand DAG: show tasks + task groups
      for (const childId of idx.childrenOf.get(dag.id) ?? []) {
        const child = idx.nodesById.get(childId);
        if (!child) continue;
        visibleSet.add(childId);
        if (child.type === "task_group" && expanded.has(childId)) {
          for (const gc of idx.childrenOf.get(childId) ?? []) visibleSet.add(gc);
        }
      }
      // Add neighborhood lineage targets of any task in this DAG
      for (const childId of idx.childrenOf.get(dag.id) ?? []) {
        const child = idx.nodesById.get(childId);
        if (!child) continue;
        if (child.type !== "task" && child.type !== "task_group") continue;
        for (const e of idx.outEdges.get(childId) ?? []) {
          if (!FLOW_KINDS.has(e.kind)) continue;
          visibleSet.add(e.target);
        }
      }
    }
  }

  // If a non-namespace/dag node is selected (e.g. via deep-link), include
  // its neighborhood directly.
  let tracePath: { nodes: Set<string>; edges: Set<string> } | undefined;
  if (selectedId) {
    const sel = idx.nodesById.get(selectedId);
    if (sel) {
      visibleSet.add(selectedId);
      if (filters.trace === "upstream") tracePath = traceUpstream(idx, selectedId);
      else if (filters.trace === "downstream") tracePath = traceDownstream(idx, selectedId);
      else {
        const nb = neighborhood(idx, selectedId);
        nb.nodes.forEach(id => visibleSet.add(id));
      }
      tracePath?.nodes.forEach(id => visibleSet.add(id));
    }
  }

  // "Show only gaps" filter
  if (filters.onlyGaps) {
    const gapIds = collectGapIds(idx);
    for (const id of Array.from(visibleSet)) {
      if (gapIds.has(id)) continue;
      // Keep if directly adjacent to a gap node
      const hasGapNeighbor = (idx.outEdges.get(id) ?? []).some(e => gapIds.has(e.target))
        || (idx.inEdges.get(id) ?? []).some(e => gapIds.has(e.source));
      if (!hasGapNeighbor) visibleSet.delete(id);
    }
  }

  // Type filter
  if (filters.type !== "all") {
    for (const id of Array.from(visibleSet)) {
      const n = idx.nodesById.get(id);
      if (n && n.type !== filters.type) visibleSet.delete(id);
    }
  }

  // Layer filter (only applies to dbt_model nodes)
  if (filters.layer !== "all") {
    for (const id of Array.from(visibleSet)) {
      const n = idx.nodesById.get(id);
      if (n?.type === "dbt_model" && n.layer !== filters.layer) visibleSet.delete(id);
    }
  }

  const visibleNodes = Array.from(visibleSet).map(id => idx.nodesById.get(id)!).filter(Boolean);
  const visibleEdges = idx.raw.edges.filter(e => {
    if (!visibleSet.has(e.source) || !visibleSet.has(e.target)) return false;
    if (e.kind === "column_lineage" && !filters.showColumnEdges) return false;
    return true;
  });

  return { nodes: visibleNodes, edges: visibleEdges, tracePath };
}

function collectGapIds(idx: LineageIndexes): Set<string> {
  const out = new Set<string>();
  const gaps = idx.raw.coverage?.gaps ?? {};
  for (const arr of [gaps.orphan_tasks, gaps.isolated_models, gaps.unused_sources, gaps.unused_macros]) {
    for (const id of arr ?? []) out.add(id);
  }
  for (const w of idx.raw.warnings ?? []) {
    if (w.source_id) out.add(w.source_id);
  }
  return out;
}

function ancestorPath(idx: LineageIndexes, id: string): string[] {
  const out: string[] = [];
  let cur: string | undefined = id;
  while (cur) {
    out.push(cur);
    const node = idx.nodesById.get(cur);
    cur = node?.parent;
  }
  return out;
}

// --- Node/edge decoration --------------------------------------------------

function decorate(
  flowNode: Node,
  idx: LineageIndexes,
  traceNodes: Set<string> | undefined,
  selectedId: string | null,
): Node {
  // Layer-group containers have no lineage node — pass through untouched.
  if (flowNode.type === "layer_group") return flowNode;
  const lineageNode = (flowNode.data as { node: LineageNode }).node;
  const gaps = idx.raw.coverage?.gaps ?? {};
  let gapMarker: "warning" | "orphan" | "isolated" | "unused" | null = null;
  if (idx.warningsByNode.get(lineageNode.id)) gapMarker = "warning";
  else if (gaps.orphan_tasks?.includes(lineageNode.id)) gapMarker = "orphan";
  else if (gaps.isolated_models?.includes(lineageNode.id)) gapMarker = "isolated";
  else if (gaps.unused_sources?.includes(lineageNode.id) || gaps.unused_macros?.includes(lineageNode.id)) gapMarker = "unused";

  const isFocused = flowNode.id === selectedId;
  const isDimmed = traceNodes && !traceNodes.has(flowNode.id);

  return {
    ...flowNode,
    data: { ...flowNode.data, gapMarker },
    style: {
      opacity: isDimmed ? 0.25 : 1,
      filter: isFocused ? "drop-shadow(0 0 0 2px var(--accent))" : undefined,
      transition: "opacity 0.2s, filter 0.2s",
    },
    selected: isFocused,
  };
}

function decorateEdge(flowEdge: Edge, traceEdges: Set<string> | undefined): Edge {
  if (!traceEdges) return flowEdge;
  const edge = (flowEdge.data as { edge?: LineageEdgeT } | undefined)?.edge;
  if (!edge) return flowEdge;
  const key = `${edge.source}${edge.target}${edge.kind}`;
  const inPath = traceEdges.has(key);
  return {
    ...flowEdge,
    style: { ...flowEdge.style, opacity: inPath ? 1 : 0.15 },
  };
}
