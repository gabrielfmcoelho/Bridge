"use client";

import { useMemo, useCallback, useState } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
  type NodeMouseHandler,
  useNodesState,
  useEdgesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import dagre from "@dagrejs/dagre";
import { useRouter } from "next/navigation";
import type { GraphData } from "@/lib/types";
import { buildTopologyView, type GroupBy, type TopologyView } from "@/lib/topology";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import PillButton from "@/components/ui/PillButton";
import HostNode from "./nodes/HostNode";
import ServiceNode from "./nodes/ServiceNode";
import DnsNode from "./nodes/DnsNode";
import ProjectNode from "./nodes/ProjectNode";
import ApiNode from "./nodes/ApiNode";
import GroupNode from "./nodes/GroupNode";

const nodeTypes = {
  host: HostNode,
  service: ServiceNode,
  dns: DnsNode,
  project: ProjectNode,
  api: ApiNode,
  stackGroup: GroupNode,
};

const NODE_WIDTH = 200;
const NODE_HEIGHT = 80;
// Room around a group's members; extra on top for its label chip.
const GROUP_PAD = 24;
const GROUP_PAD_TOP = 44;

// Desired rank: project at top (0), APIs and services below, dns, host at bottom.
// An API sits on its services' rank: it is served by them, not above them.
const RANK_ORDER: Record<string, number> = {
  project: 0,
  api: 1,
  service: 1,
  dns: 2,
  host: 3,
};

function dagreLayout(view: TopologyView, compound: boolean) {
  const g = new dagre.graphlib.Graph({ compound });
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "TB", nodesep: 60, ranksep: 100 });

  const rankMap = new Map<string, number>();
  view.nodes.forEach((node) => {
    rankMap.set(node.id, RANK_ORDER[node.type] ?? 2);
    g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  });
  // Compound: members of a group are laid out together, so its frame never
  // swallows an unrelated node.
  if (compound) {
    view.groups.forEach((grp) => {
      g.setNode(grp.id, {});
      grp.members.forEach((m) => g.setParent(m, grp.id));
    });
  }
  // Orient edges top → bottom by rank (layout only; rendered edges keep their direction).
  view.edges.forEach((edge) => {
    const src = rankMap.get(edge.source) ?? 2;
    const tgt = rankMap.get(edge.target) ?? 2;
    if (src <= tgt) g.setEdge(edge.source, edge.target);
    else g.setEdge(edge.target, edge.source);
  });
  dagre.layout(g);
  return g;
}

function getLayoutedElements(view: TopologyView, onToggleHost: (id: string) => void) {
  let g: ReturnType<typeof dagreLayout>;
  let groups = view.groups;
  try {
    g = dagreLayout(view, groups.length > 0);
  } catch {
    // ponytail: dagre's compound mode can throw on odd graphs — fall back to a flat
    // layout without frames rather than an empty canvas.
    g = dagreLayout(view, false);
    groups = [];
  }

  const abs = new Map(view.nodes.map((n) => {
    const p = g.node(n.id);
    return [n.id, { x: p.x - NODE_WIDTH / 2, y: p.y - NODE_HEIGHT / 2 }];
  }));

  // Group frames first: React Flow needs a parent before its children.
  const parentOf = new Map<string, { id: string; x: number; y: number }>();
  const groupNodes: Node[] = groups.map((grp) => {
    const pts = grp.members.map((m) => abs.get(m)!);
    const x = Math.min(...pts.map((p) => p.x)) - GROUP_PAD;
    const y = Math.min(...pts.map((p) => p.y)) - GROUP_PAD_TOP;
    const width = Math.max(...pts.map((p) => p.x)) + NODE_WIDTH + GROUP_PAD - x;
    const height = Math.max(...pts.map((p) => p.y)) + NODE_HEIGHT + GROUP_PAD - y;
    grp.members.forEach((m) => parentOf.set(m, { id: grp.id, x, y }));
    return {
      id: grp.id,
      type: "stackGroup",
      position: { x, y },
      style: { width, height },
      data: { label: grp.label },
      selectable: false,
      zIndex: -1,
    };
  });

  const nodes: Node[] = view.nodes.map((node) => {
    const pos = abs.get(node.id)!;
    const parent = parentOf.get(node.id);
    const toggle = view.hosts[node.id];
    return {
      id: node.id,
      type: node.type,
      position: parent ? { x: pos.x - parent.x, y: pos.y - parent.y } : pos,
      ...(parent ? { parentId: parent.id, extent: "parent" as const } : {}),
      data: {
        label: node.label,
        status: node.status,
        ...node.data,
        ...(toggle ? { toggle, onToggle: () => onToggleHost(node.id) } : {}),
      },
    };
  });

  const edges: Edge[] = view.edges.map((edge, i) => ({
    id: `e-${i}`,
    source: edge.source,
    target: edge.target,
    label: edge.label,
    animated: !edge.derived,
    style: { stroke: "var(--border-strong)", ...(edge.derived ? { strokeDasharray: "4 4" } : {}) },
    labelStyle: { fill: "var(--text-muted)", fontSize: 10, fontFamily: "var(--font-mono)" },
  }));

  return { nodes: [...groupNodes, ...nodes], edges };
}

export default function TopologyGraph({ data, className }: { data: GraphData; className?: string }) {
  const { t } = useLocale();
  const [groupBy, setGroupBy] = useState<GroupBy>("stack");
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});
  const [expandAll, setExpandAll] = useState(false);

  const view = useMemo(
    () => buildTopologyView(data, { focus: data.focus, overrides, expandAll, groupBy }),
    [data, overrides, expandAll, groupBy],
  );
  const toggleHost = useCallback(
    (id: string) => setOverrides((o) => ({ ...o, [id]: !view.hosts[id]?.expanded })),
    [view.hosts],
  );
  const layout = useMemo(() => getLayoutedElements(view, toggleHost), [view, toggleHost]);
  // React Flow seeds its node state once: a new key re-mounts (and re-fits)
  // whenever focus, expansion or grouping changes what is on screen.
  const layoutKey = `${groupBy}|${layout.nodes.map((n) => n.id).join(",")}`;

  const groupOptions = [
    { value: "stack" as const, label: t("topology.groupStack") },
    { value: "environment" as const, label: t("topology.groupEnvironment") },
    { value: "coolifyProject" as const, label: t("topology.groupCoolifyProject") },
    { value: "none" as const, label: t("topology.groupNone") },
  ];
  const canExpandAll = !data.focus && Object.values(view.hosts).some((h) => !h.expanded);

  return (
    <div className={`relative ${className || "w-full h-[calc(100vh-14rem)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] overflow-hidden"}`} style={{ background: "var(--bg-base)" }}>
      <div className="absolute left-2 top-2 z-10 flex items-center gap-2">
        <ToolbarSelect name={t("topology.groupBy")} icon={ICON_PATHS.cube} value={groupBy} options={groupOptions} onChange={setGroupBy} />
        {!data.focus && (canExpandAll || expandAll) && (
          <PillButton active={expandAll} onClick={() => { setExpandAll((v) => !v); setOverrides({}); }}>
            {expandAll ? t("topology.collapseAll") : t("topology.expandAll")}
          </PillButton>
        )}
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
      const [type, id] = node.id.split("-");
      switch (type) {
        case "host":
          router.push(`/hosts/${node.data.slug || ""}`);
          break;
        case "service":
          router.push(`/services/${id}`);
          break;
        case "project":
          router.push(`/projects/${id}`);
          break;
        case "api":
          router.push(`/atlas/apis/${id}`);
          break;
      }
    },
    [router]
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
      fitViewOptions={{ maxZoom: 0.8, padding: 0.3 }}
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
      <MiniMap
        nodeColor={(n) => {
          switch (n.type) {
            case "host": return "var(--cyan)";
            case "service": return "var(--accent)";
            case "dns": return "var(--success)";
            case "project": return "var(--warning)";
            case "api": return "var(--rose)";
            case "stackGroup": return "transparent";
            default: return "var(--text-faint)";
          }
        }}
        className="hidden md:block"
        style={{
          background: "var(--bg-surface)",
          border: "1px solid var(--border-subtle)",
          borderRadius: "var(--radius-md)",
        }}
      />
    </ReactFlow>
  );
}
