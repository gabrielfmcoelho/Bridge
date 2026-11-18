"use client";

import { useCallback, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MarkerType,
  type Node,
  type Edge,
  type NodeMouseHandler,
  useNodesState,
  useEdgesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import dagre from "@dagrejs/dagre";
import { useRouter } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import { appDepsHref, criticalityColor, CRITICALITY_ORDER, type AppGraph, type AppGraphNodeKind } from "@/lib/appDeps";
import GroupNode from "./nodes/GroupNode";
import AppComponentNode from "./nodes/AppComponentNode";
import AppCiNode from "./nodes/AppCiNode";
import AppExternalNode from "./nodes/AppExternalNode";

const nodeTypes = {
  component: AppComponentNode,
  ci: AppCiNode,
  external: AppExternalNode,
  stackGroup: GroupNode,
};

const NODE_WIDTH = 200;
const NODE_HEIGHT: Record<AppGraphNodeKind, number> = { component: 76, ci: 40, external: 52 };
// Room around a frame's members; extra on top for its label chip.
const GROUP_PAD = 24;
const GROUP_PAD_TOP = 44;

function dagreLayout(graph: AppGraph, compound: boolean) {
  const g = new dagre.graphlib.Graph({ compound });
  g.setDefaultEdgeLabel(() => ({}));
  // Left to right, as catalog-diagram draws: a consumer left of what it uses.
  g.setGraph({ rankdir: "LR", nodesep: 40, ranksep: 120 });
  graph.nodes.forEach((n) => g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT[n.kind] }));
  if (compound) {
    graph.groups.forEach((grp) => {
      g.setNode(grp.id, {});
      grp.members.forEach((m) => g.setParent(m, grp.id));
    });
  }
  graph.edges.forEach((e) => g.setEdge(e.source, e.target));
  dagre.layout(g);
  return g;
}

function layoutElements(graph: AppGraph) {
  let g: ReturnType<typeof dagreLayout>;
  let groups = graph.groups;
  try {
    g = dagreLayout(graph, groups.length > 0);
  } catch {
    // ponytail: dagre's compound mode can throw on odd graphs — fall back to a flat
    // layout without frames rather than an empty canvas (as TopologyGraph does).
    g = dagreLayout(graph, false);
    groups = [];
  }
  const size = new Map(graph.nodes.map((n) => [n.id, NODE_HEIGHT[n.kind]]));
  const abs = new Map(graph.nodes.map((n) => {
    const p = g.node(n.id);
    return [n.id, { x: p.x - NODE_WIDTH / 2, y: p.y - NODE_HEIGHT[n.kind] / 2 }];
  }));

  // Frames first: React Flow needs a parent before its children.
  const parentOf = new Map<string, { id: string; x: number; y: number }>();
  const groupNodes: Node[] = groups.map((grp) => {
    const pts = grp.members.map((m) => ({ ...abs.get(m)!, h: size.get(m)! }));
    const x = Math.min(...pts.map((p) => p.x)) - GROUP_PAD;
    const y = Math.min(...pts.map((p) => p.y)) - GROUP_PAD_TOP;
    const width = Math.max(...pts.map((p) => p.x)) + NODE_WIDTH + GROUP_PAD - x;
    const height = Math.max(...pts.map((p) => p.y + p.h)) + GROUP_PAD - y;
    grp.members.forEach((m) => parentOf.set(m, { id: grp.id, x, y }));
    return {
      id: grp.id,
      type: "stackGroup",
      position: { x, y },
      style: { width, height, cursor: grp.catalogued && !grp.focus ? "pointer" : "default" },
      data: { label: grp.label, ci: grp.ci, catalogued: grp.catalogued, focus: grp.focus },
      selectable: false,
      zIndex: -1,
    };
  });

  const nodes: Node[] = graph.nodes.map((n) => {
    const pos = abs.get(n.id)!;
    const parent = parentOf.get(n.id);
    return {
      id: n.id,
      type: n.kind,
      position: parent ? { x: pos.x - parent.x, y: pos.y - parent.y } : pos,
      ...(parent ? { parentId: parent.id, extent: "parent" as const } : {}),
      data: { ...n },
    };
  });

  const edges: Edge[] = graph.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    label: e.label || undefined,
    markerEnd: { type: MarkerType.ArrowClosed, color: "var(--border-strong)" },
    style: { stroke: "var(--border-strong)" },
    labelStyle: { fill: "var(--text-muted)", fontSize: 10 },
    labelBgStyle: { fill: "var(--bg-base)" },
    labelBgPadding: [4, 2] as [number, number],
  }));

  return { nodes: [...groupNodes, ...nodes], edges };
}

/** The app dependencies diagram (one CI, or the whole landscape). An arrow reads
 *  "depends on"; clicking a catalogued CI other than the focus opens its diagram. */
export default function AppDepsGraph({ graph, className }: { graph: AppGraph; className?: string }) {
  const { t } = useLocale();
  const layout = useMemo(() => layoutElements(graph), [graph]);
  // React Flow seeds its node state once: a new key re-mounts (and re-fits) on a new graph.
  const layoutKey = layout.nodes.map((n) => n.id).join(",");
  return (
    <div
      className={`relative ${className || "w-full h-[calc(100vh-16rem)] min-h-[420px] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] overflow-hidden"}`}
      style={{ background: "var(--bg-base)" }}
    >
      <div className="absolute left-2 top-2 z-10 flex flex-wrap items-center gap-3 px-3 py-1.5 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-2xs text-[var(--text-secondary)]">
        {CRITICALITY_ORDER.map((c) => (
          <span key={c} className="inline-flex items-center gap-1.5">
            <span className="w-3 h-1 rounded-full" style={{ background: criticalityColor(c) }} />
            {t(`appDeps.criticality.${c}`)}
          </span>
        ))}
        <span className="inline-flex items-center gap-1.5">
          <span className="w-3 h-3 rounded-full border border-dashed border-[var(--text-muted)]" />
          {t("appDeps.legend.external")}
        </span>
        <span>{t("appDeps.legend.arrow")}</span>
      </div>
      <Canvas key={layoutKey} nodes={layout.nodes} edges={layout.edges} />
    </div>
  );
}

function Canvas({ nodes: initialNodes, edges: initialEdges }: { nodes: Node[]; edges: Edge[] }) {
  const router = useRouter();
  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);

  const onNodeClick: NodeMouseHandler = useCallback(
    (_, node) => {
      const d = node.data as { ci?: string; catalogued?: boolean; focus?: boolean };
      if (d.ci && d.catalogued && !d.focus) router.push(appDepsHref(d.ci));
    },
    [router],
  );

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={onNodeClick}
      nodeTypes={nodeTypes}
      fitView
      fitViewOptions={{ maxZoom: 1, padding: 0.2 }}
      minZoom={0.1}
      proOptions={{ hideAttribution: true }}
      style={{ background: "var(--bg-base)" }}
    >
      <Background color="var(--border-subtle)" gap={24} size={1} />
      <Controls
        style={{
          background: "var(--bg-surface)",
          borderColor: "var(--border-default)",
          borderRadius: "var(--radius-md)",
          overflow: "hidden",
        }}
      />
    </ReactFlow>
  );
}
