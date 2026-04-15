"use client";

import { useMemo, useCallback } from "react";
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
import HostNode from "./nodes/HostNode";
import ServiceNode from "./nodes/ServiceNode";
import DnsNode from "./nodes/DnsNode";
import ProjectNode from "./nodes/ProjectNode";

const nodeTypes = {
  host: HostNode,
  service: ServiceNode,
  dns: DnsNode,
  project: ProjectNode,
};

const NODE_WIDTH = 200;
const NODE_HEIGHT = 80;

// Desired rank: project at top (0), service below, dns, host at bottom.
const RANK_ORDER: Record<string, number> = {
  project: 0,
  service: 1,
  dns: 2,
  host: 3,
};

function nodeRank(nodeId: string, nodeMap: Map<string, number>): number {
  return nodeMap.get(nodeId) ?? 2;
}

function getLayoutedElements(graphData: GraphData) {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "TB", nodesep: 60, ranksep: 100 });

  // Build a map of nodeId -> rank for quick lookup
  const rankMap = new Map<string, number>();
  graphData.nodes.forEach((node) => {
    rankMap.set(node.id, RANK_ORDER[node.type] ?? 2);
    g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  });

  // For layout: orient edges so they always flow from lower rank (top) to higher rank (bottom).
  // This tells dagre to place projects above services, services above DNS, etc.
  // The actual rendered edges will use the original source/target.
  graphData.edges.forEach((edge) => {
    const srcRank = nodeRank(edge.source, rankMap);
    const tgtRank = nodeRank(edge.target, rankMap);
    if (srcRank <= tgtRank) {
      g.setEdge(edge.source, edge.target);
    } else {
      // Reverse: source has higher rank number (lower in hierarchy), flip for layout
      g.setEdge(edge.target, edge.source);
    }
  });

  dagre.layout(g);

  const nodes: Node[] = graphData.nodes.map((node) => {
    const pos = g.node(node.id);
    return {
      id: node.id,
      type: node.type,
      position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 },
      data: { label: node.label, status: node.status, ...node.data },
    };
  });

  // Only render real edges from the graph data (not the invisible rank-ordering edges)
  const edges: Edge[] = graphData.edges.map((edge, i) => ({
    id: `e-${i}`,
    source: edge.source,
    target: edge.target,
    label: edge.label,
    animated: true,
    style: { stroke: "var(--border-strong)" },
    labelStyle: { fill: "var(--text-muted)", fontSize: 10, fontFamily: "var(--font-mono)" },
  }));

  return { nodes, edges };
}

export default function TopologyGraph({ data, className }: { data: GraphData; className?: string }) {
  const router = useRouter();

  const { nodes: layoutedNodes, edges: layoutedEdges } = useMemo(
    () => getLayoutedElements(data),
    [data]
  );

  const [nodes, , onNodesChange] = useNodesState(layoutedNodes);
  const [edges, , onEdgesChange] = useEdgesState(layoutedEdges);

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
      }
    },
    [router]
  );

  return (
    <div className={className || "w-full h-[calc(100vh-14rem)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] overflow-hidden"} style={{ background: "var(--bg-base)" }}>
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
              case "host": return "#06b6d4";
              case "service": return "#a78bfa";
              case "dns": return "#34d399";
              case "project": return "#fbbf24";
              default: return "#5a6a80";
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
    </div>
  );
}
